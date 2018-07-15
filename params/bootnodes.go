package params

import (
	"github.com/ethereum/go-ethereum/p2p/discover"
)

// MainnetBootnodes are the siot node URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []*discover.Node{
	// ETH/DEV Go Bootnodes
	discover.MustParseNode("siot://a979fb575495b8d6db44f750317d0f4622bf4c2aa3365d6af7c284339968eef29b69ad0dce72a4d8db5ebb4968de0e3bec910127f134779fbcb0cb6d3331163c@52.16.188.185:30303"), // IE
	discover.MustParseNode("siot://de471bccee3d042261d52e9bff31458daecc406142b401d4cd848f677479f73104b9fdeb090af9583d3391b7f10cb2ba9e26865dd5fca4fcdc0fb1e3b723c786@54.94.239.50:30303"),  // BR
	discover.MustParseNode("siot://1118980bf48b0a3640bdba04e0fe78b1add18e1cd99bf22d53daac1fd9972ad650df52176e7c7d89d1114cfef2bc23a2959aa54998a46afcf7d91809f0855082@52.74.57.123:30303"),  // SG

	// ETH/DEV Cpp Bootnodes
	discover.MustParseNode("siot://979b7fa28feeb35a4741660a16076f1943202cb72b6af70d327f053e248bab9ba81760f39d0701ef1d8f89cc1fbd2cacba0710a12cd5314d5e0c9021aa3637f9@5.1.83.226:30303"),
}

// TestnetBootnodes are the siot URLs of the P2P bootstrap nodes running on the
// Morden test network.
var TestnetBootnodes = []*discover.Node{
	// ETH/DEV Go Bootnodes
	discover.MustParseNode("siot://e4533109cc9bd7604e4ff6c095f7a1d807e15b38e9bfeb05d3b7c423ba86af0a9e89abbf40bd9dde4250fef114cd09270fa4e224cbeef8b7bf05a51e8260d6b8@94.242.229.4:40404"),
	discover.MustParseNode("siot://8c336ee6f03e99613ad21274f269479bf4413fb294d697ef15ab897598afb931f56beb8e97af530aee20ce2bcba5776f4a312bc168545de4d43736992c814592@94.242.229.203:30303"),

	// ETH/DEV Cpp Bootnodes
}

