package localEnv

import (
	"math/big"

	"github.com/siotchain/siot/helper"
)

// ExternalLogicRef is a reference to the externalLogic's backing object
type ExternalLogicRef interface {
	ReturnGas(*big.Int, *big.Int)
	Address() helper.Address
	Value() *big.Int
}

// ExternalLogic represents an siot externalLogic in the state database. It contains
// the the externalLogic code, calling arguments. ExternalLogic implements ExternalLogicRef
type ExternalLogic struct {
	// CallerAddress is the result of the caller which initialised this
	// externalLogic. However when the "call method" is delegated this value
	// needs to be initialised to that of the caller's caller.
	CallerAddress helper.Address
	caller        ExternalLogicRef
	self          ExternalLogicRef

	value, Gas, UsedGas, Price *big.Int

	Args []byte

	DelegateCall bool
}

// NewExternalLogic returns a new externalLogic environment for the execution of EVM.
func NewExternalLogic(caller ExternalLogicRef, object ExternalLogicRef, value, gas, price *big.Int) *ExternalLogic {
	//c := &ExternalLogic{CallerAddress: caller.Address(), caller: caller, self: object, Args: nil}
	c := &ExternalLogic{CallerAddress: caller.Address(), caller: caller, self: object, Args: nil}

	// Gas should be a pointer so it can safely be reduced through the run
	// This pointer will be off the state transition
	c.Gas = gas //new(big.Int).Set(gas)
	c.value = new(big.Int).Set(value)
	// In most cases price and value are pointers to transaction objects
	// and we don't want the transaction's values to change.
	c.Price = new(big.Int).Set(price)
	c.UsedGas = new(big.Int)

	return c
}

// AsDelegate sets the externalLogic to be a delegate call and returns the current
// externalLogic (for chaining calls)
func (c *ExternalLogic) AsDelegate() *ExternalLogic {
	c.DelegateCall = true
	// NOTE: caller must, at all times be a externalLogic. It should never happen
	// that caller is something other than a ExternalLogic.
	c.CallerAddress = c.caller.(*ExternalLogic).CallerAddress
	return c
}

// Caller returns the caller of the externalLogic.
//
// Caller will recursively call caller when the externalLogic is a delegate
// call, including that of caller's caller.
func (c *ExternalLogic) Caller() helper.Address {
	return c.CallerAddress
}

// Finalise finalises the externalLogic and returning any remaining gas to the original
// caller.
func (c *ExternalLogic) Finalise() {
	// Return the remaining gas to the caller
	c.caller.ReturnGas(c.Gas, c.Price)
}

// UseGas attempts the use gas and subtracts it and returns true on success
func (c *ExternalLogic) UseGas(gas *big.Int) (ok bool) {
	ok = useGas(c.Gas, gas)
	if ok {
		c.UsedGas.Add(c.UsedGas, gas)
	}
	return
}

// ReturnGas adds the given gas back to itself.
func (c *ExternalLogic) ReturnGas(gas, price *big.Int) {
	// Return the gas to the context
	c.Gas.Add(c.Gas, gas)
	c.UsedGas.Sub(c.UsedGas, gas)
}

// Address returns the externalLogics address
func (c *ExternalLogic) Address() helper.Address {
	return c.self.Address()
}

// Value returns the externalLogics value (sent to it from it's caller)
func (c *ExternalLogic) Value() *big.Int {
	return c.value
}
