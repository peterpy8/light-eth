// Package siot implements the Siotchain protocol.
package siot

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/ethash"
	"github.com/siotchain/siot/wallet"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/helper/httpclient"
	"github.com/siotchain/siot/blockchainCore"
	"github.com/siotchain/siot/blockchainCore/types"
	"github.com/siotchain/siot/siot/downloader"
	"github.com/siotchain/siot/siot/gasprice"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/internal/siotapi"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/miner"
	"github.com/siotchain/siot/context"
	"github.com/siotchain/siot/net/p2p"
	"github.com/siotchain/siot/configure"
	"github.com/siotchain/siot/net/rpc"
)

const (
	epochLength    = 30000
	ethashRevision = 23

	autoDAGcheckInterval = 10 * time.Hour
	autoDAGepochHeight   = epochLength / 2
)

var (
	datadirInUseErrnos = map[uint]bool{11: true, 32: true, 35: true}
	portInUseErrRE     = regexp.MustCompile("address already in use")
)

type Config struct {
	ChainConfig *configure.ChainConfig // chain configuration

	NetworkId  int    // Network ID to use for selecting peers to connect to
	Genesis    string // Genesis JSON to seed the chain database with
	FastSync   bool   // Enables the state download based fast synchronisation algorithm
	LightMode  bool   // Running in light client mode
	LightServ  int    // Maximum percentage of time allowed for serving LES requests
	LightPeers int    // Maximum number of LES client peers
	MaxPeers   int    // Maximum number of global peers

	SkipBcVersionCheck bool // e.g. blockchain export
	DatabaseCache      int
	DatabaseHandles    int

	NatSpec   bool
	DocRoot   string
	AutoDAG   bool
	PowTest   bool
	PowShared bool
	ExtraData []byte

	MinerAddr    helper.Address
	GasPrice     *big.Int
	MinerThreads int

	GpoMinGasPrice          *big.Int
	GpoMaxGasPrice          *big.Int
	GpoFullBlockRatio       int
	GpobaseStepDown         int
	GpobaseStepUp           int
	GpobaseCorrectionFactor int

	EnableJit bool
	ForceJit  bool

	TestGenesisBlock *types.Block      // Genesis block to seed the chain database with (testing only!)
	TestGenesisState database.Database // Genesis state to seed the database with (testing only!)
}

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// Siotchain implements the Siotchain full node service.
type Siotchain struct {
	chainConfig *configure.ChainConfig
	// Channel for shutting down the service
	shutdownChan  chan bool // Channel for shutting down the Siotchain
	stopDbUpgrade func()    // stop chain db sequential key upgrade
	// Handlers
	txPool          *blockchainCore.TxPool
	txMu            sync.Mutex
	blockchain      *blockchainCore.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer
	// DB interfaces
	chainDb database.Database // Block chain database

	eventMux       *subscribe.TypeMux
	pow            *ethash.Ethash
	httpclient     *httpclient.HTTPClient
	accountManager *wallet.Manager

	ApiBackend *SiotApiBackend

	miner        *miner.Miner
	Mining       bool
	MinerThreads int
	AutoDAG      bool
	autodagquit  chan bool
	mineraddr    helper.Address

	NatSpec       bool
	PowTest       bool
	netVersionId  int
	netRPCService *siotapi.PublicNetAPI
}

func (s *Siotchain) AddLesServer(ls LesServer) {
	s.lesServer = ls
}

