// siotchain is the official cmd-line client for Siotchain.
package main

import (
	//"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/siot"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/node"
	"gopkg.in/urfave/cli.v1"
	"github.com/ethereum/go-ethereum/wallet"
)


const (
	clientIdentifier = "siotchain"	// Client identifier to advertise over the network
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// Siotchain address of the siot release oracle.
	// The app that holds all commands and flags.
	app = utils.NewApp(gitCommit, "the siotchain cmd line interface")
)

func init() {
	// Initialize the Siotchain app and start node
	app.Action = siotchain    //siotchain
	app.HideVersion = true // we have a cmd to print the version
	app.Copyright = "Copyright 2018 The Siotchain Authors"
	app.Commands = []cli.Command{
		{
			Action:    initGenesis,
			Name:      "init",
			Usage:     "Initialize a new genesis block",
			ArgsUsage: "<genesisPath>",
			Category:  "BLOCKCHAIN COMMANDS",
			Description: `
The init cmd initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.
`,
		},
	}
	app.Flags = []cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.DataDirFlag,
		utils.KeyStoreDirFlag,
		utils.OlympicFlag,
		utils.FastSyncFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.MinerFlag,
		utils.GasPriceFlag,
		utils.SupportDAOFork,
		utils.OpposeDAOFork,
		utils.MinerThreadsFlag,
		utils.MiningEnabledFlag,
		utils.AutoDAGFlag,
		utils.TargetGasLimitFlag,
		utils.NATFlag,
		utils.NatspecEnabledFlag,
		utils.NoDiscoverFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.RPCEnabledFlag,
		utils.RPCListenAddrFlag,
		utils.RequestFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.IPCDisabledFlag,
		utils.IPCApiFlag,
		utils.IPCPathFlag,
		utils.ExecFlag,
		utils.PreloadJSFlag,
		utils.DevModeFlag,
		utils.TestNetFlag,
		utils.VMForceJitFlag,
		utils.VMJitCacheFlag,
		utils.VMEnableJitFlag,
		utils.NetworkIdFlag,
		utils.RPCCORSDomainFlag,
		utils.MetricsEnabledFlag,
		utils.FakePoWFlag,
		utils.GpoMinGasPriceFlag,
		utils.GpoMaxGasPriceFlag,
		utils.GpoFullBlockRatioFlag,
		utils.GpobaseStepDownFlag,
		utils.GpobaseStepUpFlag,
		utils.GpobaseCorrectionFactorFlag,
		utils.ExtraDataFlag,
	}
	app.Flags = append(app.Flags, debug.Flags...)

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		// Start system runtime metrics collection
		go metrics.CollectProcessMetrics(3 * time.Second)

		// This should be the only place where reporting is enabled
		// because it is not intended to run while testing.
		// In addition to this check, bad block reports are sent only
		// for chains with the main network genesis block and network id 1.
		siot.EnableBadBlockReporting = true

		utils.SetupNetwork(ctx)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		logger.Flush()
		debug.Exit()
		utils.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// siotchain is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the cmd line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func siotchain(ctx *cli.Context) error {
	node := makeFullNode(ctx)
	startNode(ctx, node)
	node.Wait()
	return nil
}

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(ctx *cli.Context) error {
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("must supply path to genesis JSON file")
	}

	if ctx.GlobalBool(utils.TestNetFlag.Name) {
		state.StartingNonce = 1048576 // (2**20)
	}

	stack := makeFullNode(ctx)
	chaindb := utils.MakeChainDatabase(ctx, stack)

	genesisFile, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("failed to read genesis file: %v", err)
	}

	block, err := core.WriteGenesisBlock(chaindb, genesisFile)
	if err != nil {
		utils.Fatalf("failed to write genesis block: %v", err)
	}
	glog.V(logger.Info).Infof("successfully wrote genesis block and/or chain rule set: %x", block.Hash())
	return nil
}

func makeFullNode(ctx *cli.Context) *node.Node {
	stack := utils.MakeNode(ctx, clientIdentifier, gitCommit)
	utils.RegisterSiotService(ctx, stack, utils.MakeDefaultExtraData(clientIdentifier))
	return stack
}

// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested wallet, and starts the RPC/IPC interfaces and the
// miner.
func startNode(ctx *cli.Context, stack *node.Node) {
	// Start up the node itself
	utils.StartNode(stack)

	// Unlock any account specifically requested
	accman := stack.AccountManager()
	passwords := utils.MakePasswordList(ctx)
	accounts := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for i, account := range accounts {
		if trimmed := strings.TrimSpace(account); trimmed != "" {
			unlockAccount(ctx, accman, trimmed, i, passwords)
		}
	}
	// Start auxiliary services if enabled
	if ctx.GlobalBool(utils.MiningEnabledFlag.Name) {
		var siotchain *siot.Siotchain
		if err := stack.Service(&siotchain); err != nil {
			utils.Fatalf("Siotchain service not running: %v", err)
		}
		if err := siotchain.StartMining(ctx.GlobalInt(utils.MinerThreadsFlag.Name)); err != nil {
			utils.Fatalf("Failed to start mining: %v", err)
		}
	}
}

// tries unlocking the specified account a few times.
func unlockAccount(ctx *cli.Context, accman *wallet.Manager, address string, i int, passwords []string) (wallet.Account, string) {
	account, err := utils.MakeAddress(accman, address)
	if err != nil {
		utils.Fatalf("Could not list wallet: %v", err)
	}
	for trials := 0; trials < 3; trials++ {
		prompt := fmt.Sprintf("Unlocking account %s | Attempt %d/%d", address, trials+1, 3)
		password := getPassPhrase(prompt, false, i, passwords)
		err = accman.Unlock(account, password)
		if err == nil {
			glog.V(logger.Info).Infof("Unlocked account %x", account.Address)
			return account, password
		}
		if err, ok := err.(*wallet.AmbiguousAddrError); ok {
			glog.V(logger.Info).Infof("Unlocked account %x", account.Address)
			return ambiguousAddrRecovery(accman, err, password), password
		}
		if err != wallet.ErrDecrypt {
			// No need to prompt again if the error is not decryption-related.
			break
		}
	}
	// All trials expended to unlock account, bail out
	utils.Fatalf("Failed to unlock account %s (%v)", address, err)
	return wallet.Account{}, ""
}

// getPassPhrase retrieves the passwor associated with an account, either fetched
// from a list of preloaded passphrases, or requested interactively from the user.
func getPassPhrase(prompt string, confirmation bool, i int, passwords []string) string {
	// If a list of passwords was supplied, retrieve from them
	if len(passwords) > 0 {
		if i < len(passwords) {
			return passwords[i]
		}
		return passwords[len(passwords)-1]
	}
	// Otherwise prompt the user for the password
	if prompt != "" {
		fmt.Println(prompt)
	}
	password, err := utils.Stdin.PromptPassword("Passphrase: ")
	if err != nil {
		utils.Fatalf("Failed to read passphrase: %v", err)
	}
	if confirmation {
		confirm, err := utils.Stdin.PromptPassword("Repeat passphrase: ")
		if err != nil {
			utils.Fatalf("Failed to read passphrase confirmation: %v", err)
		}
		if password != confirm {
			utils.Fatalf("Passphrases do not match")
		}
	}
	return password
}

func ambiguousAddrRecovery(am *wallet.Manager, err *wallet.AmbiguousAddrError, auth string) wallet.Account {
	fmt.Printf("Multiple key files exist for address %x:\n", err.Addr)
	for _, a := range err.Matches {
		fmt.Println("  ", a.File)
	}
	fmt.Println("Testing your passphrase against all of them...")
	var match *wallet.Account
	for _, a := range err.Matches {
		if err := am.Unlock(a, auth); err == nil {
			match = &a
			break
		}
	}
	if match == nil {
		utils.Fatalf("None of the listed files could be unlocked.")
	}
	fmt.Printf("Your passphrase unlocked %s\n", match.File)
	fmt.Println("In order to avoid this warning, you need to remove the following duplicate key files:")
	for _, a := range err.Matches {
		if a != *match {
			fmt.Println("  ", a.File)
		}
	}
	return *match
}



