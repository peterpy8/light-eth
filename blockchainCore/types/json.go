package types

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

// JSON unmarshaling utilities.

type hexBytes []byte

func (b *hexBytes) MarshalJSON() ([]byte, error) {
	if b != nil {
		return []byte(fmt.Sprintf(`"0x%x"`, []byte(*b))), nil
	}
	return nil, nil
}

func (b *hexBytes) UnmarshalJSON(input []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return fmt.Errorf("cannot unmarshal non-string into hexBytes")
	}
	input = input[1 : len(input)-1]
	if len(input) < 2 || input[0] != '0' || input[1] != 'x' {
		return fmt.Errorf("missing 0x prefix in hexBytes input %q", input)
	}
	dec := make(hexBytes, (len(input)-2)/2)
	if _, err := hex.Decode(dec, input[2:]); err != nil {
		return err
	}
	*b = dec
	return nil
}

type hexBig big.Int

func (b *hexBig) MarshalJSON() ([]byte, error) {
	if b != nil {
		return []byte(fmt.Sprintf(`"0x%x"`, (*big.Int)(b))), nil
	}
	return nil, nil
}

func (b *hexBig) UnmarshalJSON(input []byte) error {
	raw, err := checkHexNumber(input)
	if err != nil {
		return err
	}
	dec, ok := new(big.Int).SetString(string(raw), 16)
	if !ok {
		return fmt.Errorf("invalid hex number")
	}
	*b = (hexBig)(*dec)
	return nil
}

type hexUint64 uint64

func (b *hexUint64) MarshalJSON() ([]byte, error) {
	if b != nil {
		return []byte(fmt.Sprintf(`"0x%x"`, *(*uint64)(b))), nil
	}
	return nil, nil
}

func (b *hexUint64) UnmarshalJSON(input []byte) error {
	raw, err := checkHexNumber(input)
	if err != nil {
		return err
	}
	_, err = fmt.Sscanf(string(raw), "%x", b)
	return err
}

func checkHexNumber(input []byte) (raw []byte, err error) {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return nil, fmt.Errorf("cannot unmarshal non-string into hex number")
	}
	input = input[1 : len(input)-1]
	if len(input) < 2 || input[0] != '0' || input[1] != 'x' {
		return nil, fmt.Errorf("missing 0x prefix in hex number input %q", input)
	}
	if len(input) == 2 {
		return nil, fmt.Errorf("empty hex number")
	}
	raw = input[2:]
	if len(raw)%2 != 0 {
		raw = append([]byte{'0'}, raw...)
	}
	return raw, nil
}
