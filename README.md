# Kaspa Stratum Adapter

This is a lightweight daemon that allows mining to a local (or remote) kaspa node using stratum-base miners.

This daemon is confirmed working with the miners below in both dual-mining and kaspa-only modes (for those that support it) and Windows/MacOs/Linux/HiveOs.
* bzminer
* lolminer
* srbminer
* teamreadminer

No fee, forever. Do what you want with it.

Discord discussions/issues: [here](https://discord.com/channels/599153230659846165/1025501807570600027) 

Huge shoutout to https://github.com/KaffinPX/KStratum for the inspiration
  
Tips appreciated: `kaspa:qp9v6090sr8jjlkq7r3f4h9un5rtfhu3raknfg3cca9eapzee57jzew0kxwlp`

Try my 0-fee [Kaspa Pool](http://ghost-pool.io/) built with this code!


## Hive Setup
[detailed instructions here](hive-setup.md) 


# Features:

Shares-based work allocation with miner-like periodic stat output:

![image](https://user-images.githubusercontent.com/59971111/191983487-479e19ec-a8cb-4edb-afc4-55a1165e79fc.png)



Optional monitoring UI:

https://github.com/onemorebsmith/kaspa-stratum-bridge/blob/main/monitoring-setup.md

![image](https://user-images.githubusercontent.com/59971111/192025446-f20d74a5-f9e0-4290-b98b-9f56af8f23b4.png)

![image](https://user-images.githubusercontent.com/59971111/191980688-2d0faf6b-d551-4880-a316-de2303cfeb7d.png)


Prometheus API:

If the app is run with the `-prom={port}` flag the application will host stats on the port specified by `{port}`, these stats are documented in the file [prom.go](src/kaspastratum/prom.go). This is intended to be use by prometheus but the stats can be fetched and used independently if desired. `curl http://localhost:2114/metrics | grep ks_` will get a listing of current stats. All published stats have a `ks_` prefix for ease of use.

```
user:~$ curl http://localhost:2114/metrics | grep ks_
# HELP ks_estimated_network_hashrate_gauge Gauge representing the estimated network hashrate
# TYPE ks_estimated_network_hashrate_gauge gauge
ks_estimated_network_hashrate_gauge 2.43428982879776e+14
# HELP ks_network_block_count Gauge representing the network block count
# TYPE ks_network_block_count gauge
ks_network_block_count 271966
# HELP ks_network_difficulty_gauge Gauge representing the network difficulty
# TYPE ks_network_difficulty_gauge gauge
ks_network_difficulty_gauge 1.2526479386202519e+14
# HELP ks_valid_share_counter Number of shares found by worker over time
# TYPE ks_valid_share_counter counter
ks_valid_share_counter{ip="192.168.0.17",miner="SRBMiner-MULTI/1.0.8",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="002"} 276
ks_valid_share_counter{ip="192.168.0.24",miner="BzMiner-v11.1.0",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="003"} 43
ks_valid_share_counter{ip="192.168.0.65",miner="BzMiner-v11.1.0",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="001"} 307
# HELP ks_worker_job_counter Number of jobs sent to the miner by worker over time
# TYPE ks_worker_job_counter counter
ks_worker_job_counter{ip="192.168.0.17",miner="SRBMiner-MULTI/1.0.8",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="002"} 3471
ks_worker_job_counter{ip="192.168.0.24",miner="BzMiner-v11.1.0",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="003"} 3399
ks_worker_job_counter{ip="192.168.0.65",miner="BzMiner-v11.1.0",wallet="kaspa:qzk3uh2twkhu0fmuq50mdy3r2yzuwqvstq745hxs7tet25hfd4egcafcdmpdl",worker="001"} 3425

```

# Install

## Docker All-in-one

Note: This does requires that docker is installed.

  

`docker compose -f docker-compose-all.yml up -d` will run the bridge with default settings. This assumes a local kaspad node with default port settings and exposes port 5555 to incoming stratum connections.

  

This also spins up a local prometheus and grafana instance that gather stats and host the metrics dashboard. Once the services are up and running you can view the dashboard using `http://127.0.0.1:3000/d/x7cE7G74k/monitoring`

Default grafana user/pass: admin/admin

Most of the stats on the graph are averaged over an hour time period, so keep in mind that the metrics might be inaccurate for the first hour or so that the bridge is up.


## Docker (non-compose)

Note: This does not require pulling down the repo, it only requires that docker is installed.

`docker run -p 5555:5555 onemorebsmith/kaspa_bridge:latest --log=false` will run the bridge with default settings. This assumes a local kaspad node with default port settings and exposes port 5555 to incoming stratum connections.


Detailed:

`docker run -p {stratum_port}:5555 onemorebsmith/kaspa_bridge  --log=false --kaspa={kaspad_address} --stats={false}` will run the bridge targeting a kaspad node at {kaspad_address}. stratum port accepting connections on {stratum_port}, and only logging connection activity, found blocks, and errors

  

## Manual build

Install go 1.18 using whatever package manager is approprate for your system

  

run `cd cmd/kaspabridge;go build .`

  

Modify the config file in ./cmd/bridge/config.yaml with your setup, the file comments explain the various flags

  

run `./kaspabridge` in the `cmd/kaspabridge` directory

  

all-in-one (build + run) `cd cmd/kaspabridge/;go build .;./kaspabridge`
