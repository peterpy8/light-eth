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
    admin.addPeer("enode://03f4062d32ccad5c393ad64784ff5688cc743310126bd0aad3b474be59203595c0a4c047cdcb98aa872bd23adbe2ba165eabedd22525724ff5e2bb4fba2978b1@127.0.0.1:30303")
    while (net.peerCount == 0)
    {
        sleep(1000)
        console.log("[peermode]: peer count: " + net.peerCount)
    }
}

interaction()
