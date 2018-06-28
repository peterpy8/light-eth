#!/bin/sh
Datadir1="/home/vivid/.peernode/"

rm -rf $Datadir1

echo "peer node init"
build/bin/siotchain --datadir $Datadir1 init genesis.json
build/bin/siotchain --datadir $Datadir1 --port 30304 --networkid 9876 console

echo "all command executed"
