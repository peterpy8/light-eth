package miner

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/siotchain/siot/wallet"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore"
	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/configure"
	"github.com/siotchain/siot/validation"
	"gopkg.in/fatih/set.v0"
)

var jsonlogger = logger.NewJsonLogger()

const (
	resultQueueSize  = 10
	miningLogAtDepth = 5
)

// Agent can register themself with the worker
type Agent interface {
	Work() chan<- *Work
	SetReturnCh(chan<- *Result)
	Stop()
	Start()
	GetHashRate() int64
}

type uint64RingBuffer struct {
	ints []uint64 //array of all integers in buffer
	next int      //where is the next insertion? assert 0 <= next < len(ints)
}

// Work is the workers current environment and holds
// all of the current state information
type Work struct {
	config *configure.ChainConfig
	signer types.Signer

	state            *state.StateDB // apply state changes here
	ancestors        *set.Set       // ancestor set (used for checking uncle parent validity)
	family           *set.Set       // family set (used for checking uncle invalidity)
	uncles           *set.Set       // uncle set
	tcount           int            // tx count in cycle
	ownedAccounts    *set.Set
	lowGasTxs        types.Transactions
	failedTxs        types.Transactions
	localMinedBlocks *uint64RingBuffer // the most recent block numbers that were mined locally (used to check block inclusion)

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	createdAt time.Time
}

type Result struct {
	Work  *Work
	Block *types.Block
}

// worker is the main object which takes care of applying messages to the new state
type worker struct {
	config *configure.ChainConfig

	mu sync.Mutex

	// update loop
	mux    *subscribe.TypeMux
	events subscribe.Subscription
	wg     sync.WaitGroup

	agents map[Agent]struct{}
	recv   chan *Result
	pow    validation.PoW

	siot    Backend
	chain   *blockchainCore.BlockChain
	proc    blockchainCore.Validator
	chainDb database.Database

	coinbase helper.Address
	gasPrice *big.Int
	extra    []byte

	currentMu sync.Mutex
	current   *Work

	uncleMu        sync.Mutex
	possibleUncles map[helper.Hash]*types.Block

	txQueueMu sync.Mutex
	txQueue   map[helper.Hash]*types.Transaction

	// atomic status counters
	mining int32
	atWork int32

	fullValidation bool
}

func newWorker(config *configure.ChainConfig, coinbase helper.Address, siot Backend, mux *subscribe.TypeMux) *worker {
	worker := &worker{
		config:         config,
		siot:            siot,
		mux:            mux,
		chainDb:        siot.ChainDb(),
		recv:           make(chan *Result, resultQueueSize),
		gasPrice:       new(big.Int),
		chain:          siot.BlockChain(),
		proc:           siot.BlockChain().Validator(),
		possibleUncles: make(map[helper.Hash]*types.Block),
		coinbase:       coinbase,
		txQueue:        make(map[helper.Hash]*types.Transaction),
		agents:         make(map[Agent]struct{}),
		fullValidation: false,
	}
	worker.events = worker.mux.Subscribe(blockchainCore.ChainHeadEvent{}, blockchainCore.ChainSideEvent{}, blockchainCore.TxPreEvent{})
	go worker.update()

	go worker.wait()
	worker.commitNewWork()

	return worker
}

func (self *worker) SetMiner(addr helper.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.coinbase = addr
}

func (self *worker) pending() (*types.Block, *state.StateDB) {
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	if atomic.LoadInt32(&self.mining) == 0 {
		return types.NewBlock(
			self.current.header,
			self.current.txs,
			nil,
			self.current.receipts,
		), self.current.state.Copy()
	}
	return self.current.Block, self.current.state.Copy()
}

func (self *worker) start() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 1)

	// spin up agents
	for agent := range self.agents {
		agent.Start()
	}
}

