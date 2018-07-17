package filters

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/database"
	"github.com/ethereum/go-ethereum/subscribe"
	"github.com/ethereum/go-ethereum/net/rpc"
)

// filter is a helper struct that holds meta information over the filter type
// and associated subscription in the subscribe system.
type filter struct {
	typ      Type
	deadline *time.Timer // filter is inactiv when deadline triggers
	hashes   []helper.Hash
	crit     FilterCriteria
	logs     []Log
	s        *Subscription // associated subscription in subscribe system
}

// PublicFilterAPI offers support to create and manage filters. This will allow external clients to retrieve various
// information related to the Siotchain protocol such als blocks, transactions and logs.
type PublicFilterAPI struct {
	backend   Backend
	useMipMap bool
	mux       *subscribe.TypeMux
	quit      chan struct{}
	chainDb   database.Database
	events    *EventSystem
	filtersMu sync.Mutex
	filters   map[rpc.ID]*filter
}

// FilterCriteria represents a request to create a new filter.
type FilterCriteria struct {
	FromBlock *big.Int
	ToBlock   *big.Int
	Addresses []helper.Address
	Topics    [][]helper.Hash
}