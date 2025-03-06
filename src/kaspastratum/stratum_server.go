package kaspastratum

import (
	"context"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/onemorebsmith/kaspastratum/src/gostratum"
	"github.com/onemorebsmith/kaspastratum/src/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const version = "v1.2.2"
const minBlockWaitTime = 3 * time.Second

type BridgeConfig struct {
	StratumPort     string        `yaml:"stratum_port"`
	RPCServer       string        `yaml:"kaspad_address"`
	PromPort        string        `yaml:"prom_port"`
	PrintStats      bool          `yaml:"print_stats"`
	UseLogFile      bool          `yaml:"log_to_file"`
	HealthCheckPort string        `yaml:"health_check_port"`
	BlockWaitTime   time.Duration `yaml:"block_wait_time"`
	MinShareDiff    uint          `yaml:"min_share_diff"`
	VarDiff         bool          `yaml:"var_diff"`
	SharesPerMin    uint          `yaml:"shares_per_min"`
	VarDiffStats    bool          `yaml:"var_diff_stats"`
	ExtranonceSize  uint          `yaml:"extranonce_size"`
	ClampPow2       bool          `yaml:"pow2_clamp"`
}

func configureZap(cfg BridgeConfig) (*zap.SugaredLogger, func()) {
	pe := zap.NewProductionEncoderConfig()
	pe.EncodeTime = zapcore.RFC3339TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(pe)
	consoleEncoder := zapcore.NewConsoleEncoder(pe)

	bws := &utils.BufferedWriteSyncer{WS: zapcore.AddSync(colorable.NewColorableStdout()), FlushInterval: 5 * time.Second}

	if !cfg.UseLogFile {
		return zap.New(zapcore.NewCore(consoleEncoder,
			bws, zap.InfoLevel)).Sugar(), func() { bws.Stop() }
	}

	// log file fun
	logFile, err := os.OpenFile("bridge.log", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	blws := &utils.BufferedWriteSyncer{WS: zapcore.AddSync(logFile), FlushInterval: 5 * time.Second}
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, blws, zap.InfoLevel),
		zapcore.NewCore(consoleEncoder, bws, zap.InfoLevel),
	)
	return zap.New(core).Sugar(), func() { bws.Stop(); blws.Stop(); logFile.Close() }
}

func ListenAndServe(cfg BridgeConfig) error {
	logger, logCleanup := configureZap(cfg)
	defer logCleanup()

	if cfg.PromPort != "" {
		StartPromServer(logger, cfg.PromPort)
	}

	blockWaitTime := cfg.BlockWaitTime
	if blockWaitTime == 0 {
		blockWaitTime = minBlockWaitTime
	}
	ksApi, err := NewKaspaAPI(cfg.RPCServer, blockWaitTime, logger)
	if err != nil {
		return err
	}

	if cfg.HealthCheckPort != "" {
		logger.Info("enabling health check on port " + cfg.HealthCheckPort)
		http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		go http.ListenAndServe(cfg.HealthCheckPort, nil)
	}

	shareHandler := newShareHandler(ksApi.kaspad)
	minDiff := float64(cfg.MinShareDiff)
	if minDiff == 0 {
		minDiff = 4
	}
	if cfg.ClampPow2 {
		minDiff = math.Pow(2, math.Floor(math.Log2(minDiff)))
	}
	extranonceSize := cfg.ExtranonceSize
	if extranonceSize > 3 {
		extranonceSize = 3
	}
	clientHandler := newClientListener(logger, shareHandler, minDiff, int8(extranonceSize))
	handlers := gostratum.DefaultHandlers()
	// override the submit handler with an actual useful handler
	handlers[string(gostratum.StratumMethodSubmit)] =
		func(ctx *gostratum.StratumContext, event gostratum.JsonRpcEvent) error {
			if err := shareHandler.HandleSubmit(ctx, event); err != nil {
				ctx.Logger.Sugar().Error(err) // sink error
			}
			return nil
		}

	stratumConfig := gostratum.StratumListenerConfig{
		Port:           cfg.StratumPort,
		HandlerMap:     handlers,
		StateGenerator: MiningStateGenerator,
		ClientListener: clientHandler,
		Logger:         logger.Desugar(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ksApi.Start(ctx, func() {
		clientHandler.NewBlockAvailable(ksApi)
	})

	if cfg.VarDiff {
		sharesPerMin := cfg.SharesPerMin
		if sharesPerMin <= 0 {
			sharesPerMin = 20
		}
		go shareHandler.startVardiffThread(sharesPerMin, cfg.VarDiffStats, cfg.ClampPow2)
	}

	if cfg.PrintStats {
		go shareHandler.startPrintStatsThread()
	}

	go shareHandler.startPruneStatsThread()

	return gostratum.NewListener(stratumConfig).Listen(context.Background())
}
