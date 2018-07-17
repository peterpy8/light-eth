// siotchain-cli is the official cmd-line client for Siotchain interactive mode.

package main

import (
	//"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/siotchain/siot/client/utils"
	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/siot"
	"github.com/siotchain/siot/internal/debug"
	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/helper/metrics"
	"gopkg.in/urfave/cli.v1"
	"github.com/siotchain/siot/client"
	"math/big"
	"encoding/hex"
	"github.com/fatih/color"
	"github.com/siotchain/siot/net/p2p"
	"encoding/json"
	"bytes"
	"golang.org/x/net/context"
	"bufio"
	"errors"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	// Siotchain address of the siot release oracle.
	relOracle = helper.HexToAddress("0xfa7b9770ca4cb04296cac84f37736d4041251cdf")
	// The app that holds all commands and flags.
	app = utils.NewApp(gitCommit, "the siotchain interactive mode cmd line interface")
	requestmap = map[string]int{
		"getnodeinfo": 0,
		"getnodeid": 0,
		"getaccounts": 0,
		"getLastAccount": 0,     // just used to write testing script
		"getnewaccount": 1,
		"unlockaccount": 2,
		"getbalance": 1,
		"connectpeer": 1,
		"getpeers": 0,
		"setminer": 1,
		"startmine": 0,
		"stopmine": 0,
		"sendasset": 3,
	}
)

