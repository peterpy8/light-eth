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
    admin.addPeer("enode://bdcaccaf23e43a3849bf9f80ae7b63cc3320335e064f5d41dd9c4a3d6c11e1068fcb21c6ae7b621aa0433b40735be06689118d3344ca2f6bd3b6c8cd1bc405ae@127.0.0.1:30303")
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
