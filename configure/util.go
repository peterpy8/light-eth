package configure

import (
	"math/big"

	"github.com/siotchain/siot/helper"
)

var (
	TestNetGenesisHash = helper.HexToHash("") // Testnet genesis hash to enforce below configs on
	MainNetGenesisHash = helper.HexToHash("") // Mainnet genesis hash to enforce below configs on

	TestNetHomesteadBlock = big.NewInt(000000)  // Testnet homestead block
	MainNetHomesteadBlock = big.NewInt(000000) // Mainnet homestead block

	TestNetHomesteadGasRepriceBlock = big.NewInt(000000) // Testnet gas reprice block
	MainNetHomesteadGasRepriceBlock = big.NewInt(000000) // Mainnet gas reprice block

	TestNetHomesteadGasRepriceHash = helper.HexToHash("") // Testnet gas reprice block hash (used by fast sync)
	MainNetHomesteadGasRepriceHash = helper.HexToHash("") // Mainnet gas reprice block hash (used by fast sync)

	TestNetSpuriousDragon = big.NewInt(000000)
	MainNetSpuriousDragon = big.NewInt(000000)

	TestNetChainID = big.NewInt(2) // Test net default chain ID
	MainNetChainID = big.NewInt(1) // main net default chain ID
)
