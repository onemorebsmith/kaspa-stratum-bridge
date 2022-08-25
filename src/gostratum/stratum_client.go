package gostratum

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"go.uber.org/zap"
)

type StratumClient interface {
	OnAuthorize(params []any)
}

type StratumClientProtocol struct {
	connection    net.Conn
	remoteAddress string
	logger        *zap.Logger
	client        StratumClient
}

func NewClientProtocol(conn net.Conn, logger *zap.Logger) *StratumClientProtocol {
	addr := conn.RemoteAddr().String()
	return &StratumClientProtocol{
		connection:    conn,
		logger:        logger.With(zap.String("client", conn.RemoteAddr().String())),
		remoteAddress: addr,
	}
}

func (cc *StratumClientProtocol) StartListen(ctx context.Context, client StratumClient) error {
	cc.client = client
	if client == nil {
		return fmt.Errorf("client can not be nil")
	}

	for {
		err := readFromConnection(cc.connection, func(line string) error {
			event, err := stratumrpc.UnmarshalEvent(line)
			if err != nil {
				return err
			}

			switch event.Method {
			case stratumrpc.StratumMethodAuthorize:
				client.OnAuthorize(event.Params)
			}

			return nil
		})
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue // expected timeout
		}
		if ctx.Err() != nil {
			return ctx.Err() // context cancelled
		}
		if err != nil { // actual error
			cc.logger.Error("error reading from socket", zap.Error(err))
			return err
		}
	}
}
