package gostratum

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type StratumContext struct {
	ctx           context.Context
	RemoteAddr    string
	WalletAddr    string
	WorkerName    string
	RemoteApp     string
	Id            int32
	Logger        *zap.SugaredLogger
	connection    net.Conn
	disconnecting bool
	onDisconnect  chan *StratumContext
	State         any // gross, but go generics aren't mature enough this can be typed 😭
	writeLock     int32
}

var ErrorDisconnected = fmt.Errorf("disconnecting")

func (sc *StratumContext) Connected() bool {
	return !sc.disconnecting
}

func (sc *StratumContext) String() string {
	serialized, _ := json.Marshal(sc)
	return string(serialized)
}

func (sc *StratumContext) Reply(response JsonRpcResponse) error {
	if sc.disconnecting {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc response")
	}
	encoded = append(encoded, '\n')
	return sc.writeWithBackoff(encoded)
}

func (sc *StratumContext) Send(event JsonRpcEvent) error {
	if sc.disconnecting {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc event")
	}
	encoded = append(encoded, '\n')
	return sc.writeWithBackoff(encoded)
}

var errWriteBlocked = fmt.Errorf("error writing to socket, previous write pending")

func (sc *StratumContext) write(data []byte) error {
	if atomic.CompareAndSwapInt32(&sc.writeLock, 0, 1) {
		defer atomic.StoreInt32(&sc.writeLock, 0)
		deadline := time.Now().Add(5 * time.Second)
		if err := sc.connection.SetWriteDeadline(deadline); err != nil {
			return errors.Wrap(err, "failed setting write deadline for connection")
		}
		_, err := sc.connection.Write(data)
		sc.checkDisconnect(err)
		return err
	}
	return errWriteBlocked
}

func (sc *StratumContext) writeWithBackoff(data []byte) error {
	for i := 0; i < 3; i++ {
		err := sc.write(data)
		if err == nil {
			return nil
		} else if err == errWriteBlocked {
			time.Sleep(5 * time.Millisecond)
			continue
		} else {
			return err
		}
	}
	// this should virtually never happen on a 'healthy' connection. Writes
	// to the socket are actually just writing to the outgoing buffer for the
	// connection in the OS, if this blocks it's because the receiver has not
	// read from the buffer for such a length of time that the tx buffer is full
	return fmt.Errorf("failed writing to socket after 3 attempts")
}

func (sc *StratumContext) ReplyStaleShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{21, "Job not found", nil},
	})
}
func (sc *StratumContext) ReplyDupeShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{22, "Duplicate share submitted", nil},
	})
}

func (sc *StratumContext) ReplyBadShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{20, "Unknown problem", nil},
	})
}

func (sc *StratumContext) ReplyLowDiffShare(id any) error {
	return sc.Reply(JsonRpcResponse{
		Id:     id,
		Result: nil,
		Error:  []any{23, "Invalid difficulty", nil},
	})
}

func (sc *StratumContext) Disconnect() {
	if !sc.disconnecting {
		sc.Logger.Info("disconnecting")
		sc.disconnecting = true
		if sc.connection != nil {
			sc.connection.Close()
		}
		sc.onDisconnect <- sc
	}
}

func (sc *StratumContext) checkDisconnect(err error) {
	if err != nil { // actual error
		go sc.Disconnect() // potentially blocking, so async it
	}
}

// Context interface impl

func (StratumContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (StratumContext) Done() <-chan struct{} {
	return nil
}

func (StratumContext) Err() error {
	return nil
}

func (d StratumContext) Value(key any) any {
	return d.ctx.Value(key)
}
