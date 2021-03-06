package blockchainCore

import (
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/configure"
)

// GetHashFn returns a function for which the VM env can query block hashes through
// up to the limit defined by the Yellow Paper and uses the given block chain
// to query for information.
func GetHashFn(ref helper.Hash, chain *BlockChain) func(n uint64) helper.Hash {
	return func(n uint64) helper.Hash {
		for block := chain.GetBlockByHash(ref); block != nil; block = chain.GetBlock(block.ParentHash(), block.NumberU64()-1) {
			if block.NumberU64() == n {
				return block.Hash()
			}
		}

		return helper.Hash{}
	}
}

type VMEnv struct {
	chainConfig *configure.ChainConfig // Chain configuration
	state       *state.StateDB         // State to use for executing
	depth       int                    // Current execution depth
	msg         Message                // Message appliod

	header    *types.Header            // Header information
	chain     *BlockChain              // Blockchain handle
	getHashFn func(uint64) helper.Hash // getHashFn callback is used to retrieve block hashes
}

func NewEnv(state *state.StateDB, chainConfig *configure.ChainConfig, chain *BlockChain, msg Message, header *types.Header) *VMEnv {
	env := &VMEnv{
		chainConfig: chainConfig,
		chain:       chain,
		state:       state,
		header:      header,
		msg:         msg,
		getHashFn:   GetHashFn(header.ParentHash, chain),
	}

	return env
}

func (self *VMEnv) ChainConfig() *configure.ChainConfig { return self.chainConfig }
func (self *VMEnv) Origin() helper.Address              { return self.msg.From() }
func (self *VMEnv) BlockNumber() *big.Int               { return self.header.Number }
func (self *VMEnv) Coinbase() helper.Address            { return self.header.Coinbase }
func (self *VMEnv) Time() *big.Int                      { return self.header.Time }
func (self *VMEnv) Difficulty() *big.Int                { return self.header.Difficulty }
func (self *VMEnv) GasLimit() *big.Int                  { return self.header.GasLimit }
func (self *VMEnv) Value() *big.Int          { return self.msg.Value() }
func (self *VMEnv) Db() localEnv.Database    { return self.state }
func (self *VMEnv) Depth() int               { return self.depth }
func (self *VMEnv) SetDepth(i int)           { self.depth = i }
func (self *VMEnv) GetHash(n uint64) helper.Hash {
	return self.getHashFn(n)
}

func (self *VMEnv) AddLog(log *localEnv.Log) {
	self.state.AddLog(log)
}
func (self *VMEnv) CanTransfer(from helper.Address, balance *big.Int) bool {
	return self.state.GetBalance(from).Cmp(balance) >= 0
}

func (self *VMEnv) SnapshotDatabase() int {
	return self.state.Snapshot()
}

func (self *VMEnv) RevertToSnapshot(snapshot int) {
	self.state.RevertToSnapshot(snapshot)
}

func (self *VMEnv) Transfer(from, to localEnv.Account, amount *big.Int) {
	Transfer(from, to, amount)
}

func (self *VMEnv) Call(me localEnv.ExternalLogicRef, addr helper.Address, data []byte, gas, price, value *big.Int) ([]byte, error) {
	return Call(self, me, addr, data, gas, price, value)
}
func (self *VMEnv) CallCode(me localEnv.ExternalLogicRef, addr helper.Address, data []byte, gas, price, value *big.Int) ([]byte, error) {
	return CallCode(self, me, addr, data, gas, price, value)
}

func (self *VMEnv) DelegateCall(me localEnv.ExternalLogicRef, addr helper.Address, data []byte, gas, price *big.Int) ([]byte, error) {
	return DelegateCall(self, me, addr, data, gas, price)
}

func (self *VMEnv) Create(me localEnv.ExternalLogicRef, data []byte, gas, price, value *big.Int) ([]byte, helper.Address, error) {
	return Create(self, me, data, gas, price, value)
}
