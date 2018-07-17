// Package client provides a client for the Siotchain RPC API.
package client

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/helper/rlp"
	"github.com/ethereum/go-ethereum/net/rpc"
	"golang.org/x/net/context"
	"github.com/ethereum/go-ethereum/net/p2p"
	"github.com/ethereum/go-ethereum/internal/siotapi"
)

// Client defines typed wrappers for the Siotchain RPC API.
type Client struct {
	c *rpc.Client
}

// Dial connects a client to the given URL.
func Dial(rawurl string) (*Client, error) {
	c, err := rpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}
	return NewClient(c), nil
}

// NewClient creates a client that uses the given RPC client.
func NewClient(c *rpc.Client) *Client {
	return &Client{c}
}

// Blockchain Access

// BlockByHash returns the given full block.
//
// Note that loading full blocks requires two requests. Use HeaderByHash
// if you don't need all transactions or uncle headers.
func (ec *Client) BlockByHash(ctx context.Context, hash helper.Hash) (*types.Block, error) {
	return ec.getBlock(ctx, "siot_getBlockByHash", hash, true)
}

// BlockByNumber returns a block from the current canonical chain. If number is nil, the
// latest known block is returned.
//
// Note that loading full blocks requires two requests. Use HeaderByNumber
// if you don't need all transactions or uncle headers.
func (ec *Client) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return ec.getBlock(ctx, "siot_getBlockByNumber", toBlockNumArg(number), true)
}

type rpcBlock struct {
	Hash         helper.Hash          `json:"hash"`
	Transactions []*types.Transaction `json:"transactions"`
	UncleHashes  []helper.Hash        `json:"uncles"`
}

func (ec *Client) getBlock(ctx context.Context, method string, args ...interface{}) (*types.Block, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	}
	// Decode header and transactions.
	var head *types.Header
	var body rpcBlock
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	// Quick-verify transaction and uncle lists. This mostly helps with debugging the server.
	if head.UncleHash == types.EmptyUncleHash && len(body.UncleHashes) > 0 {
		return nil, fmt.Errorf("server returned non-empty uncle list but block header indicates no uncles")
	}
	if head.UncleHash != types.EmptyUncleHash && len(body.UncleHashes) == 0 {
		return nil, fmt.Errorf("server returned empty uncle list but block header indicates uncles")
	}
	if head.TxHash == types.EmptyRootHash && len(body.Transactions) > 0 {
		return nil, fmt.Errorf("server returned non-empty transaction list but block header indicates no transactions")
	}
	if head.TxHash != types.EmptyRootHash && len(body.Transactions) == 0 {
		return nil, fmt.Errorf("server returned empty transaction list but block header indicates transactions")
	}
	// Load uncles because they are not included in the block response.
	var uncles []*types.Header
	if len(body.UncleHashes) > 0 {
		uncles = make([]*types.Header, len(body.UncleHashes))
		reqs := make([]rpc.BatchElem, len(body.UncleHashes))
		for i := range reqs {
			reqs[i] = rpc.BatchElem{
				Method: "siot_getUncleByBlockHashAndIndex",
				Args:   []interface{}{body.Hash, fmt.Sprintf("%#x", i)},
				Result: &uncles[i],
			}
		}
		if err := ec.c.BatchCallContext(ctx, reqs); err != nil {
			return nil, err
		}
		for i := range reqs {
			if reqs[i].Error != nil {
				return nil, reqs[i].Error
			}
		}
	}
	return types.NewBlockWithHeader(head).WithBody(body.Transactions, uncles), nil
}

// HeaderByHash returns the block header with the given hash.
func (ec *Client) HeaderByHash(ctx context.Context, hash helper.Hash) (*types.Header, error) {
	var head *types.Header
	err := ec.c.CallContext(ctx, &head, "siot_getBlockByHash", hash, false)
	return head, err
}

// HeaderByNumber returns a block header from the current canonical chain. If number is
// nil, the latest known header is returned.
func (ec *Client) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	var head *types.Header
	err := ec.c.CallContext(ctx, &head, "siot_getBlockByNumber", toBlockNumArg(number), false)
	return head, err
}

