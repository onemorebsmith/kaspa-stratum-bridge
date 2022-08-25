package gostratum

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type DisconnectChannel chan StratumContext
type EventHandler func(ctx StratumContext, event stratumrpc.JsonRpcEvent) error
type StratumHandlerMap map[string]EventHandler

type StratumStats struct {
	Disconnects int64
}

type StratumListener struct {
	logger       *zap.Logger
	clients      sync.Map
	port         string
	shuttingDown bool

	SpawnClientHandler func(StratumContext) StratumClient
	disconnectChannel  DisconnectChannel

	stats       StratumStats
	workerGroup sync.WaitGroup

	handlers StratumHandlerMap
}

func NewListener(port string, logger *zap.Logger, handlers StratumHandlerMap) *StratumListener {
	listener := &StratumListener{
		logger: logger.With(
			zap.String("component", "stratum"),
			zap.String("address", port),
		),
		port:        port,
		clients:     sync.Map{},
		workerGroup: sync.WaitGroup{},
		handlers:    handlers,
	}

	return listener
}

func (s *StratumListener) Listen(ctx context.Context) error {
	s.shuttingDown = false

	serverContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	lc := net.ListenConfig{}
	server, err := lc.Listen(ctx, "tcp", s.port)
	if err != nil {
		return errors.Wrapf(err, "failed listening to socket %s", s.port)
	}
	defer server.Close()

	go s.disconnectListener(serverContext)
	go s.tcpListener(serverContext, server)

	// block here until the context is killed
	<-ctx.Done() // context cancelled, so kill the server
	s.shuttingDown = true
	server.Close()
	s.workerGroup.Wait()
	return context.Canceled
}

func (s *StratumListener) newClient(ctx context.Context, connection net.Conn) {
	addr := connection.RemoteAddr().String()
	clientContext := StratumContext{
		ctx:        ctx,
		RemoteAddr: addr,
		Logger:     s.logger.With(zap.String("client", addr)),
		connection: connection,
	}

	s.logger.Info("new client connecting", zap.String("client", addr))
	s.clients.Store(addr, &clientContext)
	go spawnClientListener(clientContext, connection, s)
}

func (s *StratumListener) HandleEvent(ctx StratumContext, event stratumrpc.JsonRpcEvent) error {
	if handler, exists := s.handlers[string(event.Method)]; exists {
		return handler(ctx, event)
	}
	s.logger.Warn(fmt.Sprintf("unhandled event '%+v'", event))
	return nil
}

func (s *StratumListener) disconnectListener(ctx context.Context) {
	s.workerGroup.Add(1)
	defer s.workerGroup.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-s.disconnectChannel:
			_, exists := s.clients.LoadAndDelete(client)
			if exists {
				s.logger.Info("client disconnecting", zap.Any("client", client))
				s.stats.Disconnects++
			}
		}
	}
}

func (s *StratumListener) tcpListener(ctx context.Context, server net.Listener) {
	s.workerGroup.Add(1)
	defer s.workerGroup.Done()
	for { // listen and spin forever
		connection, err := server.Accept()
		if err != nil {
			if s.shuttingDown {
				s.logger.Error("stopping listening due to server shutdown")
				return
			}
			s.logger.Error("failed to accept incoming connection", zap.Error(err))
			continue
		}
		s.newClient(ctx, connection)
	}
}
