package utils

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"github.com/ethereum/ethash"
	"github.com/siotchain/siot/wallet"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore"
	"github.com/siotchain/siot/blockchainCore/state"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/siot"
	"github.com/siotchain/siot/database"
	"github.com/siotchain/siot/subscribe"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"github.com/siotchain/siot/helper/metrics"
	"github.com/siotchain/siot/context"
	"github.com/siotchain/siot/net/p2p/discover"
	"github.com/siotchain/siot/net/p2p/nat"
	"github.com/siotchain/siot/configure"
	"github.com/siotchain/siot/validation"
	"github.com/siotchain/siot/net/rpc"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}cmd{{if .Flags}} [cmd options]{{end}} [arguments...]

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	cli.CommandHelpTemplate = `{{.Name}}{{if .Subcommands}} cmd{{end}}{{if .Flags}} [cmd options]{{end}} [arguments...]
{{if .Description}}{{.Description}}
{{end}}{{if .Subcommands}}
SUBCOMMANDS:
	{{range .Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .Flags}}
OPTIONS:
	{{range .Flags}}{{.}}
	{{end}}{{end}}
`
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	app.Email = ""
	app.Version = Version
	app.Usage = usage
	return app
}

// These are all the cmd line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate cmd definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.

var (
	// General settings
	DataDirFlag = DirectoryFlag{
		Name:  "datapath",
		Usage: "Target directory to save the databases and account keystore",
		Value: DirectoryString{context.DefaultDataDir()},
	}
	KeyStoreDirFlag = DirectoryFlag{
		Name:  "keystore",
		Usage: "Directory for the keystore (default = inside the datadir)",
	}
	NetworkIdFlag = cli.IntFlag{
		Name:  "chainnetwork",
		Usage: "Network identifier",
		Value: siot.NetworkId,
	}
	IPFlag = cli.StringFlag{
		Name:  "IP",
		Usage: "IP address (integer, 0=Olympic, 1=Frontier, 2=Morden)",
		Value: siot.IP,
	}
	OlympicFlag = cli.BoolFlag{
		Name:  "olympic",
		Usage: "Olympic network: pre-configured pre-release test network",
	}
	TestNetFlag = cli.BoolFlag{
		Name:  "testnet",
		Usage: "Morden network: pre-configured test network with modified starting nonces (replay protection)",
	}
	DevModeFlag = cli.BoolFlag{
		Name:  "dev",
		Usage: "Developer mode: pre-configured private network with several debugging flags",
	}
	IdentityFlag = cli.StringFlag{
		Name:  "identity",
		Usage: "Custom node name",
	}
	NatspecEnabledFlag = cli.BoolFlag{
		Name:  "natspec",
		Usage: "Enable NatSpec confirmation notice",
	}
	DocRootFlag = DirectoryFlag{
		Name:  "docroot",
		Usage: "Document Root for HTTPClient file scheme",
		Value: DirectoryString{homeDir()},
	}
	FastSyncFlag = cli.BoolFlag{
		Name:  "fast",
		Usage: "Enable fast syncing through state downloads",
	}
	CacheFlag = cli.IntFlag{
		Name:  "cache",
		Usage: "Megabytes of memory allocated to internal caching (min 16MB / database forced)",
		Value: 128,
	}
	TrieCacheGenFlag = cli.IntFlag{
		Name:  "trie-cache-gens",
		Usage: "Number of trie node generations to keep in memory",
		Value: int(state.MaxTrieCacheGen),
	}
	// Fork settings
	SupportDAOFork = cli.BoolFlag{
		Name:  "support-dao-fork",
		Usage: "Updates the chain rules to support the DAO hard-fork",
	}
	OpposeDAOFork = cli.BoolFlag{
		Name:  "oppose-dao-fork",
		Usage: "Updates the chain rules to oppose the DAO hard-fork",
	}
	// Miner settings
	MiningEnabledFlag = cli.BoolFlag{
		Name:  "mine",
		Usage: "Enable mining",
	}
	MinerThreadsFlag = cli.IntFlag{
		Name:  "minerthreads",
		Usage: "Number of CPU threads to use for mining",
		Value: runtime.NumCPU(),
	}
	TargetGasLimitFlag = cli.StringFlag{
		Name:  "targetgaslimit",
		Usage: "Target gas limit sets the artificial target gas floor for the blocks to mine",
		Value: configure.GenesisGasLimit.String(),
	}
	AutoDAGFlag = cli.BoolFlag{
		Name:  "autodag",
		Usage: "Enable automatic DAG pregeneration",
	}
	MinerFlag = cli.StringFlag{
		Name:  "miner",
		Usage: "Public address for block mining rewards (default = first account created)",
		Value: "0",
	}
	GasPriceFlag = cli.StringFlag{
		Name:  "gasprice",
		Usage: "Minimal gas price to accept for mining a transactions",
		Value: new(big.Int).Mul(big.NewInt(20), helper.Shannon).String(),
	}
	ExtraDataFlag = cli.StringFlag{
		Name:  "extradata",
		Usage: "Block extra data set by the miner (default = client version)",
	}
	// Account settings
	UnlockedAccountFlag = cli.StringFlag{
		Name:  "unlock",
		Usage: "Comma separated list of wallet to unlock",
		Value: "",
	}
	PasswordFileFlag = cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-inteactive password input",
		Value: "",
	}

	VMForceJitFlag = cli.BoolFlag{
		Name:  "forcejit",
		Usage: "Force the JIT VM to take precedence",
	}
	VMJitCacheFlag = cli.IntFlag{
		Name:  "jitcache",
		Usage: "Amount of cached JIT VM programs",
		Value: 64,
	}
	VMEnableJitFlag = cli.BoolFlag{
		Name:  "jitvm",
		Usage: "Enable the JIT VM",
	}

	// logging and debug settings
	MetricsEnabledFlag = cli.BoolFlag{
		Name:  metrics.MetricsEnabledFlag,
		Usage: "Enable metrics collection and reporting",
	}
	FakePoWFlag = cli.BoolFlag{
		Name:  "fakepow",
		Usage: "Disables proof-of-work verification",
	}

	// RPC settings
	RPCEnabledFlag = cli.BoolFlag{
		Name:  "rpc",
		Usage: "Enable the HTTP-RPC server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "rpcip",
		Usage: "HTTP-RPC server listening interface",
		Value: context.DefaultHTTPHost,
	}
	RPCPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: context.DefaultHTTPPort,
	}
	RequestFlag = cli.StringFlag{
		Name:  "request",
		Usage: "Request for JSON RPC call, if no request specified, will go into the interactive mode",
		Value: rpc.DefaultRPCRequest,
	}
	RPCCORSDomainFlag = cli.StringFlag{
		Name:  "rpccorsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value: "",
	}
	RPCApiFlag = cli.StringFlag{
		Name:  "rpcapi",
		Usage: "API's offered over the HTTP-RPC interface",
		Value: rpc.DefaultHTTPApis,
	}
	IPCDisabledFlag = cli.BoolFlag{
		Name:  "ipcdisable",
		Usage: "Disable the IPC-RPC server",
	}
	IPCApiFlag = cli.StringFlag{
		Name:  "ipcapi",
		Usage: "APIs offered over the IPC-RPC interface",
		Value: rpc.DefaultIPCApis,
	}
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
		Value: DirectoryString{"siotchain.ipc"},
	}
	WSEnabledFlag = cli.BoolFlag{
		Name:  "ws",
		Usage: "Enable the WS-RPC server",
	}
	WSListenAddrFlag = cli.StringFlag{
		Name:  "wsaddr",
		Usage: "WS-RPC server listening interface",
		Value: context.DefaultWSHost,
	}
	WSPortFlag = cli.IntFlag{
		Name:  "wsport",
		Usage: "WS-RPC server listening port",
		Value: context.DefaultWSPort,
	}
	WSApiFlag = cli.StringFlag{
		Name:  "wsapi",
		Usage: "API's offered over the WS-RPC interface",
		Value: rpc.DefaultHTTPApis,
	}
	WSAllowedOriginsFlag = cli.StringFlag{
		Name:  "wsorigins",
		Usage: "Origins from which to accept websockets requests",
		Value: "",
	}
	ExecFlag = cli.StringFlag{
		Name:  "exec",
		Usage: "Execute JavaScript statement (only in combination with console/attach)",
	}
	PreloadJSFlag = cli.StringFlag{
		Name:  "preload",
		Usage: "Comma separated list of JavaScript files to preload into the console",
	}

	// Network Settings
	MaxPeersFlag = cli.IntFlag{
		Name:  "maxpeers",
		Usage: "Maximum number of network peers (network disabled if set to 0)",
		Value: 25,
	}
	MaxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	ListenPortFlag = cli.IntFlag{
		Name:  "networkport",
		Usage: "Network listening port",
		Value: 10000,
	}
	BootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "Comma separated siot node URLs for P2P discovery bootstrap",
		Value: "",
	}
	NodeKeyFileFlag = cli.StringFlag{
		Name:  "nodekey",
		Usage: "P2P node key file",
	}
	NodeKeyHexFlag = cli.StringFlag{
		Name:  "nodekeyhex",
		Usage: "P2P node key as hex (for testing)",
	}
	NATFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "any",
	}
	NoDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}

	// Gas price oracle settings
	GpoMinGasPriceFlag = cli.StringFlag{
		Name:  "gpomin",
		Usage: "Minimum suggested gas price",
		Value: new(big.Int).Mul(big.NewInt(20), helper.Shannon).String(),
	}
	GpoMaxGasPriceFlag = cli.StringFlag{
		Name:  "gpomax",
		Usage: "Maximum suggested gas price",
		Value: new(big.Int).Mul(big.NewInt(500), helper.Shannon).String(),
	}
	GpoFullBlockRatioFlag = cli.IntFlag{
		Name:  "gpofull",
		Usage: "Full block threshold for gas price calculation (%)",
		Value: 80,
	}
	GpobaseStepDownFlag = cli.IntFlag{
		Name:  "gpobasedown",
		Usage: "Suggested gas price base step down ratio (1/1000)",
		Value: 10,
	}
	GpobaseStepUpFlag = cli.IntFlag{
		Name:  "gpobaseup",
		Usage: "Suggested gas price base step up ratio (1/1000)",
		Value: 100,
	}
	GpobaseCorrectionFactorFlag = cli.IntFlag{
		Name:  "gpobasecf",
		Usage: "Suggested gas price base correction factor (%)",
		Value: 110,
	}
)

