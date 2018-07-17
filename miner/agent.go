package miner

import (
	"sync"

	"sync/atomic"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/pow"
)

type CpuAgent struct {
	mu sync.Mutex

	workCh        chan *Work
	quit          chan struct{}
	quitCurrentOp chan struct{}
	returnCh      chan<- *Result

	index int
	pow   pow.PoW

	isMining int32 // isMining indicates whether the agent is currently mining
}

func NewCpuAgent(index int, pow pow.PoW) *CpuAgent {
	miner := &CpuAgent{
		pow:    pow,
		index:  index,
		quit:   make(chan struct{}),
		workCh: make(chan *Work, 1),
	}

	return miner
}

func (self *CpuAgent) Work() chan<- *Work            { return self.workCh }
func (self *CpuAgent) Pow() pow.PoW                  { return self.pow }
func (self *CpuAgent) SetReturnCh(ch chan<- *Result) { self.returnCh = ch }

func (self *CpuAgent) Stop() {
	close(self.quit)
}

func (self *CpuAgent) Start() {

	if !atomic.CompareAndSwapInt32(&self.isMining, 0, 1) {
		return // agent already started
	}

	go self.update()
}

func (self *CpuAgent) update() {
out:
	for {
		select {
		case work := <-self.workCh:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
			}
			self.quitCurrentOp = make(chan struct{})
			go self.mine(work, self.quitCurrentOp)
			self.mu.Unlock()
		case <-self.quit:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
				self.quitCurrentOp = nil
			}
			self.mu.Unlock()
			break out
		}
	}

done:
	// Empty work channel
	for {
		select {
		case <-self.workCh:
		default:
			close(self.workCh)
			break done
		}
	}

	atomic.StoreInt32(&self.isMining, 0)
}

func (self *CpuAgent) mine(work *Work, stop <-chan struct{}) {
	glog.V(logger.Debug).Infof("(re)started agent[%d]. mining...\n", self.index)

	// Mine
	nonce, mixDigest := self.pow.Search(work.Block, stop, self.index)
	if nonce != 0 {
		block := work.Block.WithMiningResult(nonce, helper.BytesToHash(mixDigest))
		self.returnCh <- &Result{work, block}
	} else {
		self.returnCh <- nil
	}
}

func (self *CpuAgent) GetHashRate() int64 {
	return self.pow.GetHashrate()
}
