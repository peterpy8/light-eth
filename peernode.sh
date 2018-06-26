#!/bin/sh
Datadir1="/home/vivid/.peernode/*"

rm -rf $Datadir1

echo "peer node init"
build/bin/geth --datadir $Datadir1 init genesis.json
build/bin/geth --datadir $Datadir1 --port 30304 --networkid 9876 --exec 'loadScript("peernode.js")' console

echo "all command executed"
