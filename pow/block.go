package pow

import (
	"math/big"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/core/types"
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
