package kaspastratum

import (
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
)

type BridgeConfig struct {
	StratumPort string `yaml:"stratum_port"`
	RPCServer   string `yaml:"kaspad_address"`
	MiningAddr  string `yaml:"miner_address"`
}

func parseConfig() (*BridgeConfig, error) {
	cfg := &BridgeConfig{}
	parser := flags.NewParser(cfg, flags.PrintErrors|flags.HelpFlag)
	_, err := parser.Parse()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse command line flags")
	}
	return cfg, nil
}
