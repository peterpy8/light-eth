// Package siotapi implements the general Siotchain API functions.
package siotapi

import (
	"math/big"

	"github.com/siotchain/siot/wallet"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/core"
	"github.com/siotchain/siot/core/types"
	"github.com/siotchain/siot/core/vm"
	"github.com/siotchain/siot/siot/downloader"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/params"
	"github.com/siotchain/siot/net/rpc"
	"golang.org/x/net/context"
)

// Backend interface provides the helper API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// general Siotchain API
	Downloader() *downloader.Downloader
	ProtocolVersion() int
	SuggestPrice(ctx context.Context) (*big.Int, error)
	ChainDb() database.Database
	EventMux() *subscribe.TypeMux
	AccountManager() *wallet.Manager
	// BlockChain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (State, *types.Header, error)
	GetBlock(ctx context.Context, blockHash helper.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash helper.Hash) (types.Receipts, error)
	GetTd(blockHash helper.Hash) *big.Int
	GetLocalEnv(ctx context.Context, msg core.Message, state State, header *types.Header) (vm.Environment, func() error, error)
	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	RemoveTx(txHash helper.Hash)
	GetPoolTransactions() types.Transactions
	GetPoolTransaction(txHash helper.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr helper.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent() (map[helper.Address]types.Transactions, map[helper.Address]types.Transactions)

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
}

type State interface {
	GetBalance(ctx context.Context, addr helper.Address) (*big.Int, error)
	GetCode(ctx context.Context, addr helper.Address) ([]byte, error)
	GetState(ctx context.Context, a helper.Address, b helper.Hash) (helper.Hash, error)
	GetNonce(ctx context.Context, addr helper.Address) (uint64, error)
}

func GetAPIs(apiBackend Backend) []rpc.API {
	var compiler []rpc.API
	all := []rpc.API{
		{
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicSiotchainAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
			Public:    true,
		}, {
			Namespace: "user",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend),
			Public:    false,
		},
	}
	return append(compiler, all...)
}