// TransactionByHash returns the transaction with the given hash.
func (ec *Client) TransactionByHash(ctx context.Context, hash helper.Hash) (*types.Transaction, error) {
	var tx *types.Transaction
	err := ec.c.CallContext(ctx, &tx, "siot_getTransactionByHash", hash)
	if err == nil {
		if _, r, _ := tx.RawSignatureValues(); r == nil {
			return nil, fmt.Errorf("server returned transaction without signature")
		}
	}
	return tx, err
}

// TransactionCount returns the total number of transactions in the given block.
func (ec *Client) TransactionCount(ctx context.Context, blockHash helper.Hash) (uint, error) {
	var num rpc.HexNumber
	err := ec.c.CallContext(ctx, &num, "siot_getBlockTransactionCountByHash", blockHash)
	return num.Uint(), err
}

// TransactionInBlock returns a single transaction at index in the given block.
func (ec *Client) TransactionInBlock(ctx context.Context, blockHash helper.Hash, index uint) (*types.Transaction, error) {
	var tx *types.Transaction
	err := ec.c.CallContext(ctx, &tx, "siot_getTransactionByBlockHashAndIndex", blockHash, index)
	if err == nil {
		var signer types.Signer = types.HomesteadSigner{}
		if tx.Protected() {
			signer = types.NewEIP155Signer(tx.ChainId())
		}
		if _, r, _ := types.SignatureValues(signer, tx); r == nil {
			return nil, fmt.Errorf("server returned transaction without signature")
		}
	}
	return tx, err
}

// TransactionReceipt returns the receipt of a transaction by transaction hash.
// Note that the receipt is not available for pending transactions.
func (ec *Client) TransactionReceipt(ctx context.Context, txHash helper.Hash) (*types.Receipt, error) {
	var r *types.Receipt
	err := ec.c.CallContext(ctx, &r, "siot_getTransactionReceipt", txHash)
	if err == nil && r != nil && len(r.PostState) == 0 {
		return nil, fmt.Errorf("server returned receipt without post state")
	}
	return r, err
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	return fmt.Sprintf("%#x", number)
}

type rpcProgress struct {
	StartingBlock rpc.HexNumber
	CurrentBlock  rpc.HexNumber
	HighestBlock  rpc.HexNumber
	PulledStates  rpc.HexNumber
	KnownStates   rpc.HexNumber
}

// SyncProgress retrieves the current progress of the sync algorithm. If there's
// no sync currently running, it returns nil.
func (ec *Client) SyncProgress(ctx context.Context) (*siotchain.SyncProgress, error) {
	var raw json.RawMessage
	if err := ec.c.CallContext(ctx, &raw, "siot_syncing"); err != nil {
		return nil, err
	}
	// Handle the possible response types
	var syncing bool
	if err := json.Unmarshal(raw, &syncing); err == nil {
		return nil, nil // Not syncing (always false)
	}
	var progress *rpcProgress
	if err := json.Unmarshal(raw, &progress); err != nil {
		return nil, err
	}
	return &siotchain.SyncProgress{
		StartingBlock: progress.StartingBlock.Uint64(),
		CurrentBlock:  progress.CurrentBlock.Uint64(),
		HighestBlock:  progress.HighestBlock.Uint64(),
		PulledStates:  progress.PulledStates.Uint64(),
		KnownStates:   progress.KnownStates.Uint64(),
	}, nil
}

// SubscribeNewHead subscribes to notifications about the current blockchain head
// on the given channel.
func (ec *Client) SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (siotchain.Subscription, error) {
	return ec.c.SiotSubscribe(ctx, ch, "newHeads", map[string]struct{}{})
}

// State Access
// TODO WEI: add client api to handle rpc call
func (ec *Client) NodeInfoAt(ctx context.Context) (*p2p.NodeInfo, error) {
	var result p2p.NodeInfo
	err := ec.c.CallContext(ctx, &result, "manage_nodeInfo")
	return (*p2p.NodeInfo)(&result), err
}

func (ec *Client) ListAccountsAt(ctx context.Context) ([]rpc.HexBytes, error) {
	var result []rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "user_listAccounts")
	return result, err
}

func (ec *Client) NewAccount(ctx context.Context, password string) (rpc.HexBytes, error) {
	var result rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "user_newAccount", password)
	return result, err
}

func (ec *Client) UnlockAccount(ctx context.Context, account helper.Address, password string) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "user_unlockAccount", account, password)
	return result, err
}

