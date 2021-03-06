package blockchainCore

import (
	"fmt"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/configure"
)

var (
	Big0 = big.NewInt(0)
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If externalLogic creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp            *GasPool
	msg           Message
	gas, gasPrice *big.Int
	initialGas    *big.Int
	value         *big.Int
	data          []byte
	state         localEnv.Database

	env localEnv.Environment
}

// Message represents a message sent to a externalLogic.
type Message interface {
	From() helper.Address
	//FromFrontier() (helper.Address, error)
	To() *helper.Address

	GasPrice() *big.Int
	Gas() *big.Int
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

func MessageCreatesExternalLogic(msg Message) bool {
	return msg.To() == nil
}

// IntrinsicGas computes the 'intrinsic gas' for a message
// with the given data.
func IntrinsicGas(data []byte, externalLogicCreation, homestead bool) *big.Int {
	igas := new(big.Int)
	if externalLogicCreation && homestead {
		igas.Set(configure.TxGasExternalLogicCreation)
	} else {
		igas.Set(configure.TxGas)
	}
	if len(data) > 0 {
		var nz int64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		m := big.NewInt(nz)
		m.Mul(m, configure.TxDataNonZeroGas)
		igas.Add(igas, m)
		m.SetInt64(int64(len(data)) - nz)
		m.Mul(m, configure.TxDataZeroGas)
		igas.Add(igas, m)
	}
	return igas
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(env localEnv.Environment, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:         gp,
		env:        env,
		msg:        msg,
		gas:        new(big.Int),
		gasPrice:   msg.GasPrice(),
		initialGas: new(big.Int),
		value:      msg.Value(),
		data:       msg.Data(),
		state:      env.Db(),
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any Siotchain Env execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(env localEnv.Environment, msg Message, gp *GasPool) ([]byte, *big.Int, error) {
	st := NewStateTransition(env, msg, gp)

	ret, _, gasUsed, err := st.TransitionDb()
	return ret, gasUsed, err
}

func (self *StateTransition) from() localEnv.Account {
	f := self.msg.From()
	if !self.state.Exist(f) {
		return self.state.CreateAccount(f)
	}
	return self.state.GetAccount(f)
}

func (self *StateTransition) to() localEnv.Account {
	if self.msg == nil {
		return nil
	}
	to := self.msg.To()
	if to == nil {
		return nil // externalLogic creation
	}

	if !self.state.Exist(*to) {
		return self.state.CreateAccount(*to)
	}
	return self.state.GetAccount(*to)
}

func (self *StateTransition) useGas(amount *big.Int) error {
	if self.gas.Cmp(amount) < 0 {
		return localEnv.OutOfGasError
	}
	self.gas.Sub(self.gas, amount)

	return nil
}

func (self *StateTransition) addGas(amount *big.Int) {
	self.gas.Add(self.gas, amount)
}

func (self *StateTransition) buyGas() error {
	mgas := self.msg.Gas()
	mgval := new(big.Int).Mul(mgas, self.gasPrice)

	sender := self.from()
	if sender.Balance().Cmp(mgval) < 0 {
		return fmt.Errorf("insufficient coinbase for gas (%x). Req %v, has %v", sender.Address().Bytes()[:4], mgval, sender.Balance())
	}
	if err := self.gp.SubGas(mgas); err != nil {
		return err
	}
	self.addGas(mgas)
	self.initialGas.Set(mgas)
	sender.SubBalance(mgval)
	return nil
}

func (self *StateTransition) preCheck() (err error) {
	msg := self.msg
	sender := self.from()

	// Make sure this transaction's nonce is correct
	if msg.CheckNonce() {
		if n := self.state.GetNonce(sender.Address()); n != msg.Nonce() {
			return NonceError(msg.Nonce(), n)
		}
	}

	// Pre-pay gas
	if err = self.buyGas(); err != nil {
		if IsGasLimitErr(err) {
			return err
		}
		return InvalidTxError(err)
	}

	return nil
}

// TransitionDb will move the state by applying the message against the given environment.
func (self *StateTransition) TransitionDb() (ret []byte, requiredGas, usedGas *big.Int, err error) {
	if err = self.preCheck(); err != nil {
		return
	}
	msg := self.msg
	sender := self.from() // err checked in preCheck

	homestead := self.env.ChainConfig().IsHomestead(self.env.BlockNumber())
	externalLogicCreation := MessageCreatesExternalLogic(msg)
	// Pay intrinsic gas
	if err = self.useGas(IntrinsicGas(self.data, externalLogicCreation, homestead)); err != nil {
		return nil, nil, nil, InvalidTxError(err)
	}

	vmenv := self.env
	//var addr helper.Address
	if externalLogicCreation {
		ret, _, err = vmenv.Create(sender, self.data, self.gas, self.gasPrice, self.value)
		if homestead && err == localEnv.CodeStoreOutOfGasError {
			self.gas = Big0
		}

		if err != nil {
			ret = nil
			glog.V(logger.Core).Infoln("VM create err:", err)
		}
	} else {
		// Increment the nonce for the next transaction
		self.state.SetNonce(sender.Address(), self.state.GetNonce(sender.Address())+1)
		ret, err = vmenv.Call(sender, self.to().Address(), self.data, self.gas, self.gasPrice, self.value)
		if err != nil {
			glog.V(logger.Core).Infoln("VM call err:", err)
		}
	}

	if err != nil && IsValueTransferErr(err) {
		return nil, nil, nil, InvalidTxError(err)
	}

	// We aren't interested in errors here. Errors returned by the VM are non-consensus errors and therefor shouldn't bubble up
	if err != nil {
		err = nil
	}

	requiredGas = new(big.Int).Set(self.gasUsed())

	self.refundGas()
	self.state.AddBalance(self.env.Coinbase(), new(big.Int).Mul(self.gasUsed(), self.gasPrice))

	return ret, requiredGas, self.gasUsed(), err
}

func (self *StateTransition) refundGas() {
	// Return siot for remaining gas to the sender account,
	// exchanged at the original rate.
	sender := self.from() // err already checked
	remaining := new(big.Int).Mul(self.gas, self.gasPrice)
	sender.AddBalance(remaining)

	// Apply refund counter, capped to half of the used gas.
	uhalf := remaining.Div(self.gasUsed(), helper.Big2)
	refund := helper.BigMin(uhalf, self.state.GetRefund())
	self.gas.Add(self.gas, refund)
	self.state.AddBalance(sender.Address(), refund.Mul(refund, self.gasPrice))

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	self.gp.AddGas(self.gas)
}

func (self *StateTransition) gasUsed() *big.Int {
	return new(big.Int).Sub(self.initialGas, self.gas)
}
