package localEnv

import (
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/params"
)

// Environment is an EVM requirement and helper which allows access to outside
// information such as states.
type Environment interface {
	// The current ruleset
	ChainConfig() *params.ChainConfig
	// The state database
	Db() Database
	// Creates a restorable snapshot
	SnapshotDatabase() int
	// Set database to previous snapshot
	RevertToSnapshot(int)
	// Address of the original invoker (first occurrence of the VM invoker)
	Origin() helper.Address
	// The block number this VM is invoked on
	BlockNumber() *big.Int
	// The n'th hash ago from this block number
	GetHash(uint64) helper.Hash
	// The handler's address
	Coinbase() helper.Address
	// The current time (block time)
	Time() *big.Int
	// Difficulty set on the current block
	Difficulty() *big.Int
	// The gas limit of the block
	GasLimit() *big.Int
	// Determines whether it's possible to transact
	CanTransfer(from helper.Address, balance *big.Int) bool
	// Transfers amount from one account to the other
	Transfer(from, to Account, amount *big.Int)
	// Adds a LOG to the state
	AddLog(*Log)
	// Type of the VM
	//Vm() Vm
	// Get the curret calling depth
	Depth() int
	// Set the current calling depth
	SetDepth(i int)
	// Call another externalLogic
	Call(me ExternalLogicRef, addr helper.Address, data []byte, gas, price, value *big.Int) ([]byte, error)
	// Take another's externalLogic code and execute within our own context
	CallCode(me ExternalLogicRef, addr helper.Address, data []byte, gas, price, value *big.Int) ([]byte, error)
	// Same as CallCode except sender and value is propagated from parent to child scope
	DelegateCall(me ExternalLogicRef, addr helper.Address, data []byte, gas, price *big.Int) ([]byte, error)
	// Create a new externalLogic
	Create(me ExternalLogicRef, data []byte, gas, price, value *big.Int) ([]byte, helper.Address, error)
}

// Vm is the basic interface for an implementation of the EVM.
type Vm interface {
	// Run should execute the given externalLogic with the input given in in
	// and return the externalLogic execution return bytes or an error if it
	// failed.
	Run(c *ExternalLogic, in []byte) ([]byte, error)
}

// Database is a EVM database for full state querying.
type Database interface {
	GetAccount(helper.Address) Account
	CreateAccount(helper.Address) Account

	AddBalance(helper.Address, *big.Int)
	GetBalance(helper.Address) *big.Int

	GetNonce(helper.Address) uint64
	SetNonce(helper.Address, uint64)

	GetCodeHash(helper.Address) helper.Hash
	GetCodeSize(helper.Address) int
	GetCode(helper.Address) []byte
	SetCode(helper.Address, []byte)

	AddRefund(*big.Int)
	GetRefund() *big.Int

	GetState(helper.Address, helper.Hash) helper.Hash
	SetState(helper.Address, helper.Hash, helper.Hash)

	Suicide(helper.Address) bool
	HasSuicided(helper.Address) bool

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided wallet.
	Exist(helper.Address) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(helper.Address) bool
}

// Account represents a externalLogic or basic Siotchain account.
type Account interface {
	SubBalance(amount *big.Int)
	AddBalance(amount *big.Int)
	SetBalance(*big.Int)
	SetNonce(uint64)
	Balance() *big.Int
	Address() helper.Address
	ReturnGas(*big.Int, *big.Int)
	SetCode(helper.Hash, []byte)
	ForEachStorage(cb func(key, value helper.Hash) bool)
	Value() *big.Int
}
