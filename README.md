# Kaspa Stratum/GRPC bridge
This is a quick applet that listens for incoming stratum connections from miners and does the translation between stratum events and the expected events from kaspad. 

# Install

## Manual build
Install go 1.18 using whatever package manager is approprate for your system

run `cd cmd/bridge;go build .`

Modify the config file in ./cmd/bridge/config.yaml with your setup
```
    # stratum_port: the port that will be listening for incoming stratum traffic
    stratum_port: 8080
    # kaspad_address: address/port of the rpc server for kaspad, typically 16110
    kaspad_address: localhost:16110
    # miner_address: address to mine to
    miner_address: kaspa:{your_address_here}
```


run `./bridge` in the `cmd/bridge` directory


all-in-one (build + run) `cd cmd/bridge/;go build .;./bridge`


## Easy way (docker) -- TODO
-- WIP

Modify the config file in ./cmd/bridge/config.yaml with your setup

run `docker build .`

### Buy me a coffee?
Tips appreciated: `kaspa:qp9v6090sr8jjlkq7r3f4h9un5rtfhu3raknfg3cca9eapzee57jzew0kxwlp`
