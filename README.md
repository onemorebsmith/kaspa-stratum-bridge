# Kaspa Stratum/GRPC bridge
This is an daemon that listens for incoming stratum connections from miners and does the translation between stratum events and the expected events from kaspad. This allows solo-mining kaspa with a local node (or on a public node) while mining with stratum-based miners such as lolminer.

2-3ms response time via `stratum-ping` using a local node and remote miner

Tested on x64 macos & ubuntu w/ lolminer

No fee, forever. Do what you want with it. 

Huge shoutout to https://github.com/KaffinPX/KStratum for the inspiration

# Install
## Docker

`docker run -p 5555:5555 onemorebsmith/kaspa_bridge` will run the bridge with default settings. This assumes a local kaspad node with default port settings and exposes port 5555 to incoming stratum connections. 

Detailed:

`docker run -p {stratum_port}:5555 onemorebsmith/kaspa_bridge --kaspa {kaspad_address} --stats {false}` will run the bridge targeting a kaspad node at {kaspad_address}. stratum port accepting connections on {stratum_port}, and only logging connection activity, found blocks, and errors

## Manual build
Install go 1.18 using whatever package manager is approprate for your system

run `cd cmd/kaspabridge;go build .`

Modify the config file in ./cmd/bridge/config.yaml with your setup
```
    # stratum_port: the port that will be listening for incoming stratum traffic, 
    # Note `:PORT` format is needed if not specifiying a specific ip range 
    stratum_port: :8080
    # kaspad_address: address/port of the rpc server for kaspad, typically 16110
    kaspad_address: localhost:16110
```

run `./kaspabridge` in the `cmd/kaspabridge` directory

all-in-one (build + run) `cd cmd/kaspabridge/;go build .;./kaspabridge`

# TODO
* Docker
* Command-line flags support
* 'Pool'-like process, issue smaller diff to miners
* Discord/telegram notifications




Tips appreciated: `kaspa:qp9v6090sr8jjlkq7r3f4h9un5rtfhu3raknfg3cca9eapzee57jzew0kxwlp`