func init() {
	// Initialize the SIOT app and start CLI interactive mode
	app.Action = siotchain_cli   //siotchain
	app.HideVersion = true // we have a cmd to print the version
	app.Copyright = "Copyright 2018 The Siotchain Authors"
	app.Commands = []cli.Command{}
	app.Flags = []cli.Flag{
		utils.RPCListenAddrFlag,
		utils.RequestFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.NetworkIdFlag,
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

// siotchain_cli is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the cmd line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func siotchain_cli(ctx *cli.Context) error {
	host := ctx.GlobalString(utils.RPCListenAddrFlag.Name)
	port := strconv.Itoa(ctx.GlobalInt(utils.RPCPortFlag.Name))
	urlChunks := []string{"http://", host, ":", port}
	url := strings.Join(urlChunks,"")
	//fmt.Println(url)
	client, _:= client.Dial(url)
	requestString := ctx.GlobalString(utils.RequestFlag.Name)

	if  requestString != "" {
		//fmt.Println(ctx.GlobalString(utils.RequestFlag.Name))
		handleRequest(ctx, client, requestString)
	} else {
		fmt.Println("go into console mode and wait for user input")
		readInput(ctx, client)
		//fmt.Println(glog.GetVerbosity())
	}
	return nil
}

func readInput(ctx *cli.Context, client *client.Client) error {
	scanner := bufio.NewScanner(os.Stdin)
	for true {
		var input string
		fmt.Print("> ")
		scanner.Scan()
		// fmt.Println("start")
		input = scanner.Text()
		if err := scanner.Err(); err != nil {
			os.Exit(1)
		}
		if input == "exit" {
			break
		}

		//for _, chunk := range chunks {
		//	fmt.Println(chunk)
		//}
		if input == "" {
			fmt.Println("request is empty, you need to input a request")
			continue
		}
		handleRequest(ctx, client, input)
	}
	return nil
}

func handleRequest(cliCtx *cli.Context, client *client.Client, input string) error {
	green := color.New(color.FgGreen).PrintfFunc()
	inputUppercase := strings.ToLower(strings.TrimSpace(input))
	chunks := strings.Split(inputUppercase, " ")
	numofparams := len(chunks) - 1
	ctx := context.Background()

	switch {
	case chunks[0] == "getnodeinfo":
		if numofparams == requestmap["getnodeinfo"] {
			result, err := client.NodeInfoAt(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			result_display := &p2p.NodeInfoDisplay{ID: result.ID, URL: result.Siot, ListenAddr: result.ListenAddr,
				SiotNetwork: strconv.Itoa(cliCtx.GlobalInt(utils.NetworkIdFlag.Name))}
			resultJson, _ := json.Marshal(result_display)
			b, _ := prettyprint(resultJson)
			fmt.Printf("%s\n", b)

		} else {
			fmt.Println("incorrect format: getnodeinfo has no params")
		}
	case chunks[0] == "getnodeid":
		if numofparams == requestmap["getnodeid"] {
			result, err := client.NodeInfoAt(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("%s\n", result.ID)

		} else {
			fmt.Println("incorrect format: getnodeinfo has no params")
		}
	case chunks[0] == "getaccounts":
		if numofparams == requestmap["getaccounts"] {
			result, err := client.ListAccountsAt(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(result) == 0 {
				fmt.Println("[]")
				break
			}
			for _, addr := range result {
				hexaddr := byteArrayToString(addr)
				green("0x%s", hexaddr)
				fmt.Println()
			}
		} else {
			fmt.Println("incorrect format: should be")
		}
	case chunks[0] == "getlastaccount":
		if numofparams == requestmap["getlastaccount"] {
			result, err := client.ListAccountsAt(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(result) == 0 {
				fmt.Println("[]")
				break
			}
			hexaddr := byteArrayToString(result[len(result)-1])
			green("0x%s", hexaddr)
			fmt.Println()
		} else {
			fmt.Println("incorrect format: should be")
		}
	case chunks[0] == "getnewaccount":
		if numofparams == requestmap["getnewaccount"] {
			result, err := client.NewAccount(ctx, chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			//for _, a := range result {
			//	fmt.Printf("%v, ", a)
			//}
			//fmt.Println()
			addr := byteArrayToString(result)
			green("0x%s\n", addr)
		} else {
			fmt.Println("incorrect format: should be getNewaccount [password]")
		}
	case chunks[0] == "unlockaccount":
		if numofparams == requestmap["unlockaccount"] {
			addrString, err := parseInput(chunks[1])
			addr_common := stringAddrToCommonAddr(addrString)
			result, err := client.UnlockAccount(ctx, helper.Address(addr_common), chunks[2])
			if err != nil {
				fmt.Println(err)
				break
			}
			if result == true {
				fmt.Println("successfully unlock account")
			}
		} else {
			fmt.Println("incorrect format: should be unlockAccount [address] [password]")
		}
	case chunks[0] == "getbalance":
		if numofparams == requestmap["getbalance"] {
			addrString, err := parseInput(chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			addr_common := stringAddrToCommonAddr(addrString)
			result, err := client.BalanceAt(ctx, helper.Address(addr_common), nil)
			if err != nil {
				fmt.Println(err)
				break
			}
			value := result.Div(result, big.NewInt(1000000000000))
			stringValue := value.String()
			green("balance: %s\n", stringValue)
		} else {
			fmt.Println("incorrect format: should be getBalance [address]")
		}
	case chunks[0] == "connectpeer":
		if numofparams == requestmap["connectpeer"] {
			_, err := client.AddPeer(ctx, chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Println("connected to peer")
		} else {
			fmt.Println("incorrect format: should be connectPeer [url of the peer]")
		}
	case chunks[0] == "getpeers":
		if numofparams == requestmap["getpeers"] {
			result, err := client.GetPeers(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(result) == 0 {
				fmt.Println("no peer node existed")
			} else {
				fmt.Println("peer id list: ")
				for _, peer := range result {
					green("%s\n", peer.ID)
				}
			}
		} else {
			fmt.Println("incorrect format: should be getPeers")
		}
	case chunks[0] == "setminer":
		if numofparams == requestmap["setminer"] {
			addrString, err := parseInput(chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			addr_common := stringAddrToCommonAddr(addrString)
			_, minerErr := client.SetMiner(ctx, helper.Address(addr_common))
			if minerErr != nil {
				fmt.Println(err)
				break
			}
			fmt.Println("successfully set a miner")
		} else {
			fmt.Println("incorrect format: should be setminer [address]")
		}
	case chunks[0] == "startmine":
		if numofparams == requestmap["startmine"] {
			// get account list
			//accountsResult, err := client.ListAccountsAt(ctx)
			//if err != nil {
			//	fmt.Println(err)
			//	break
			//}
			//if len(accountsResult) == 0 {
			//	err := errors.New("no account existed for mining")
			//	fmt.Println(err)
			//	break
			//}
			//_, setMinerErr := client.SetMiner(ctx, byteArrayToCommonAddr(accountsResult[0]))
			//if err != nil {
			//	fmt.Println(setMinerErr)
			//	break
			//}
			//fmt.Println(setminerResult)
			_, miningErr := client.StartMining(ctx)
			if miningErr != nil {
				fmt.Println(miningErr)
				break
			}
			fmt.Println("mining started")
		} else {
			fmt.Println("incorrect format: should be startMine")
		}
	case chunks[0] == "stopmine":
		if numofparams == requestmap["stopmine"] {
			_, err := client.StopMining(ctx)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Println("mining stopped")
		} else {
			fmt.Println("incorrect format: should be stopMine")
		}
	case chunks[0] == "sendasset":
		if numofparams == requestmap["sendasset"] {
			addrString1, err := parseInput(chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			addrString2, err := parseInput(chunks[2])
			if err != nil {
				fmt.Println(err)
				break
			}
			sender_common := stringAddrToCommonAddr(addrString1)
			receiver_common := stringAddrToCommonAddr(addrString2)
			value := new(big.Int)
			value, ok := value.SetString(chunks[3], 10)
			if !ok {
				fmt.Println("SetString: error")
				break
			}
			result, err := client.SendAsset(ctx, helper.Address(sender_common), helper.Address(receiver_common), value)
			if err != nil {
				fmt.Println(err)
				break
			}
			hexHash := hex.EncodeToString(result)
			green("%s\n", hexHash)
		} else {
			fmt.Println("incorrect format: should be sendAsset [from] [to] [password]")
		}
	default:
		fmt.Println("undefined cmd")
	}
	return nil
}

func prettyprint(b []byte) ([]byte, error) {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	return out.Bytes(), err
}

func parseInput(input string) (string, error) {
	// TODO: save 42 to a constant value
	length := len(input)
	if length != 42 {
		return "", errors.New("input address should have the length of 40 and have a prefix of 0x, e.g. 0x9821e8c1dc176c92cac40b3c1fdb795aa1b38f89")
	}
	if !strings.HasPrefix(input, "0x") {
		return "", errors.New("input should have prefix of 0x")
	}
	return input[2:length], nil
}

func byteArrayToString(addr []byte) string {
	stringAddr := 	hex.EncodeToString(addr)
	return stringAddr
}

func stringAddrToCommonAddr(account string) helper.Address {
	var addr_common [20]byte
	addr, _ := hex.DecodeString(account)
	for index, a := range addr {
		addr_common[index] = a
		//fmt.Printf("%v, ", addr_common[index])
	}
	//fmt.Println()
	return addr_common
}
