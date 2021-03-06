package state

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/helper/rlp"
	"github.com/siotchain/siot/trie"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (self Code) String() string {
	return string(self) //strings.Join(Disassemble(self), " ")
}

type Storage map[helper.Hash]helper.Hash

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = value
	}

	return cpy
}

// StateObject represents an Siotchain account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type StateObject struct {
	address helper.Address // Siotchain address of this account
	data    Account
	db      *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie *trie.SecureTrie // storage trie, which becomes non-nil on first access
	code Code             // externalLogic bytecode, which gets set when code is loaded

	cachedStorage Storage // Storage entry cache to avoid duplicate reads
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
	onDirty   func(addr helper.Address) // Callback method to mark a state object newly dirty
}

// empty returns whether the account is considered empty.
func (s *StateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.BitLen() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// Account is the Siotchain consensus representation of wallet.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     helper.Hash // merkle root of the storage trie
	CodeHash []byte
}

// newObject creates a state object.
func newObject(db *StateDB, address helper.Address, data Account, onDirty func(addr helper.Address)) *StateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &StateObject{db: db, address: address, data: data, cachedStorage: make(Storage), dirtyStorage: make(Storage), onDirty: onDirty}
}

// EncodeRLP implements rlp.Encoder.
func (c *StateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *StateObject) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateObject) markSuicided() {
	self.suicided = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v X\n", self.Address(), self.Nonce(), self.Balance())
	}
}

func (c *StateObject) getTrie(db trie.Database) *trie.SecureTrie {
	if c.trie == nil {
		var err error
		c.trie, err = trie.NewSecure(c.data.Root, db, 0)
		if err != nil {
			c.trie, _ = trie.NewSecure(helper.Hash{}, db, 0)
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return c.trie
}

// GetState returns a value in account storage.
func (self *StateObject) GetState(db trie.Database, key helper.Hash) helper.Hash {
	value, exists := self.cachedStorage[key]
	if exists {
		return value
	}
	// Load from DB in case it is missing.
	if enc := self.getTrie(db).Get(key[:]); len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
		}
		value.SetBytes(content)
	}
	if (value != helper.Hash{}) {
		self.cachedStorage[key] = value
	}
	return value
}

// SetState updates a value in account storage.
func (self *StateObject) SetState(db trie.Database, key, value helper.Hash) {
	self.db.journal = append(self.db.journal, storageChange{
		account:  &self.address,
		key:      key,
		prevalue: self.GetState(db, key),
	})
	self.setState(key, value)
}

func (self *StateObject) setState(key, value helper.Hash) {
	self.cachedStorage[key] = value
	self.dirtyStorage[key] = value

	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (self *StateObject) updateTrie(db trie.Database) {
	tr := self.getTrie(db)
	for key, value := range self.dirtyStorage {
		delete(self.dirtyStorage, key)
		if (value == helper.Hash{}) {
			tr.Delete(key[:])
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		tr.Update(key[:], v)
	}
}

// UpdateRoot sets the trie root to the current root hash of
func (self *StateObject) updateRoot(db trie.Database) {
	self.updateTrie(db)
	self.data.Root = self.trie.Hash()
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *StateObject) CommitTrie(db trie.Database, dbw trie.DatabaseWriter) error {
	self.updateTrie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.trie.CommitTo(dbw)
	if err == nil {
		self.data.Root = root
	}
	return err
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (c *StateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Cmp(helper.Big0) == 0 && !c.empty() {
		return
	}
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))

	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v (+ %v)\n", c.Address(), c.Nonce(), c.Balance(), amount)
	}
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (c *StateObject) SubBalance(amount *big.Int) {
	if amount.Cmp(helper.Big0) == 0 {
		return
	}
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))

	if glog.V(logger.Core) {
		glog.Infof("%x: #%d %v (- %v)\n", c.Address(), c.Nonce(), c.Balance(), amount)
	}
}

func (self *StateObject) SetBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.Balance),
	})
	self.setBalance(amount)
}

func (self *StateObject) setBalance(amount *big.Int) {
	self.data.Balance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (c *StateObject) ReturnGas(gas, price *big.Int) {}

func (self *StateObject) deepCopy(db *StateDB, onDirty func(addr helper.Address)) *StateObject {
	stateObject := newObject(db, self.address, self.data, onDirty)
	stateObject.trie = self.trie
	stateObject.code = self.code
	stateObject.dirtyStorage = self.dirtyStorage.Copy()
	stateObject.cachedStorage = self.dirtyStorage.Copy()
	stateObject.suicided = self.suicided
	stateObject.dirtyCode = self.dirtyCode
	stateObject.deleted = self.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the externalLogic/account
func (c *StateObject) Address() helper.Address {
	return c.address
}

// Code returns the externalLogic code associated with this object, if any.
func (self *StateObject) Code(db trie.Database) []byte {
	if self.code != nil {
		return self.code
	}
	if bytes.Equal(self.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.Get(self.CodeHash())
	if err != nil {
		self.setError(fmt.Errorf("can't load code hash %x: %v", self.CodeHash(), err))
	}
	self.code = code
	return code
}

func (self *StateObject) SetCode(codeHash helper.Hash, code []byte) {
	prevcode := self.Code(self.db.db)
	self.db.journal = append(self.db.journal, codeChange{
		account:  &self.address,
		prevhash: self.CodeHash(),
		prevcode: prevcode,
	})
	self.setCode(codeHash, code)
}

func (self *StateObject) setCode(codeHash helper.Hash, code []byte) {
	self.code = code
	self.data.CodeHash = codeHash[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *StateObject) SetNonce(nonce uint64) {
	self.db.journal = append(self.db.journal, nonceChange{
		account: &self.address,
		prev:    self.data.Nonce,
	})
	self.setNonce(nonce)
}

func (self *StateObject) setNonce(nonce uint64) {
	self.data.Nonce = nonce
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *StateObject) CodeHash() []byte {
	return self.data.CodeHash
}

func (self *StateObject) Balance() *big.Int {
	return self.data.Balance
}

func (self *StateObject) Nonce() uint64 {
	return self.data.Nonce
}

// Never called, but must be present to allow StateObject to be used
// as a localEnv.Account interface that also satisfies the localEnv.ExternalLogicRef
// interface. Interfaces are awesome.
func (self *StateObject) Value() *big.Int {
	panic("Value on StateObject should never be called")
}

func (self *StateObject) ForEachStorage(cb func(key, value helper.Hash) bool) {
	// When iterating over the storage check the cache first
	for h, value := range self.cachedStorage {
		cb(h, value)
	}

	it := self.getTrie(self.db.db).Iterator()
	for it.Next() {
		// ignore cached values
		key := helper.BytesToHash(self.trie.GetKey(it.Key))
		if _, ok := self.cachedStorage[key]; !ok {
			cb(key, helper.BytesToHash(it.Value))
		}
	}
}
