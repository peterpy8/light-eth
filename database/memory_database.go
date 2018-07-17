package database

import (
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/helper"
)

/*
 * This is a test memory database. Do not use for any production it does not get persisted
 */
type MemDatabase struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func NewMemDatabase() (*MemDatabase, error) {
	return &MemDatabase{
		db: make(map[string][]byte),
	}, nil
}

func (db *MemDatabase) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.db[string(key)] = helper.CopyBytes(value)
	return nil
}

func (db *MemDatabase) Set(key []byte, value []byte) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.Put(key, value)
}

func (db *MemDatabase) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if entry, ok := db.db[string(key)]; ok {
		return entry, nil
	}
	return nil, errors.New("not found")
}

func (db *MemDatabase) Keys() [][]byte {
	db.lock.RLock()
	defer db.lock.RUnlock()

	keys := [][]byte{}
	for key, _ := range db.db {
		keys = append(keys, []byte(key))
	}
	return keys
}

/*
func (db *MemDatabase) GetKeys() []*helper.Key {
	data, _ := db.Get([]byte("KeyRing"))

	return []*helper.Key{helper.NewKeyFromBytes(data)}
}
*/

func (db *MemDatabase) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	delete(db.db, string(key))
	return nil
}

func (db *MemDatabase) Close() {}

func (db *MemDatabase) NewBatch() Batch {
	return &memBatch{db: db}
}

type kv struct{ k, v []byte }

type memBatch struct {
	db     *MemDatabase
	writes []kv
	lock   sync.RWMutex
}

func (b *memBatch) Put(key, value []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.writes = append(b.writes, kv{helper.CopyBytes(key), helper.CopyBytes(value)})
	return nil
}

func (b *memBatch) Write() error {
	b.lock.RLock()
	defer b.lock.RUnlock()

	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, kv := range b.writes {
		b.db.db[string(kv.k)] = kv.v
	}
	return nil
}
