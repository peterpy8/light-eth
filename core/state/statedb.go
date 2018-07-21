// Package state provides a caching layer atop the Siotchain state trie.
package state

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/core/localEnv"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/helper/rlp"
	"github.com/siotchain/siot/trie"
	"github.com/hashicorp/golang-lru"
)

// The starting nonce determines the default nonce when new wallet are being
// created.
var StartingNonce uint64

// Trie cache generation limit after which to evic trie nodes from memory.
var MaxTrieCacheGen = uint16(120)

const (
	// Number of past tries to keep. This value is chosen such that
	// reasonable chain reorg depths will hit an existing trie.
	maxPastTries = 12

	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000
)

type revision struct {
	id           int
	journalIndex int
}

// StateDBs within the Siotchain protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * ExternalLogics
// * Accounts
type StateDB struct {
	db            database.Database
	trie          *trie.SecureTrie
	pastTries     []*trie.SecureTrie
	codeSizeCache *lru.Cache

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[helper.Address]*StateObject
	stateObjectsDirty map[helper.Address]struct{}

	// The refund counter, also used by state transitioning.
	refund *big.Int

	thash, bhash helper.Hash
	txIndex      int
	logs         map[helper.Hash]localEnv.Logs
	logSize      uint

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        journal
	validRevisions []revision
	nextRevisionId int

	lock sync.Mutex
}

// Create a new state from a given trie
func New(root helper.Hash, db database.Database) (*StateDB, error) {
	tr, err := trie.NewSecure(root, db, MaxTrieCacheGen)
	if err != nil {
		return nil, err
	}
	csc, _ := lru.New(codeSizeCacheSize)
	return &StateDB{
		db:                db,
		trie:              tr,
		codeSizeCache:     csc,
		stateObjects:      make(map[helper.Address]*StateObject),
		stateObjectsDirty: make(map[helper.Address]struct{}),
		refund:            new(big.Int),
		logs:              make(map[helper.Hash]localEnv.Logs),
	}, nil
}

// New creates a new statedb by reusing any journalled tries to avoid costly
// disk io.
func (self *StateDB) New(root helper.Hash) (*StateDB, error) {
	self.lock.Lock()
	defer self.lock.Unlock()

	tr, err := self.openTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:                self.db,
		trie:              tr,
		codeSizeCache:     self.codeSizeCache,
		stateObjects:      make(map[helper.Address]*StateObject),
		stateObjectsDirty: make(map[helper.Address]struct{}),
		refund:            new(big.Int),
		logs:              make(map[helper.Hash]localEnv.Logs),
	}, nil
}

// Reset clears out all emphemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *StateDB) Reset(root helper.Hash) error {
	self.lock.Lock()
	defer self.lock.Unlock()

	tr, err := self.openTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.stateObjects = make(map[helper.Address]*StateObject)
	self.stateObjectsDirty = make(map[helper.Address]struct{})
	self.thash = helper.Hash{}
	self.bhash = helper.Hash{}
	self.txIndex = 0
	self.logs = make(map[helper.Hash]localEnv.Logs)
	self.logSize = 0
	self.clearJournalAndRefund()

	return nil
}

// openTrie creates a trie. It uses an existing trie if one is available
// from the journal if available.
func (self *StateDB) openTrie(root helper.Hash) (*trie.SecureTrie, error) {
	for i := len(self.pastTries) - 1; i >= 0; i-- {
		if self.pastTries[i].Hash() == root {
			tr := *self.pastTries[i]
			return &tr, nil
		}
	}
	return trie.NewSecure(root, self.db, MaxTrieCacheGen)
}

func (self *StateDB) pushTrie(t *trie.SecureTrie) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if len(self.pastTries) >= maxPastTries {
		copy(self.pastTries, self.pastTries[1:])
		self.pastTries[len(self.pastTries)-1] = t
	} else {
		self.pastTries = append(self.pastTries, t)
	}
}

func (self *StateDB) StartRecord(thash, bhash helper.Hash, ti int) {
	self.thash = thash
	self.bhash = bhash
	self.txIndex = ti
}

