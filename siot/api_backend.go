package siot

import (
	"math/big"

	"github.com/ethereum/go-ethereum/wallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/siot/downloader"
	"github.com/ethereum/go-ethereum/siot/gasprice"
	"github.com/ethereum/go-ethereum/siotdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/siotapi"
	"github.com/ethereum/go-ethereum/params"
	rpc "github.com/ethereum/go-ethereum/rpc"
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

func (b *SiotApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.siot.blockchain.GetBlockByHash(blockHash), nil
}

func (b *SiotApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return core.GetBlockReceipts(b.siot.chainDb, blockHash, core.GetBlockNumber(b.siot.chainDb, blockHash)), nil
}

func (b *SiotApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.siot.blockchain.GetTdByHash(blockHash)
}

func (b *SiotApiBackend) GetVMEnv(ctx context.Context, msg core.Message, state siotapi.State, header *types.Header) (vm.Environment, func() error, error) {
	statedb := state.(SiotApiState).state
	from := statedb.GetOrNewStateObject(msg.From())
	from.SetBalance(common.MaxBig)
	vmError := func() error { return nil }
	return core.NewEnv(statedb, b.siot.chainConfig, b.siot.blockchain, msg, header), vmError, nil
}

func (b *SiotApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	b.siot.txPool.SetLocal(signedTx)
	return b.siot.txPool.Add(signedTx)
}

func (b *SiotApiBackend) RemoveTx(txHash common.Hash) {
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

func (b *SiotApiBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.Get(hash)
}

func (b *SiotApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.State().GetNonce(addr), nil
}

func (b *SiotApiBackend) Stats() (pending int, queued int) {
	b.siot.txMu.Lock()
	defer b.siot.txMu.Unlock()

	return b.siot.txPool.Stats()
}

func (b *SiotApiBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
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

func (b *SiotApiBackend) ChainDb() siotdb.Database {
	return b.siot.ChainDb()
}

func (b *SiotApiBackend) EventMux() *event.TypeMux {
	return b.siot.EventMux()
}

func (b *SiotApiBackend) AccountManager() *wallet.Manager {
	return b.siot.AccountManager()
}

type SiotApiState struct {
	state *state.StateDB
}

func (s SiotApiState) GetBalance(ctx context.Context, addr common.Address) (*big.Int, error) {
	return s.state.GetBalance(addr), nil
}

func (s SiotApiState) GetCode(ctx context.Context, addr common.Address) ([]byte, error) {
	return s.state.GetCode(addr), nil
}

func (s SiotApiState) GetState(ctx context.Context, a common.Address, b common.Hash) (common.Hash, error) {
	return s.state.GetState(a, b), nil
}

func (s SiotApiState) GetNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return s.state.GetNonce(addr), nil
}