func (self *worker) stop() {
	self.wg.Wait()

	self.mu.Lock()
	defer self.mu.Unlock()
	if atomic.LoadInt32(&self.mining) == 1 {
		// Stop all agents.
		for agent := range self.agents {
			agent.Stop()
			// Remove CPU agents.
			if _, ok := agent.(*CpuAgent); ok {
				delete(self.agents, agent)
			}
		}
	}

	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.atWork, 0)
}

func (self *worker) register(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.agents[agent] = struct{}{}
	agent.SetReturnCh(self.recv)
}

func (self *worker) unregister(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	delete(self.agents, agent)
	agent.Stop()
}

func (self *worker) update() {
	for event := range self.events.Chan() {
		// A real subscribe arrived, process interesting content
		switch ev := event.Data.(type) {
		case blockchainCore.ChainHeadEvent:
			self.commitNewWork()
		case blockchainCore.ChainSideEvent:
			self.uncleMu.Lock()
			self.possibleUncles[ev.Block.Hash()] = ev.Block
			self.uncleMu.Unlock()
		case blockchainCore.TxPreEvent:
			// Apply transaction to the pending state if we're not mining
			if atomic.LoadInt32(&self.mining) == 0 {
				self.currentMu.Lock()

				acc, _ := types.Sender(self.current.signer, ev.Tx)
				txs := map[helper.Address]types.Transactions{acc: types.Transactions{ev.Tx}}
				txset := types.NewTransactionsByPriceAndNonce(txs)

				self.current.commitTransactions(self.mux, txset, self.gasPrice, self.chain)
				self.currentMu.Unlock()
			}
		}
	}
}

func newLocalMinedBlock(blockNumber uint64, prevMinedBlocks *uint64RingBuffer) (minedBlocks *uint64RingBuffer) {
	if prevMinedBlocks == nil {
		minedBlocks = &uint64RingBuffer{next: 0, ints: make([]uint64, miningLogAtDepth+1)}
	} else {
		minedBlocks = prevMinedBlocks
	}

	minedBlocks.ints[minedBlocks.next] = blockNumber
	minedBlocks.next = (minedBlocks.next + 1) % len(minedBlocks.ints)
	return minedBlocks
}

func (self *worker) wait() {
	for {
		for result := range self.recv {
			atomic.AddInt32(&self.atWork, -1)

			if result == nil {
				continue
			}
			block := result.Block
			work := result.Work

			if self.fullValidation {
				if _, err := self.chain.InsertChain(types.Blocks{block}); err != nil {
					glog.V(logger.Error).Infoln("mining err", err)
					continue
				}
				go self.mux.Post(blockchainCore.NewMinedBlockEvent{Block: block})
			} else {
				work.state.Commit(self.config.IsSiotImpr2(block.Number()))
				parent := self.chain.GetBlock(block.ParentHash(), block.NumberU64()-1)
				if parent == nil {
					glog.V(logger.Error).Infoln("Invalid block found during mining")
					continue
				}

				auxValidator := self.siot.BlockChain().AuxValidator()
				if err := blockchainCore.ValidateHeader(self.config, auxValidator, block.Header(), parent.Header(), true, false); err != nil && err != blockchainCore.BlockFutureErr {
					glog.V(logger.Error).Infoln("Invalid header on mined block:", err)
					continue
				}

				stat, err := self.chain.WriteBlock(block)
				if err != nil {
					glog.V(logger.Error).Infoln("error writing block to chain", err)
					continue
				}

				// update block hash since it is now available and not when the receipt/log of individual transactions were created
				for _, r := range work.receipts {
					for _, l := range r.Logs {
						l.BlockHash = block.Hash()
					}
				}
				for _, log := range work.state.Logs() {
					log.BlockHash = block.Hash()
				}

				// check if canon block and write transactions
				if stat == blockchainCore.CanonStatTy {
					// This puts transactions in a extra db for rpc
					blockchainCore.WriteTransactions(self.chainDb, block)
					// store the receipts
					blockchainCore.WriteReceipts(self.chainDb, work.receipts)
					// Write map map bloom filters
					blockchainCore.WriteMipmapBloom(self.chainDb, block.NumberU64(), work.receipts)
				}

				// broadcast before waiting for validation
				go func(block *types.Block, logs localEnv.Logs, receipts []*types.Receipt) {
					self.mux.Post(blockchainCore.NewMinedBlockEvent{Block: block})
					self.mux.Post(blockchainCore.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})

					if stat == blockchainCore.CanonStatTy {
						self.mux.Post(blockchainCore.ChainHeadEvent{Block: block})
						self.mux.Post(logs)
					}
					if err := blockchainCore.WriteBlockReceipts(self.chainDb, block.Hash(), block.NumberU64(), receipts); err != nil {
						glog.V(logger.Warn).Infoln("error writing block receipts:", err)
					}
				}(block, work.state.Logs(), work.receipts)
			}

			// check staleness and display confirmation
			canonBlock := self.chain.GetBlockByNumber(block.NumberU64())
			if canonBlock != nil && canonBlock.Hash() != block.Hash() {
			} else {
				work.localMinedBlocks = newLocalMinedBlock(block.Number().Uint64(), work.localMinedBlocks)
			}

			self.commitNewWork()
		}
	}
}

