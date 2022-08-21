package kaspastratum

import (
	"encoding/json"
	"fmt"
	"syscall"

	"io"
	"log"
	"net"
	"strings"
	"sync/atomic"

	"github.com/pkg/errors"
)

type MinerConnection struct {
	connection net.Conn
	counter    int32
	server     *StratumServer
	diff       float64
	tag        string
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

func (mc *MinerConnection) RunStratum(s *StratumServer) {
	mc.tag = mc.connection.RemoteAddr().String()
	mc.server = s
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
			mc.log(fmt.Sprintf("[stratum] received %s", e.Method))
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
		mc.log("subscribed")
		// me : `{"id":1,"jsonrpc":"2.0","results":[true,"EthereumStratum/1.0.0"]}`
		err := mc.SendResult(StratumResult{
			Version: "2.0",
			Id:      event.Id,
			Result:  []any{true, "EthereumStratum/1.0.0"},
		})
		if err != nil {
			return err
		}
	case "mining.authorize":
		mc.log("authorized")
		if err := mc.SendResult(StratumResult{
			Version: "2.0",
			Id:      event.Id,
			Result:  true,
		}); err != nil {
			return err
		}
		// send a default diff, we'll calculate the actual diff later when
		// a new block template is ready
		if err := mc.SendEvent(StratumEvent{
			Version: "2.0",
			Method:  "mining.set_difficulty",
			Params:  []any{5.0},
		}); err != nil {
			return err
		}
	case "mining.submit":
		mc.log("found block")
		res := mc.server.SubmitResult(event)
		if err := mc.SendResult(*res); err != nil {
			return err
		}
	}
	return nil
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

func (mc *MinerConnection) NewBlockTemplate(job BlockJob, diff float64) {
	if mc.diff != diff {
		// new difficulty level, update the client
		mc.diff = diff
		if err := mc.SendEvent(StratumEvent{
			Version: "2.0",
			Method:  "mining.set_difficulty",
			Id:      job.JobId,
			Params:  []any{diff},
		}); err != nil {
			mc.log(err.Error())
		}
	}

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
