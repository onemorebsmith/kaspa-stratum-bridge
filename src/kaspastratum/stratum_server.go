package kaspastratum

import (
	"fmt"
	"log"
	"math/big"
	"net"
	"sort"
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
	PrintStats  bool   `yaml:"print_stats"`
}

type StratumServer struct {
	cfg         BridgeConfig
	kaspad      *rpcclient.RPCClient
	clients     map[string]*MinerConnection
	clientLock  sync.RWMutex
	blocksFound int64
	stales      int64
	rejections  int64
	disconnects int64
}

func (mc *StratumServer) log(msg string) {
	log.Printf("[bridge] %s", msg)
}

func (s *StratumServer) spawnClient(conn net.Conn) {
	remote := NewConnection(conn, s)
	s.clientLock.Lock()
	s.clients[remote.remoteAddress] = remote
	s.clientLock.Unlock()
	go remote.RunStratum(s)
}

func ListenAndServe(cfg BridgeConfig) error {
	s := &StratumServer{
		cfg:        cfg,
		clientLock: sync.RWMutex{},
		clients:    make(map[string]*MinerConnection),
	}
	client, err := rpcclient.NewRPCClient(cfg.RPCServer)
	if err != nil {
		return err
	}
	s.kaspad = client

	s.waitForSync()
	s.log("kaspa node is fully synced, starting bridge")
	go s.startBlockTemplateListener()

	// net listener below here
	server, err := net.Listen("tcp", cfg.StratumPort)
	if err != nil {
		return errors.Wrap(err, "error listening")
	}
	defer server.Close()

	if cfg.PrintStats {
		go s.startStatsThread()
	}

	for { // listen and spin forever
		connection, err := server.Accept()
		if err != nil {
			s.log(fmt.Sprintf("failed to accept incoming connection: %s", err))
			continue
		}
		s.spawnClient(connection)
	}
}

func (s *StratumServer) SubmitResult(block *appmessage.RPCBlock, nonce *big.Int) *StratumResult {
	s.log("submitting block to kaspad")
	s.log(fmt.Sprintf("Submitting nonce: %d", nonce.Uint64()))
	converted, err := appmessage.RPCBlockToDomainBlock(block)
	if err != nil {
		s.log(fmt.Sprintf("failed to cast block to mutable block: %+v", err))
	}
	mutable := converted.Header.ToMutable()
	mutable.SetNonce(nonce.Uint64())
	msg, err := s.kaspad.SubmitBlock(&externalapi.DomainBlock{
		Header:       mutable.ToImmutable(),
		Transactions: converted.Transactions,
	})
	if err != nil {
		s.log(fmt.Sprintf("failed to submit block: %+v", err))
	}
	switch msg {
	default:
		s.log("[Server] block accepted!!")
		s.blocksFound++
		return &StratumResult{
			Result: true,
		}
	case appmessage.RejectReasonNone:
		s.blocksFound++
		s.log("[Server] block accepted!!")
		return &StratumResult{
			Result: true,
		}
		// :)
	case appmessage.RejectReasonBlockInvalid:
		s.rejections++
		s.log("[Server] block reject, unknown issue (probably bad pow)")
		// :'(
		return &StratumResult{
			Result: []any{20, "Unknown problem", nil},
		}
	case appmessage.RejectReasonIsInIBD:
		s.stales++
		s.log("[Server] block reject, stale")
		// stale
		return &StratumResult{
			Result: []any{21, "Job not found", nil},
		}
	}
}

func (s *StratumServer) disconnected(mc *MinerConnection) {
	s.clientLock.Lock()
	if _, exists := s.clients[mc.remoteAddress]; exists {
		delete(s.clients, mc.remoteAddress)
		s.disconnects++
	}
	s.clientLock.Unlock()
}

func (s *StratumServer) startBlockTemplateListener() {
	blockReadyChan := make(chan bool)
	err := s.kaspad.RegisterForNewBlockTemplateNotifications(func(_ *appmessage.NewBlockTemplateNotificationMessage) {
		blockReadyChan <- true
	})
	if err != nil {
		s.log("fatal: failed to register for block notifications from kaspa")
	}

	blockReady := func() {
		s.clientLock.Lock()
		defer s.clientLock.Unlock()
		for _, v := range s.clients {
			if v != nil { // this shouldn't happen but apparently it did
				go v.NewBlockAvailable()
			}
		}
	}

	const tickerTime = 500 * time.Millisecond
	ticker := time.NewTicker(tickerTime)
	for {
		select {
		case <-blockReadyChan:
			blockReady()
			ticker.Reset(tickerTime)
		case <-ticker.C: // timeout, manually check for new blocks
			blockReady()
		}
	}
}

func (s *StratumServer) waitForSync() error {
	for {
		clientInfo, err := s.kaspad.GetInfo()
		if err != nil {
			return errors.Wrapf(err, "error fetching server info from kaspad @ %s", s.cfg.RPCServer)
		}
		if clientInfo.IsSynced {
			break
		}
		s.log("Kaspa is not synced, waiting for sync before starting bridge")
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (s *StratumServer) startStatsThread() error {
	start := time.Now()
	for {
		time.Sleep(10 * time.Second)
		s.clientLock.RLock()
		str := "\n========================================================\n"
		str += fmt.Sprintf("uptime %s | mined %d | stales %d | reject %d | disconn: %d\n",
			time.Since(start).Round(time.Second), s.blocksFound, s.stales, s.rejections, s.disconnects)
		str += "--------------------------------------------------------\n"
		str += "worker\t| avg hashrate\t| shares\t| uptime\n"
		str += "--------------------------------------------------------\n"
		var lines []string
		totalRate := float64(0)
		for _, v := range s.clients {
			rate := v.GetAverageHashrateGHz()
			totalRate += rate
			lines = append(lines, fmt.Sprintf("%s\t| %0.2fGH/s\t| %d\t| %s",
				v.tag, v.GetAverageHashrateGHz(), v.sharesFound, time.Since(v.startTime).Round(time.Second)))
		}
		sort.Strings(lines)
		str += strings.Join(lines, "\n")
		str += "\n--------------------------------------------------------\n"
		str += fmt.Sprintf("total\t| %0.2fGH/s", totalRate)
		str += "\n========================================================\n"
		s.clientLock.RUnlock()
		log.Println(str)
	}
}
