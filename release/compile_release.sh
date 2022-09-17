CMD_PATH="../cmd/kaspabridge"
rm *.zip
rm *.tar.gz
rm -rf staging
rm -rf ks_bridge_win64
rm -rf ks_bridge_linux
mkdir -p ks_bridge_win64;env GOOS=windows GOARCH=amd64 go build -o ks_bridge_win64/ks_bridge.exe ${CMD_PATH};cp ${CMD_PATH}/config.yaml ks_bridge_win64/
mkdir -p ks_bridge_linux;env GOOS=linux GOARCH=amd64 go build -o ks_bridge_linux/ks_bridge ${CMD_PATH};cp ${CMD_PATH}/config.yaml ks_bridge_linux/
ks_bridge_v1.0_kaspad_12_5_Linux64.tar.gz

mkdir -p staging
zip -r staging/ks_bridge_win64.zip ks_bridge_win64
tar -czvf staging/ks_bridge_linux.tar.gz ks_bridge_linux