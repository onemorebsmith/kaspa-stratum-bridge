
# Kaspa Stratum Adapter

This is a daemon that allows mining to a local (or remote) kaspa node using stratum-base miners.

2-3ms response time using local node and remote miner:

![image](https://user-images.githubusercontent.com/59971111/186201719-be398c46-f861-4c45-a4aa-5264ad084566.png)

Shares-based work allocation with periodic stat output:

![image](https://user-images.githubusercontent.com/59971111/186201915-a9d0bbc3-9a21-474b-8240-5e4b2b1ed7bb.png)

  
Tested on windows, x64 macos & ubuntu w/ lolminer, SRBMiner, & bzminer for both solo-mining and dual mining.

No fee, forever. Do what you want with it.

Huge shoutout to https://github.com/KaffinPX/KStratum for the inspiration
  

Tips appreciated: `kaspa:qp9v6090sr8jjlkq7r3f4h9un5rtfhu3raknfg3cca9eapzee57jzew0kxwlp`

  

# Install

## Docker All-in-one

Note: This does requires that docker is installed.

  

`docker compose -f docker-compose-all.yml up -d` will run the bridge with default settings. This assumes a local kaspad node with default port settings and exposes port 5555 to incoming stratum connections.

  

This also spins up a local prometheus and grafana instance that gather stats and host the metrics dashboard. Once the services are up and running you can view the dashboard using `http://127.0.0.1:3000/d/x7cE7G74k/monitoring`

  

Most of the stats on the graph are averaged over an hour time period, so keep in mind that the metrics might be inaccurate for the first hour or so that the bridge is up.

  

## Docker (non-compose)

Note: This does not require pulling down the repo, it only requires that docker is installed.

  

`docker run -p 5555:5555 onemorebsmith/kaspa_bridge:latest` will run the bridge with default settings. This assumes a local kaspad node with default port settings and exposes port 5555 to incoming stratum connections.

  

Detailed:

  

`docker run -p {stratum_port}:5555 onemorebsmith/kaspa_bridge --kaspa {kaspad_address} --stats {false}` will run the bridge targeting a kaspad node at {kaspad_address}. stratum port accepting connections on {stratum_port}, and only logging connection activity, found blocks, and errors

  

## Manual build

Install go 1.18 using whatever package manager is approprate for your system

  

run `cd cmd/kaspabridge;go build .`

  

Modify the config file in ./cmd/bridge/config.yaml with your setup, the file comments explain the various flags

  

run `./kaspabridge` in the `cmd/kaspabridge` directory

  

all-in-one (build + run) `cd cmd/kaspabridge/;go build .;./kaspabridge`

  

## Metrics

If the app is run with the `-prom={port}` flag the application will host stats on the port specified by `{port}`, these stats are documented in the file [prom.go](src/kaspastratum/prom.go). This is intended to be use by prometheus but the stats can be fetched and used independently if desired. `curl http://localhost:2114/metrics | grep ks_` will get a listing of current stats. All published stats have a `ks_` prefix for ease of use.
