package gostratum

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type StratumContext struct {
	ctx        context.Context
	RemoteAddr string
	Logger     *zap.Logger
	connection net.Conn
	State      any // gross, but go generics aren't mature enough this can be typed 😭
}

func (sc *StratumContext) Reply(response stratumrpc.JsonRpcResponse) error {
	encoded, err := json.Marshal(response)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc response")
	}
	_, err = sc.connection.Write(encoded)
	return err
}

func (sc *StratumContext) Send(event stratumrpc.JsonRpcEvent) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc event")
	}
	_, err = sc.connection.Write(encoded)
	return err
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
