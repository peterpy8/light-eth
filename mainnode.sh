#!/bin/sh
Datadir0="/home/vivid/.mainnode/"

#build/bin/siotchain --datadir $Datadir0 removedb
#build/bin/siotchain --datadir $Datadir1 removedb

#rm -rf ~/.ethash
rm -rf $Datadir0


echo "node datadir cleared"

echo "main node init"
build/bin/siotchain --datadir $Datadir0 init genesis.json
build/bin/siotchain --datadir $Datadir0 --port 30303 --networkid 9876 console
