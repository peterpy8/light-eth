package blockchainCore

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strings"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/configure"
)

// WriteGenesisBlock writes the genesis block to the database as block number 0
func WriteGenesisBlock(chainDb database.Database, reader io.Reader) (*types.Block, error) {
	contents, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var genesis struct {
		ChainConfig *configure.ChainConfig `json:"config"`
		Nonce       string
		Timestamp   string
		ParentHash  string
		ExtraData   string
		GasLimit    string
		Difficulty  string
		Mixhash     string
		Coinbase    string
		Alloc       map[string]struct {
			Code    string
			Storage map[string]string
			Balance string
		}
	}

	if err := json.Unmarshal(contents, &genesis); err != nil {
		return nil, err
	}

	genesis.ChainConfig.HomesteadBlock = configure.TestNetHomesteadBlock
	genesis.ChainConfig.SiotImpr1Block = configure.TestNetSpuriousDragon
	genesis.ChainConfig.SiotImpr2Block = configure.TestNetSpuriousDragon

	// creating with empty hash always works
	statedb, _ := state.New(helper.Hash{}, chainDb)
	for addr, account := range genesis.Alloc {
		address := helper.HexToAddress(addr)
		statedb.AddBalance(address, helper.String2Big(account.Balance))
		statedb.SetCode(address, helper.Hex2Bytes(account.Code))
		for key, value := range account.Storage {
			statedb.SetState(address, helper.HexToHash(key), helper.HexToHash(value))
		}
	}
	root, stateBatch := statedb.CommitBatch(false)

	difficulty := helper.String2Big(genesis.Difficulty)
	block := types.NewBlock(&types.Header{
		Nonce:      types.EncodeNonce(helper.String2Big(genesis.Nonce).Uint64()),
		Time:       helper.String2Big(genesis.Timestamp),
		ParentHash: helper.HexToHash(genesis.ParentHash),
		Extra:      helper.FromHex(genesis.ExtraData),
		GasLimit:   helper.String2Big(genesis.GasLimit),
		Difficulty: difficulty,
		MixDigest:  helper.HexToHash(genesis.Mixhash),
		Coinbase:   helper.HexToAddress(genesis.Coinbase),
		Root:       root,
	}, nil, nil, nil)

	if block := GetBlock(chainDb, block.Hash(), block.NumberU64()); block != nil {
		glog.V(logger.Info).Infoln("Genesis block already in chain. Writing canonical number")
		err := WriteCanonicalHash(chainDb, block.Hash(), block.NumberU64())
		if err != nil {
			return nil, err
		}
		return block, nil
	}

	if err := stateBatch.Write(); err != nil {
		return nil, fmt.Errorf("cannot write state: %v", err)
	}
	if err := WriteTd(chainDb, block.Hash(), block.NumberU64(), difficulty); err != nil {
		return nil, err
	}
	if err := WriteBlock(chainDb, block); err != nil {
		return nil, err
	}
	if err := WriteBlockReceipts(chainDb, block.Hash(), block.NumberU64(), nil); err != nil {
		return nil, err
	}
	if err := WriteCanonicalHash(chainDb, block.Hash(), block.NumberU64()); err != nil {
		return nil, err
	}
	if err := WriteHeadBlockHash(chainDb, block.Hash()); err != nil {
		return nil, err
	}
	if err := WriteChainConfig(chainDb, block.Hash(), genesis.ChainConfig); err != nil {
		return nil, err
	}

	return block, nil
}

// GenesisBlockForTesting creates a block in which addr has the given wei balance.
// The state trie of the block is written to db. the passed db needs to contain a state root
func GenesisBlockForTesting(db database.Database, addr helper.Address, balance *big.Int) *types.Block {
	statedb, _ := state.New(helper.Hash{}, db)
	obj := statedb.GetOrNewStateObject(addr)
	obj.SetBalance(balance)
	root, err := statedb.Commit(false)
	if err != nil {
		panic(fmt.Sprintf("cannot write state: %v", err))
	}
	block := types.NewBlock(&types.Header{
		Difficulty: configure.GenesisDifficulty,
		GasLimit:   configure.GenesisGasLimit,
		Root:       root,
	}, nil, nil, nil)
	return block
}

type GenesisAccount struct {
	Address helper.Address
	Balance *big.Int
}

