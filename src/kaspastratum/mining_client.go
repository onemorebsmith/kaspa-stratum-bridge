package kaspastratum

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"syscall"
	"time"

	"io"
	"log"
	"net"
	"strings"
	"sync/atomic"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/domain/consensus/utils/pow"
	"github.com/pkg/errors"
)

type MinerConnection struct {
	connection   net.Conn
	counter      int32
	server       *StratumServer
	tag          string
	minerAddress string
	bigDiff      big.Int

	jobLock     sync.Mutex
	jobs        map[string]*appmessage.RPCBlock
	sharesFound int64
	startTime   time.Time
}

type BlockJob struct {
	Header    []byte
	Jobs      []uint64
	Timestamp int64
	JobId     int
}

func (mc *MinerConnection) log(msg string) {
	log.Printf("[%s] %s", mc.tag, msg)
}

func (mc *MinerConnection) listen() ([]*StratumEvent, error) {
	buffer := make([]byte, 1024)
	_, err := mc.connection.Read(buffer)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading from connection %s", mc.connection.RemoteAddr().String())
	}
	asStr := string(buffer)
	asStr = strings.ReplaceAll(asStr, "\x00", "")
	var events []*StratumEvent
	for _, v := range strings.Split(asStr, "\n") {
		if len(v) == 0 {
			continue
		}
		event := &StratumEvent{}
		if err := json.Unmarshal([]byte(v), event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func NewConnection(connection net.Conn, server *StratumServer) *MinerConnection {
	return &MinerConnection{
		connection: connection,
		server:     server,
		tag:        connection.RemoteAddr().String(),
		jobLock:    sync.Mutex{},
		jobs:       make(map[string]*appmessage.RPCBlock),
		startTime:  time.Now(),
	}
}

func (mc *MinerConnection) RunStratum(s *StratumServer) {
	for {
		events, err := mc.listen()
		if err != nil {
			if checkDisconnect(err) {
				mc.log("disconnected")
				mc.server.disconnected(mc)
				return
			}
			mc.log(fmt.Sprintf("error processing connection: %s", err))
			return
		}
		for _, e := range events {
			if err := mc.processEvent(e); err != nil {
				mc.log(err.Error())
				return
			}
		}
	}
}

func (mc *MinerConnection) processEvent(event *StratumEvent) error {
	switch event.Method {
	case "mining.subscribe":
		err := mc.SendResult(StratumResult{
			Version: "2.0",
			Id:      event.Id,
			Result:  []any{true, "EthereumStratum/1.0.0"},
		})
		if err != nil {
			return err
		}
	case "mining.authorize":
		return mc.HandleAuthorize(event)
	case "mining.submit":
		return mc.HandleSubmit(event)
	}
	return nil
}

func (mc *MinerConnection) HandleSubmit(event *StratumEvent) error {
	if len(event.Params) < 2 {
		log.Printf("malformed event, expected at least 2 params")
		return nil
	}
	jobId, ok := event.Params[1].(string)
	if !ok {
		log.Printf("unexpected type for param 1: %+v", event.Params...)
		return nil
	}
	mc.jobLock.Lock()
	block, exists := mc.jobs[jobId]
	mc.jobLock.Unlock()
	if !exists {
		mc.log(fmt.Sprintf("job does not exist: %+v", event.Params...))
		return nil
	}
	noncestr, ok := event.Params[2].(string)
	if !ok {
		mc.log(fmt.Sprintf("unexpected type for param 2: %+v", event.Params...))
		return nil
	}
	noncestr = strings.Replace(noncestr, "0x", "", 1)
	nonce := big.Int{}
	nonce.SetString(noncestr, 16)

	converted, err := appmessage.RPCBlockToDomainBlock(block)
	if err != nil {
		mc.log(fmt.Sprintf("failed to cast block to mutable block: %+v", err))
	}
	mutableHeader := converted.Header.ToMutable()
	mutableHeader.SetNonce(nonce.Uint64())
	powState := pow.NewState(mutableHeader)
	powValue := powState.CalculateProofOfWorkValue()

	mc.sharesFound++

	// The block hash must be less or equal than the claimed target.
	if powValue.Cmp(&powState.Target) <= 0 {
		mc.log("found block")
		res := mc.server.SubmitResult(block, &nonce)
		return mc.SendResult(*res)
	}

	return mc.SendResult(StratumResult{
		Result: true,
	})
}

func (mc *MinerConnection) GetShares() int64 {
	return mc.sharesFound
}

func (mc *MinerConnection) GetAverageHashrateGHz() float64 {
	return (float64(mc.sharesFound) * shareValue) / time.Since(mc.startTime).Seconds()
}

func (mc *MinerConnection) HandleAuthorize(event *StratumEvent) error {
	if len(event.Params) < 1 {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address")
	}
	address, ok := event.Params[0].(string)
	if !ok {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address string")
	}

	split := strings.Split(address, ".")
	if len(split) > 1 {
		mc.log(fmt.Sprintf("mapped %s to worker %s, replacing tag", mc.tag, split[1]))
		mc.tag = split[1]
		address = split[0]
	}
	mc.minerAddress = address
	mc.log(fmt.Sprintf("authorizing -> %s", address))
	nonce := rand.Uint32() // two bytes
	// extra noonce
	if err := mc.SendEvent(StratumEvent{
		Version: "2.0",
		Method:  "mining.set_extranonce",
		Params:  []any{nonce, 4},
	}); err != nil {
		return err
	}
	// send a default diff, we'll calculate the actual diff later when
	// a new block template is ready
	if err := mc.SendEvent(StratumEvent{
		Version: "2.0",
		Method:  "mining.set_difficulty",
		Params:  []any{fixedDifficulty},
	}); err != nil {
		return err
	}
	if err := mc.SendResult(StratumResult{
		Version: "2.0",
		Id:      event.Id,
		Result:  true,
	}); err != nil {
		return err
	}
	return nil
}

func (mc *MinerConnection) SendEvent(res StratumEvent) error {
	res.Version = "2.0"
	if res.Id == 0 {
		res.Id = int(atomic.AddInt32(&mc.counter, 1))
	}
	encoded, err := json.Marshal(res)
	if err != nil {
		return errors.Wrap(err, "failed encoding stratum result to client")
	}
	encoded = append(encoded, '\n')
	_, err = mc.connection.Write(encoded)
	if checkDisconnect(err) {
		mc.log("disconnected")
		mc.server.disconnected(mc)
	}
	return err
}

func (mc *MinerConnection) SendResult(res StratumResult) error {
	res.Version = "2.0"
	encoded, err := json.Marshal(res)
	if err != nil {
		return errors.Wrap(err, "failed encoding stratum result to client")
	}
	encoded = append(encoded, '\n')
	_, err = mc.connection.Write(encoded)
	if checkDisconnect(err) {
		mc.log("disconnected")
		mc.server.disconnected(mc)
	}
	return err
}

func (mc *MinerConnection) NewBlockAvailable() {
	template, err := mc.server.kaspad.GetBlockTemplate(mc.minerAddress, `"kaspa-stratum-bridge=["onemorebsmith"]`)
	if err != nil {
		mc.log(fmt.Sprintf("failed fetching new block template from kaspa: %s", err))
		return
	}
	blockCounter++
	blockId := blockCounter % 128
	mc.jobLock.Lock()
	mc.jobs[fmt.Sprintf("%d", blockId)] = template.Block
	mc.jobLock.Unlock()

	mc.bigDiff = CalculateTarget(uint64(template.Block.Header.Bits))
	job := BlockJob{
		Timestamp: template.Block.Header.Timestamp,
		JobId:     blockId,
	}
	job.Header, err = SerializeBlockHeader(template.Block)
	if err != nil {
		mc.log(fmt.Sprintf("failed to serialize block header: %s", err))
		return
	}
	job.Jobs = GenerateJobHeader(job.Header)

	// normal notify flow
	if err := mc.SendEvent(StratumEvent{
		Version: "2.0",
		Method:  "mining.notify",
		Id:      job.JobId,
		Params:  []any{fmt.Sprintf("%d", job.JobId), job.Jobs, job.Timestamp},
	}); err != nil {
		mc.log(err.Error())
	}
}

func checkDisconnect(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	return false
}
