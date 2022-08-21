package gostratum

import (
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
)

type BridgeConfig struct {
	RPCServer  string `short:"s" long:"rpcserver" description:"RPC server to connect to"`
	MiningAddr string `long:"miningaddr" description:"Address to mine to"`
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
