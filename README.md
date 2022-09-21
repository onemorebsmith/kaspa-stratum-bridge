# Kaspa Stratum/GRPC bridge
## This release does not support bzminer, v1.1 will address this issue

This is an daemon that allows mining to a local (or remote) kaspa node using stratum-base miners. 


2-3ms response time using local node and remote miner:

![image](https://user-images.githubusercontent.com/59971111/186201719-be398c46-f861-4c45-a4aa-5264ad084566.png)

Shares-based work allocation with periodic stat output:

![image](https://user-images.githubusercontent.com/59971111/186201915-a9d0bbc3-9a21-474b-8240-5e4b2b1ed7bb.png)


Tested on windows, x64 macos & ubuntu w/ lolminer and SRBMiner

No fee, forever. Do what you want with it. 

Huge shoutout to https://github.com/KaffinPX/KStratum for the inspiration

# Install
## Docker

Note: This does not require pulling down the repo, it only requires that docker is installed.

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
* Discord/telegram notifications
* WebUI of some sort
* System-level tests




Tips appreciated: `kaspa:qp9v6090sr8jjlkq7r3f4h9un5rtfhu3raknfg3cca9eapzee57jzew0kxwlp`
