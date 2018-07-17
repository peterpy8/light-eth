package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/helper"
	"github.com/ethereum/go-ethereum/helper/rlp"
)

var errMissingLogFields = errors.New("missing required JSON log fields")

// Log represents a externalLogic log event. These events are generated by the LOG
// opcode and stored/indexed by the node.
type Log struct {
	// Consensus fields.
	Address helper.Address // address of the externalLogic that generated the event
	Topics  []helper.Hash  // list of topics provided by the externalLogic.
	Data    []byte         // supplied by the externalLogic, usually ABI-encoded

	// Derived fields (don't reorder!).
	BlockNumber uint64      // block in which the transaction was included
	TxHash      helper.Hash // hash of the transaction
	TxIndex     uint        // index of the transaction in the block
	BlockHash   helper.Hash // hash of the block in which the transaction was included
	Index       uint        // index of the log in the receipt
}

type jsonLog struct {
	Address     *helper.Address `json:"address"`
	Topics      *[]helper.Hash  `json:"topics"`
	Data        string          `json:"data"`
	BlockNumber string          `json:"blockNumber"`
	TxIndex     string          `json:"transactionIndex"`
	TxHash      *helper.Hash    `json:"transactionHash"`
	BlockHash   *helper.Hash    `json:"blockHash"`
	Index       string          `json:"logIndex"`
}

func NewLog(address helper.Address, topics []helper.Hash, data []byte, number uint64) *Log {
	return &Log{Address: address, Topics: topics, Data: data, BlockNumber: number}
}

func (l *Log) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{l.Address, l.Topics, l.Data})
}

func (l *Log) DecodeRLP(s *rlp.Stream) error {
	var log struct {
		Address helper.Address
		Topics  []helper.Hash
		Data    []byte
	}
	if err := s.Decode(&log); err != nil {
		return err
	}
	l.Address, l.Topics, l.Data = log.Address, log.Topics, log.Data
	return nil
}

func (l *Log) String() string {
	return fmt.Sprintf(`log: %x %x %x %x %d %x %d`, l.Address, l.Topics, l.Data, l.TxHash, l.TxIndex, l.BlockHash, l.Index)
}

// MarshalJSON implements json.Marshaler.
func (r *Log) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonLog{
		Address:     &r.Address,
		Topics:      &r.Topics,
		Data:        fmt.Sprintf("0x%x", r.Data),
		BlockNumber: fmt.Sprintf("0x%x", r.BlockNumber),
		TxIndex:     fmt.Sprintf("0x%x", r.TxIndex),
		TxHash:      &r.TxHash,
		BlockHash:   &r.BlockHash,
		Index:       fmt.Sprintf("0x%x", r.Index),
	})
}

// UnmarshalJSON implements json.Umarshaler.
func (r *Log) UnmarshalJSON(input []byte) error {
	var dec jsonLog
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Address == nil || dec.Topics == nil || dec.Data == "" || dec.BlockNumber == "" ||
		dec.TxIndex == "" || dec.TxHash == nil || dec.BlockHash == nil || dec.Index == "" {
		return errMissingLogFields
	}
	declog := Log{
		Address:   *dec.Address,
		Topics:    *dec.Topics,
		TxHash:    *dec.TxHash,
		BlockHash: *dec.BlockHash,
	}
	if _, err := fmt.Sscanf(dec.Data, "0x%x", &declog.Data); err != nil {
		return fmt.Errorf("invalid hex log data")
	}
	if _, err := fmt.Sscanf(dec.BlockNumber, "0x%x", &declog.BlockNumber); err != nil {
		return fmt.Errorf("invalid hex log block number")
	}
	if _, err := fmt.Sscanf(dec.TxIndex, "0x%x", &declog.TxIndex); err != nil {
		return fmt.Errorf("invalid hex log tx index")
	}
	if _, err := fmt.Sscanf(dec.Index, "0x%x", &declog.Index); err != nil {
		return fmt.Errorf("invalid hex log index")
	}
	*r = declog
	return nil
}

type Logs []*Log

// LogForStorage is a wrapper around a Log that flattens and parses the entire
// content of a log, as opposed to only the consensus fields originally (by hiding
// the rlp interface methods).
type LogForStorage Log
