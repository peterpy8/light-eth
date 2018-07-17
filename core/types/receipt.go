package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/core/vm"
	"github.com/siotchain/siot/helper/rlp"
)

var (
	errMissingReceiptPostState = errors.New("missing post state root in JSON receipt")
	errMissingReceiptFields    = errors.New("missing required JSON receipt fields")
)

// Receipt represents the results of a transaction.
type Receipt struct {
	// Consensus fields
	PostState         []byte
	CumulativeGasUsed *big.Int
	Bloom             Bloom
	Logs              vm.Logs

	// Implementation fields (don't reorder!)
	TxHash               helper.Hash
	ExternalLogicAddress helper.Address
	GasUsed              *big.Int
}

type jsonReceipt struct {
	PostState         *helper.Hash         `json:"root"`
	CumulativeGasUsed *hexBig              `json:"cumulativeGasUsed"`
	Bloom             *Bloom               `json:"logsBloom"`
	Logs              *vm.Logs             `json:"logs"`
	TxHash            *helper.Hash         `json:"transactionHash"`
	ExternalLogicAddress   *helper.Address `json:"externalLogicAddress"`
	GasUsed           *hexBig              `json:"gasUsed"`
}

// NewReceipt creates a barebone transaction receipt, copying the init fields.
func NewReceipt(root []byte, cumulativeGasUsed *big.Int) *Receipt {
	return &Receipt{PostState: helper.CopyBytes(root), CumulativeGasUsed: new(big.Int).Set(cumulativeGasUsed)}
}

// EncodeRLP implements rlp.Encoder, and flattens the consensus fields of a receipt
// into an RLP stream.
func (r *Receipt) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{r.PostState, r.CumulativeGasUsed, r.Bloom, r.Logs})
}

// DecodeRLP implements rlp.Decoder, and loads the consensus fields of a receipt
// from an RLP stream.
func (r *Receipt) DecodeRLP(s *rlp.Stream) error {
	var receipt struct {
		PostState         []byte
		CumulativeGasUsed *big.Int
		Bloom             Bloom
		Logs              vm.Logs
	}
	if err := s.Decode(&receipt); err != nil {
		return err
	}
	r.PostState, r.CumulativeGasUsed, r.Bloom, r.Logs = receipt.PostState, receipt.CumulativeGasUsed, receipt.Bloom, receipt.Logs
	return nil
}

// MarshalJSON encodes receipts into the web3 RPC response block format.
func (r *Receipt) MarshalJSON() ([]byte, error) {
	root := helper.BytesToHash(r.PostState)

	return json.Marshal(&jsonReceipt{
		PostState:         &root,
		CumulativeGasUsed: (*hexBig)(r.CumulativeGasUsed),
		Bloom:             &r.Bloom,
		Logs:              &r.Logs,
		TxHash:            &r.TxHash,
		ExternalLogicAddress:   &r.ExternalLogicAddress,
		GasUsed:           (*hexBig)(r.GasUsed),
	})
}

// UnmarshalJSON decodes the web3 RPC receipt format.
func (r *Receipt) UnmarshalJSON(input []byte) error {
	var dec jsonReceipt
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	// Ensure that all fields are set. PostState is checked separately because it is a
	// recent addition to the RPC spec (as of August 2016) and older implementations might
	// not provide it. Note that ExternalLogicAddress is not checked because it can be null.
	if dec.PostState == nil {
		return errMissingReceiptPostState
	}
	if dec.CumulativeGasUsed == nil || dec.Bloom == nil ||
		dec.Logs == nil || dec.TxHash == nil || dec.GasUsed == nil {
		return errMissingReceiptFields
	}
	*r = Receipt{
		PostState:         (*dec.PostState)[:],
		CumulativeGasUsed: (*big.Int)(dec.CumulativeGasUsed),
		Bloom:             *dec.Bloom,
		Logs:              *dec.Logs,
		TxHash:            *dec.TxHash,
		GasUsed:           (*big.Int)(dec.GasUsed),
	}
	if dec.ExternalLogicAddress != nil {
		r.ExternalLogicAddress = *dec.ExternalLogicAddress
	}
	return nil
}

// String implements the Stringer interface.
func (r *Receipt) String() string {
	return fmt.Sprintf("receipt{med=%x cgas=%v bloom=%x logs=%v}", r.PostState, r.CumulativeGasUsed, r.Bloom, r.Logs)
}

// ReceiptForStorage is a wrapper around a Receipt that flattens and parses the
// entire content of a receipt, as opposed to only the consensus fields originally.
type ReceiptForStorage Receipt

// EncodeRLP implements rlp.Encoder, and flattens all content fields of a receipt
// into an RLP stream.
func (r *ReceiptForStorage) EncodeRLP(w io.Writer) error {
	logs := make([]*vm.LogForStorage, len(r.Logs))
	for i, log := range r.Logs {
		logs[i] = (*vm.LogForStorage)(log)
	}
	return rlp.Encode(w, []interface{}{r.PostState, r.CumulativeGasUsed, r.Bloom, r.TxHash, r.ExternalLogicAddress, logs, r.GasUsed})
}

// DecodeRLP implements rlp.Decoder, and loads both consensus and implementation
// fields of a receipt from an RLP stream.
func (r *ReceiptForStorage) DecodeRLP(s *rlp.Stream) error {
	var receipt struct {
		PostState            []byte
		CumulativeGasUsed    *big.Int
		Bloom                Bloom
		TxHash               helper.Hash
		ExternalLogicAddress helper.Address
		Logs                 []*vm.LogForStorage
		GasUsed              *big.Int
	}
	if err := s.Decode(&receipt); err != nil {
		return err
	}
	// Assign the consensus fields
	r.PostState, r.CumulativeGasUsed, r.Bloom = receipt.PostState, receipt.CumulativeGasUsed, receipt.Bloom
	r.Logs = make(vm.Logs, len(receipt.Logs))
	for i, log := range receipt.Logs {
		r.Logs[i] = (*vm.Log)(log)
	}
	// Assign the implementation fields
	r.TxHash, r.ExternalLogicAddress, r.GasUsed = receipt.TxHash, receipt.ExternalLogicAddress, receipt.GasUsed

	return nil
}

// Receipts is a wrapper around a Receipt array to implement DerivableList.
type Receipts []*Receipt

// Len returns the number of receipts in this list.
func (r Receipts) Len() int { return len(r) }

// GetRlp returns the RLP encoding of one receipt from the list.
func (r Receipts) GetRlp(i int) []byte {
	bytes, err := rlp.EncodeToBytes(r[i])
	if err != nil {
		panic(err)
	}
	return bytes
}
