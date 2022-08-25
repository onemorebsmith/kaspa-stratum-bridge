package gostratum

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/onemorebsmith/kaspastratum/src/gostratum/testmocks"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func testLogger() *zap.Logger {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.DebugLevel,
	))
}

func TestAcceptContextLifetime(t *testing.T) {
	logger := testLogger()

	listener := NewListener(":12345", logger)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	defer cancel()
	listener.Listen(ctx)
}

func TestNewClient(t *testing.T) {
	logger := testLogger()
	listener := NewListener(":12345", logger)

	called := false
	var client *DefaultClient
	listener.OnNewClient = func(scp *StratumClientProtocol) StratumClient {
		called = true
		client = NewDefaultClient(scp)
		return client
	}

	mc := testmocks.NewMockConnection()
	listener.newClient(mc)
	if !called {
		t.Fatalf("callback not called properly")
	}
	// send in the authorize event
	mc.AsyncWriteTestDataToReadBuffer(testmocks.NewAuthorizeEvent())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := client.proto.StartListen(ctx, client); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error during listen: %s", err)
	}
	if client.state != StratumStateAuthorized {
		t.Fatalf("client in unexpected state, expected %d, got %d", StratumStateAuthorized, client.state)
	}
}
