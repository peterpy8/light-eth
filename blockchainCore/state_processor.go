package blockchainCore

import (
	"math/big"

	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/configure"
)

var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *configure.ChainConfig
	bc     *BlockChain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *configure.ChainConfig, bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
	}
}

// Process processes the state changes according to the Siotchain rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB) (types.Receipts, localEnv.Logs, *big.Int, error) {
	var (
		receipts     types.Receipts
		totalUsedGas  = big.NewInt(0)
		err          error
		header              = block.Header()
		allLogs      localEnv.Logs
		gp                      = new(GasPool).AddGas(block.GasLimit())
	)
	// Mutate the the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		ApplyDAOHardFork(statedb)
	}
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		statedb.StartRecord(tx.Hash(), block.Hash(), i)
		receipt, logs, _, err := ApplyTransaction(p.config, p.bc, gp, statedb, header, tx, totalUsedGas)
		if err != nil {
			return nil, nil, nil, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, logs...)
	}
	AccumulateRewards(statedb, header, block.Uncles())

	return receipts, allLogs, totalUsedGas, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment.
//
// ApplyTransactions returns the generated receipts and localEnv logs during the
// execution of the state transition phase.
func ApplyTransaction(config *configure.ChainConfig, bc *BlockChain, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *big.Int) (*types.Receipt, localEnv.Logs, *big.Int, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, nil, nil, err
	}

	_, gas, err := ApplyMessage(NewEnv(statedb, config, bc, msg, header), msg, gp)
	if err != nil {
		return nil, nil, nil, err
	}

	// Update the state with pending changes
	usedGas.Add(usedGas, gas)
	receipt := types.NewReceipt(statedb.IntermediateRoot(config.IsSiotImpr2(header.Number)).Bytes(), usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = new(big.Int).Set(gas)
	if MessageCreatesExternalLogic(msg) {
		receipt.ExternalLogicAddress = crypto.CreateAddress(msg.From(), tx.Nonce())
	}

	logs := statedb.GetLogs(tx.Hash())
	receipt.Logs = logs
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	glog.V(logger.Debug).Infoln(receipt)

	return receipt, logs, gas, err
}

// AccumulateRewards credits the coinbase of the given block with the
// mining reward. The total reward consists of the static block reward
// and rewards for included uncles. The coinbase of each uncle block is
// also rewarded.
func AccumulateRewards(statedb *state.StateDB, header *types.Header, uncles []*types.Header) {
	reward := new(big.Int).Set(BlockReward)
	r := new(big.Int)
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, BlockReward)
		r.Div(r, big8)
		statedb.AddBalance(uncle.Coinbase, r)

		r.Div(BlockReward, big32)
		reward.Add(reward, r)
	}
	statedb.AddBalance(header.Coinbase, reward)
}
