package localEnv

import (
	"math/big"

	"github.com/siotchain/siot/configure"
)

var (
	GasQuickStep   = big.NewInt(2)
	GasFastestStep = big.NewInt(3)
	GasFastStep    = big.NewInt(5)
	GasMidStep     = big.NewInt(8)
	GasSlowStep    = big.NewInt(10)
	GasExtStep     = big.NewInt(20)

	GasReturn = big.NewInt(0)
	GasStop   = big.NewInt(0)

	GasExternalLogicByte = big.NewInt(200)

	n64 = big.NewInt(64)
)

// calcGas returns the actual gas cost of the call.
//
// The cost of gas was changed during the homestead price change HF. To allow for EIP150
// to be implemented. The returned gas is gas - base * 63 / 64.
func callGas(gasTable configure.GasTable, availableGas, base, callCost *big.Int) *big.Int {
	if gasTable.CreateBySuicide != nil {
		availableGas = new(big.Int).Sub(availableGas, base)
		g := new(big.Int).Div(availableGas, n64)
		g.Sub(availableGas, g)

		if g.Cmp(callCost) < 0 {
			return g
		}
	}
	return callCost
}

// casts a arbitrary number to the amount of words (sets of 32 bytes)
func toWordSize(size *big.Int) *big.Int {
	tmp := new(big.Int)
	tmp.Add(size, u256(31))
	tmp.Div(tmp, u256(32))
	return tmp
}

type req struct {
	stackPop  int
	gas       *big.Int
	stackPush int
}