// MakeDataDir retrieves the currently requested data directory, terminating
// if none (or the empty string) is specified. If the node is starting a testnet,
// the a subdirectory of the specified datadir will be used.
func MakeDataDir(ctx *cli.Context) string {
	if path := ctx.GlobalString(DataDirFlag.Name); path != "" {
		// TODO: choose a different location outside of the regular datadir.
		if ctx.GlobalBool(TestNetFlag.Name) {
			return filepath.Join(path, "testnet")
		}
		return path
	}
	Fatalf("Cannot determine default data directory, please set manually (--datadir)")
	return ""
}

// MakeIPCPath creates an IPC path configuration from the set cmd line flags,
// returning an empty string if IPC was explicitly disabled, or the set path.
func MakeIPCPath(ctx *cli.Context) string {
	if ctx.GlobalBool(IPCDisabledFlag.Name) {
		return ""
	}
	return ctx.GlobalString(IPCPathFlag.Name)
}

// MakeNodeKey creates a node key from set cmd line flags, either loading it
// from a file or as a specified hex value. If neither flags were provided, this
// method returns nil and an emphemeral key is to be generated.
func MakeNodeKey(ctx *cli.Context) *ecdsa.PrivateKey {
	var (
		hex  = ctx.GlobalString(NodeKeyHexFlag.Name)
		file = ctx.GlobalString(NodeKeyFileFlag.Name)

		key *ecdsa.PrivateKey
		err error
	)
	switch {
	case file != "" && hex != "":
		Fatalf("Options %q and %q are mutually exclusive", NodeKeyFileFlag.Name, NodeKeyHexFlag.Name)

	case file != "":
		if key, err = crypto.LoadECDSA(file); err != nil {
			Fatalf("Option %q: %v", NodeKeyFileFlag.Name, err)
		}

	case hex != "":
		if key, err = crypto.HexToECDSA(hex); err != nil {
			Fatalf("Option %q: %v", NodeKeyHexFlag.Name, err)
		}
	}
	return key
}

