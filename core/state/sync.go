package state

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/database"
	"github.com/ethereum/go-ethereum/helper/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// StateSync is the main state synchronisation scheduler, which provides yet the
// unknown state hashes to retrieve, accepts node data associated with said hashes
// and reconstructs the state database step by step until all is done.
type StateSync trie.TrieSync

// NewStateSync create a new state trie download scheduler.
func NewStateSync(root helper.Hash, database database.Database) *StateSync {
	var syncer *trie.TrieSync

	callback := func(leaf []byte, parent helper.Hash) error {
		var obj struct {
			Nonce    uint64
			Balance  *big.Int
			Root     helper.Hash
			CodeHash []byte
		}
		if err := rlp.Decode(bytes.NewReader(leaf), &obj); err != nil {
			return err
		}
		syncer.AddSubTrie(obj.Root, 64, parent, nil)
		syncer.AddRawEntry(helper.BytesToHash(obj.CodeHash), 64, parent)

		return nil
	}
	syncer = trie.NewTrieSync(root, database, callback)
	return (*StateSync)(syncer)
}

// Missing retrieves the known missing nodes from the state trie for retrieval.
func (s *StateSync) Missing(max int) []helper.Hash {
	return (*trie.TrieSync)(s).Missing(max)
}

// Process injects a batch of retrieved trie nodes data, returning if something
// was committed to the database and also the index of an entry if processing of
// it failed.
func (s *StateSync) Process(list []trie.SyncResult) (bool, int, error) {
	return (*trie.TrieSync)(s).Process(list)
}

// Pending returns the number of state entries currently pending for download.
func (s *StateSync) Pending() int {
	return (*trie.TrieSync)(s).Pending()
}
