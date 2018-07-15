package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// MainnetChainConfig is the chain parameters to run a node on the main network.
var MainnetChainConfig = &ChainConfig{
	HomesteadBlock: MainNetHomesteadBlock,
	DAOForkBlock:   MainNetDAOForkBlock,
	DAOForkSupport: true,
	EIP150Block:    MainNetHomesteadGasRepriceBlock,
	EIP150Hash:     MainNetHomesteadGasRepriceHash,
	EIP155Block:    MainNetSpuriousDragon,
	EIP158Block:    MainNetSpuriousDragon,
}

// TestnetChainConfig is the chain parameters to run a node on the test network.
var TestnetChainConfig = &ChainConfig{
	HomesteadBlock: TestNetHomesteadBlock,
	DAOForkBlock:   TestNetDAOForkBlock,
	DAOForkSupport: false,
	EIP150Block:    TestNetHomesteadGasRepriceBlock,
	EIP150Hash:     TestNetHomesteadGasRepriceHash,
	EIP155Block:    TestNetSpuriousDragon,
	EIP158Block:    TestNetSpuriousDragon,
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
	EIP150Block *big.Int    `json:"eip150Block"` // EIP150 HF block (nil = no fork)
	EIP150Hash  common.Hash `json:"eip150Hash"`  // EIP150 HF hash (fast sync aid)

	EIP155Block *big.Int `json:"eip155Block"` // EIP155 HF block
	EIP158Block *big.Int `json:"eip158Block"` // EIP158 HF block
}

var (
	TestChainConfig = &ChainConfig{big.NewInt(1), new(big.Int), new(big.Int), true, new(big.Int), common.Hash{}, new(big.Int), new(big.Int)}
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
	case c.EIP158Block != nil && num.Cmp(c.EIP158Block) >= 0:
		return GasTableEIP158
	case c.EIP150Block != nil && num.Cmp(c.EIP150Block) >= 0:
		return GasTableHomesteadGasRepriceFork
	default:
		return GasTableHomestead
	}
}

func (c *ChainConfig) IsEIP150(num *big.Int) bool {
	if c.EIP150Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.EIP150Block) >= 0

}

func (c *ChainConfig) IsEIP155(num *big.Int) bool {
	if c.EIP155Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.EIP155Block) >= 0

}

func (c *ChainConfig) IsEIP158(num *big.Int) bool {
	if c.EIP158Block == nil || num == nil {
		return false
	}
	return num.Cmp(c.EIP158Block) >= 0

}

// Rules wraps ChainConfig and is merely syntatic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainId                                   *big.Int
	IsHomestead, IsEIP150, IsEIP155, IsEIP158 bool
}

func (c *ChainConfig) Rules(num *big.Int) Rules {
	return Rules{ChainId: new(big.Int).Set(c.ChainId), IsHomestead: c.IsHomestead(num), IsEIP150: c.IsEIP150(num), IsEIP155: c.IsEIP155(num), IsEIP158: c.IsEIP158(num)}
}
