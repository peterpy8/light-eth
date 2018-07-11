#!/bin/sh

echo "test the cases when rpc port is incorrect"
echo "test case 1: incorrect rpc port ------"
echo `build/bin/siotchain --rpcport 8000 --request "getNodeInfo" siotchain-cli`

echo "-----------------------------------------------------------------------------"
echo "test the cases when input format is incorrect"
echo "test case 2: incorrect input format for getNewAccount -------------"
echo `build/bin/siotchain --rpcport 8888 --request "getNewAccount" siotchain-cli`

echo "-----------------------------------------------------------------------------"
echo "test for correct input"
nodeInfo=`build/bin/siotchain --rpcport 8888 --request "getNodeInfo" siotchain-cli`
echo "test case 3: get node information ---------------"
echo $nodeInfo


acct1=`build/bin/siotchain --rpcport 8888 --request "getNewAccount 123" siotchain-cli`
echo "test case 4: create new account acct1 --------------"
echo $acct1

echo "test case 5: unlock acct1"
echo `build/bin/siotchain --rpcport 8888 --request " unlockAccount $acct1 123" siotchain-cli`

acct2=`build/bin/siotchain --rpcport 8888 --request "getNewAccount 123" siotchain-cli`
echo "test case 6: create new account acct2 --------------"
echo $acct2

echo "test case 7: unlock acct2 ------------"
echo `build/bin/siotchain --rpcport 8888 --request " unlockAccount $acct2 123" siotchain-cli`

echo "test case 8: get account list ------------"
echo `build/bin/siotchain --rpcport 8888 --request "getAccounts" siotchain-cli`

echo "test case 9: get acct1 balance before mining -------------"
echo `build/bin/siotchain --rpcport 8888 --request "getbalance $acct1" siotchain-cli`

echo "test case 10: set acct1 as miner"
echo `build/bin/siotchain --rpcport 8888 --request "setMiner $acct1" siotchain-cli`

echo "test case 11: start mining ----------------"
echo `build/bin/siotchain --rpcport 8888 --request "StartMine" siotchain-cli`

sleep 5s

echo "test case 12: stop mining -----------------"
echo `build/bin/siotchain --rpcport 8888 --request "stopMine" siotchain-cli`

echo "test case 13: get acct1 balance after mining -------------"
echo `build/bin/siotchain --rpcport 8888 --request "getbalance $acct1" siotchain-cli`

echo "test case 14: send asset of 1000000 from acct1 to acct2 -------------"
echo `build/bin/siotchain --rpcport 8888 --request "sendAsset $acct1 $acct2 1000000" siotchain-cli`

echo "test case 15: mining to record transaction"
echo `build/bin/siotchain --rpcport 8888 --request "StartMine" siotchain-cli`
sleep 5s
echo `build/bin/siotchain --rpcport 8888 --request "stopMine" siotchain-cli`

echo "test case 16: get acct2 balance after transaction committed to the block, should be 1000000 -------------"
echo `build/bin/siotchain --rpcport 8888 --request "getbalance $acct2 " siotchain-cli`
