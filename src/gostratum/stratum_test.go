package gostratum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mattn/go-colorable"
	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/onemorebsmith/kaspastratum/src/gostratum/testmocks"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func testLogger() *zap.SugaredLogger {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.DebugLevel,
	)).Sugar()
}

func TestAcceptContextLifetime(t *testing.T) {
	logger := testLogger()

	listener := NewListener(DefaultConfig(logger))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	defer cancel()
	listener.Listen(ctx)
}

func TestNewClient(t *testing.T) {
	logger := testLogger()
	listener := NewListener(DefaultConfig(logger))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	mc := testmocks.NewMockConnection()
	listener.newClient(ctx, mc)
	// send in the authorize event
	mc.AsyncWriteTestDataToReadBuffer(testmocks.NewAuthorizeEvent())

	responseReceived := false
	mc.ReadTestDataFromBuffer(func(b []byte) {
		expected := stratumrpc.JsonRpcResponse{
			Id:      "1",
			Version: "2.0",
			Error:   nil,
			Result:  true,
		}
		decoded := stratumrpc.JsonRpcResponse{}
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(&expected, &decoded); d != "" {
			t.Fatalf("response incorrect: %s", d)
		}
		// done
		responseReceived = true
	})

	if !responseReceived {
		t.Fatalf("failed to properly respond to authorize")
	}
}