// makeNodeUserIdent creates the user identifier from CLI flags.
func makeNodeUserIdent(ctx *cli.Context) string {
	var comps []string
	if identity := ctx.GlobalString(IdentityFlag.Name); len(identity) > 0 {
		comps = append(comps, identity)
	}
	if ctx.GlobalBool(VMEnableJitFlag.Name) {
		comps = append(comps, "JIT")
	}
	return strings.Join(comps, "/")
}

// MakeBootstrapNodes creates a list of bootstrap nodes from the cmd line
// flags, reverting to pre-configured ones if none have been specified.
func MakeBootstrapNodes(ctx *cli.Context) []*discover.Node {
	// Return pre-configured nodes if none were manually requested
	if !ctx.GlobalIsSet(BootnodesFlag.Name) {
		if ctx.GlobalBool(TestNetFlag.Name) {
			return configure.TestnetBootnodes
		}
		return configure.MainnetBootnodes
	}
	// Otherwise parse and use the CLI bootstrap nodes
	bootnodes := []*discover.Node{}

	for _, url := range strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",") {
		node, err := discover.ParseNode(url)
		if err != nil {
			glog.V(logger.Error).Infof("Bootstrap URL %s: %v\n", url, err)
			continue
		}
		bootnodes = append(bootnodes, node)
	}
	return bootnodes
}

