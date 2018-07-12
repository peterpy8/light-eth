#!/bin/sh
Datadir1="/home/vivid/.peernode/"

rm -rf $Datadir1

echo "peer node init"
build/bin/siotchain --dir $Datadir1 init genesis.json
build/bin/siotchain --dir $Datadir1 --networkport 10001 --rpc --rpcport 8887 --chainnetwork 9876

echo "all command executed"
