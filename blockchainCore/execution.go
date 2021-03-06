package blockchainCore

import (
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/localEnv"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/configure"
)

// Call executes within the given externalLogic
func Call(env localEnv.Environment, caller localEnv.ExternalLogicRef, addr helper.Address, input []byte, gas, gasPrice, value *big.Int) (ret []byte, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if env.Depth() > int(configure.CallCreateDepth.Int64()) {
		caller.ReturnGas(gas, gasPrice)

		return nil, localEnv.DepthError
	}
	if !env.CanTransfer(caller.Address(), value) {
		caller.ReturnGas(gas, gasPrice)

		return nil, ValueTransferErr("insufficient funds to transfer value. Req %v, has %v", value, env.Db().GetBalance(caller.Address()))
	}

	snapshotPreTransfer := env.SnapshotDatabase()
	var (
		from  = env.Db().GetAccount(caller.Address())
		to   localEnv.Account
	)
	if !env.Db().Exist(addr) {
		if localEnv.Precompiled[addr.Str()] == nil && env.ChainConfig().IsSiotImpr2(env.BlockNumber()) && value.BitLen() == 0 {
			caller.ReturnGas(gas, gasPrice)
			return nil, nil
		}

		to = env.Db().CreateAccount(addr)
	} else {
		to = env.Db().GetAccount(addr)
	}
	env.Transfer(from, to, value)

	// initialise a new externalLogic and set the code that is to be used by the
	// Siotchain Env. The externalLogic is a scoped environment for this execution context
	// only.
	externalLogic := localEnv.NewExternalLogic(caller, to, value, gas, gasPrice)
	//externalLogic.SetCallCode(&addr, env.Db().GetCodeHash(addr), env.Db().GetCode(addr))
	defer externalLogic.Finalise()

	//ret, err = env.Vm().Run(externalLogic, input)
	// When an error was returned by the Siotchain Env or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		externalLogic.UseGas(externalLogic.Gas)

		env.RevertToSnapshot(snapshotPreTransfer)
	}
	//return ret, err
	return nil, nil
}

// CallCode executes the given address' code as the given externalLogic address
func CallCode(env localEnv.Environment, caller localEnv.ExternalLogicRef, addr helper.Address, input []byte, gas, gasPrice, value *big.Int) (ret []byte, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if env.Depth() > int(configure.CallCreateDepth.Int64()) {
		caller.ReturnGas(gas, gasPrice)

		return nil, localEnv.DepthError
	}
	if !env.CanTransfer(caller.Address(), value) {
		caller.ReturnGas(gas, gasPrice)

		return nil, ValueTransferErr("insufficient funds to transfer value. Req %v, has %v", value, env.Db().GetBalance(caller.Address()))
	}

	var (
		snapshotPreTransfer = env.SnapshotDatabase()
		to                  = env.Db().GetAccount(caller.Address())
	)
	// initialise a new externalLogic and set the code that is to be used by the
	// Siotchain Env. The externalLogic is a scoped environment for this execution context
	// only.
	externalLogic := localEnv.NewExternalLogic(caller, to, value, gas, gasPrice)
	//externalLogic.SetCallCode(&addr, env.Db().GetCodeHash(addr), env.Db().GetCode(addr))
	defer externalLogic.Finalise()

	//ret, err = env.Vm().Run(externalLogic, input)
	if err != nil {
		externalLogic.UseGas(externalLogic.Gas)

		env.RevertToSnapshot(snapshotPreTransfer)
	}

	return nil, nil
}

// Create creates a new externalLogic with the given code
func Create(env localEnv.Environment, caller localEnv.ExternalLogicRef, code []byte, gas, gasPrice, value *big.Int) (ret []byte, address helper.Address, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if env.Depth() > int(configure.CallCreateDepth.Int64()) {
		caller.ReturnGas(gas, gasPrice)

		return nil, helper.Address{}, localEnv.DepthError
	}
	if !env.CanTransfer(caller.Address(), value) {
		caller.ReturnGas(gas, gasPrice)

		return nil, helper.Address{}, ValueTransferErr("insufficient funds to transfer value. Req %v, has %v", value, env.Db().GetBalance(caller.Address()))
	}

	// Create a new account on the state
	nonce := env.Db().GetNonce(caller.Address())
	env.Db().SetNonce(caller.Address(), nonce+1)

	snapshotPreTransfer := env.SnapshotDatabase()
	var (
		addr = crypto.CreateAddress(caller.Address(), nonce)
		from = env.Db().GetAccount(caller.Address())
		to   = env.Db().CreateAccount(addr)
	)
	if env.ChainConfig().IsSiotImpr2(env.BlockNumber()) {
		env.Db().SetNonce(addr, 1)
	}
	env.Transfer(from, to, value)

	// initialise a new externalLogic and set the code that is to be used by the
	// Siotchain Env. The externalLogic is a scoped environment for this execution context
	// only.
	externalLogic := localEnv.NewExternalLogic(caller, to, value, gas, gasPrice)
	//externalLogic.SetCallCode(&addr, crypto.Keccak256Hash(code), code)
	defer externalLogic.Finalise()

	//ret, err = env.Vm().Run(externalLogic, nil)
	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > configure.MaxCodeSize
	// if the externalLogic creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		dataGas := big.NewInt(int64(len(ret)))
		dataGas.Mul(dataGas, configure.CreateDataGas)
		if externalLogic.UseGas(dataGas) {
			env.Db().SetCode(addr, ret)
		} else {
			err = localEnv.CodeStoreOutOfGasError
		}
	}

	// When an error was returned by the Siotchain Env or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if maxCodeSizeExceeded ||
		(err != nil && (env.ChainConfig().IsHomestead(env.BlockNumber()) || err != localEnv.CodeStoreOutOfGasError)) {
		externalLogic.UseGas(externalLogic.Gas)
		env.RevertToSnapshot(snapshotPreTransfer)

		// Nothing should be returned when an error is thrown.
		return nil, addr, err
	}

	//return ret, addr, err
	return nil, addr, nil
}

// DelegateCall is equivalent to CallCode except that sender and value propagates from parent scope to child scope
func DelegateCall(env localEnv.Environment, caller localEnv.ExternalLogicRef, addr helper.Address, input []byte, gas, gasPrice *big.Int) (ret []byte, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if env.Depth() > int(configure.CallCreateDepth.Int64()) {
		caller.ReturnGas(gas, gasPrice)
		return nil, localEnv.DepthError
	}

	var (
		snapshot = env.SnapshotDatabase()
		to       = env.Db().GetAccount(caller.Address())
	)

	// Iinitialise a new externalLogic and make initialise the delegate values
	externalLogic := localEnv.NewExternalLogic(caller, to, caller.Value(), gas, gasPrice).AsDelegate()
	//externalLogic.SetCallCode(&addr, env.Db().GetCodeHash(addr), env.Db().GetCode(addr))
	defer externalLogic.Finalise()

	//ret, err = env.Vm().Run(externalLogic, input)
	if err != nil {
		externalLogic.UseGas(externalLogic.Gas)

		env.RevertToSnapshot(snapshot)
	}

	//return ret, err
	return nil, nil
}

// generic transfer method
func Transfer(from, to localEnv.Account, amount *big.Int) {
	from.SubBalance(amount)
	to.AddBalance(amount)
}