// New creates a new Siotchain object (including the
// initialisation of the helper Siotchain object)
func New(ctx *context.ServiceContext, config *Config) (*Siotchain, error) {
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	stopDbUpgrade := upgradeSequentialKeys(chainDb)
	if err := SetupGenesisBlock(&chainDb, config); err != nil {
		return nil, err
	}
	pow, err := CreatePoW(config)
	if err != nil {
		return nil, err
	}

	siot := &Siotchain{
		chainDb:        chainDb,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		pow:            pow,
		shutdownChan:   make(chan bool),
		stopDbUpgrade:  stopDbUpgrade,
		httpclient:     httpclient.New(config.DocRoot),
		netVersionId:   config.NetworkId,
		NatSpec:        config.NatSpec,
		PowTest:        config.PowTest,
		mineraddr:      config.MinerAddr,
		MinerThreads:   config.MinerThreads,
		AutoDAG:        config.AutoDAG,
	}

	if err := upgradeChainDatabase(chainDb); err != nil {
		return nil, err
	}
	if err := addMipmapBloomBins(chainDb); err != nil {
		return nil, err
	}

	glog.V(logger.Debug).Infof("Protocol Versions: %v, Network Id: %v", ProtocolVersions, config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := blockchainCore.GetBlockChainVersion(chainDb)
		if bcVersion != blockchainCore.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run siotchain upgradedb.\n", bcVersion, blockchainCore.BlockChainVersion)
		}
		blockchainCore.WriteBlockChainVersion(chainDb, blockchainCore.BlockChainVersion)
	}

	// load the genesis block or write a new one if no genesis
	// block is prenent in the database.
	genesis := blockchainCore.GetBlock(chainDb, blockchainCore.GetCanonicalHash(chainDb, 0), 0)
	if genesis == nil {
		genesis, err = blockchainCore.WriteDefaultGenesisBlock(chainDb)
		if err != nil {
			return nil, err
		}
		glog.V(logger.Info).Infoln("WARNING: Wrote default Siotchain genesis block")
	}

	if config.ChainConfig == nil {
		return nil, errors.New("missing chain config")
	}
	blockchainCore.WriteChainConfig(chainDb, genesis.Hash(), config.ChainConfig)

	siot.chainConfig = config.ChainConfig

	siot.blockchain, err = blockchainCore.NewBlockChain(chainDb, siot.chainConfig, siot.pow, siot.EventMux())
	if err != nil {
		if err == blockchainCore.ErrNoGenesis {
			return nil, fmt.Errorf(`No chain found. Please initialise a new chain using the "init" subcommand.`)
		}
		return nil, err
	}
	newPool := blockchainCore.NewTxPool(siot.chainConfig, siot.EventMux(), siot.blockchain.State, siot.blockchain.GasLimit)
	siot.txPool = newPool

	maxPeers := config.MaxPeers
	if config.LightServ > 0 {
		// if we are running a light server, limit the number of Siot peers so that we reserve some space for incoming LES connections
		// temporary solution until the new peer connectivity API is finished
		halfPeers := maxPeers / 2
		maxPeers -= config.LightPeers
		if maxPeers < halfPeers {
			maxPeers = halfPeers
		}
	}

	if siot.protocolManager, err = NewProtocolManager(siot.chainConfig, config.FastSync, config.NetworkId, maxPeers, siot.eventMux, siot.txPool, siot.pow, siot.blockchain, chainDb); err != nil {
		return nil, err
	}
	siot.miner = miner.New(siot, siot.chainConfig, siot.EventMux(), siot.pow)
	siot.miner.SetGasPrice(config.GasPrice)
	siot.miner.SetExtra(config.ExtraData)

	gpoParams := &gasprice.GpoParams{
		GpoMinGasPrice:          config.GpoMinGasPrice,
		GpoMaxGasPrice:          config.GpoMaxGasPrice,
		GpoFullBlockRatio:       config.GpoFullBlockRatio,
		GpobaseStepDown:         config.GpobaseStepDown,
		GpobaseStepUp:           config.GpobaseStepUp,
		GpobaseCorrectionFactor: config.GpobaseCorrectionFactor,
	}
	gpo := gasprice.NewGasPriceOracle(siot.blockchain, chainDb, siot.eventMux, gpoParams)
	siot.ApiBackend = &SiotApiBackend{siot, gpo}

	return siot, nil
}

// CreateDB creates the chain database.
func CreateDB(ctx *context.ServiceContext, config *Config, name string) (database.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if db, ok := db.(*database.LDBDatabase); ok {
		db.Meter("siot/db/chaindata/")
	}
	return db, err
}