func (self *StateDB) AddLog(log *localEnv.Log) {
	self.journal = append(self.journal, addLogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.BlockHash = self.bhash
	log.TxIndex = uint(self.txIndex)
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *StateDB) GetLogs(hash helper.Hash) localEnv.Logs {
	return self.logs[hash]
}

func (self *StateDB) Logs() localEnv.Logs {
	var logs localEnv.Logs
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

func (self *StateDB) AddRefund(gas *big.Int) {
	self.journal = append(self.journal, refundChange{prev: new(big.Int).Set(self.refund)})
	self.refund.Add(self.refund, gas)
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided wallet.
func (self *StateDB) Exist(addr helper.Address) bool {
	return self.GetStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existant
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (self *StateDB) Empty(addr helper.Address) bool {
	so := self.GetStateObject(addr)
	return so == nil || so.empty()
}

func (self *StateDB) GetAccount(addr helper.Address) localEnv.Account {
	return self.GetStateObject(addr)
}

// Retrieve the balance from the given address or 0 if object not found
func (self *StateDB) GetBalance(addr helper.Address) *big.Int {
	stateObject := self.GetStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return helper.Big0
}

func (self *StateDB) GetNonce(addr helper.Address) uint64 {
	stateObject := self.GetStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return StartingNonce
}

func (self *StateDB) GetCode(addr helper.Address) []byte {
	stateObject := self.GetStateObject(addr)
	if stateObject != nil {
		code := stateObject.Code(self.db)
		key := helper.BytesToHash(stateObject.CodeHash())
		self.codeSizeCache.Add(key, len(code))
		return code
	}
	return nil
}

func (self *StateDB) GetCodeSize(addr helper.Address) int {
	stateObject := self.GetStateObject(addr)
	if stateObject == nil {
		return 0
	}
	key := helper.BytesToHash(stateObject.CodeHash())
	if cached, ok := self.codeSizeCache.Get(key); ok {
		return cached.(int)
	}
	size := len(stateObject.Code(self.db))
	if stateObject.dbErr == nil {
		self.codeSizeCache.Add(key, size)
	}
	return size
}

func (self *StateDB) GetCodeHash(addr helper.Address) helper.Hash {
	stateObject := self.GetStateObject(addr)
	if stateObject == nil {
		return helper.Hash{}
	}
	return helper.BytesToHash(stateObject.CodeHash())
}

func (self *StateDB) GetState(a helper.Address, b helper.Hash) helper.Hash {
	stateObject := self.GetStateObject(a)
	if stateObject != nil {
		return stateObject.GetState(self.db, b)
	}
	return helper.Hash{}
}

func (self *StateDB) HasSuicided(addr helper.Address) bool {
	stateObject := self.GetStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

func (self *StateDB) AddBalance(addr helper.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

func (self *StateDB) SetBalance(addr helper.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (self *StateDB) SetNonce(addr helper.Address, nonce uint64) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (self *StateDB) SetCode(addr helper.Address, code []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (self *StateDB) SetState(addr helper.Address, key helper.Hash, value helper.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(self.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// GetStateObject will return a non-nil account after Suicide.
func (self *StateDB) Suicide(addr helper.Address) bool {
	stateObject := self.GetStateObject(addr)
	if stateObject == nil {
		return false
	}
	self.journal = append(self.journal, suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)
	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (self *StateDB) updateStateObject(stateObject *StateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.trie.Update(addr[:], data)
}

// deleteStateObject removes the given object from the state trie.
func (self *StateDB) deleteStateObject(stateObject *StateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.trie.Delete(addr[:])
}

// Retrieve a state object given my the address. Returns nil if not found.
func (self *StateDB) GetStateObject(addr helper.Address) (stateObject *StateObject) {
	// Prefer 'live' objects.
	if obj := self.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc := self.trie.Get(addr[:])
	if len(enc) == 0 {
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		glog.Errorf("can't decode object at %x: %v", addr[:], err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(self, addr, data, self.MarkStateObjectDirty)
	self.setStateObject(obj)
	return obj
}

func (self *StateDB) setStateObject(object *StateObject) {
	self.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil
func (self *StateDB) GetOrNewStateObject(addr helper.Address) *StateObject {
	stateObject := self.GetStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = self.createObject(addr)
	}
	return stateObject
}

// MarkStateObjectDirty adds the specified object to the dirty map to avoid costly
// state object cache iteration to find a handful of modified ones.
func (self *StateDB) MarkStateObjectDirty(addr helper.Address) {
	self.stateObjectsDirty[addr] = struct{}{}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createObject(addr helper.Address) (newobj, prev *StateObject) {
	prev = self.GetStateObject(addr)
	newobj = newObject(self, addr, Account{}, self.MarkStateObjectDirty)
	newobj.setNonce(StartingNonce) // sets the object to dirty
	if prev == nil {
		if glog.V(logger.Core) {
			glog.Infof("(+) %x\n", addr)
		}
		self.journal = append(self.journal, createObjectChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetObjectChange{prev: prev})
	}
	self.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a externalLogic does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that coinbase doesn't disappear.
func (self *StateDB) CreateAccount(addr helper.Address) localEnv.Account {
	new, prev := self.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
	return new
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) Copy() *StateDB {
	self.lock.Lock()
	defer self.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                self.db,
		trie:              self.trie,
		pastTries:         self.pastTries,
		codeSizeCache:     self.codeSizeCache,
		stateObjects:      make(map[helper.Address]*StateObject, len(self.stateObjectsDirty)),
		stateObjectsDirty: make(map[helper.Address]struct{}, len(self.stateObjectsDirty)),
		refund:            new(big.Int).Set(self.refund),
		logs:              make(map[helper.Hash]localEnv.Logs, len(self.logs)),
		logSize:           self.logSize,
	}
	// Copy the dirty states and logs
	for addr, _ := range self.stateObjectsDirty {
		state.stateObjects[addr] = self.stateObjects[addr].deepCopy(state, state.MarkStateObjectDirty)
		state.stateObjectsDirty[addr] = struct{}{}
	}
	for hash, logs := range self.logs {
		state.logs[hash] = make(localEnv.Logs, len(logs))
		copy(state.logs[hash], logs)
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (self *StateDB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, len(self.journal)})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(self.validRevisions), func(i int) bool {
		return self.validRevisions[i].id >= revid
	})
	if idx == len(self.validRevisions) || self.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := self.validRevisions[idx].journalIndex

	// Replay the journal to undo changes.
	for i := len(self.journal) - 1; i >= snapshot; i-- {
		self.journal[i].undo(self)
	}
	self.journal = self.journal[:snapshot]

	// Remove invalidated snapshots from the stack.
	self.validRevisions = self.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
// The return value must not be modified by the caller and will become
// invalid at the next call to AddRefund.
func (self *StateDB) GetRefund() *big.Int {
	return self.refund
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) helper.Hash {
	for addr, _ := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]
		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
	return s.trie.Hash()
}

// DeleteSuicides flags the suicided objects for deletion so that it
// won't be referenced again when called / queried up on.
//
// DeleteSuicides should not be used for consensus related updates
// under any circumstances.
func (s *StateDB) DeleteSuicides() {
	// Reset refund so that any used-gas calculations can use this method.
	s.clearJournalAndRefund()

	for addr, _ := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]

		// If the object has been removed by a suicide
		// flag the object as deleted.
		if stateObject.suicided {
			stateObject.deleted = true
		}
		delete(s.stateObjectsDirty, addr)
	}
}

// Commit commits all state changes to the database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root helper.Hash, err error) {
	root, batch := s.CommitBatch(deleteEmptyObjects)
	return root, batch.Write()
}

// CommitBatch commits all state changes to a write batch but does not
// execute the batch. It is used to validate state changes against
// the root hash stored in a block.
func (s *StateDB) CommitBatch(deleteEmptyObjects bool) (root helper.Hash, batch database.Batch) {
	batch = s.db.NewBatch()
	root, _ = s.commit(batch, deleteEmptyObjects)

	glog.V(logger.Debug).Infof("Trie cache stats: %d misses, %d unloads", trie.CacheMisses(), trie.CacheUnloads())
	return root, batch
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = nil
	s.validRevisions = s.validRevisions[:0]
	s.refund = new(big.Int)
}

func (s *StateDB) commit(dbw trie.DatabaseWriter, deleteEmptyObjects bool) (root helper.Hash, err error) {
	defer s.clearJournalAndRefund()

	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any externalLogic code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				if err := dbw.Put(stateObject.CodeHash(), stateObject.code); err != nil {
					return helper.Hash{}, err
				}
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db, dbw); err != nil {
				return helper.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = s.trie.CommitTo(dbw)
	if err == nil {
		s.pushTrie(s.trie)
	}
	return root, err
}
