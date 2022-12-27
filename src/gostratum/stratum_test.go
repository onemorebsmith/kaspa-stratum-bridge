package gostratum

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mattn/go-colorable"
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
	mc := NewMockConnection()
	listener.newClient(ctx, mc)
	// send in the authorize event
	event, _ := json.Marshal(NewEvent("1", "mining.authorize", []any{
		"", "test",
	}))
	mc.AsyncWriteTestDataToReadBuffer(string(event))

	responseReceived := false
	mc.ReadTestDataFromBuffer(func(b []byte) {
		expected := JsonRpcResponse{
			Id:     "1",
			Error:  nil,
			Result: true,
		}
		decoded := JsonRpcResponse{}
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

func TestWalletValidation(t *testing.T) {
	tests := []struct {
		in        string
		expected  string
		shouldErr bool
	}{
		{
			in:       "kaspa:qqayxgcjfh6d7uxpj4w3qzjvx73vdehfx22fl6cacmn44rpj5geg2rxyuhga4,Rig_3784816",
			expected: "kaspa:qqayxgcjfh6d7uxpj4w3qzjvx73vdehfx22fl6cacmn44rpj5geg2rxyuhga4",
		},
		{
			in:       "kaspa:qqkrl0er5ka5snd55gr9rcf6rlpx8nln8gf3jxf83w4dc0khfqmauy6qs83zm,Rig_3784816",
			expected: "kaspa:qqkrl0er5ka5snd55gr9rcf6rlpx8nln8gf3jxf83w4dc0khfqmauy6qs83zm",
		},
		{
			in:       "qqkrl0er5ka5snd55gr9rcf6rlpx8nln8gf3jxf83w4dc0khfqmauy6qs83zm,Rig_3784816",
			expected: "kaspa:qqkrl0er5ka5snd55gr9rcf6rlpx8nln8gf3jxf83w4dc0khfqmauy6qs83zm",
		},
	}

	for _, v := range tests {
		cleaned, err := CleanWallet(v.in)
		if err != nil && !v.shouldErr {
			t.Fatalf("Unexpected error for wallet %+v", v)
		}
		if cleaned != v.expected {
			t.Fatalf("expected %s, got %s", v.expected, cleaned)
		}
	}
}
