package gostratum

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type StratumContext struct {
	ctx           context.Context
	RemoteAddr    string
	WalletAddr    string
	WorkerName    string
	RemoteApp     string
	Logger        *zap.SugaredLogger
	connection    net.Conn
	disconnecting bool
	onDisconnect  chan *StratumContext
	State         any // gross, but go generics aren't mature enough this can be typed 😭
}

var ErrorDisconnected = fmt.Errorf("disconnecting")

func (sc *StratumContext) Reply(response stratumrpc.JsonRpcResponse) error {
	if sc.disconnecting {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc response")
	}
	encoded = append(encoded, '\n')
	_, err = sc.connection.Write(encoded)
	sc.checkDisconnect(err)
	return err
}

func (sc *StratumContext) Send(event stratumrpc.JsonRpcEvent) error {
	if sc.disconnecting {
		return ErrorDisconnected
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return errors.Wrap(err, "failed encoding jsonrpc event")
	}
	encoded = append(encoded, '\n')
	_, err = sc.connection.Write(encoded)
	sc.checkDisconnect(err)
	return err
}

func (sc *StratumContext) checkDisconnect(err error) {
	if err != nil { // actual error
		sc.disconnecting = true
		sc.onDisconnect <- sc
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
