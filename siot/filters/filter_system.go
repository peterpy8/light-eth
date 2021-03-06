// Package filters implements an Siotchain filtering system for block,
// transactions and log events.
package filters

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/net/rpc"
	"golang.org/x/net/context"
)

// Type determines the kind of filter and is used to put the filter in to
// the correct bucket when added.
type Type byte

const (
	// UnknownSubscription indicates an unkown subscription type
	UnknownSubscription Type = iota
	// LogsSubscription queries for new or removed (chain reorg) logs
	LogsSubscription
	// PendingLogsSubscription queries for logs for the pending block
	PendingLogsSubscription
	// PendingTransactionsSubscription queries tx hashes for pending
	// transactions entering the pending state
	PendingTransactionsSubscription
	// BlocksSubscription queries hashes for blocks that are imported
	BlocksSubscription
)

var (
	ErrInvalidSubscriptionID = errors.New("invalid id")
)

// Log is a helper that can hold additional information about localEnv.Log
// necessary for the RPC interface.
type Log struct {
	*localEnv.Log
	Removed bool `json:"removed"`
}

func (l *Log) MarshalJSON() ([]byte, error) {
	fields := map[string]interface{}{
		"address":          l.Address,
		"data":             fmt.Sprintf("0x%x", l.Data),
		"blockNumber":      fmt.Sprintf("%#x", l.BlockNumber),
		"logIndex":         fmt.Sprintf("%#x", l.Index),
		"blockHash":        l.BlockHash,
		"transactionHash":  l.TxHash,
		"transactionIndex": fmt.Sprintf("%#x", l.TxIndex),
		"topics":           l.Topics,
		"removed":          l.Removed,
	}

	return json.Marshal(fields)
}

type subscription struct {
	id        rpc.ID
	typ       Type
	created   time.Time
	logsCrit  FilterCriteria
	logs      chan []Log
	hashes    chan helper.Hash
	headers   chan *types.Header
	installed chan struct{} // closed when the filter is installed
	err       chan error    // closed when the filter is uninstalled
}

// EventSystem creates subscriptions, processes events and broadcasts them to the
// subscription which match the subscription criteria.
type EventSystem struct {
	mux       *subscribe.TypeMux
	sub       subscribe.Subscription
	backend   Backend
	lightMode bool
	lastHead  *types.Header
	install   chan *subscription // install filter for subscribe notification
	uninstall chan *subscription // remove filter for subscribe notification
}

// NewEventSystem creates a new manager that listens for subscribe on the given mux,
// parses and filters them. It uses the all map to retrieve filter changes. The
// work loop holds its own index that is used to forward events to filters.
//
// The returned manager has a loop that needs to be stopped with the Stop function
// or by stopping the given mux.
func NewEventSystem(mux *subscribe.TypeMux, backend Backend, lightMode bool) *EventSystem {
	m := &EventSystem{
		mux:       mux,
		backend:   backend,
		lightMode: lightMode,
		install:   make(chan *subscription),
		uninstall: make(chan *subscription),
	}

	go m.eventLoop()

	return m
}

// Subscription is created when the client registers itself for a particular subscribe.
type Subscription struct {
	ID        rpc.ID
	f         *subscription
	es        *EventSystem
	unsubOnce sync.Once
}

// Err returns a channel that is closed when unsubscribed.
func (sub *Subscription) Err() <-chan error {
	return sub.f.err
}

// Unsubscribe uninstalls the subscription from the subscribe broadcast loop.
func (sub *Subscription) Unsubscribe() {
	sub.unsubOnce.Do(func() {
	uninstallLoop:
		for {
			// write uninstall request and consume logs/hashes. This prevents
			// the eventLoop broadcast method to deadlock when writing to the
			// filter subscribe channel while the subscription loop is waiting for
			// this method to return (and thus not reading these events).
			select {
			case sub.es.uninstall <- sub.f:
				break uninstallLoop
			case <-sub.f.logs:
			case <-sub.f.hashes:
			case <-sub.f.headers:
			}
		}

		// wait for filter to be uninstalled in work loop before returning
		// this ensures that the manager won't use the subscribe channel which
		// will probably be closed by the client asap after this method returns.
		<-sub.Err()
	})
}

// subscribe installs the subscription in the subscribe broadcast loop.
func (es *EventSystem) subscribe(sub *subscription) *Subscription {
	es.install <- sub
	<-sub.installed
	return &Subscription{ID: sub.id, f: sub, es: es}
}

// SubscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel.
func (es *EventSystem) SubscribeLogs(crit FilterCriteria, logs chan []Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       LogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan helper.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}

	return es.subscribe(sub)
}

// SubscribePendingLogs creates a subscription that will write pending logs matching the
// given criteria to the given channel.
func (es *EventSystem) SubscribePendingLogs(crit FilterCriteria, logs chan []Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan helper.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}

	return es.subscribe(sub)
}

// SubscribePendingTxEvents creates a sbuscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) SubscribePendingTxEvents(hashes chan helper.Hash) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingTransactionsSubscription,
		created:   time.Now(),
		logs:      make(chan []Log),
		hashes:    hashes,
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}

	return es.subscribe(sub)
}

// SubscribeNewHeads creates a subscription that writes the header of a block that is
// imported in the chain.
func (es *EventSystem) SubscribeNewHeads(headers chan *types.Header) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       BlocksSubscription,
		created:   time.Now(),
		logs:      make(chan []Log),
		hashes:    make(chan helper.Hash),
		headers:   headers,
		installed: make(chan struct{}),
		err:       make(chan error),
	}

	return es.subscribe(sub)
}

