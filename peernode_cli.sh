#!/bin/sh

mainNodeId=`build/bin/siotchain --rpcport 8888 --request " getNodeId" siotchain-cli`
echo "test case 1: get main node id ---------------"
echo $mainNodeId

url="siot://$mainNodeId@127.0.0.1:30303"
echo $url
echo "test case 2: connect peernode to mainnode --------------"
echo `build/bin/siotchain --rpcport 8887 --request "connectPeer $url" siotchain-cli`

echo "test case 3: get peer id list -------------"
echo `build/bin/siotchain --rpcport 8887 --request "getPeers" siotchain-cli`


acct=`build/bin/siotchain --rpcport 8887 --request "getNewAccount 123" siotchain-cli`
echo "test case 4: create new account acct --------------"
echo $acct

echo "test case 5: get peernode acct balance before transaction with mainnode -------------"
echo `build/bin/siotchain --rpcport 8887 --request "getbalance $acct" siotchain-cli`

# get the account address of mainnode acct2
mainnode_acct2=`build/bin/siotchain --rpcport 8888 --request "getLastAccount" siotchain-cli`
echo $mainnode_acct2
echo "test case 7: incorrect input format for sendAsset ----------------"
echo `build/bin/siotchain --rpcport 8888 --request "sendAsset $mainnode_acct2 $acct" siotchain-cli`

echo "test case 8: send asset from mainnode acct2 to peernode acct ----------------"
echo `build/bin/siotchain --rpcport 8888 --request "sendAsset $mainnode_acct2 $acct 900000" siotchain-cli`

echo "test case 9: set mainnode acct2 as miner"
echo `build/bin/siotchain --rpcport 8888 --request "setMiner $mainnode_acct2" siotchain-cli`

echo "test case 9: mining to record transaction"
echo `build/bin/siotchain --rpcport 8888 --request "StartMine" siotchain-cli`
sleep 5s
echo `build/bin/siotchain --rpcport 8888 --request "stopMine" siotchain-cli`

echo "test case 10: get peernode acct balance after transaction committed to the block: should be 900000 -------------"
echo `build/bin/siotchain --rpcport 8887 --request "getbalance $acct" siotchain-cli`

