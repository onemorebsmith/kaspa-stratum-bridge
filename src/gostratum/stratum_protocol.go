package gostratum

import (
	"context"
	"net"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type StratumListener struct {
	logger  *zap.Logger
	clients sync.Map
	port    string

	OnNewClient func(*StratumClientProtocol) StratumClient
}

func NewListener(port string, logger *zap.Logger) *StratumListener {
	listener := &StratumListener{
		logger: logger.With(
			zap.String("component", "stratum"),
			zap.String("address", port),
		),
		port:    port,
		clients: sync.Map{},
	}

	return listener
}

func (s *StratumListener) Listen(ctx context.Context) error {
	lc := net.ListenConfig{}
	server, err := lc.Listen(ctx, "tcp", s.port)
	if err != nil {
		return errors.Wrap(err, "error listening")
	}
	killed := false
	defer server.Close()

	go func() {
		<-ctx.Done() // context cancelled, so kill the server
		killed = true
		server.Close()
	}()

	for { // listen and spin forever
		connection, err := server.Accept()
		if err != nil {
			if killed {
				return errors.Wrap(err, "listening cancelled")
			}
			s.logger.Error("failed to accept incoming connection", zap.Error(err))
			continue
		}
		s.newClient(connection)
	}
}

func (s *StratumListener) newClient(connection net.Conn) {
	addr := connection.RemoteAddr().String()
	s.logger.Info("new client connecting", zap.String("client", connection.RemoteAddr().String()))
	protocol := NewClientProtocol(connection, s.logger)
	var clientHandler StratumClient
	if s.OnNewClient != nil {
		clientHandler = s.OnNewClient(protocol)
	} else {
		clientHandler = NewDefaultClient(protocol)
	}
	s.clients.Store(addr, clientHandler)
}

func (s *StratumListener) Disconnected(client *StratumClientProtocol) {
	v, exists := s.clients.LoadAndDelete(client.remoteAddress)
	_ = v
	_ = exists
}