type filterIndex map[Type]map[rpc.ID]*subscription

// broadcast subscribe to filters that match criteria.
func (es *EventSystem) broadcast(filters filterIndex, ev *subscribe.Event) {
	if ev == nil {
		return
	}

	switch e := ev.Data.(type) {
	case localEnv.Logs:
		if len(e) > 0 {
			for _, f := range filters[LogsSubscription] {
				if ev.Time.After(f.created) {
					if matchedLogs := filterLogs(convertLogs(e, false), f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
						f.logs <- matchedLogs
					}
				}
			}
		}
	case blockchainCore.RemovedLogsEvent:
		for _, f := range filters[LogsSubscription] {
			if ev.Time.After(f.created) {
				if matchedLogs := filterLogs(convertLogs(e.Logs, true), f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
					f.logs <- matchedLogs
				}
			}
		}
	case blockchainCore.PendingLogsEvent:
		for _, f := range filters[PendingLogsSubscription] {
			if ev.Time.After(f.created) {
				if matchedLogs := filterLogs(convertLogs(e.Logs, false), f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
					f.logs <- matchedLogs
				}
			}
		}
	case blockchainCore.TxPreEvent:
		for _, f := range filters[PendingTransactionsSubscription] {
			if ev.Time.After(f.created) {
				f.hashes <- e.Tx.Hash()
			}
		}
	case blockchainCore.ChainEvent:
		for _, f := range filters[BlocksSubscription] {
			if ev.Time.After(f.created) {
				f.headers <- e.Block.Header()
			}
		}
		if es.lightMode && len(filters[LogsSubscription]) > 0 {
			es.lightFilterNewHead(e.Block.Header(), func(header *types.Header, remove bool) {
				for _, f := range filters[LogsSubscription] {
					if ev.Time.After(f.created) {
						if matchedLogs := es.lightFilterLogs(header, f.logsCrit.Addresses, f.logsCrit.Topics, remove); len(matchedLogs) > 0 {
							f.logs <- matchedLogs
						}
					}
				}
			})
		}
	}
}

func (es *EventSystem) lightFilterNewHead(newHeader *types.Header, callBack func(*types.Header, bool)) {
	oldh := es.lastHead
	es.lastHead = newHeader
	if oldh == nil {
		return
	}
	newh := newHeader
	// find helper ancestor, create list of rolled back and new block hashes
	var oldHeaders, newHeaders []*types.Header
	for oldh.Hash() != newh.Hash() {
		if oldh.Number.Uint64() >= newh.Number.Uint64() {
			oldHeaders = append(oldHeaders, oldh)
			oldh = blockchainCore.GetHeader(es.backend.ChainDb(), oldh.ParentHash, oldh.Number.Uint64()-1)
		}
		if oldh.Number.Uint64() < newh.Number.Uint64() {
			newHeaders = append(newHeaders, newh)
			newh = blockchainCore.GetHeader(es.backend.ChainDb(), newh.ParentHash, newh.Number.Uint64()-1)
			if newh == nil {
				// happens when CHT syncing, nothing to do
				newh = oldh
			}
		}
	}
	// roll back old blocks
	for _, h := range oldHeaders {
		callBack(h, true)
	}
	// check new blocks (array is in reverse order)
	for i := len(newHeaders) - 1; i >= 0; i-- {
		callBack(newHeaders[i], false)
	}
}

// filter logs of a single header in light client mode
func (es *EventSystem) lightFilterLogs(header *types.Header, addresses []helper.Address, topics [][]helper.Hash, remove bool) []Log {
	//fmt.Println("lightFilterLogs", header.Number.Uint64(), remove)
	if bloomFilter(header.Bloom, addresses, topics) {
		//fmt.Println("bloom match")
		// Get the logs of the block
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
		receipts, err := es.backend.GetReceipts(ctx, header.Hash())
		if err != nil {
			return nil
		}
		var unfiltered []Log
		for _, receipt := range receipts {
			rl := make([]Log, len(receipt.Logs))
			for i, l := range receipt.Logs {
				rl[i] = Log{l, remove}
			}
			unfiltered = append(unfiltered, rl...)
		}
		logs := filterLogs(unfiltered, addresses, topics)
		//fmt.Println("found", len(logs))
		return logs
	}
	return nil
}

// eventLoop (un)installs filters and processes mux events.
func (es *EventSystem) eventLoop() {
	var (
		index = make(filterIndex)
		sub   = es.mux.Subscribe(blockchainCore.PendingLogsEvent{}, blockchainCore.RemovedLogsEvent{}, localEnv.Logs{}, blockchainCore.TxPreEvent{}, blockchainCore.ChainEvent{})
	)
	for {
		select {
		case ev, active := <-sub.Chan():
			if !active { // system stopped
				return
			}
			es.broadcast(index, ev)
		case f := <-es.install:
			if _, found := index[f.typ]; !found {
				index[f.typ] = make(map[rpc.ID]*subscription)
			}
			index[f.typ][f.id] = f
			close(f.installed)
		case f := <-es.uninstall:
			delete(index[f.typ], f.id)
			close(f.err)
		}
	}
}

// convertLogs is a helper utility that converts localEnv.Logs to []filter.Log.
func convertLogs(in localEnv.Logs, removed bool) []Log {
	logs := make([]Log, len(in))
	for i, l := range in {
		logs[i] = Log{l, removed}
	}
	return logs
}
