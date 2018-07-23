function sleep(milliseconds) {
  var start = new Date().getTime();
  for (var i = 0; i < 1e7; i++) {
    if ((new Date().getTime() - start) > milliseconds){
      break;
    }
  }
}

function interaction() {
    console.log("[peermode]: peer count: " + web3.net.peerCount)
    console.log("[peermode]: connect to [::]:30303")
    admin.addPeer("siot://4eeade3187bf60789cf2e96220b07bb1282c6940da248c80a5f202f23cb7e503b8d0167dddce87ba7d73f1209e279161c8511da4391a13cbde14168aecae7487@127.0.0.1:30303")
    while (web3.net.peerCount == 0)
    {
        sleep(1000)
        console.log("[peermode]: peer count: " + web3.net.peerCount)
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
                if (balance1 > 0) {
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
