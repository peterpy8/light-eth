#!/bin/sh
Datadir0="/home/vivid/.mainnode/"

#build/bin/siotchain --datadir $Datadir0 removedb
#build/bin/siotchain --datadir $Datadir1 removedb
#rm -rf ~/.ethash

rm -rf $Datadir0

echo "node datadir cleared"
echo "main node init"

build/bin/siotchain --dir $Datadir0 init genesis.json
build/bin/siotchain --dir $Datadir0 --networkport 10000 --rpc --rpcport 8888 --chainnetwork 9876
