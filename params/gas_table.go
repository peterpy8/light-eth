package params

import "math/big"

type GasTable struct {
	ExtcodeSize *big.Int
	ExtcodeCopy *big.Int
	Balance     *big.Int
	SLoad       *big.Int
	Calls       *big.Int
	Suicide     *big.Int

	ExpByte *big.Int

	// CreateBySuicide occurs when the
	// refunded account is one that does
	// not exist. This logic is similar
	// to call. May be left nil. Nil means
	// not charged.
	CreateBySuicide *big.Int
}

var (
	// GasTableHomestead contain the gas prices for
	// the homestead phase.
	GasTableHomestead = GasTable{
		ExtcodeSize: big.NewInt(20),
		ExtcodeCopy: big.NewInt(20),
		Balance:     big.NewInt(20),
		SLoad:       big.NewInt(50),
		Calls:       big.NewInt(40),
		Suicide:     big.NewInt(0),
		ExpByte:     big.NewInt(10),

		// explicitly set to nil to indicate
		// this rule does not apply to homestead.
		CreateBySuicide: nil,
	}

	// GasTableHomestead contain the gas re-prices for
	// the homestead phase.
	//
	// TODO rename to GasTableEIP150
	GasTableHomesteadGasRepriceFork = GasTable{
		ExtcodeSize: big.NewInt(700),
		ExtcodeCopy: big.NewInt(700),
		Balance:     big.NewInt(400),
		SLoad:       big.NewInt(200),
		Calls:       big.NewInt(700),
		Suicide:     big.NewInt(5000),
		ExpByte:     big.NewInt(10),

		CreateBySuicide: big.NewInt(25000),
	}

	GasTableEIP158 = GasTable{
		ExtcodeSize: big.NewInt(700),
		ExtcodeCopy: big.NewInt(700),
		Balance:     big.NewInt(400),
		SLoad:       big.NewInt(200),
		Calls:       big.NewInt(700),
		Suicide:     big.NewInt(5000),
		ExpByte:     big.NewInt(50),

		CreateBySuicide: big.NewInt(25000),
	}
)
