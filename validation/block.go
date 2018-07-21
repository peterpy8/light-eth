package validation

import (
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/core/types"
)

type Block interface {
	Difficulty() *big.Int
	HashNoNonce() helper.Hash
	Nonce() uint64
	MixDigest() helper.Hash
	NumberU64() uint64
}

type ChainManager interface {
	GetBlockByNumber(uint64) *types.Block
	CurrentBlock() *types.Block
}
