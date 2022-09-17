CMD_PATH="../cmd/kaspabridge"
rm -rf release
mkdir -p release
cd release
mkdir -p ks_bridge_win64;env GOOS=windows GOARCH=amd64 go build -o ks_bridge_win64/ks_bridge.exe ${CMD_PATH};cp ${CMD_PATH}/config.yaml ks_bridge_win64/
mkdir -p ks_bridge_linux;env GOOS=linux GOARCH=amd64 go build -o ks_bridge_linux/ks_bridge ${CMD_PATH};cp ${CMD_PATH}/config.yaml ks_bridge_linux/

zip -r ks_bridge_win64.zip ks_bridge_win64
tar -czvf ks_bridge_linux.tar.gz ks_bridge_linux