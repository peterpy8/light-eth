// Package siotchain defines interfaces for interacting with Siotchain.
package siotchain

import (
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"golang.org/x/net/context"
)

// TODO: move subscription to package subscribe

// Subscription represents an subscribe subscription where events are
// delivered on a data channel.
type Subscription interface {
	// Unsubscribe cancels the sending of events to the data channel
	// and closes the error channel.
	Unsubscribe()
	// Err returns the subscription error channel. The error channel receives
	// a value if there is an issue with the subscription (e.g. the network connection
	// delivering the events has been closed). Only one value will ever be sent.
	// The error channel is closed by Unsubscribe.
	Err() <-chan error
}

// ChainReader provides access to the blockchain. The methods in this interface access raw
// data from either the canonical chain (when requesting by block number) or any
// blockchain fork that was previously downloaded and processed by the node. The block
// number argument can be nil to select the latest canonical block. Reading block headers
// should be preferred over full blocks whenever possible.
type ChainReader interface {
	BlockByHash(ctx context.Context, hash helper.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	HeaderByHash(ctx context.Context, hash helper.Hash) (*types.Header, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	TransactionCount(ctx context.Context, blockHash helper.Hash) (uint, error)
	TransactionInBlock(ctx context.Context, blockHash helper.Hash, index uint) (*types.Transaction, error)
	TransactionByHash(ctx context.Context, txHash helper.Hash) (*types.Transaction, error)
	TransactionReceipt(ctx context.Context, txHash helper.Hash) (*types.Receipt, error)
}

// ChainStateReader wraps access to the state trie of the canonical blockchain. Note that
// implementations of the interface may be unable to return state values for old blocks.
// In many cases, using CallExternalLogic can be preferable to reading raw externalLogic storage.
type ChainStateReader interface {
	BalanceAt(ctx context.Context, account helper.Address, blockNumber *big.Int) (*big.Int, error)
	StorageAt(ctx context.Context, account helper.Address, key helper.Hash, blockNumber *big.Int) ([]byte, error)
	CodeAt(ctx context.Context, account helper.Address, blockNumber *big.Int) ([]byte, error)
	NonceAt(ctx context.Context, account helper.Address, blockNumber *big.Int) (uint64, error)
}

// SyncProgress gives progress indications when the node is synchronising with
// the Siotchain network.
type SyncProgress struct {
	StartingBlock uint64 // Block number where sync began
	CurrentBlock  uint64 // Current block number where sync is at
	HighestBlock  uint64 // Highest alleged block number in the chain
	PulledStates  uint64 // Number of state trie entries already downloaded
	KnownStates   uint64 // Total number os state trie entries known about
}

// ChainSyncReader wraps access to the node's current sync status. If there's no
// sync currently running, it returns nil.
type ChainSyncReader interface {
	SyncProgress(ctx context.Context) (*SyncProgress, error)
}

// A ChainHeadEventer returns notifications whenever the canonical head block is updated.
type ChainHeadEventer interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (Subscription, error)
}

// CallMsg contains parameters for externalLogic calls.
type CallMsg struct {
	From     helper.Address  // the sender of the 'transaction'
	To       *helper.Address // the destination externalLogic (nil for externalLogic creation)
	Gas      *big.Int        // if nil, the call executes with near-infinite gas
	GasPrice *big.Int        // wei <-> gas exchange ratio
	Value    *big.Int        // amount of wei sent along with the call
	Data     []byte          // input data, usually an ABI-encoded externalLogic method invocation
}

// A ExternalLogicCaller provides externalLogic calls, essentially transactions that are executed by
// the EVM but not mined into the blockchain. ExternalLogicCall is a low-level method to
// execute such calls. For applications which are structured around specific externalLogics,
// the abigen tool provides a nicer, properly typed way to perform calls.
type ExternalLogicCaller interface {
	CallExternalLogic(ctx context.Context, call CallMsg, blockNumber *big.Int) ([]byte, error)
}

// FilterQuery contains options for contact log filtering.
type FilterQuery struct {
	FromBlock *big.Int         // beginning of the queried range, nil means genesis block
	ToBlock   *big.Int         // end of the range, nil means latest block
	Addresses []helper.Address // restricts matches to events created by specific externalLogics

	// The Topic list restricts matches to particular subscribe topics. Each subscribe has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}}, {B}}        matches topic A in first position, B in second position
	// {{A, B}}, {C, D}}  matches topic (A OR B) in first position, (C OR D) in second position
	Topics [][]helper.Hash
}

// LogFilterer provides access to externalLogic log events using a one-off query or continuous
// subscribe subscription.
type LogFilterer interface {
	FilterLogs(ctx context.Context, q FilterQuery) ([]localEnv.Log, error)
	SubscribeFilterLogs(ctx context.Context, q FilterQuery, ch chan<- localEnv.Log) (Subscription, error)
}

// TransactionSender wraps transaction sending. The SendTransaction method injects a
// signed transaction into the pending transaction pool for execution. If the transaction
// was a externalLogic creation, the TransactionReceipt method can be used to retrieve the
// externalLogic address after the transaction has been mined.
//
// The transaction must be signed and have a valid nonce to be included. Consumers of the
// API can use package wallet to maintain local private keys and need can retrieve the
// next available nonce using PendingNonceAt.
type TransactionSender interface {
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// GasPricer wraps the gas price oracle, which monitors the blockchain to determine the
// optimal gas price given current fee market conditions.
type GasPricer interface {
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// A PendingStateReader provides access to the pending state, which is the result of all
// known executable transactions which have not yet been included in the blockchain. It is
// commonly used to display the result of ’unconfirmed’ actions (e.g. wallet value
// transfers) initiated by the user. The PendingNonceAt operation is a good way to
// retrieve the next available transaction nonce for a specific account.
type PendingStateReader interface {
	PendingBalanceAt(ctx context.Context, account helper.Address) (*big.Int, error)
	PendingStorageAt(ctx context.Context, account helper.Address, key helper.Hash) ([]byte, error)
	PendingCodeAt(ctx context.Context, account helper.Address) ([]byte, error)
	PendingNonceAt(ctx context.Context, account helper.Address) (uint64, error)
	PendingTransactionCount(ctx context.Context) (uint, error)
}

// PendingExternalLogicCaller can be used to perform calls against the pending state.
type PendingExternalLogicCaller interface {
	PendingCallExternalLogic(ctx context.Context, call CallMsg) ([]byte, error)
}

// GasEstimator wraps EstimateGas, which tries to estimate the gas needed to execute a
// specific transaction based on the pending state. There is no guarantee that this is the
// true gas limit requirement as other transactions may be added or removed by miners, but
// it should provide a basis for setting a reasonable default.
type GasEstimator interface {
	EstimateGas(ctx context.Context, call CallMsg) (usedGas *big.Int, err error)
}

// A PendingStateEventer provides access to real time notifications about changes to the
// pending state.
type PendingStateEventer interface {
	SubscribePendingTransactions(ctx context.Context, ch chan<- *types.Transaction) (Subscription, error)
}
