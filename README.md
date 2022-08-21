# Kaspa Stratum/GRPC bridge
This is a quick applet that listens for incoming stratum connections from miners and does the translation between stratum events and the expected events from kaspad

# Install
## Easy way (docker)
Modify the config file in ./cmd/bridge/config.yaml with your setup
run `docker build .`