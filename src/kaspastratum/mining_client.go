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
	connection   net.Conn
	counter      int32
	Disconnected bool
	server       *StratumServer
	diff         float64
}

func (mc *MinerConnection) listen() (*StratumEvent, error) {
	buffer := make([]byte, 1024*10)
	_, err := mc.connection.Read(buffer)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading from connection %s", mc.connection.RemoteAddr().String())
	}
	asStr := string(buffer)
	asStr = strings.TrimRight(asStr, "\x00")
	log.Printf("raw: %s", string(buffer))
	event := &StratumEvent{}
	return event, json.Unmarshal([]byte(asStr), event)
}

func (mc *MinerConnection) RunStratum(s *StratumServer) {
	mc.server = s
	for {
		event, err := mc.listen()
		if err != nil {
			if err == io.EOF {
				mc.Disconnected = true
				log.Printf("client closed connection: %s", err)
				return
			}
			log.Printf("error processing connection: %s", err)
			return
		}
		log.Printf("event received: %+v", event)
		if err := mc.processEvent(event); err != nil {
			log.Println(err)
			return
		}
	}
}

func (mc *MinerConnection) processEvent(event *StratumEvent) error {
	switch event.Method {
	case "mining.subscribe":
		log.Printf("subscribed %s", mc.connection.RemoteAddr())
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
		// ref: '{"id":1,"jsonrpc":"2.0","result":[true,"EthereumStratum/1.0.0"]}
		// me:   {"id":500,"jsonrpc":"2.0","method":"","results":[true,"EthereumStratum/1.0.0"]}`
		log.Printf("authorized %s", mc.connection.RemoteAddr())
		mc.SendResult(StratumResult{
			Version: "2.0",
			Id:      event.Id,
			Result:  true,
		})
		mc.SendEvent(StratumEvent{
			Version: "2.0",
			Method:  "mining.set_difficulty",
			Params:  []any{5.0},
		})
	case "mining.submit":
		res := mc.server.SubmitResult(event)
		mc.SendResult(*res)
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
	return false
}

func (mc *MinerConnection) SendEvent(res StratumEvent) error {
	res.Version = "2.0"
	// mine: {"id":500,"jsonrpc":"2.0","results":true}
	// ref:

	if res.Id == 0 {
		res.Id = int(atomic.AddInt32(&mc.counter, 1))
	}
	encoded, err := json.Marshal(res)
	if err != nil {
		return errors.Wrap(err, "failed encoding stratum result to client")
	}
	encoded = append(encoded, '\n')
	//log.Printf("[proxy] sending event: `%s`", string(encoded))
	_, err = mc.connection.Write(encoded)
	if checkDisconnect(err) {
		mc.Disconnected = true
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
	//log.Printf("[proxy] response: `%s`", string(encoded))
	_, err = mc.connection.Write(encoded)
	if checkDisconnect(err) {
		mc.Disconnected = true
	}
	return err
}

func (mc *MinerConnection) NewBlockTemplate(job BlockJob, diff float64) {
	if mc.diff != diff {
		mc.diff = diff
		if err := mc.SendEvent(StratumEvent{
			Version: "2.0",
			Method:  "mining.set_difficulty",
			Id:      job.JobId,
			Params:  []any{diff},
		}); err != nil {
			log.Println(err)
		}
	}

	if err := mc.SendEvent(StratumEvent{
		Version: "2.0",
		Method:  "mining.notify",
		Id:      job.JobId,
		Params:  []any{fmt.Sprintf("%d", job.JobId), job.Jobs, job.Timestamp},
	}); err != nil {
		log.Println(err)
	}
	// {"id":17,"jsonrpc":"2.0","method":"mining.notify","params":[17,[3141038299,1241394483,834470638,3828983134],1661058113822]}
	// {"jsonrpc":"2.0","method":"mining.notify","params":["00057887",[70619300376954216,10456097544244484708,1847936161638071063,10706809211909976528],16610493313604],"id":null}

}
