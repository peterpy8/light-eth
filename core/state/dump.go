package state

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/helper/rlp"
)

type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`
}

type Dump struct {
	Root     string                 `json:"root"`
	Accounts map[string]DumpAccount `json:"wallet"`
}

func (self *StateDB) RawDump() Dump {
	dump := Dump{
		Root:     helper.Bytes2Hex(self.trie.Root()),
		Accounts: make(map[string]DumpAccount),
	}

	it := self.trie.Iterator()
	for it.Next() {
		addr := self.trie.GetKey(it.Key)
		var data Account
		if err := rlp.DecodeBytes(it.Value, &data); err != nil {
			panic(err)
		}

		obj := newObject(nil, helper.BytesToAddress(addr), data, nil)
		account := DumpAccount{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     helper.Bytes2Hex(data.Root[:]),
			CodeHash: helper.Bytes2Hex(data.CodeHash),
			Code:     helper.Bytes2Hex(obj.Code(self.db)),
			Storage:  make(map[string]string),
		}
		storageIt := obj.getTrie(self.db).Iterator()
		for storageIt.Next() {
			account.Storage[helper.Bytes2Hex(self.trie.GetKey(storageIt.Key))] = helper.Bytes2Hex(storageIt.Value)
		}
		dump.Accounts[helper.Bytes2Hex(addr)] = account
	}
	return dump
}

func (self *StateDB) Dump() []byte {
	json, err := json.MarshalIndent(self.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return json
}