// push sends a new work task to currently live miner agents.
func (self *worker) push(work *Work) {
	if atomic.LoadInt32(&self.mining) != 1 {
		return
	}
	for agent := range self.agents {
		atomic.AddInt32(&self.atWork, 1)
		if ch := agent.Work(); ch != nil {
			ch <- work
		}
	}
}

// makeCurrent creates a new environment for the current cycle.
func (self *worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := self.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	work := &Work{
		config:    self.config,
		signer:    types.NewSiotImpr1Signer(self.config.ChainId),
		state:     state,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		header:    header,
		createdAt: time.Now(),
	}

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			work.family.Add(uncle.Hash())
		}
		work.family.Add(ancestor.Hash())
		work.ancestors.Add(ancestor.Hash())
	}
	accounts := self.siot.AccountManager().Accounts()

	// Keep track of transactions which return errors so they can be removed
	work.tcount = 0
	work.ownedAccounts = accountAddressesSet(accounts)
	if self.current != nil {
		work.localMinedBlocks = self.current.localMinedBlocks
	}
	self.current = work
	return nil
}

func (w *worker) setGasPrice(p *big.Int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// calculate the minimal gas price the miner accepts when sorting out transactions.
	const pct = int64(90)
	w.gasPrice = gasprice(p, pct)

	w.mux.Post(blockchainCore.GasPriceChanged{Price: w.gasPrice})
}

func (self *worker) isBlockLocallyMined(current *Work, deepBlockNum uint64) bool {
	//Did this instance mine a block at {deepBlockNum} ?
	var isLocal = false
	for idx, blockNum := range current.localMinedBlocks.ints {
		if deepBlockNum == blockNum {
			isLocal = true
			current.localMinedBlocks.ints[idx] = 0 //prevent showing duplicate logs
			break
		}
	}
	//Short-circuit on false, because the previous and following tests must both be true
	if !isLocal {
		return false
	}

	//Does the block at {deepBlockNum} send earnings to my coinbase?
	var block = self.chain.GetBlockByNumber(deepBlockNum)
	return block != nil && block.Coinbase() == self.coinbase
}

func (self *worker) logLocalMinedBlocks(current, previous *Work) {
	if previous != nil && current.localMinedBlocks != nil {
		nextBlockNum := current.Block.NumberU64()
		for checkBlockNum := previous.Block.NumberU64(); checkBlockNum < nextBlockNum; checkBlockNum++ {
			inspectBlockNum := checkBlockNum - miningLogAtDepth
			if self.isBlockLocallyMined(current, inspectBlockNum) {
			}
		}
	}
}

