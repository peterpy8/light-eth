package blockchainCore

import (
	"fmt"
	"math/big"
	"time"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/configure"
	"github.com/siotchain/siot/validation"
	"gopkg.in/fatih/set.v0"
	"github.com/siotchain/siot/logger"
)

var (
	ExpDiffPeriod = big.NewInt(100000)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type BlockValidator struct {
	config *configure.ChainConfig // Chain configuration options
	bc     *BlockChain            // Canonical block chain
	Pow    validation.PoW         // Proof of work used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(config *configure.ChainConfig, blockchain *BlockChain, pow validation.PoW) *BlockValidator {
	validator := &BlockValidator{
		config: config,
		Pow:    pow,
		bc:     blockchain,
	}
	return validator
}

// ValidateBlock validates the given block's header and uncles and verifies the
// the block header's transaction and uncle roots.
//
// ValidateBlock does not validate the header's pow. The pow work validated
// separately so we can process them in parallel.
//
// ValidateBlock also validates and makes sure that any previous state (or present)
// state that might or might not be present is checked to make sure that fast
// sync has done it's job proper. This prevents the block validator from accepting
// false positives where a header is present but the state is not.
func (v *BlockValidator) ValidateBlock(block *types.Block) error {
	if v.bc.HasBlock(block.Hash()) {
		if _, err := state.New(block.Root(), v.bc.chainDb); err == nil {
			return &KnownBlockError{block.Number(), block.Hash()}
		}
	}
	parent := v.bc.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return ParentError(block.ParentHash())
	}
	if _, err := state.New(parent.Root(), v.bc.chainDb); err != nil {
		return ParentError(block.ParentHash())
	}

	header := block.Header()
	// validate the block header
	if err := ValidateHeader(v.config, v.Pow, header, parent.Header(), false, false); err != nil {
		return err
	}
	// verify the uncles are correctly rewarded
	if err := v.VerifyUncles(block, parent); err != nil {
		return err
	}

	// Verify UncleHash before running other uncle validations
	unclesSha := types.CalcUncleHash(block.Uncles())
	if unclesSha != header.UncleHash {
		return fmt.Errorf("invalid uncles root hash. received=%x calculated=%x", header.UncleHash, unclesSha)
	}

	// The transactions Trie's root (R = (Tr [[i, RLP(T1)], [i, RLP(T2)], ... [n, RLP(Tn)]]))
	// can be used by light clients to make sure they've received the correct Txs
	txSha := types.DeriveSha(block.Transactions())
	if txSha != header.TxHash {
		return fmt.Errorf("invalid transaction root hash. received=%x calculated=%x", header.TxHash, txSha)
	}

	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *BlockValidator) ValidateState(block, parent *types.Block, statedb *state.StateDB, receipts types.Receipts, usedGas *big.Int) (err error) {
	header := block.Header()
	if block.GasUsed().Cmp(usedGas) != 0 {
		return ValidationError(fmt.Sprintf("gas used error (%v / %v)", block.GasUsed(), usedGas))
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != header.Bloom {
		return fmt.Errorf("unable to replicate block's bloom=%x vs calculated bloom=%x", header.Bloom, rbloom)
	}
	// Tre receipt Trie's root (R = (Tr [[H1, R1], ... [Hn, R1]]))
	receiptSha := types.DeriveSha(receipts)
	if receiptSha != header.ReceiptHash {
		return fmt.Errorf("invalid receipt root hash. received=%x calculated=%x", header.ReceiptHash, receiptSha)
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(v.config.IsSiotImpr2(header.Number)); header.Root != root {
		glog.V(logger.Debug).Infof("invalid merkle root: header=%x computed=%x", header.Root, root)
		return nil
	}
	return nil
}

// VerifyUncles verifies the given block's uncles and applies the Siotchain
// consensus rules to the various block headers included; it will return an
// error if any of the included uncle headers were invalid. It returns an error
// if the validation failed.
func (v *BlockValidator) VerifyUncles(block, parent *types.Block) error {
	// validate that there are at most 2 uncles included in this block
	if len(block.Uncles()) > 2 {
		return ValidationError("Block can only contain maximum 2 uncles (contained %v)", len(block.Uncles()))
	}

	uncles := set.New()
	ancestors := make(map[helper.Hash]*types.Block)
	for _, ancestor := range v.bc.GetBlocksFromHash(block.ParentHash(), 7) {
		ancestors[ancestor.Hash()] = ancestor
		// Include ancestors uncles in the uncle set. Uncles must be unique.
		for _, uncle := range ancestor.Uncles() {
			uncles.Add(uncle.Hash())
		}
	}
	ancestors[block.Hash()] = block
	uncles.Add(block.Hash())

	for i, uncle := range block.Uncles() {
		hash := uncle.Hash()
		if uncles.Has(hash) {
			// Error not unique
			return UncleError("uncle[%d](%x) not unique", i, hash[:4])
		}
		uncles.Add(hash)

		if ancestors[hash] != nil {
			branch := fmt.Sprintf("  O - %x\n  |\n", block.Hash())
			for h := range ancestors {
				branch += fmt.Sprintf("  O - %x\n  |\n", h)
			}
			glog.Infoln(branch)
			return UncleError("uncle[%d](%x) is ancestor", i, hash[:4])
		}

		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == parent.Hash() {
			return UncleError("uncle[%d](%x)'s parent is not ancestor (%x)", i, hash[:4], uncle.ParentHash[0:4])
		}

		if err := ValidateHeader(v.config, v.Pow, uncle, ancestors[uncle.ParentHash].Header(), true, true); err != nil {
			return ValidationError(fmt.Sprintf("uncle[%d](%x) header invalid: %v", i, hash[:4], err))
		}
	}

	return nil
}

// ValidateHeader validates the given header and, depending on the validation arg,
// checks the proof of work of the given header. Returns an error if the
// validation failed.
func (v *BlockValidator) ValidateHeader(header, parent *types.Header, checkPow bool) error {
	// Short circuit if the parent is missing.
	if parent == nil {
		return ParentError(header.ParentHash)
	}
	// Short circuit if the header's already known or its parent is missing
	if v.bc.HasHeader(header.Hash()) {
		return nil
	}
	return ValidateHeader(v.config, v.Pow, header, parent, checkPow, false)
}

// Validates a header. Returns an error if the header is invalid.
//
// See YP section 4.3.4. "Block Header Validity"
func ValidateHeader(config *configure.ChainConfig, pow validation.PoW, header *types.Header, parent *types.Header, checkPow, uncle bool) error {
	if big.NewInt(int64(len(header.Extra))).Cmp(configure.MaximumExtraDataSize) == 1 {
		return fmt.Errorf("Header extra data too long (%d)", len(header.Extra))
	}

	if uncle {
		if header.Time.Cmp(helper.MaxBig) == 1 {
			return BlockTSTooBigErr
		}
	} else {
		if header.Time.Cmp(big.NewInt(time.Now().Unix())) == 1 {
			return BlockFutureErr
		}
	}
	if header.Time.Cmp(parent.Time) != 1 {
		return BlockEqualTSErr
	}

	expd := CalcDifficulty(config, header.Time.Uint64(), parent.Time.Uint64(), parent.Number, parent.Difficulty)
	if expd.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("Difficulty check failed for header %v, %v", header.Difficulty, expd)
	}

	a := new(big.Int).Set(parent.GasLimit)
	a = a.Sub(a, header.GasLimit)
	a.Abs(a)
	b := new(big.Int).Set(parent.GasLimit)
	b = b.Div(b, configure.GasLimitBoundDivisor)
	if !(a.Cmp(b) < 0) || (header.GasLimit.Cmp(configure.MinGasLimit) == -1) {
		return fmt.Errorf("GasLimit check failed for header %v (%v > %v)", header.GasLimit, a, b)
	}

	num := new(big.Int).Set(parent.Number)
	num.Sub(header.Number, num)
	if num.Cmp(big.NewInt(1)) != 0 {
		return BlockNumberErr
	}

	if checkPow {
		// Verify the nonce of the header. Return an error if it's not valid
		if !pow.Verify(types.NewBlockWithHeader(header)) {
			return &BlockNonceErr{header.Number, header.Hash(), header.Nonce.Uint64()}
		}
	}
	// If all checks passed, validate the extra-data field for hard forks
	if err := ValidateDAOHeaderExtraData(config, header); err != nil {
		return err
	}
	if config.SiotImpr0Block != nil && config.SiotImpr0Block.Cmp(header.Number) == 0 {
		if config.SiotImpr0Hash != (helper.Hash{}) && config.SiotImpr0Hash != header.Hash() {
			return ValidationError("Homestead gas reprice fork hash mismatch: have 0x%x, want 0x%x", header.Hash(), config.SiotImpr0Hash)
		}
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func CalcDifficulty(config *configure.ChainConfig, time, parentTime uint64, parentNumber, parentDiff *big.Int) *big.Int {
	//if config.IsHomestead(new(big.Int).Add(parentNumber, helper.Big1)) {
	//	return calcDifficultyHomestead(time, parentTime, parentNumber, parentDiff)
	//} else {
	//	return calcDifficultyFrontier(time, parentTime, parentNumber, parentDiff)
	//}
	return big.NewInt(0x2000)
}

func calcDifficultyHomestead(time, parentTime uint64, parentNumber, parentDiff *big.Int) *big.Int {
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).SetUint64(parentTime)

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(helper.Big1, x)

	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}

	y.Div(parentDiff, configure.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parentDiff, x)

	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(configure.MinimumDifficulty) < 0 {
		x.Set(configure.MinimumDifficulty)
	}

	// for the exponential factor
	periodCount := new(big.Int).Add(parentNumber, helper.Big1)
	periodCount.Div(periodCount, ExpDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	if periodCount.Cmp(helper.Big1) > 0 {
		y.Sub(periodCount, helper.Big2)
		y.Exp(helper.Big2, y, nil)
		x.Add(x, y)
	}

	return x
}

func calcDifficultyFrontier(time, parentTime uint64, parentNumber, parentDiff *big.Int) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parentDiff, configure.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.SetUint64(parentTime)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(configure.DurationLimit) < 0 {
		diff.Add(parentDiff, adjust)
	} else {
		diff.Sub(parentDiff, adjust)
	}
	if diff.Cmp(configure.MinimumDifficulty) < 0 {
		diff.Set(configure.MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parentNumber, helper.Big1)
	periodCount.Div(periodCount, ExpDiffPeriod)
	if periodCount.Cmp(helper.Big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, helper.Big2)
		expDiff.Exp(helper.Big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = helper.BigMax(diff, configure.MinimumDifficulty)
	}

	return diff
}

// CalcGasLimit computes the gas limit of the next block after parent.
// The result may be modified by the caller.
// This is miner strategy, not consensus protocol.
func CalcGasLimit(parent *types.Block) *big.Int {
	// contrib = (parentGasUsed * 3 / 2) / 1024
	contrib := new(big.Int).Mul(parent.GasUsed(), big.NewInt(3))
	contrib = contrib.Div(contrib, big.NewInt(2))
	contrib = contrib.Div(contrib, configure.GasLimitBoundDivisor)

	// decay = parentGasLimit / 1024 -1
	decay := new(big.Int).Div(parent.GasLimit(), configure.GasLimitBoundDivisor)
	decay.Sub(decay, big.NewInt(1))

	/*
		strategy: gasLimit of block-to-mine is set based on parent's
		gasUsed value.  if parentGasUsed > parentGasLimit * (2/3) then we
		increase it, otherwise lower it (or leave it unchanged if it's right
		at that usage) the amount increased/decreased depends on how far away
		from parentGasLimit * (2/3) parentGasUsed is.
	*/
	gl := new(big.Int).Sub(parent.GasLimit(), decay)
	gl = gl.Add(gl, contrib)
	gl.Set(helper.BigMax(gl, configure.MinGasLimit))

	// however, if we're now below the target (TargetGasLimit) we increase the
	// limit as much as we can (parentGasLimit / 1024 -1)
	if gl.Cmp(configure.TargetGasLimit) < 0 {
		gl.Add(parent.GasLimit(), decay)
		gl.Set(helper.BigMin(gl, configure.TargetGasLimit))
	}
	return gl
}
