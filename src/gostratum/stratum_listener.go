package gostratum

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type DisconnectChannel chan *StratumContext
type StateGenerator func() any
type EventHandler func(ctx *StratumContext, event JsonRpcEvent) error

type StratumClientListener interface {
	OnConnect(ctx *StratumContext)
	OnDisconnect(ctx *StratumContext)
}

type StratumHandlerMap map[string]EventHandler

type StratumStats struct {
	Disconnects int64
}

type StratumListenerConfig struct {
	Logger         *zap.Logger
	HandlerMap     StratumHandlerMap
	ClientListener StratumClientListener
	StateGenerator StateGenerator
	Port           string
}

type StratumListener struct {
	StratumListenerConfig
	shuttingDown      bool
	disconnectChannel DisconnectChannel
	stats             StratumStats
	workerGroup       sync.WaitGroup
}

func NewListener(cfg StratumListenerConfig) *StratumListener {
	listener := &StratumListener{
		StratumListenerConfig: cfg,
		workerGroup:           sync.WaitGroup{},
		disconnectChannel:     make(DisconnectChannel),
	}

	listener.Logger = listener.Logger.With(
		zap.String("component", "stratum"),
		zap.String("address", listener.Port),
	)

	if listener.StateGenerator == nil {
		listener.Logger.Warn("no state generator provided, using default")
		listener.StateGenerator = func() any { return nil }
	}

	return listener
}

func (s *StratumListener) Listen(ctx context.Context) error {
	s.shuttingDown = false

	serverContext, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := net.ListenConfig{}
	server, err := lc.Listen(ctx, "tcp", s.Port)
	if err != nil {
		return errors.Wrapf(err, "failed listening to socket %s", s.Port)
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
	parts := strings.Split(addr, ":")
	if len(parts) > 0 {
		addr = parts[0] // trim off the port
	}
	clientContext := &StratumContext{
		parentContext: ctx,
		RemoteAddr:    addr,
		Logger:        s.Logger.With(zap.String("client", addr)),
		connection:    connection,
		State:         s.StateGenerator(),
		onDisconnect:  s.disconnectChannel,
	}

	s.Logger.Info(fmt.Sprintf("new client connecting - %s", addr))

	if s.ClientListener != nil { // TODO: should this be before we spawn the handler?
		s.ClientListener.OnConnect(clientContext)
	}

	go spawnClientListener(clientContext, connection, s)

}

func (s *StratumListener) HandleEvent(ctx *StratumContext, event JsonRpcEvent) error {
	if handler, exists := s.HandlerMap[string(event.Method)]; exists {
		return handler(ctx, event)
	}
	//s.Logger.Warn(fmt.Sprintf("unhandled event '%+v'", event))
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
			s.Logger.Info(fmt.Sprintf("client disconnecting - %s", client.RemoteAddr))
			s.stats.Disconnects++
			if s.ClientListener != nil {
				s.ClientListener.OnDisconnect(client)
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
				s.Logger.Error("stopping listening due to server shutdown")
				return
			}
			s.Logger.Error("failed to accept incoming connection", zap.Error(err))
			continue
		}
		s.newClient(ctx, connection)
	}
}