// SetupGenesisBlock initializes the genesis block for an Siotchain service
func SetupGenesisBlock(chainDb *database.Database, config *Config) error {
	// Load up any custom genesis block if requested
	if len(config.Genesis) > 0 {
		block, err := blockchainCore.WriteGenesisBlock(*chainDb, strings.NewReader(config.Genesis))
		if err != nil {
			return err
		}
		glog.V(logger.Info).Infof("Successfully wrote custom genesis block: %x", block.Hash())
	}
	// Load up a test setup if directly injected
	if config.TestGenesisState != nil {
		*chainDb = config.TestGenesisState
	}
	if config.TestGenesisBlock != nil {
		blockchainCore.WriteTd(*chainDb, config.TestGenesisBlock.Hash(), config.TestGenesisBlock.NumberU64(), config.TestGenesisBlock.Difficulty())
		blockchainCore.WriteBlock(*chainDb, config.TestGenesisBlock)
		blockchainCore.WriteCanonicalHash(*chainDb, config.TestGenesisBlock.Hash(), config.TestGenesisBlock.NumberU64())
		blockchainCore.WriteHeadBlockHash(*chainDb, config.TestGenesisBlock.Hash())
	}
	return nil
}

// CreatePoW creates the required type of PoW instance for an Siotchain service
func CreatePoW(config *Config) (*ethash.Ethash, error) {
	switch {
	case config.PowTest:
		glog.V(logger.Info).Infof("ethash used in test mode")
		return ethash.NewForTesting()
	case config.PowShared:
		glog.V(logger.Info).Infof("ethash used in shared mode")
		return ethash.NewShared(), nil

	default:
		return ethash.New(), nil
	}
}

// APIs returns the collection of RPC services the Siotchain package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Siotchain) APIs() []rpc.API {
	return append(siotapi.GetAPIs(s.ApiBackend), []rpc.API{
		{
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicSiotchainAPI(s),
			Public:    true,
		}, {
			Namespace: "siot",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "siot",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "manage",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		},
	}...)
}

func (s *Siotchain) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Siotchain) Mineraddr() (eb helper.Address, err error) {
	eb = s.mineraddr
	if (eb == helper.Address{}) {
		firstAccount, err := s.AccountManager().AccountByIndex(0)
		eb = firstAccount.Address
		if err != nil {
			return eb, fmt.Errorf("Miner address must be explicitly specified")
		}
	}
	return eb, nil
}

// set in js console via admin interface or wrapper from cli flags
func (self *Siotchain) SetMiner(mineraddr helper.Address) {
	self.mineraddr = mineraddr
	self.miner.SetMiner(mineraddr)
}

func (s *Siotchain) StartMining(threads int) error {
	eb, err := s.Mineraddr()
	if err != nil {
		err = fmt.Errorf("Cannot start mining without miner address: %v", err)
		glog.V(logger.Error).Infoln(err)
		return err
	}
	go s.miner.Start(eb, threads)
	fmt.Println("Mining started")
	return nil
}

func (s *Siotchain) StopMining()         { s.miner.Stop()
fmt.Println("Mining stopped")}
func (s *Siotchain) IsMining() bool      { return s.miner.Mining() }
func (s *Siotchain) Miner() *miner.Miner { return s.miner }

func (s *Siotchain) AccountManager() *wallet.Manager        { return s.accountManager }
func (s *Siotchain) BlockChain() *blockchainCore.BlockChain { return s.blockchain }
func (s *Siotchain) TxPool() *blockchainCore.TxPool         { return s.txPool }
func (s *Siotchain) EventMux() *subscribe.TypeMux           { return s.eventMux }
func (s *Siotchain) Pow() *ethash.Ethash                    { return s.pow }
func (s *Siotchain) ChainDb() database.Database             { return s.chainDb }
func (s *Siotchain) IsListening() bool                      { return true } // Always listening
func (s *Siotchain) SiotVersion() int                       { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Siotchain) NetVersion() int                        { return s.netVersionId }
func (s *Siotchain) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Siotchain) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	} else {
		return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
	}
}

