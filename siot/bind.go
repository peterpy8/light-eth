package siot

import (
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/siotapi"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
)

// ExternalLogicBackend implements bind.ExternalLogicBackend with direct calls to Siotchain
// internals to support operating on externalLogics within subprotocols like siot and
// swarm.
//
// Internally this backend uses the already exposed API endpoints of the Siotchain
// object. These should be rewritten to internal Go method calls when the Go API
// is refactored to support a clean library use.
type ExternalLogicBackend struct {
	eapi  *siotapi.PublicSiotchainAPI       // Wrapper around the Siotchain object to access metadata
	bcapi *siotapi.PublicBlockChainAPI      // Wrapper around the blockchain to access chain data
	txapi *siotapi.PublicTransactionPoolAPI // Wrapper around the transaction pool to access transaction data
}

// NewExternalLogicBackend creates a new native externalLogic backend using an existing
// Siotchain object.
func NewExternalLogicBackend(apiBackend siotapi.Backend) *ExternalLogicBackend {
	return &ExternalLogicBackend{
		eapi:  siotapi.NewPublicSiotchainAPI(apiBackend),
		bcapi: siotapi.NewPublicBlockChainAPI(apiBackend),
		txapi: siotapi.NewPublicTransactionPoolAPI(apiBackend),
	}
}

// CodeAt retrieves any code associated with the externalLogic from the local API.
func (b *ExternalLogicBackend) CodeAt(ctx context.Context, externalLogic common.Address, blockNum *big.Int) ([]byte, error) {
	out, err := b.bcapi.GetCode(ctx, externalLogic, toBlockNumber(blockNum))
	return common.FromHex(out), err
}

// CodeAt retrieves any code associated with the externalLogic from the local API.
func (b *ExternalLogicBackend) PendingCodeAt(ctx context.Context, externalLogic common.Address) ([]byte, error) {
	out, err := b.bcapi.GetCode(ctx, externalLogic, rpc.PendingBlockNumber)
	return common.FromHex(out), err
}

// ExternalLogicCall implements bind.ExternalLogicCaller executing an Siotchain externalLogic
// call with the specified data as the input. The pending flag requests execution
// against the pending block, not the stable head of the chain.
func (b *ExternalLogicBackend) CallExternalLogic(ctx context.Context, msg siotchain.CallMsg, blockNum *big.Int) ([]byte, error) {
	out, err := b.bcapi.Call(ctx, toCallArgs(msg), toBlockNumber(blockNum))
	return common.FromHex(out), err
}

// ExternalLogicCall implements bind.ExternalLogicCaller executing an Siotchain externalLogic
// call with the specified data as the input. The pending flag requests execution
// against the pending block, not the stable head of the chain.
func (b *ExternalLogicBackend) PendingCallExternalLogic(ctx context.Context, msg siotchain.CallMsg) ([]byte, error) {
	out, err := b.bcapi.Call(ctx, toCallArgs(msg), rpc.PendingBlockNumber)
	return common.FromHex(out), err
}

func toCallArgs(msg siotchain.CallMsg) siotapi.CallArgs {
	args := siotapi.CallArgs{
		To:   msg.To,
		From: msg.From,
		Data: common.ToHex(msg.Data),
	}
	if msg.Gas != nil {
		args.Gas = *rpc.NewHexNumber(msg.Gas)
	}
	if msg.GasPrice != nil {
		args.GasPrice = *rpc.NewHexNumber(msg.GasPrice)
	}
	if msg.Value != nil {
		args.Value = *rpc.NewHexNumber(msg.Value)
	}
	return args
}

func toBlockNumber(num *big.Int) rpc.BlockNumber {
	if num == nil {
		return rpc.LatestBlockNumber
	}
	return rpc.BlockNumber(num.Int64())
}

// PendingAccountNonce implements bind.ExternalLogicTransactor retrieving the current
// pending nonce associated with an account.
func (b *ExternalLogicBackend) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	out, err := b.txapi.GetTransactionCount(ctx, account, rpc.PendingBlockNumber)
	return out.Uint64(), err
}

// SuggestGasPrice implements bind.ExternalLogicTransactor retrieving the currently
// suggested gas price to allow a timely execution of a transaction.
func (b *ExternalLogicBackend) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return b.eapi.GasPrice(ctx)
}

// EstimateGasLimit implements bind.ExternalLogicTransactor triing to estimate the gas
// needed to execute a specific transaction based on the current pending state of
// the backend blockchain. There is no guarantee that this is the true gas limit
// requirement as other transactions may be added or removed by miners, but it
// should provide a basis for setting a reasonable default.
func (b *ExternalLogicBackend) EstimateGas(ctx context.Context, msg siotchain.CallMsg) (*big.Int, error) {
	out, err := b.bcapi.EstimateGas(ctx, toCallArgs(msg))
	return out.BigInt(), err
}

// SendTransaction implements bind.ExternalLogicTransactor injects the transaction
// into the pending pool for execution.
func (b *ExternalLogicBackend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	raw, _ := rlp.EncodeToBytes(tx)
	_, err := b.txapi.SendRawTransaction(ctx, common.ToHex(raw))
	return err
}
