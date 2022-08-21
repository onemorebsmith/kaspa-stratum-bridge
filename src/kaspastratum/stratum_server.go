package kaspastratum

import (
	"fmt"
	"log"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/pkg/errors"
)

type BridgeConfig struct {
	StratumPort string `yaml:"stratum_port"`
	RPCServer   string `yaml:"kaspad_address"`
	MiningAddr  string `yaml:"miner_address"`
}

type StratumServer struct {
	newBlockChan chan struct{}
	cfg          BridgeConfig
	kaspad       *rpcclient.RPCClient
	clients      map[string]*MinerConnection
	clientLock   sync.RWMutex

	jobs map[string]*appmessage.RPCBlock
}

func (s *StratumServer) spawnClient(conn net.Conn) {
	remote := &MinerConnection{
		connection: conn,
	}
	s.clientLock.Lock()
	s.clients[conn.RemoteAddr().String()] = remote
	s.clientLock.Unlock()
	go remote.RunStratum(s)
}

func ListenAndServe(cfg BridgeConfig) (*StratumServer, error) {
	s := &StratumServer{
		cfg:        cfg,
		clientLock: sync.RWMutex{},
		clients:    make(map[string]*MinerConnection),
		jobs:       make(map[string]*appmessage.RPCBlock),
	}
	client, err := rpcclient.NewRPCClient(cfg.RPCServer)
	if err != nil {
		log.Printf("failed to connect to kaspa server: %s", err)
	}
	s.kaspad = client

	go func() {
		const tickerTime = 500 * time.Millisecond
		ticker := time.NewTicker(tickerTime)
		for {
			select {
			case <-s.newBlockChan:
				s.newBlockReady()
				ticker.Reset(tickerTime)
			case <-ticker.C: // timeout, manually check for new blocks
				s.newBlockReady()
			}
		}
	}()

	_ = client
	server, err := net.Listen("tcp", ":8080")
	if err != nil {
		return nil, errors.Wrap(err, "error listening")
	}
	defer server.Close()

	err = client.RegisterForNewBlockTemplateNotifications(func(_ *appmessage.NewBlockTemplateNotificationMessage) {
		select {
		case s.newBlockChan <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return s, errors.Wrap(err, "failed to register for block notifications")
	}

	for {
		connection, err := server.Accept()
		if err != nil {
			log.Printf("failed to accept incoming connection: %s", err)
			continue
		}
		s.spawnClient(connection)
	}
}

type BlockJob struct {
	Header    []byte
	Jobs      []uint64
	Timestamp int64
	JobId     int
}

func (s *StratumServer) SubmitResult(incoming *StratumEvent) *StratumResult {
	log.Println("[Server] Submitting block to kaspad")
	jobId, ok := incoming.Params[1].(string)
	if !ok {
		log.Printf("unexpected type for param 1: %+v", incoming.Params...)
		return nil
	}
	block, exists := s.jobs[jobId]
	if !exists {
		log.Printf("job does not exist: %+v", incoming.Params...)
		return nil
	}
	noncestr, ok := incoming.Params[2].(string)
	if !ok {
		log.Printf("unexpected type for param 2: %+v", incoming.Params...)
		return nil
	}
	noncestr = strings.Replace(noncestr, "0x", "", 1)
	nonce := big.Int{}
	nonce.SetString(noncestr, 16)
	log.Printf("Submitting nonce: %d", nonce.Uint64())
	converted, err := appmessage.RPCBlockToDomainBlock(block)
	if err != nil {
		log.Printf("failed to cast block to mutable block: %+v", err)
	}
	mutable := converted.Header.ToMutable()
	mutable.SetNonce(nonce.Uint64())
	msg, err := s.kaspad.SubmitBlock(&externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: converted.Transactions,
	})
	if err != nil {
		log.Printf("failed to submit block: %+v", err)
	}
	switch msg {
	case appmessage.RejectReasonNone:
		log.Println("[Server] block accepted!!")
		return &StratumResult{
			Result: true,
		}
		// :)
	case appmessage.RejectReasonBlockInvalid:
		log.Println("[Server] block reject, unknown issue (probably bad pow)")
		// :'(
		return &StratumResult{
			Result: []any{20, "Unknown problem", nil},
		}
	case appmessage.RejectReasonIsInIBD:
		log.Println("[Server] block reject, stale")
		// stale
		return &StratumResult{
			Result: []any{21, "Job not found", nil},
		}
	}
	log.Println(msg)
	return nil
}

var blockCounter = 0

func (s *StratumServer) disconnected(mc *MinerConnection) {
	s.clientLock.Lock()
	delete(s.clients, mc.connection.RemoteAddr().String())
	s.clientLock.Unlock()
}

func (s *StratumServer) newBlockReady() {
	template, err := s.kaspad.GetBlockTemplate(s.cfg.MiningAddr, `"kaspa-proxy.dev=["emthreeve"]`)
	if err != nil {
		log.Println(err)
		return
	}
	blockCounter++
	blockId := blockCounter % 128
	s.jobs[fmt.Sprintf("%d", blockId)] = template.Block

	newDiff := CalculateTarget(uint64(template.Block.Header.Bits))
	job := BlockJob{
		Timestamp: template.Block.Header.Timestamp,
		JobId:     blockId,
	}
	job.Header, err = SerializeBlockHeader(template.Block)
	if err != nil {
		log.Printf("failed to serialize block header: %s", err)
		return
	}
	job.Jobs = GenerateJobHeader(job.Header)

	s.clientLock.RLock()
	defer s.clientLock.RUnlock()
	for _, v := range s.clients {
		v.NewBlockTemplate(job, newDiff)
	}
}
