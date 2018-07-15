package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// GetHashFn returns a function for which the VM env can query block hashes through
// up to the limit defined by the Yellow Paper and uses the given block chain
// to query for information.
func GetHashFn(ref common.Hash, chain *BlockChain) func(n uint64) common.Hash {
	return func(n uint64) common.Hash {
		for block := chain.GetBlockByHash(ref); block != nil; block = chain.GetBlock(block.ParentHash(), block.NumberU64()-1) {
			if block.NumberU64() == n {
				return block.Hash()
			}
		}

		return common.Hash{}
	}
}

type VMEnv struct {
	chainConfig *params.ChainConfig // Chain configuration
	state       *state.StateDB      // State to use for executing
	depth       int                 // Current execution depth
	msg         Message             // Message appliod

	header    *types.Header            // Header information
	chain     *BlockChain              // Blockchain handle
	getHashFn func(uint64) common.Hash // getHashFn callback is used to retrieve block hashes
}

func NewEnv(state *state.StateDB, chainConfig *params.ChainConfig, chain *BlockChain, msg Message, header *types.Header) *VMEnv {
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

func (self *VMEnv) ChainConfig() *params.ChainConfig { return self.chainConfig }
func (self *VMEnv) Origin() common.Address           { return self.msg.From() }
func (self *VMEnv) BlockNumber() *big.Int            { return self.header.Number }
func (self *VMEnv) Coinbase() common.Address         { return self.header.Coinbase }
func (self *VMEnv) Time() *big.Int                   { return self.header.Time }
func (self *VMEnv) Difficulty() *big.Int             { return self.header.Difficulty }
func (self *VMEnv) GasLimit() *big.Int               { return self.header.GasLimit }
func (self *VMEnv) Value() *big.Int                  { return self.msg.Value() }
func (self *VMEnv) Db() vm.Database                  { return self.state }
func (self *VMEnv) Depth() int                       { return self.depth }
func (self *VMEnv) SetDepth(i int)                   { self.depth = i }
func (self *VMEnv) GetHash(n uint64) common.Hash {
	return self.getHashFn(n)
}

func (self *VMEnv) AddLog(log *vm.Log) {
	self.state.AddLog(log)
}
func (self *VMEnv) CanTransfer(from common.Address, balance *big.Int) bool {
	return self.state.GetBalance(from).Cmp(balance) >= 0
}

func (self *VMEnv) SnapshotDatabase() int {
	return self.state.Snapshot()
}

func (self *VMEnv) RevertToSnapshot(snapshot int) {
	self.state.RevertToSnapshot(snapshot)
}

func (self *VMEnv) Transfer(from, to vm.Account, amount *big.Int) {
	Transfer(from, to, amount)
}

func (self *VMEnv) Call(me vm.ContractRef, addr common.Address, data []byte, gas, price, value *big.Int) ([]byte, error) {
	return Call(self, me, addr, data, gas, price, value)
}
func (self *VMEnv) CallCode(me vm.ContractRef, addr common.Address, data []byte, gas, price, value *big.Int) ([]byte, error) {
	return CallCode(self, me, addr, data, gas, price, value)
}

func (self *VMEnv) DelegateCall(me vm.ContractRef, addr common.Address, data []byte, gas, price *big.Int) ([]byte, error) {
	return DelegateCall(self, me, addr, data, gas, price)
}

func (self *VMEnv) Create(me vm.ContractRef, data []byte, gas, price, value *big.Int) ([]byte, common.Address, error) {
	return Create(self, me, data, gas, price, value)
}