func (self *worker) commitNewWork() {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	tstart := time.Now()
	parent := self.chain.CurrentBlock()
	tstamp := tstart.Unix()
	if parent.Time().Cmp(new(big.Int).SetInt64(tstamp)) >= 0 {
		tstamp = parent.Time().Int64() + 1
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); tstamp > now+4 {
		wait := time.Duration(tstamp-now) * time.Second
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, helper.Big1),
		Difficulty: blockchainCore.CalcDifficulty(self.config, uint64(tstamp), parent.Time().Uint64(), parent.Number(), parent.Difficulty()),
		GasLimit:   blockchainCore.CalcGasLimit(parent),
		GasUsed:    new(big.Int),
		Coinbase:   self.coinbase,
		Extra:      self.extra,
		Time:       big.NewInt(tstamp),
	}
	// If we are care about hard-fork check whether to override the extra-data or not
	if daoBlock := self.config.DAOForkBlock; daoBlock != nil {
		// Check whether the block is among the fork extra-override range
		limit := new(big.Int).Add(daoBlock, configure.DAOForkExtraRange)
		if header.Number.Cmp(daoBlock) >= 0 && header.Number.Cmp(limit) < 0 {
			// Depending whether we support or oppose the fork, override differently
			if self.config.DAOForkSupport {
				header.Extra = helper.CopyBytes(configure.DAOForkBlockExtra)
			} else if bytes.Compare(header.Extra, configure.DAOForkBlockExtra) == 0 {
				header.Extra = []byte{} // If miner opposes, don't let it use the reserved extra-data
			}
		}
	}
	previous := self.current
	// Could potentially happen if starting to mine in an odd state.
	err := self.makeCurrent(parent, header)
	if err != nil {
		glog.V(logger.Info).Infoln("Could not create new env for mining, retrying on next block.")
		return
	}
	// Create the current work task and check any fork transitions needed
	work := self.current
	if self.config.DAOForkSupport && self.config.DAOForkBlock != nil && self.config.DAOForkBlock.Cmp(header.Number) == 0 {
		blockchainCore.ApplyDAOHardFork(work.state)
	}
	txs := types.NewTransactionsByPriceAndNonce(self.siot.TxPool().Pending())
	work.commitTransactions(self.mux, txs, self.gasPrice, self.chain)

	self.siot.TxPool().RemoveBatch(work.lowGasTxs)
	self.siot.TxPool().RemoveBatch(work.failedTxs)

	// compute uncles for the new block.
	var (
		uncles    []*types.Header
		badUncles []helper.Hash
	)
	for hash, uncle := range self.possibleUncles {
		if len(uncles) == 2 {
			break
		}
		if err := self.commitUncle(work, uncle.Header()); err != nil {
			if glog.V(logger.Ridiculousness) {
				glog.V(logger.Detail).Infof("Bad uncle found and will be removed (%x)\n", hash[:4])
				glog.V(logger.Detail).Infoln(uncle)
			}
			badUncles = append(badUncles, hash)
		} else {
			glog.V(logger.Debug).Infof("commiting %x as uncle\n", hash[:4])
			uncles = append(uncles, uncle.Header())
		}
	}
	for _, hash := range badUncles {
		delete(self.possibleUncles, hash)
	}

	if atomic.LoadInt32(&self.mining) == 1 {
		// commit state root after all state transitions.
		blockchainCore.AccumulateRewards(work.state, header, uncles)
		header.Root = work.state.IntermediateRoot(self.config.IsSiotImpr2(header.Number))
	}

	// create the new block whose nonce will be mined.
	work.Block = types.NewBlock(header, work.txs, uncles, work.receipts)

	// We only care about logging if we're actually mining.
	if atomic.LoadInt32(&self.mining) == 1 {
		self.logLocalMinedBlocks(work, previous)
	}
	self.push(work)
}

func (self *worker) commitUncle(work *Work, uncle *types.Header) error {
	hash := uncle.Hash()
	if work.uncles.Has(hash) {
		return blockchainCore.UncleError("Uncle not unique")
	}
	if !work.ancestors.Has(uncle.ParentHash) {
		return blockchainCore.UncleError(fmt.Sprintf("Uncle's parent unknown (%x)", uncle.ParentHash[0:4]))
	}
	if work.family.Has(hash) {
		return blockchainCore.UncleError(fmt.Sprintf("Uncle already in family (%x)", hash))
	}
	work.uncles.Add(uncle.Hash())
	return nil
}

