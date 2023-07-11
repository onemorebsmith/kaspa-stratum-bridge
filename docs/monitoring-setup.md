# Configuring monitoring (Grafana + Prometheus)

## Reqirements

Docker must be installed! Visit https://www.docker.com/ and follow the setup instructions on the docker website


## Configuration

To begin you need to pull down the repo (or use the zipped source from the release). I'll use the release code for this example. 

Go to the latest release ([v1.1](https://github.com/onemorebsmith/kaspa-stratum-bridge/releases/tag/v1.1) at the time of writing) and download the source code. Download the zip archive for windows, tar.gz for everything else.

![image](https://user-images.githubusercontent.com/59971111/192021218-01d83e83-3ad4-4ce2-87b4-080ff30b6693.png)

Unzip the source in a directory of your choice and open a shell/cmd prompt.

![image](https://user-images.githubusercontent.com/59971111/192022638-0c772814-c47e-4f41-b579-4fcf5b387394.png)

At this point if you can not progress without docker installed. Go install it if you haven't already. 

For this example I'll be running everything in docker -- including the bridge. So type the following from the root folder to stand up everything:

`docker compose -f docker-compose-all-src.yml up -d --build`

Youll see output about downloading images and such and eventually see output like below: 

![image](https://user-images.githubusercontent.com/59971111/192023410-4d5d09c4-2b52-4405-ae5c-3c113e33c4c8.png)

At this point everything is running successfully in the background. 

- ks_bridge is running on port :5555
- prometheus is running on port :9090
- grafana is running on port :3000

You may point your miners the IP address of the computer you installed on at port 5555. I you're unsure your current IP then run `ipconfig` on windows and `ifconfig` in linux. You'll put this IP and the port into your miner config.

## Accessing grafana

Assuming the setup went correctly you'll be able to access grafana by visiting <http://127.0.0.1:3000/d/x7cE7G74k1/ksb-monitoring>

![image](https://user-images.githubusercontent.com/59971111/192024515-dd487a3a-3d15-4d21-bfbf-189b2db69782.png)

The default user/password is admin/admin. Grafana will prompt you to change the password but you can just ignore it (hit skip).

You will then be redirected to the mining dashboard. It'll look like below until you start getting info from your miners. 

![Monitoring Dashboard Without Data](/docs/images/dashboard_no_data.png)

At this point you're configured and good to go. Many of the stats on the graph are averaged over a configurable time period (24hr default - use the 'resolution' dropdown on the top left of the page to change this), so keep in mind that the metrics might be incomplete during this initial period. Also note that there are 'wallet_filter' and 'show_balances' dropdowns as well. These filter the database and hide your balance if you don't want that exposed. The monitoring UI is also accessable on any device on your local network (including your phone!) if you use the host computers ip address -- just type in the ip and port such as `http://192.168.0.25/3000` (this is an example, this exact link probablly wont work for you)

 