func (ec *Client) LockAccount(ctx context.Context) (helper.Address, error) {
	var result helper.Address
	err := ec.c.CallContext(ctx, &result, "user_lockAccount")
	return result, err
}
// BalanceAt returns the wei balance of the given account.
// The block number can be nil, in which case the balance is taken from the latest known block.
func (ec *Client) BalanceAt(ctx context.Context, account helper.Address, blockNumber *big.Int) (*big.Int, error) {
	var result rpc.HexNumber
	err := ec.c.CallContext(ctx, &result, "siot_getBalance", account, toBlockNumArg(blockNumber))
	return (*big.Int)(&result), err
}

func (ec *Client) SendAsset(ctx context.Context, sender helper.Address, receiver helper.Address, value *big.Int) (rpc.HexBytes, error) {
	var result rpc.HexBytes
	value.Mul(value, big.NewInt(1000000000000))
	args := siotapi.SendTxArgs{From: sender, To: &receiver, Value: rpc.NewHexNumber(value), Data: ""}
	err := ec.c.CallContext(ctx, &result, "siot_sendTransaction", args)
	return result, err
}

func (ec *Client) AddPeer(ctx context.Context, url string) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "manage_addPeer", url)
	return result, err
}

func (ec *Client) GetPeers(ctx context.Context) ([]*p2p.PeerInfo, error) {
	var result []*p2p.PeerInfo
	err := ec.c.CallContext(ctx, &result, "manage_peers")
	return result, err
}

func (ec *Client) SetMiner(ctx context.Context, account helper.Address) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "miner_setMiner", account)
	return result, err
}

func (ec *Client) StartMining(ctx context.Context) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "miner_start")
	return result, err
}

func (ec *Client) StopMining(ctx context.Context) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "miner_stop")
	return result, err
}



// StorageAt returns the value of key in the externalLogic storage of the given account.
// The block number can be nil, in which case the value is taken from the latest known block.
func (ec *Client) StorageAt(ctx context.Context, account helper.Address, key helper.Hash, blockNumber *big.Int) ([]byte, error) {
	var result rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "siot_getStorageAt", account, key, toBlockNumArg(blockNumber))
	return result, err
}

// CodeAt returns the externalLogic code of the given account.
// The block number can be nil, in which case the code is taken from the latest known block.
func (ec *Client) CodeAt(ctx context.Context, account helper.Address, blockNumber *big.Int) ([]byte, error) {
	var result rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "siot_getCode", account, toBlockNumArg(blockNumber))
	return result, err
}

// NonceAt returns the account nonce of the given account.
// The block number can be nil, in which case the nonce is taken from the latest known block.
func (ec *Client) NonceAt(ctx context.Context, account helper.Address, blockNumber *big.Int) (uint64, error) {
	var result rpc.HexNumber
	err := ec.c.CallContext(ctx, &result, "siot_getTransactionCount", account, toBlockNumArg(blockNumber))
	return result.Uint64(), err
}

// Filters

// FilterLogs executes a filter query.
func (ec *Client) FilterLogs(ctx context.Context, q siotchain.FilterQuery) ([]vm.Log, error) {
	var result []vm.Log
	err := ec.c.CallContext(ctx, &result, "siot_getLogs", toFilterArg(q))
	return result, err
}

// SubscribeFilterLogs subscribes to the results of a streaming filter query.
func (ec *Client) SubscribeFilterLogs(ctx context.Context, q siotchain.FilterQuery, ch chan<- vm.Log) (siotchain.Subscription, error) {
	return ec.c.SiotSubscribe(ctx, ch, "logs", toFilterArg(q))
}

func toFilterArg(q siotchain.FilterQuery) interface{} {
	arg := map[string]interface{}{
		"fromBlock": toBlockNumArg(q.FromBlock),
		"toBlock":   toBlockNumArg(q.ToBlock),
		"addresses": q.Addresses,
		"topics":    q.Topics,
	}
	if q.FromBlock == nil {
		arg["fromBlock"] = "0x0"
	}
	return arg
}

// Pending State

// PendingBalanceAt returns the wei balance of the given account in the pending state.
func (ec *Client) PendingBalanceAt(ctx context.Context, account helper.Address) (*big.Int, error) {
	var result rpc.HexNumber
	err := ec.c.CallContext(ctx, &result, "siot_getBalance", account, "pending")
	return (*big.Int)(&result), err
}

