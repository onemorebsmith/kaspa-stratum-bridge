package gostratum

import (
	"fmt"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/onemorebsmith/kaspastratum/src/gostratum/stratumrpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func DefaultLogger() *zap.SugaredLogger {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(colorable.NewColorableStdout()),
		zapcore.DebugLevel,
	)).Sugar()
}

func DefaultConfig(logger *zap.SugaredLogger) StratumListenerConfig {
	return StratumListenerConfig{
		StateGenerator: func() any { return nil },
		HandlerMap:     DefaultHandlers(),
		Port:           ":5555",
		Logger:         logger,
	}
}

func DefaultHandlers() StratumHandlerMap {
	return StratumHandlerMap{
		string(stratumrpc.StratumMethodSubscribe): HandleSubscribe,
		string(stratumrpc.StratumMethodAuthorize): HandleAuthorize,
		string(stratumrpc.StratumMethodSubmit):    HandleSubmit,
	}
}

func HandleAuthorize(ctx *StratumContext, event stratumrpc.JsonRpcEvent) error {
	if len(event.Params) < 1 {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address")
	}
	address, ok := event.Params[0].(string)
	if !ok {
		return fmt.Errorf("malformed event from miner, expected param[1] to be address string")
	}
	parts := strings.Split(address, ".")
	var workerName string
	if len(parts) >= 2 {
		address = parts[0]
		workerName = parts[1]
	}
	ctx.WalletAddr = address
	ctx.WorkerName = workerName

	if err := ctx.Reply(stratumrpc.NewResponse(event, true, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to authorize")
	}
	ctx.Logger.Info("client authorized, address: ", ctx.WalletAddr)
	return nil
}

func HandleSubscribe(ctx *StratumContext, event stratumrpc.JsonRpcEvent) error {
	if err := ctx.Reply(stratumrpc.NewResponse(event,
		[]any{true, "EthereumStratum/1.0.0"}, nil)); err != nil {
		return errors.Wrap(err, "failed to send response to subscribe")
	}
	if len(event.Params) > 0 {
		app, ok := event.Params[0].(string)
		if ok {
			ctx.RemoteApp = app
		}
	}

	ctx.Logger.Info("client subscribed ", zap.Any("context", ctx))
	return nil
}

func HandleSubmit(ctx *StratumContext, event stratumrpc.JsonRpcEvent) error {
	// stub
	ctx.Logger.Info("work submission")
	return nil
}
