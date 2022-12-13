package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/onemorebsmith/kaspastratum/src/kaspastratum"
	"gopkg.in/yaml.v2"
)

func main() {
	pwd, _ := os.Getwd()
	fullPath := path.Join(pwd, "config.yaml")
	log.Printf("loading config @ `%s`", fullPath)
	rawCfg, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Printf("config file not found: %s", err)
		os.Exit(1)
	}
	cfg := kaspastratum.BridgeConfig{}
	if err := yaml.Unmarshal(rawCfg, &cfg); err != nil {
		log.Printf("failed parsing config file: %s", err)
		os.Exit(1)
	}

	flag.StringVar(&cfg.StratumPort, "stratum", cfg.StratumPort, "stratum port to listen on, default `:5555`")
	flag.BoolVar(&cfg.PrintStats, "stats", cfg.PrintStats, "true to show periodic stats to console, default `true`")
	flag.StringVar(&cfg.RPCServer, "kaspa", cfg.RPCServer, "address of the kaspad node, default `localhost:16110`")
	flag.DurationVar(&cfg.BlockWaitTime, "blockwait", cfg.BlockWaitTime, "time in ms to wait before manually requesting new block, default `500`")
	flag.UintVar(&cfg.MinShareDiff, "mindiff", cfg.MinShareDiff, "minimum share difficulty to accept from miner(s), default `4`")
	flag.UintVar(&cfg.ExtranonceSize, "extranonce", cfg.ExtranonceSize, "size in bytes of extranonce, default `0`")
	flag.StringVar(&cfg.PromPort, "prom", cfg.PromPort, "address to serve prom stats, default `:2112`")
	flag.BoolVar(&cfg.UseLogFile, "log", cfg.UseLogFile, "if true will output errors to log file, default `true`")
	flag.StringVar(&cfg.HealthCheckPort, "hcp", cfg.HealthCheckPort, `(rarely used) if defined will expose a health check on /readyz, default ""`)
	flag.Parse()

	log.Println("----------------------------------")
	log.Printf("initializing bridge")
	log.Printf("\tkaspad:          %s", cfg.RPCServer)
	log.Printf("\tstratum:         %s", cfg.StratumPort)
	log.Printf("\tprom:            %s", cfg.PromPort)
	log.Printf("\tstats:           %t", cfg.PrintStats)
	log.Printf("\tlog:             %t", cfg.UseLogFile)
	log.Printf("\tmin diff:        %d", cfg.MinShareDiff)
	log.Printf("\tblock wait:      %s", cfg.BlockWaitTime)
	log.Printf("\textranonce size: %d", cfg.ExtranonceSize)
	log.Printf("\thealth check:    %s", cfg.HealthCheckPort)
	log.Println("----------------------------------")

	if err := kaspastratum.ListenAndServe(cfg); err != nil {
		log.Println(err)
	}
}