// PendingStorageAt returns the value of key in the externalLogic storage of the given account in the pending state.
func (ec *Client) PendingStorageAt(ctx context.Context, account helper.Address, key helper.Hash) ([]byte, error) {
	var result rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "siot_getStorageAt", account, key, "pending")
	return result, err
}

// PendingCodeAt returns the externalLogic code of the given account in the pending state.
func (ec *Client) PendingCodeAt(ctx context.Context, account helper.Address) ([]byte, error) {
	var result rpc.HexBytes
	err := ec.c.CallContext(ctx, &result, "siot_getCode", account, "pending")
	return result, err
}

// PendingNonceAt returns the account nonce of the given account in the pending state.
// This is the nonce that should be used for the next transaction.
func (ec *Client) PendingNonceAt(ctx context.Context, account helper.Address) (uint64, error) {
	var result rpc.HexNumber
	err := ec.c.CallContext(ctx, &result, "siot_getTransactionCount", account, "pending")
	return result.Uint64(), err
}

// PendingTransactionCount returns the total number of transactions in the pending state.
func (ec *Client) PendingTransactionCount(ctx context.Context) (uint, error) {
	var num rpc.HexNumber
	err := ec.c.CallContext(ctx, &num, "siot_getBlockTransactionCountByNumber", "pending")
	return num.Uint(), err
}

// TODO: SubscribePendingTransactions (needs server side)

// ExternalLogic Calling

// CallExternalLogic executes a message call transaction, which is directly executed in the VM
// of the node, but never mined into the blockchain.
//
// blockNumber selects the block height at which the call runs. It can be nil, in which
// case the code is taken from the latest known block. Note that state from very old
// blocks might not be available.
func (ec *Client) CallExternalLogic(ctx context.Context, msg siotchain.CallMsg, blockNumber *big.Int) ([]byte, error) {
	var hex string
	err := ec.c.CallContext(ctx, &hex, "siot_call", toCallArg(msg), toBlockNumArg(blockNumber))
	if err != nil {
		return nil, err
	}
	return helper.FromHex(hex), nil
}

// PendingCallExternalLogic executes a message call transaction using the EVM.
// The state seen by the externalLogic call is the pending state.
func (ec *Client) PendingCallExternalLogic(ctx context.Context, msg siotchain.CallMsg) ([]byte, error) {
	var hex string
	err := ec.c.CallContext(ctx, &hex, "siot_call", toCallArg(msg), "pending")
	if err != nil {
		return nil, err
	}
	return helper.FromHex(hex), nil
}

// SuggestGasPrice retrieves the currently suggested gas price to allow a timely
// execution of a transaction.
func (ec *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	var hex rpc.HexNumber
	if err := ec.c.CallContext(ctx, &hex, "siot_gasPrice"); err != nil {
		return nil, err
	}
	return (*big.Int)(&hex), nil
}

// EstimateGas tries to estimate the gas needed to execute a specific transaction based on
// the current pending state of the backend blockchain. There is no guarantee that this is
// the true gas limit requirement as other transactions may be added or removed by miners,
// but it should provide a basis for setting a reasonable default.
func (ec *Client) EstimateGas(ctx context.Context, msg siotchain.CallMsg) (*big.Int, error) {
	var hex rpc.HexNumber
	err := ec.c.CallContext(ctx, &hex, "siot_estimateGas", toCallArg(msg))
	if err != nil {
		return nil, err
	}
	return (*big.Int)(&hex), nil
}

// SendTransaction injects a signed transaction into the pending pool for execution.
//
// If the transaction was a externalLogic creation use the TransactionReceipt method to get the
// externalLogic address after the transaction has been mined.
func (ec *Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	return ec.c.CallContext(ctx, nil, "siot_sendRawTransaction", helper.ToHex(data))
}

func toCallArg(msg siotchain.CallMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = fmt.Sprintf("%#x", msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = fmt.Sprintf("%#x", msg.Value)
	}
	if msg.Gas != nil {
		arg["gas"] = fmt.Sprintf("%#x", msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = fmt.Sprintf("%#x", msg.GasPrice)
	}
	return arg
}
