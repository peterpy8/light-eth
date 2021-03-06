package blockchainCore

import (
	"math/big"

	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
)

// Validator is an interface which defines the standard for block validation.
//
// The validator is responsible for validating incoming block or, if desired,
// validates headers for fast validation.
//
// ValidateBlock validates the given block and should return an error if it
// failed to do so and should be used for "full" validation.
//
// ValidateHeader validates the given header and parent and returns an error
// if it failed to do so.
//
// ValidateState validates the given statedb and optionally the receipts and
// gas used. The implementer should decide what to do with the given input.
type Validator interface {
	HeaderValidator
	ValidateBlock(block *types.Block) error
	ValidateState(block, parent *types.Block, state *state.StateDB, receipts types.Receipts, usedGas *big.Int) error
}

// HeaderValidator is an interface for validating headers only
//
// ValidateHeader validates the given header and parent and returns an error
// if it failed to do so.
type HeaderValidator interface {
	ValidateHeader(header, parent *types.Header, checkPow bool) error
}

// Processor is an interface for processing blocks using a given initial state.
//
// Process takes the block to be processed and the statedb upon which the
// initial state is based. It should return the receipts generated, amount
// of gas used in the process and return an error if any of the internal rules
// failed.
type Processor interface {
	Process(block *types.Block, statedb *state.StateDB) (types.Receipts, localEnv.Logs, *big.Int, error)
}
