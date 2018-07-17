package state

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/helper/rlp"
	"github.com/siotchain/siot/trie"
)

// NodeIterator is an iterator to traverse the entire state trie post-order,
// including all of the externalLogic code and externalLogic state tries.
type NodeIterator struct {
	state *StateDB // State being iterated

	stateIt *trie.NodeIterator // Primary iterator for the global state trie
	dataIt  *trie.NodeIterator // Secondary iterator for the data trie of a externalLogic

	accountHash helper.Hash // Hash of the node containing the account
	codeHash    helper.Hash // Hash of the externalLogic source code
	code        []byte      // Source code associated with a externalLogic

	Hash   helper.Hash // Hash of the current entry being iterated (nil if not standalone)
	Entry  interface{} // Current state entry being iterated (internal representation)
	Parent helper.Hash // Hash of the first full ancestor node (nil if current is the root)

	Error error // Failure set in case of an internal error in the iterator
}

// NewNodeIterator creates an post-order state node iterator.
func NewNodeIterator(state *StateDB) *NodeIterator {
	return &NodeIterator{
		state: state,
	}
}

// Next moves the iterator to the next node, returning whether there are any
// further nodes. In case of an internal error this method returns false and
// sets the Error field to the encountered failure.
func (it *NodeIterator) Next() bool {
	// If the iterator failed previously, don't do anything
	if it.Error != nil {
		return false
	}
	// Otherwise step forward with the iterator and report any errors
	if err := it.step(); err != nil {
		it.Error = err
		return false
	}
	return it.retrieve()
}

// step moves the iterator to the next entry of the state trie.
func (it *NodeIterator) step() error {
	// Abort if we reached the end of the iteration
	if it.state == nil {
		return nil
	}
	// Initialize the iterator if we've just started
	if it.stateIt == nil {
		it.stateIt = it.state.trie.NodeIterator()
	}
	// If we had data nodes previously, we surely have at least state nodes
	if it.dataIt != nil {
		if cont := it.dataIt.Next(); !cont {
			if it.dataIt.Error != nil {
				return it.dataIt.Error
			}
			it.dataIt = nil
		}
		return nil
	}
	// If we had source code previously, discard that
	if it.code != nil {
		it.code = nil
		return nil
	}
	// Step to the next state trie node, terminating if we're out of nodes
	if cont := it.stateIt.Next(); !cont {
		if it.stateIt.Error != nil {
			return it.stateIt.Error
		}
		it.state, it.stateIt = nil, nil
		return nil
	}
	// If the state trie node is an internal entry, leave as is
	if !it.stateIt.Leaf {
		return nil
	}
	// Otherwise we've reached an account node, initiate data iteration
	var account struct {
		Nonce    uint64
		Balance  *big.Int
		Root     helper.Hash
		CodeHash []byte
	}
	if err := rlp.Decode(bytes.NewReader(it.stateIt.LeafBlob), &account); err != nil {
		return err
	}
	dataTrie, err := trie.New(account.Root, it.state.db)
	if err != nil {
		return err
	}
	it.dataIt = trie.NewNodeIterator(dataTrie)
	if !it.dataIt.Next() {
		it.dataIt = nil
	}
	if bytes.Compare(account.CodeHash, emptyCodeHash) != 0 {
		it.codeHash = helper.BytesToHash(account.CodeHash)
		it.code, err = it.state.db.Get(account.CodeHash)
		if err != nil {
			return fmt.Errorf("code %x: %v", account.CodeHash, err)
		}
	}
	it.accountHash = it.stateIt.Parent
	return nil
}

// retrieve pulls and caches the current state entry the iterator is traversing.
// The method returns whether there are any more data left for inspection.
func (it *NodeIterator) retrieve() bool {
	// Clear out any previously set values
	it.Hash, it.Entry = helper.Hash{}, nil

	// If the iteration's done, return no available data
	if it.state == nil {
		return false
	}
	// Otherwise retrieve the current entry
	switch {
	case it.dataIt != nil:
		it.Hash, it.Entry, it.Parent = it.dataIt.Hash, it.dataIt.Node, it.dataIt.Parent
		if it.Parent == (helper.Hash{}) {
			it.Parent = it.accountHash
		}
	case it.code != nil:
		it.Hash, it.Entry, it.Parent = it.codeHash, it.code, it.accountHash
	case it.stateIt != nil:
		it.Hash, it.Entry, it.Parent = it.stateIt.Hash, it.stateIt.Node, it.stateIt.Parent
	}
	return true
}
