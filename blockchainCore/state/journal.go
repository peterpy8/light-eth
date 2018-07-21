package state

import (
	"math/big"

	"github.com/siotchain/siot/helper"
)

type journalEntry interface {
	undo(*StateDB)
}

type journal []journalEntry

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *helper.Address
	}
	resetObjectChange struct {
		prev *StateObject
	}
	suicideChange struct {
		account     *helper.Address
		prev        bool // whether account had already suicided
		prevbalance *big.Int
	}

	// Changes to individual wallet.
	balanceChange struct {
		account *helper.Address
		prev    *big.Int
	}
	nonceChange struct {
		account *helper.Address
		prev    uint64
	}
	storageChange struct {
		account       *helper.Address
		key, prevalue helper.Hash
	}
	codeChange struct {
		account            *helper.Address
		prevcode, prevhash []byte
	}

	// Changes to other state values.
	refundChange struct {
		prev *big.Int
	}
	addLogChange struct {
		txhash helper.Hash
	}
)

func (ch createObjectChange) undo(s *StateDB) {
	s.GetStateObject(*ch.account).deleted = true
	delete(s.stateObjects, *ch.account)
	delete(s.stateObjectsDirty, *ch.account)
}

func (ch resetObjectChange) undo(s *StateDB) {
	s.setStateObject(ch.prev)
}

func (ch suicideChange) undo(s *StateDB) {
	obj := s.GetStateObject(*ch.account)
	if obj != nil {
		obj.suicided = ch.prev
		obj.setBalance(ch.prevbalance)
	}
}

func (ch balanceChange) undo(s *StateDB) {
	s.GetStateObject(*ch.account).setBalance(ch.prev)
}

func (ch nonceChange) undo(s *StateDB) {
	s.GetStateObject(*ch.account).setNonce(ch.prev)
}

func (ch codeChange) undo(s *StateDB) {
	s.GetStateObject(*ch.account).setCode(helper.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch storageChange) undo(s *StateDB) {
	s.GetStateObject(*ch.account).setState(ch.key, ch.prevalue)
}

func (ch refundChange) undo(s *StateDB) {
	s.refund = ch.prev
}

func (ch addLogChange) undo(s *StateDB) {
	logs := s.logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.logs, ch.txhash)
	} else {
		s.logs[ch.txhash] = logs[:len(logs)-1]
	}
}
