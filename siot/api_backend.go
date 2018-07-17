package siot

import (
	"math/big"

	"github.com/siotchain/siot/wallet"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/core"
	"github.com/siotchain/siot/core/state"
	"github.com/siotchain/siot/core/types"
	"github.com/siotchain/siot/core/vm"
	"github.com/siotchain/siot/siot/downloader"
	"github.com/siotchain/siot/siot/gasprice"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/internal/siotapi"
	"github.com/siotchain/siot/params"
	"github.com/siotchain/siot/net/rpc"
	"golang.org/x/net/context"
)

// SiotApiBackend implements siotapi.Backend for full nodes
type SiotApiBackend struct {
	siot *Siotchain
	gpo  *gasprice.GasPriceOracle
}

func (b *SiotApiBackend) ChainConfig() *params.ChainConfig {
	return b.siot.chainConfig
}

func (b *SiotApiBackend) CurrentBlock() *types.Block {
	return b.siot.blockchain.CurrentBlock()
}

func (b *SiotApiBackend) SetHead(number uint64) {
	b.siot.blockchain.SetHead(number)
}

func (b *SiotApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, _ := b.siot.miner.Pending()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.siot.blockchain.CurrentBlock().Header(), nil
	}
	return b.siot.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *SiotApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, _ := b.siot.miner.Pending()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.siot.blockchain.CurrentBlock(), nil
	}
	return b.siot.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *SiotApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (siotapi.State, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.siot.miner.Pending()
		return SiotApiState{state}, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.siot.BlockChain().StateAt(header.Root)
	return SiotApiState{stateDb}, header, err
}

func (b *SiotApiBackend) GetBlock(ctx context.Context, blockHash helper.Hash) (*types.Block, error) {
	return b.siot.blockchain.GetBlockByHash(blockHash), nil
}

func (b *SiotApiBackend) GetReceipts(ctx context.Context, blockHash helper.Hash) (types.Receipts, error) {
	return core.GetBlockReceipts(b.siot.chainDb, blockHash, core.GetBlockNumber(b.siot.chainDb, blockHash)), nil
}

func (b *SiotApiBackend) GetTd(blockHash helper.Hash) *big.Int {
	return b.siot.blockchain.GetTdByHash(blockHash)
}

func (b *SiotApiBackend) GetLocalEnv(ctx context.Context, msg core.Message, state siotapi.State, header *types.Header) (vm.Environment, func() error, error) {
	statedb := state.(SiotApiState).state
	from := statedb.GetOrNewStateObject(msg.From())
	from.SetBalance(helper.MaxBig)
	vmError := func() error { return nil }
	return core.NewEnv(statedb, b.siot.chainConfig, b.siot.blockchain, msg, header), vmError, nil
}

func (b *SiotApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	b.siot.txPool.SetLocal(signedTx)
	return b.siot.txPool.Add(signedTx)
}

func (b *SiotApiBackend) RemoveTx(txHash helper.Hash) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	b.siot.txPool.Remove(txHash)
}

func (b *SiotApiBackend) GetPoolTransactions() types.Transactions {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	var txs types.Transactions
	for _, batch := range b.siot.txPool.Pending() {
		txs = append(txs, batch...)
	}
	return txs
}

func (b *SiotApiBackend) GetPoolTransaction(hash helper.Hash) *types.Transaction {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.Get(hash)
}

func (b *SiotApiBackend) GetPoolNonce(ctx context.Context, addr helper.Address) (uint64, error) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.State().GetNonce(addr), nil
}

func (b *SiotApiBackend) Stats() (pending int, queued int) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.Stats()
}

func (b *SiotApiBackend) TxPoolContent() (map[helper.Address]types.Transactions, map[helper.Address]types.Transactions) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.TxPool().Content()
}

func (b *SiotApiBackend) Downloader() *downloader.Downloader {
	return b.siot.Downloader()
}

func (b *SiotApiBackend) ProtocolVersion() int {
	return b.siot.SiotVersion()
}

func (b *SiotApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(), nil
}

func (b *SiotApiBackend) ChainDb() database.Database {
	return b.siot.ChainDb()
}

func (b *SiotApiBackend) EventMux() *subscribe.TypeMux {
	return b.siot.EventMux()
}

func (b *SiotApiBackend) AccountManager() *wallet.Manager {
	return b.siot.AccountManager()
}

type SiotApiState struct {
	state *state.StateDB
}

func (s SiotApiState) GetBalance(ctx context.Context, addr helper.Address) (*big.Int, error) {
	return s.state.GetBalance(addr), nil
}

func (s SiotApiState) GetCode(ctx context.Context, addr helper.Address) ([]byte, error) {
	return s.state.GetCode(addr), nil
}

func (s SiotApiState) GetState(ctx context.Context, a helper.Address, b helper.Hash) (helper.Hash, error) {
	return s.state.GetState(a, b), nil
}

func (s SiotApiState) GetNonce(ctx context.Context, addr helper.Address) (uint64, error) {
	return s.state.GetNonce(addr), nil
}
