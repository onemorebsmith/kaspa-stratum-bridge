package kaspastratum

import (
	"context"
	"os"

	"github.com/mattn/go-colorable"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type BridgeConfig struct {
	StratumPort string `yaml:"stratum_port"`
	RPCServer   string `yaml:"kaspad_address"`
	PromPort    string `yaml:"prom_port"`
	PrintStats  bool   `yaml:"print_stats"`
}

func ListenAndServe(cfg BridgeConfig) error {
	logFile, err := os.OpenFile("bridge.log", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.RFC3339TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(pe)
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), zap.InfoLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(colorable.NewColorableStdout()), zap.InfoLevel),
	)
	logger := zap.New(core).Sugar()

	if cfg.PromPort != "" {
		StartPromServer(logger, cfg.PromPort)
	}

	ksApi, err := NewKaspaAPI(cfg.RPCServer, logger)
	if err != nil {
		return err
	}

	shareHandler := newShareHandler(ksApi.kaspad)
	clientHandler := newClientListener(logger, shareHandler)
	handlers := gostratum.DefaultHandlers()
	// override the submit handler with an actual useful handler
	handlers[string(gostratum.StratumMethodSubmit)] =
		func(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) error {
			return shareHandler.HandleSubmit(ctx, event)
		}

	stratumConfig := gostratum.StratumListenerConfig{
		Port:           cfg.StratumPort,
		HandlerMap:     handlers,
		StateGenerator: MiningStateGenerator,
		ClientListener: clientHandler,
		Logger:         logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ksApi.Start(ctx, func() {
		clientHandler.NewBlockAvailable(ksApi)
	})

	if cfg.PrintStats {
		go shareHandler.startStatsThread()
	}

	server := gostratum.NewListener(stratumConfig)
	server.Listen(context.Background())
	return nil
}