// MakeListenAddress creates a TCP listening address string from set cmd
// line flags.
func MakeListenAddress(ctx *cli.Context) string {
	return fmt.Sprintf(":%d", ctx.GlobalInt(ListenPortFlag.Name))
}

// MakeNAT creates a port mapper from set cmd line flags.
func MakeNAT(ctx *cli.Context) nat.Interface {
	natif, err := nat.Parse(ctx.GlobalString(NATFlag.Name))
	if err != nil {
		Fatalf("Option %s: %v", NATFlag.Name, err)
	}
	return natif
}

// MakeRPCModules splits input separated by a comma and trims excessive white
// space from the substrings.
func MakeRPCModules(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// MakeHTTPRpcHost creates the HTTP RPC listener interface string from the set
// cmd line flags, returning empty if the HTTP endpoint is disabled.
func MakeHTTPRpcHost(ctx *cli.Context) string {
	if !ctx.GlobalBool(RPCEnabledFlag.Name) {
		return ""
	}
	return ctx.GlobalString(RPCListenAddrFlag.Name)
}

// MakeWSRpcHost creates the WebSocket RPC listener interface string from the set
// cmd line flags, returning empty if the HTTP endpoint is disabled.
func MakeWSRpcHost(ctx *cli.Context) string {
	if !ctx.GlobalBool(WSEnabledFlag.Name) {
		return ""
	}
	return ctx.GlobalString(WSListenAddrFlag.Name)
}

// MakeDatabaseHandles raises out the number of allowed file handles per process
// for Siotchain and returns half of the allowance to assign to the database.
func MakeDatabaseHandles() int {
	if err := raiseFdLimit(2048); err != nil {
		Fatalf("Failed to raise file descriptor allowance: %v", err)
	}
	limit, err := getFdLimit()
	if err != nil {
		Fatalf("Failed to retrieve file descriptor allowance: %v", err)
	}
	if limit > 2048 { // cap database file descriptors even if more is available
		limit = 2048
	}
	return limit / 2 // Leave half for networking and other stuff
}

// MakeAddress converts an account specified directly as a hex encoded string or
// a key index in the key store to an internal account representation.
func MakeAddress(accman *wallet.Manager, account string) (wallet.Account, error) {
	// If the specified account is a valid address, return it
	if helper.IsHexAddress(account) {
		return wallet.Account{Address: helper.HexToAddress(account)}, nil
	}
	// Otherwise try to interpret the account as a keystore index
	index, err := strconv.Atoi(account)
	if err != nil {
		return wallet.Account{}, fmt.Errorf("invalid account address or index %q", account)
	}
	return accman.AccountByIndex(index)
}

// MakeMiner retrieves the miner either from the directly specified
// cmd line flags or from the keystore if CLI indexed.
func MakeMiner(accman *wallet.Manager, ctx *cli.Context) helper.Address {
	accounts := accman.Accounts()
	if !ctx.GlobalIsSet(MinerFlag.Name) && len(accounts) == 0 {
		glog.V(logger.Debug).Infoln("WARNING: No miner set and no wallet found as default")
		return helper.Address{}
	}
	minerBase := ctx.GlobalString(MinerFlag.Name)
	if minerBase == "" {
		return helper.Address{}
	}
	// If the specified minerBase is a valid address, return it
	account, err := MakeAddress(accman, minerBase)
	if err != nil {
		Fatalf("Option %q: %v", MinerFlag.Name, err)
	}
	return account.Address
}

// MakeMinerExtra resolves extradata for the miner from the set cmd line flags
// or returns a default one composed on the client, runtime and OS metadata.
func MakeMinerExtra(extra []byte, ctx *cli.Context) []byte {
	if ctx.GlobalIsSet(ExtraDataFlag.Name) {
		return []byte(ctx.GlobalString(ExtraDataFlag.Name))
	}
	return extra
}

// MakePasswordList reads password lines from the file specified by --password.
func MakePasswordList(ctx *cli.Context) []string {
	path := ctx.GlobalString(PasswordFileFlag.Name)
	if path == "" {
		return nil
	}
	text, err := ioutil.ReadFile(path)
	if err != nil {
		Fatalf("Failed to read password file: %v", err)
	}
	lines := strings.Split(string(text), "\n")
	// Sanitise DOS line endings.
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

// MakeNode configures a node with no services from cmd line flags.
func MakeNode(ctx *cli.Context, name, gitCommit string) *context.Node {
	vsn := Version

	config := &context.Config{
		DataDir:           MakeDataDir(ctx),
		KeyStoreDir:       ctx.GlobalString(KeyStoreDirFlag.Name),
		PrivateKey:        MakeNodeKey(ctx),
		Name:              name,
		Version:           vsn,
		UserIdent:         makeNodeUserIdent(ctx),
		BootstrapNodes:    MakeBootstrapNodes(ctx),
		ListenAddr:        MakeListenAddress(ctx),
		NAT:               MakeNAT(ctx),
		MaxPeers:          ctx.GlobalInt(MaxPeersFlag.Name),
		MaxPendingPeers:   ctx.GlobalInt(MaxPendingPeersFlag.Name),
		IPCPath:           MakeIPCPath(ctx),
		HTTPHost:          MakeHTTPRpcHost(ctx),
		HTTPPort:          ctx.GlobalInt(RPCPortFlag.Name),
		HTTPCors:          ctx.GlobalString(RPCCORSDomainFlag.Name),
		HTTPModules:       MakeRPCModules(ctx.GlobalString(RPCApiFlag.Name)),
		WSHost:            MakeWSRpcHost(ctx),
		WSPort:            ctx.GlobalInt(WSPortFlag.Name),
		WSOrigins:         ctx.GlobalString(WSAllowedOriginsFlag.Name),
		WSModules:         MakeRPCModules(ctx.GlobalString(WSApiFlag.Name)),
	}
	if ctx.GlobalBool(DevModeFlag.Name) {
		if !ctx.GlobalIsSet(DataDirFlag.Name) {
			config.DataDir = filepath.Join(os.TempDir(), "/siotchain_dev_mode")
		}
		// --dev mode does not need p2p networking.
		config.MaxPeers = 0
		config.ListenAddr = ":0"
	}
	stack, err := context.New(config)
	if err != nil {
		Fatalf("Failed to create the protocol stack: %v", err)
	}
	return stack
}

// RegisterSiotService configures siot.Siotchain from cmd line flags and adds it to the
// given node.
func RegisterSiotService(ctx *cli.Context, stack *context.Node, extra []byte) {
	// Avoid conflicting network flags
	networks, netFlags := 0, []cli.BoolFlag{DevModeFlag, TestNetFlag, OlympicFlag}
	for _, flag := range netFlags {
		if ctx.GlobalBool(flag.Name) {
			networks++
		}
	}
	if networks > 1 {
		Fatalf("The %v flags are mutually exclusive", netFlags)
	}

	// initialise new random number generator
	// get enabled jit flag
	ctx.GlobalSet(VMEnableJitFlag.Name, "false")
	jitEnabled := ctx.GlobalBool(VMEnableJitFlag.Name)

	ethConf := &siot.Config{
		MinerAddr:       MakeMiner(stack.AccountManager(), ctx),
		ChainConfig:     MakeChainConfig(ctx, stack),
		FastSync:        ctx.GlobalBool(FastSyncFlag.Name),
		MaxPeers:        ctx.GlobalInt(MaxPeersFlag.Name),
		DatabaseCache:   ctx.GlobalInt(CacheFlag.Name),
		DatabaseHandles: MakeDatabaseHandles(),
		NetworkId:       ctx.GlobalInt(NetworkIdFlag.Name),
		MinerThreads:    ctx.GlobalInt(MinerThreadsFlag.Name),
		ExtraData:       MakeMinerExtra(extra, ctx),
		NatSpec:         ctx.GlobalBool(NatspecEnabledFlag.Name),
		DocRoot:                 ctx.GlobalString(DocRootFlag.Name),
		EnableJit:               jitEnabled,
		ForceJit:                ctx.GlobalBool(VMForceJitFlag.Name),
		GasPrice:                helper.String2Big(ctx.GlobalString(GasPriceFlag.Name)),
		GpoMinGasPrice:          helper.String2Big(ctx.GlobalString(GpoMinGasPriceFlag.Name)),
		GpoMaxGasPrice:          helper.String2Big(ctx.GlobalString(GpoMaxGasPriceFlag.Name)),
		GpoFullBlockRatio:       ctx.GlobalInt(GpoFullBlockRatioFlag.Name),
		GpobaseStepDown:         ctx.GlobalInt(GpobaseStepDownFlag.Name),
		GpobaseStepUp:           ctx.GlobalInt(GpobaseStepUpFlag.Name),
		GpobaseCorrectionFactor: ctx.GlobalInt(GpobaseCorrectionFactorFlag.Name),
		AutoDAG:                 ctx.GlobalBool(AutoDAGFlag.Name) || ctx.GlobalBool(MiningEnabledFlag.Name),
	}

	// Override any default configs in dev mode or the test net
	switch {
	case ctx.GlobalBool(OlympicFlag.Name):
		if !ctx.GlobalIsSet(NetworkIdFlag.Name) {
			ethConf.NetworkId = 1
		}
		ethConf.Genesis = blockchainCore.OlympicGenesisBlock()

	case ctx.GlobalBool(DevModeFlag.Name):
		ethConf.Genesis = blockchainCore.OlympicGenesisBlock()
		if !ctx.GlobalIsSet(GasPriceFlag.Name) {
			ethConf.GasPrice = new(big.Int)
		}
		ethConf.PowTest = true
	}
	// Override any global options pertaining to the Siotchain protocol
	if gen := ctx.GlobalInt(TrieCacheGenFlag.Name); gen > 0 {
		state.MaxTrieCacheGen = uint16(gen)
	}

	if err := stack.Register(func(ctx *context.ServiceContext) (context.Service, error) {
		fullNode, err := siot.New(ctx, ethConf)
		return fullNode, err
	}); err != nil {
		Fatalf("Failed to register the Siotchain full node service: %v", err)
	}
}

// SetupNetwork configures the system for either the main net or some test network.
func SetupNetwork(ctx *cli.Context) {
	switch {
	case ctx.GlobalBool(OlympicFlag.Name):
		configure.DurationLimit = big.NewInt(8)
		configure.GenesisGasLimit = big.NewInt(3141592)
		configure.MinGasLimit = big.NewInt(125000)
		configure.MaximumExtraDataSize = big.NewInt(1024)
		NetworkIdFlag.Value = 0
		blockchainCore.BlockReward = big.NewInt(1.5e+18)
		blockchainCore.ExpDiffPeriod = big.NewInt(math.MaxInt64)
	}
	configure.TargetGasLimit = helper.String2Big(ctx.GlobalString(TargetGasLimitFlag.Name))
}

// MakeChainConfig reads the chain configuration from the database in ctx.Datadir.
func MakeChainConfig(ctx *cli.Context, stack *context.Node) *configure.ChainConfig {
	db := MakeChainDatabase(ctx, stack)
	defer db.Close()

	return MakeChainConfigFromDb(ctx, db)
}

// MakeChainConfigFromDb reads the chain configuration from the given database.
func MakeChainConfigFromDb(ctx *cli.Context, db database.Database) *configure.ChainConfig {
	// If the chain is already initialized, use any existing chain configs
	config := new(configure.ChainConfig)

	genesis := blockchainCore.GetBlock(db, blockchainCore.GetCanonicalHash(db, 0), 0)
	if genesis != nil {
		storedConfig, err := blockchainCore.GetChainConfig(db, genesis.Hash())
		switch err {
		case nil:
			config = storedConfig
		case blockchainCore.ChainConfigNotFoundErr:
			// No configs found, use empty, will populate below
		default:
			Fatalf("Could not make chain configuration: %v", err)
		}
	}
	// set chain id in case it's zero.
	if config.ChainId == nil {
		config.ChainId = new(big.Int)
	}
	// Check whether we are allowed to set default config configure or not:
	//  - If no genesis is set, we're running either mainnet or testnet (private nets use `siotchain init`)
	//  - If a genesis is already set, ensure we have a configuration for it (mainnet or testnet)
	defaults := genesis == nil ||
		(genesis.Hash() == configure.MainNetGenesisHash && !ctx.GlobalBool(TestNetFlag.Name)) ||
		(genesis.Hash() == configure.TestNetGenesisHash && ctx.GlobalBool(TestNetFlag.Name))

	// Set any missing chainConfig fields due to them being unset or system upgrade
	if defaults {
		if config.HomesteadBlock == nil {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.HomesteadBlock = configure.TestNetHomesteadBlock
			} else {
				config.HomesteadBlock = configure.MainNetHomesteadBlock
			}
		}
		if config.DAOForkBlock == nil {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.DAOForkBlock = configure.TestNetDAOForkBlock
			} else {
				config.DAOForkBlock = configure.MainNetDAOForkBlock
			}
			config.DAOForkSupport = true
		}
		if config.SiotImpr0Block == nil {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.SiotImpr0Block = configure.TestNetHomesteadGasRepriceBlock
			} else {
				config.SiotImpr0Block = configure.MainNetHomesteadGasRepriceBlock
			}
		}
		if config.SiotImpr0Hash == (helper.Hash{}) {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.SiotImpr0Hash = configure.TestNetHomesteadGasRepriceHash
			} else {
				config.SiotImpr0Hash = configure.MainNetHomesteadGasRepriceHash
			}
		}
		if config.SiotImpr1Block == nil {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.SiotImpr0Block = configure.TestNetSpuriousDragon
			} else {
				config.SiotImpr1Block = configure.MainNetSpuriousDragon
			}
		}
		if config.SiotImpr2Block == nil {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.SiotImpr2Block = configure.TestNetSpuriousDragon
			} else {
				config.SiotImpr2Block = configure.MainNetSpuriousDragon
			}
		}
		if config.ChainId.BitLen() == 0 {
			if ctx.GlobalBool(TestNetFlag.Name) {
				config.ChainId = configure.TestNetChainID
			} else {
				config.ChainId = configure.MainNetChainID
			}
		}
		config.DAOForkSupport = true
	}

	// Force override any existing configs if explicitly requested
	switch {
	case ctx.GlobalBool(SupportDAOFork.Name):
		config.DAOForkSupport = true
	case ctx.GlobalBool(OpposeDAOFork.Name):
		config.DAOForkSupport = false
	}
	return config
}

