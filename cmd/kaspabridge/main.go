package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/onemorebsmith/kaspastratum/src/kaspastratum"
	"gopkg.in/yaml.v2"
)

func main() {
	log.SetOutput(os.Stdout)

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
	log.Println("----------------------------------")
	log.Printf("initializing bridge")
	log.Printf("\tkaspad:      %s", cfg.RPCServer)
	log.Printf("\tstratum:     %s", cfg.StratumPort)
	log.Printf("\tstats:       %t", cfg.PrintStats)
	log.Println("----------------------------------")

	if err := kaspastratum.ListenAndServe(cfg); err != nil {
		log.Println(err)
	}
}
