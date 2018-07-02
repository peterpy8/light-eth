function sleep(milliseconds) {
  var start = new Date().getTime();
  for (var i = 0; i < 1e7; i++) {
    if ((new Date().getTime() - start) > milliseconds){
      break;
    }
  }
}

function interaction() {
    console.log("[peermode]: peer count: " + net.peerCount)
    console.log("[peermode]: connect to [::]:30303")
    admin.addPeer("enode://f34a12aa31e2a96e25e9cc027b83ae7af134c4e5fb24bad1d5cc3325555592a1e4e4ce9e89df461d42d5312393117db6e70144dae50893ff867183101f018b13@127.0.0.1:30303")
    while (net.peerCount == 0)
    {
        sleep(1000)
        console.log("[peermode]: peer count: " + net.peerCount)
    }

    personal.newAccount("123456")
    personal.newAccount("123456")
    acc0 = web3.eth.accounts[0]
    acc1 = web3.eth.accounts[1]
    console.log("[mainnode]: account list: " + eth.accounts)
    console.log("[mainnode]: acc0 balance: " + web3.fromWei(web3.eth.getBalance(acc0)))
    console.log("[mainnode]: acc1 balance: " + web3.fromWei(web3.eth.getBalance(acc1)))

    miner.setEtherbase(eth.accounts[0])
    miner.start()

    while (1) {
        balance = web3.fromWei(web3.eth.getBalance(acc0))
        if (balance > 1) {
            personal.unlockAccount(acc0, "123456")
            eth.sendTransaction({from:acc0,to:acc1,value:web3.toWei(1,"ether")})
            while (1) {
                balance0 = web3.fromWei(web3.eth.getBalance(acc0))
                balance1 = web3.fromWei(web3.eth.getBalance(acc1))
                if (balance0 >0 && balance1 > 0) {
                    miner.stop()
                    break
                }
            }
            break
        }
    }

    console.log("[mainnode]: acc0 balance: " + web3.fromWei(web3.eth.getBalance(acc0)))
    console.log("[mainnode]: acc1 balance: " + web3.fromWei(web3.eth.getBalance(acc1)))
    console.log(admin.nodeInfo.id)
}

interaction()