func ChainDbName(ctx *cli.Context) string {
	return "chaindata"
}

// MakeChainDatabase open an LevelDB using the flags passed to the client and will hard crash if it fails.
func MakeChainDatabase(ctx *cli.Context, stack *context.Node) database.Database {
	var (
		cache   = ctx.GlobalInt(CacheFlag.Name)
		handles = MakeDatabaseHandles()
		name    = ChainDbName(ctx)
	)

	chainDb, err := stack.OpenDatabase(name, cache, handles)
	if err != nil {
		Fatalf("Could not open database: %v", err)
	}
	return chainDb
}

// MakeChain creates a chain manager from set cmd line flags.
func MakeChain(ctx *cli.Context, stack *context.Node) (chain *blockchainCore.BlockChain, chainDb database.Database) {
	var err error
	chainDb = MakeChainDatabase(ctx, stack)

	if ctx.GlobalBool(OlympicFlag.Name) {
		_, err := blockchainCore.WriteTestNetGenesisBlock(chainDb)
		if err != nil {
			glog.Fatalln(err)
		}
	}
	chainConfig := MakeChainConfigFromDb(ctx, chainDb)

	pow := validation.PoW(blockchainCore.FakePow{})
	if !ctx.GlobalBool(FakePoWFlag.Name) {
		pow = ethash.New()
	}
	chain, err = blockchainCore.NewBlockChain(chainDb, chainConfig, pow, new(subscribe.TypeMux))
	if err != nil {
		Fatalf("Could not start chainmanager: %v", err)
	}
	return chain, chainDb
}
