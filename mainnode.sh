#!/bin/sh
Datadir0="/home/vivid/.mainnode/*"

#build/bin/geth --datadir $Datadir0 removedb
#build/bin/geth --datadir $Datadir1 removedb

#rm -rf ~/.ethash
rm -rf $Datadir0


echo "node datadir cleared"

echo "main node init"
build/bin/geth --datadir $Datadir0 init genesis.json
build/bin/geth --datadir $Datadir0 --port 30303 --networkid 9876 --exec 'loadScript("mainnode.js")' console