func (env *Work) commitTransactions(mux *subscribe.TypeMux, txs *types.TransactionsByPriceAndNonce, gasPrice *big.Int, bc *blockchainCore.BlockChain) {
	gp := new(blockchainCore.GasPool).AddGas(env.header.GasLimit)

	var coalescedLogs localEnv.Logs

	for {
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the SiotImpr1 signer regardless of the current hf.
		from, _ := types.Sender(env.signer, tx)
		// Check whether the tx is replay protected. If we're not in the SiotImpr1 hf
		// phase, start ignoring the sender until we do.
		if tx.Protected() && !env.config.IsSiotImpr1(env.header.Number) {
			glog.V(logger.Detail).Infof("Transaction (%x) is replay protected, but we haven't yet hardforked. Transaction will be ignored until we hardfork.\n", tx.Hash())

			txs.Pop()
			continue
		}

		// Ignore any transactions (and wallet subsequently) with low gas limits
		if tx.GasPrice().Cmp(gasPrice) < 0 && !env.ownedAccounts.Has(from) {
			// Pop the current low-priced transaction without shifting in the next from the account
			glog.V(logger.Info).Infof("Transaction (%x) below gas price (tx=%v ask=%v). All sequential txs from this address(%x) will be ignored\n", tx.Hash().Bytes()[:4], helper.CurrencyToString(tx.GasPrice()), helper.CurrencyToString(gasPrice), from[:4])

			env.lowGasTxs = append(env.lowGasTxs, tx)
			txs.Pop()

			continue
		}
		// Start executing the transaction
		env.state.StartRecord(tx.Hash(), helper.Hash{}, env.tcount)

		err, logs := env.commitTransaction(tx, bc, gp)
		switch {
		case blockchainCore.IsGasLimitErr(err):
			// Pop the current out-of-gas transaction without shifting in the next from the account
			glog.V(logger.Detail).Infof("Gas limit reached for (%x) in this block. Continue to try smaller txs\n", from[:4])
			txs.Pop()

		case err != nil:
			// Pop the current failed transaction without shifting in the next from the account
			glog.V(logger.Detail).Infof("Transaction (%x) failed, will be removed: %v\n", tx.Hash().Bytes()[:4], err)
			env.failedTxs = append(env.failedTxs, tx)
			txs.Pop()

		default:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			env.tcount++
			txs.Shift()
		}
	}
	if len(coalescedLogs) > 0 || env.tcount > 0 {
		go func(logs localEnv.Logs, tcount int) {
			if len(logs) > 0 {
				mux.Post(blockchainCore.PendingLogsEvent{Logs: logs})
			}
			if tcount > 0 {
				mux.Post(blockchainCore.PendingStateEvent{})
			}
		}(coalescedLogs, env.tcount)
	}
}

func (env *Work) commitTransaction(tx *types.Transaction, bc *blockchainCore.BlockChain, gp *blockchainCore.GasPool) (error, localEnv.Logs) {
	snap := env.state.Snapshot()

	receipt, logs, _, err := blockchainCore.ApplyTransaction(env.config, bc, gp, env.state, env.header, tx, env.header.GasUsed)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		return err, nil
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)

	return nil, logs
}

// TODO: remove or use
func (self *worker) HashRate() int64 {
	return 0
}

// gasprice calculates a reduced gas price based on the pct
// XXX Use big.Rat?
func gasprice(price *big.Int, pct int64) *big.Int {
	p := new(big.Int).Set(price)
	p.Div(p, big.NewInt(100))
	p.Mul(p, big.NewInt(pct))
	return p
}

func accountAddressesSet(accounts []wallet.Account) *set.Set {
	accountSet := set.New()
	for _, account := range accounts {
		accountSet.Add(account.Address)
	}
	return accountSet
}
