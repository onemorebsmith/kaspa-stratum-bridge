# Hive configuration
## Local bridge (1 bridge per miner)
This assumes that you will be running the ks_bridge on the mining computer. I've provided 2 template flight sheets that can be imported to jumpstart the config process 

---

I'll be using bzminer here, so I'm focusing on the file [here](misc/hive-templates/kas_bz_local_adapter.json). Hive gives the option to import from a file or from your clipboard; I'll be copying directly from the clipboard in this example. 

Navigate to the template that you want and hit the 'copy file contents' button

![image](https://user-images.githubusercontent.com/59971111/191894038-4435237f-d68e-4248-bb44-60e968319ad3.png)

This will load the contents of the file in your clipboard. Now go to hive, create a new flight sheet, and click the import from clipboard button

![image](https://user-images.githubusercontent.com/59971111/191893779-01ebdceb-da8d-455e-9e87-389685a59cc1.png)

Once the import happens you'll be presented with a flightsheet filled with angry little red boxes like below.

![image](https://user-images.githubusercontent.com/59971111/191894254-4b27961c-5b08-4f9a-af20-2b04922ae2f4.png)

 Set the coin & wallet for both miners in the flight sheet, and choose 'Configure in miner' for the pool. Strangly you cannot edit the miner configs until you touch the pool config, so re-set it to 'Configure in miner' if the UI is being weird. 
 
At this point the flight sheet should be happy:
![image](https://user-images.githubusercontent.com/59971111/191894518-0b02dacf-9904-4f53-81de-5bb952568146.png)

Last step, you just need to edit the custom miner section with the address of the kaspa node you're mining to. This needs the full ip/port so it'll look similar to `192.168.0.12:16110` save the changes after you're finished

![image](https://user-images.githubusercontent.com/59971111/191894833-98eabaa2-2210-4ac5-a974-8ad94573a4e5.png)


You should be good to create the flight sheet now. However, you should verify the configurations in both miners before you run it. Never run something blindly off the internet :)

This is what the configuration generally should look like

![image](https://user-images.githubusercontent.com/59971111/191895060-79033098-4303-496d-b5dd-d4cabcc88946.png)

The important parts are the 'Install URL' (make sure that points back to this repo somewhere) and double check that your wallet address is set correctly. 

You can verify that the bridge is working by either feching miner logs, or checking on the main miner--if it's mining then the bridge is working!



## Remote bridge (1 bridge for N miners)
This is the preferred configuration as it's a bit easier to setup and the bridge can easily handle multiple miners.

Ideally you'd run the bridge on the same machine that your kaspad node is running on. You can follow any of the installation methods defined in the main readme of the repo.

Once the bridge is running, miners can be configured simply by replacing the pool URL with the ip:port of the bridge. In the example below the bridge is running on `192.168.0.25:5555`

![image](https://user-images.githubusercontent.com/59971111/191895463-974c75eb-b31f-4196-a0ac-2f0a689fe829.png)