func WriteGenesisBlockForTesting(db database.Database, accounts ...GenesisAccount) *types.Block {
	accountJson := "{"
	for i, account := range accounts {
		if i != 0 {
			accountJson += ","
		}
		accountJson += fmt.Sprintf(`"0x%x":{"balance":"0x%x"}`, account.Address, account.Balance.Bytes())
	}
	accountJson += "}"

	testGenesis := fmt.Sprintf(`{
	"nonce":"0x%x",
	"gasLimit":"0x%x",
	"difficulty":"0x%x",
	"alloc": %s
}`, types.EncodeNonce(0), configure.GenesisGasLimit.Bytes(), configure.GenesisDifficulty.Bytes(), accountJson)
	block, _ := WriteGenesisBlock(db, strings.NewReader(testGenesis))
	return block
}

// WriteDefaultGenesisBlock assembles the official Siotchain genesis block and
// writes it - along with all associated state - into a chain database.
func WriteDefaultGenesisBlock(chainDb database.Database) (*types.Block, error) {
	return WriteGenesisBlock(chainDb, strings.NewReader(DefaultGenesisBlock()))
}

// WriteTestNetGenesisBlock assembles the Morden test network genesis block and
// writes it - along with all associated state - into a chain database.
func WriteTestNetGenesisBlock(chainDb database.Database) (*types.Block, error) {
	return WriteGenesisBlock(chainDb, strings.NewReader(TestNetGenesisBlock()))
}

// WriteOlympicGenesisBlock assembles the Olympic genesis block and writes it
// along with all associated state into a chain database.
func WriteOlympicGenesisBlock(db database.Database) (*types.Block, error) {
	return WriteGenesisBlock(db, strings.NewReader(OlympicGenesisBlock()))
}

// DefaultGenesisBlock assembles a JSON string representing the default Siotchain
// genesis block.
func DefaultGenesisBlock() string {
	reader, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(defaultGenesisBlock)))
	if err != nil {
		panic(fmt.Sprintf("failed to access default genesis: %v", err))
	}
	blob, err := ioutil.ReadAll(reader)
	if err != nil {
		panic(fmt.Sprintf("failed to load default genesis: %v", err))
	}
	return string(blob)
}

// OlympicGenesisBlock assembles a JSON string representing the Olympic genesis
// block.
func OlympicGenesisBlock() string {
	return fmt.Sprintf(`{
		"nonce":"0x%x",
		"gasLimit":"0x%x",
		"difficulty":"0x%x",
		"alloc": {
			"0000000000000000000000000000000000000001": {"balance": "1"},
			"0000000000000000000000000000000000000002": {"balance": "1"},
			"0000000000000000000000000000000000000003": {"balance": "1"},
			"0000000000000000000000000000000000000004": {"balance": "1"},
			"dbdbdb2cbd23b783741e8d7fcf51e459b497e4a6": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"e4157b34ea9615cfbde6b4fda419828124b70c78": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"b9c015918bdaba24b4ff057a92a3873d6eb201be": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"6c386a4b26f73c802f34673f7248bb118f97424a": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"cd2a3d9f938e13cd947ec05abc7fe734df8dd826": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"2ef47100e0787b915105fd5e3f4ff6752079d5cb": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"e6716f9544a56c530d868e4bfbacb172315bdead": {"balance": "1606938044258990275541962092341162602522202993782792835301376"},
			"1a26338f0d905e295fccb71fa9ea849ffa12aaf4": {"balance": "1606938044258990275541962092341162602522202993782792835301376"}
		}
	}`, types.EncodeNonce(42), configure.GenesisGasLimit.Bytes(), configure.GenesisDifficulty.Bytes())
}

// TestNetGenesisBlock assembles a JSON string representing the Morden test net
// genenis block.
func TestNetGenesisBlock() string {
	return fmt.Sprintf(`{
		"nonce": "0x%x",
		"difficulty": "0x20000",
		"mixhash": "0x00000000000000000000000000000000000000647572616c65787365646c6578",
		"coinbase": "0x0000000000000000000000000000000000000000",
		"timestamp": "0x00",
		"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"extraData": "0x",
		"gasLimit": "0x2FEFD8",
		"alloc": {
			"0000000000000000000000000000000000000001": { "balance": "1" },
			"0000000000000000000000000000000000000002": { "balance": "1" },
			"0000000000000000000000000000000000000003": { "balance": "1" },
			"0000000000000000000000000000000000000004": { "balance": "1" },
			"102e61f5d8f9bc71d0ad4a084df4e65e05ce0e1c": { "balance": "1606938044258990275541962092341162602522202993782792835301376" }
		}
	}`, types.EncodeNonce(0x6d6f7264656e))
}
