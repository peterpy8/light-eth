package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/params"
)

// PrecompiledAccount represents a native Siotchain externalLogic
type PrecompiledAccount struct {
	Gas func(l int) *big.Int
	fn  func(in []byte) []byte
}

// Call calls the native function
func (self PrecompiledAccount) Call(in []byte) []byte {
	return self.fn(in)
}

// Precompiled contains the default set of Siotchain externalLogics
var Precompiled = PrecompiledExternalLogics()

// PrecompiledExternalLogics returns the default set of precompiled Siotchain
// externalLogics defined by the Siotchain yellow paper.
func PrecompiledExternalLogics() map[string]*PrecompiledAccount {
	return map[string]*PrecompiledAccount{
		// ECRECOVER
		string(helper.LeftPadBytes([]byte{1}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			return params.EcrecoverGas
		}, ecrecoverFunc},

		// SHA256
		string(helper.LeftPadBytes([]byte{2}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, params.Sha256WordGas)
			return n.Add(n, params.Sha256Gas)
		}, sha256Func},

		// RIPEMD160
		string(helper.LeftPadBytes([]byte{3}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, params.Ripemd160WordGas)
			return n.Add(n, params.Ripemd160Gas)
		}, ripemd160Func},

		string(helper.LeftPadBytes([]byte{4}, 20)): &PrecompiledAccount{func(l int) *big.Int {
			n := big.NewInt(int64(l+31) / 32)
			n.Mul(n, params.IdentityWordGas)

			return n.Add(n, params.IdentityGas)
		}, memCpy},
	}
}

func sha256Func(in []byte) []byte {
	return crypto.Sha256(in)
}

func ripemd160Func(in []byte) []byte {
	return helper.LeftPadBytes(crypto.Ripemd160(in), 32)
}

const ecRecoverInputLength = 128

func ecrecoverFunc(in []byte) []byte {
	in = helper.RightPadBytes(in, 128)
	// "in" is (hash, v, r, s), each 32 bytes
	// but for ecrecover we want (r, s, v)

	r := helper.BytesToBig(in[64:96])
	s := helper.BytesToBig(in[96:128])
	// Treat V as a 256bit integer
	vbig := helper.Bytes2Big(in[32:64])
	v := byte(vbig.Uint64())

	// tighter sig s values in homestead only apply to tx sigs
	if !crypto.ValidateSignatureValues(v, r, s, false) {
		glog.V(logger.Detail).Infof("ECRECOVER error: v, r or s value invalid")
		return nil
	}

	// v needs to be at the end and normalized for libsecp256k1
	vbignormal := new(big.Int).Sub(vbig, big.NewInt(27))
	vnormal := byte(vbignormal.Uint64())
	rsv := append(in[64:128], vnormal)
	pubKey, err := crypto.Ecrecover(in[:32], rsv)
	// make sure the public key is a valid one
	if err != nil {
		glog.V(logger.Detail).Infoln("ECRECOVER error: ", err)
		return nil
	}

	// the first byte of pubkey is bitcoin heritage
	return helper.LeftPadBytes(crypto.Keccak256(pubKey[1:])[12:], 32)
}

func memCpy(in []byte) []byte {
	return in
}