// Start implements node.Service, starting all internal goroutines needed by the
// Siotchain protocol implementation.
func (s *Siotchain) Start(srvr *p2p.Server) error {
	s.netRPCService = siotapi.NewPublicNetAPI(srvr, s.NetVersion())
	if s.AutoDAG {
		s.StartAutoDAG()
	}
	s.protocolManager.Start()
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Siotchain protocol.
func (s *Siotchain) Stop() error {
	if s.stopDbUpgrade != nil {
		s.stopDbUpgrade()
	}
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.StopAutoDAG()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}

// This function will wait for a shutdown and resumes main thread execution
func (s *Siotchain) WaitForShutdown() {
	<-s.shutdownChan
}

// StartAutoDAG() spawns a go routine that checks the DAG every autoDAGcheckInterval
// by default that is 10 times per epoch
// in epoch n, if we past autoDAGepochHeight within-epoch blocks,
// it calls ethash.MakeDAG  to pregenerate the DAG for the next epoch n+1
// if it does not exist yet as well as remove the DAG for epoch n-1
// the loop quits if autodagquit channel is closed, it can safely restart and
// stop any number of times.
// For any more sophisticated pattern of DAG generation, use CLI subcommand
// makedag
func (self *Siotchain) StartAutoDAG() {
	if self.autodagquit != nil {
		return // already started
	}
	go func() {
		glog.V(logger.Debug).Infof("Automatic pregeneration of ethash DAG ON (ethash dir: %s)", ethash.DefaultDir)
		var nextEpoch uint64
		timer := time.After(0)
		self.autodagquit = make(chan bool)
		for {
			select {
			case <-timer:
				currentBlock := self.BlockChain().CurrentBlock().NumberU64()
				thisEpoch := currentBlock / epochLength
				if nextEpoch <= thisEpoch {
					if currentBlock%epochLength > autoDAGepochHeight {
						if thisEpoch > 0 {
							previousDag, previousDagFull := dagFiles(thisEpoch - 1)
							os.Remove(filepath.Join(ethash.DefaultDir, previousDag))
							os.Remove(filepath.Join(ethash.DefaultDir, previousDagFull))
							glog.V(logger.Info).Infof("removed DAG for epoch %d (%s)", thisEpoch-1, previousDag)
						}
						nextEpoch = thisEpoch + 1
						dag, _ := dagFiles(nextEpoch)
						if _, err := os.Stat(dag); os.IsNotExist(err) {
							glog.V(logger.Info).Infof("Pregenerating DAG for epoch %d (%s)", nextEpoch, dag)
							err := ethash.MakeDAG(nextEpoch*epochLength, "") // "" -> ethash.DefaultDir
							if err != nil {
								glog.V(logger.Error).Infof("Error generating DAG for epoch %d (%s)", nextEpoch, dag)
								return
							}
						} else {
							glog.V(logger.Error).Infof("DAG for epoch %d (%s)", nextEpoch, dag)
						}
					}
				}
				timer = time.After(autoDAGcheckInterval)
			case <-self.autodagquit:
				return
			}
		}
	}()
}

// stopAutoDAG stops automatic DAG pregeneration by quitting the loop
func (self *Siotchain) StopAutoDAG() {
	if self.autodagquit != nil {
		close(self.autodagquit)
		self.autodagquit = nil
	}
	glog.V(logger.Debug).Infof("Automatic pregeneration of ethash DAG OFF (ethash dir: %s)", ethash.DefaultDir)
}

// HTTPClient returns the light http client used for fetching offchain docs
// (natspec, source for verification)
func (self *Siotchain) HTTPClient() *httpclient.HTTPClient {
	return self.httpclient
}

// dagFiles(epoch) returns the two alternative DAG filenames (not a path)
// 1) <revision>-<hex(seedhash[8])> 2) full-R<revision>-<hex(seedhash[8])>
func dagFiles(epoch uint64) (string, string) {
	seedHash, _ := ethash.GetSeedHash(epoch * epochLength)
	dag := fmt.Sprintf("full-R%d-%x", ethashRevision, seedHash[:8])
	return dag, "full-R" + dag
}
