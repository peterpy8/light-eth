package configure

import (
	"math/big"

	"github.com/siotchain/siot/helper"
)

// MainnetChainConfig is the chain parameters to run a node on the main network.
var MainnetChainConfig = &ChainConfig{
	HomesteadBlock: MainNetHomesteadBlock,
	DAOForkBlock:   MainNetDAOForkBlock,
	DAOForkSupport: true,
	SiotImpr0Block: MainNetHomesteadGasRepriceBlock,
	SiotImpr0Hash:  MainNetHomesteadGasRepriceHash,
	SiotImpr1Block: MainNetSpuriousDragon,
	SiotImpr2Block: MainNetSpuriousDragon,
}

// TestnetChainConfig is the chain parameters to run a node on the test network.
var TestnetChainConfig = &ChainConfig{
	HomesteadBlock: TestNetHomesteadBlock,
	DAOForkBlock:   TestNetDAOForkBlock,
	DAOForkSupport: false,
	SiotImpr0Block: TestNetHomesteadGasRepriceBlock,
	SiotImpr0Hash:  TestNetHomesteadGasRepriceHash,
	SiotImpr1Block: TestNetSpuriousDragon,
	SiotImpr2Block: TestNetSpuriousDragon,
}

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	ChainId *big.Int `json:"chainId"` // Chain id identifies the current chain and is used for replay protection

	HomesteadBlock *big.Int `json:"homesteadBlock"` // Homestead switch block (nil = no fork, 0 = already homestead)
	DAOForkBlock   *big.Int `json:"daoForkBlock"`   // TheDAO hard-fork switch block (nil = no fork)
	DAOForkSupport bool     `json:"daoForkSupport"` // Whether the nodes supports or opposes the DAO hard-fork

	// EIP150 implements the Gas price changes
	SiotImpr0Block *big.Int    `json:"siotImpr0Block"` // SiotImpr0 block
	SiotImpr0Hash  helper.Hash `json:"siotImpr0Hash"`     // SiotImpr0 hash

	SiotImpr1Block *big.Int `json:"siotImpr1Block"` // SiotImpr1 block
	SiotImpr2Block *big.Int `json:"siotImpr2Block"`    // SiotImpr2 block
}

var (
	TestChainConfig = &ChainConfig{big.NewInt(1), new(big.Int), new(big.Int), true, new(big.Int), helper.Hash{}, new(big.Int), new(big.Int)}
	TestRules       = TestChainConfig.Rules(new(big.Int))
)

// IsHomestead returns whether num is either equal to the homestead block or greater.
func (c *ChainConfig) IsHomestead(num *big.Int) bool {
	if c.HomesteadBlock == nil || num == nil {
		return false
	}
	return num.Cmp(c.HomesteadBlock) >= 0
}

// GasTable returns the gas table corresponding to the current phase (homestead or homestead reprice).
//
// The returned GasTable's fields shouldn't, under any circumstances, be changed.
func (c *ChainConfig) GasTable(num *big.Int) GasTable {
	if num == nil {
		return GasTableHomestead
	}

	switch {
	case c.SiotImpr2Block != nil && num.Cmp(c.SiotImpr2Block) >= 0:
		return GasTableEIP158
	case c.SiotImpr0Block != nil && num.Cmp(c.SiotImpr0Block) >= 0:
		return GasTableHomesteadGasRepriceFork
	default:
		return GasTableHomestead
	}
}

func (c *ChainConfig) IsSiotImpr0(num *big.Int) bool {
	if c.SiotImpr0Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.SiotImpr0Block) >= 0

}

func (c *ChainConfig) IsSiotImpr1(num *big.Int) bool {
	if c.SiotImpr1Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.SiotImpr1Block) >= 0

}

func (c *ChainConfig) IsSiotImpr2(num *big.Int) bool {
	if c.SiotImpr2Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.SiotImpr2Block) >= 0

}

// Rules wraps ChainConfig and is merely syntatic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainId                                            *big.Int
	IsHomestead, IsSiotImpr0, IsSiotImpr1, IsSiotImpr2 bool
}

func (c *ChainConfig) Rules(num *big.Int) Rules {
	return Rules{ChainId: new(big.Int).Set(c.ChainId), IsHomestead: c.IsHomestead(num), IsSiotImpr0: c.IsSiotImpr0(num), IsSiotImpr1: c.IsSiotImpr1(num), IsSiotImpr2: c.IsSiotImpr2(num)}
}
