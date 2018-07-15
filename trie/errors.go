package trie

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// MissingNodeError is returned by the trie functions (TryGet, TryUpdate, TryDelete)
// in the case where a trie node is not present in the local database. Contains
// information necessary for retrieving the missing node through an ODR service.
//
// NodeHash is the hash of the missing node
//
// RootHash is the original root of the trie that contains the node
//
// Key is a binary-encoded key that contains the prefix that leads to the first
// missing node and optionally a suffix that hints on which further nodes should
// also be retrieved
//
// PrefixLen is the nibble length of the key prefix that leads from the root to
// the missing node
//
// SuffixLen is the nibble length of the remaining part of the key that hints on
// which further nodes should also be retrieved (can be zero when there are no
// such hints in the error message)
type MissingNodeError struct {
	RootHash, NodeHash   common.Hash
	Key                  []byte
	PrefixLen, SuffixLen int
}

func (err *MissingNodeError) Error() string {
	return fmt.Sprintf("Missing trie node %064x", err.NodeHash)
}
