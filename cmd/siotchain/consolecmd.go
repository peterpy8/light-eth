// Copyright 2016 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"gopkg.in/urfave/cli.v1"
	"bufio"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/net/context"
	"strconv"
	"github.com/fatih/color"
	"encoding/hex"
	"encoding/json"
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"errors"
	"github.com/ethereum/go-ethereum/p2p"
	"math/big"
)

// TODO:
// wei needs to deletes the geth console description
var (
	cliCommand = cli.Command{
		Action:     interact,
		Name:      "siotchain-cli",
		Usage:     "Start an interactive JavaScript environment",
		ArgsUsage: "", // TODO: Write this!
		Category:  "CONSOLE COMMANDS",
		Description: `
The Sitochain console is an interactive shell for user request for JSON RPC call
`,
	}
	consoleCommand = cli.Command{
		Action:    localConsole,
		Name:      "console",
		Usage:     "Start an interactive JavaScript environment",
		ArgsUsage: "", // TODO: Write this!
		Category:  "CONSOLE COMMANDS",
		Description: `
The Geth console is an interactive shell for the JavaScript runtime environment
which exposes a node admin interface as well as the Ðapp JavaScript API.
See https://github.com/ethereum/go-ethereum/wiki/Javascipt-Console
`,
	}
//	attachCommand = cli.Command{
//		Action:    remoteConsole,
//		Name:      "attach",
//		Usage:     "Start an interactive JavaScript environment (connect to node)",
//		ArgsUsage: "", // TODO: Write this!
//		Category:  "CONSOLE COMMANDS",
//		Description: `
//The Geth console is an interactive shell for the JavaScript runtime environment
//which exposes a node admin interface as well as the Ðapp JavaScript API.
//See https://github.com/ethereum/go-ethereum/wiki/Javascipt-Console.
//This command allows to open a console on a running siotchain node.
//`,
//	}
//	javascriptCommand = cli.Command{
//		Action:    ephemeralConsole,
//		Name:      "js",
//		Usage:     "Execute the specified JavaScript files",
//		ArgsUsage: "", // TODO: Write this!
//		Category:  "CONSOLE COMMANDS",
//		Description: `
//The JavaScript VM exposes a node admin interface as well as the Ðapp
//JavaScript API. See https://github.com/ethereum/go-ethereum/wiki/Javascipt-Console
//`,
//	}
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

type Inputs []string

type Payload struct {
	Jsonrpc string   `json:"jsonrpc"`
	Method string   `json:"method"`
	Params []string   `json:"params"`
	ID 		int   	`json:"id"`
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

func stringAddrToCommonAddr(account string) common.Address {
	var addr_common [20]byte
	addr, _ := hex.DecodeString(account)
	for index, a := range addr {
		addr_common[index] = a
		//fmt.Printf("%v, ", addr_common[index])
	}
	//fmt.Println()
	return addr_common
}

func byteArrayToCommonAddr(account rpc.HexBytes) common.Address {
	var addr_common [20]byte
	for index, a := range account {
		addr_common[index] = a
		//fmt.Printf("%v, ", addr_common[index])
	}
	//fmt.Println()
	return addr_common
}

func handleRequest(cliCtx *cli.Context, client *ethclient.Client, input string) error {
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
					SiotNetwork: cliCtx.GlobalString(utils.NetworkIdFlag.Name)}
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
			result, err := client.UnlockAccount(ctx, common.Address(addr_common), chunks[2])
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
			result, err := client.BalanceAt(ctx, common.Address(addr_common), nil)
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
		fmt.Println(requestmap["setminer"])
		if numofparams == requestmap["setminer"] {
			fmt.Println("request for setting a miner")
			addrString, err := parseInput(chunks[1])
			if err != nil {
				fmt.Println(err)
				break
			}
			addr_common := stringAddrToCommonAddr(addrString)
			result, err := client.SetMiner(ctx, common.Address(addr_common))
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Println(result)
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
			result, err := client.SendAsset(ctx, common.Address(sender_common), common.Address(receiver_common), value)
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
		fmt.Println("undefined command")
	}
	return nil
}

func readInput(ctx *cli.Context, client *ethclient.Client) error {
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

func interact(ctx *cli.Context) error {
	host := ctx.GlobalString(utils.RPCListenAddrFlag.Name)
	port := strconv.Itoa(ctx.GlobalInt(utils.RPCPortFlag.Name))
	urlChunks := []string{"http://", host, ":", port}
	url := strings.Join(urlChunks,"")
	//fmt.Println(url)
	client, _:= ethclient.Dial(url)
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

// localConsole starts a new siotchain node, attaching a JavaScript console to it at the
// same time.
func localConsole(ctx *cli.Context) error {
	// Create and start the node based on the CLI flags
	node := makeFullNode(ctx)
	startNode(ctx, node)
	defer node.Stop()

	// Attach to the newly started node and start the JavaScript console
	client, err := node.Attach()
	if err != nil {
		utils.Fatalf("Failed to attach to the inproc siotchain: %v", err)
	}
	config := console.Config{
		DataDir: node.DataDir(),
		DocRoot: ctx.GlobalString(utils.JSpathFlag.Name),
		Client:  client,
		Preload: utils.MakeConsolePreloads(ctx),
	}
	console, err := console.New(config)
	if err != nil {
		utils.Fatalf("Failed to start the JavaScript console: %v", err)
	}
	defer console.Stop(false)

	// If only a short execution was requested, evaluate and return
	if script := ctx.GlobalString(utils.ExecFlag.Name); script != "" {
		console.Evaluate(script)
		return nil
	}
	// Otherwise print the welcome screen and enter interactive mode
	console.Welcome()
	console.Interactive()

	return nil
}

// remoteConsole will connect to a remote siotchain instance, attaching a JavaScript
// console to it.
//func remoteConsole(ctx *cli.Context) error {
//	// Attach to a remotely running siotchain instance and start the JavaScript console
//	client, err := dialRPC(ctx.Args().First())
//	if err != nil {
//		utils.Fatalf("Unable to attach to remote siotchain: %v", err)
//	}
//	config := console.Config{
//		DataDir: utils.MakeDataDir(ctx),
//		DocRoot: ctx.GlobalString(utils.JSpathFlag.Name),
//		Client:  client,
//		Preload: utils.MakeConsolePreloads(ctx),
//	}
//	console, err := console.New(config)
//	if err != nil {
//		utils.Fatalf("Failed to start the JavaScript console: %v", err)
//	}
//	defer console.Stop(false)
//
//	// If only a short execution was requested, evaluate and return
//	if script := ctx.GlobalString(utils.ExecFlag.Name); script != "" {
//		console.Evaluate(script)
//		return nil
//	}
//	// Otherwise print the welcome screen and enter interactive mode
//	console.Welcome()
//	console.Interactive()
//
//	return nil
//}

// dialRPC returns a RPC client which connects to the given endpoint.
// The check for empty endpoint implements the defaulting logic
// for "siotchain attach" and "siotchain monitor" with no argument.
func dialRPC(endpoint string) (*rpc.Client, error) {
	if endpoint == "" {
		endpoint = node.DefaultIPCEndpoint(clientIdentifier)
	} else if strings.HasPrefix(endpoint, "rpc:") || strings.HasPrefix(endpoint, "ipc:") {
		// Backwards compatibility with siotchain < 1.5 which required
		// these prefixes.
		endpoint = endpoint[4:]
	}
	return rpc.Dial(endpoint)
}

// ephemeralConsole starts a new siotchain node, attaches an ephemeral JavaScript
// console to it, and each of the files specified as arguments and tears the
// everything down.
//func ephemeralConsole(ctx *cli.Context) error {
//	// Create and start the node based on the CLI flags
//	node := makeFullNode(ctx)
//	startNode(ctx, node)
//	defer node.Stop()
//
//	// Attach to the newly started node and start the JavaScript console
//	client, err := node.Attach()
//	if err != nil {
//		utils.Fatalf("Failed to attach to the inproc siotchain: %v", err)
//	}
//	config := console.Config{
//		DataDir: node.DataDir(),
//		DocRoot: ctx.GlobalString(utils.JSpathFlag.Name),
//		Client:  client,
//		Preload: utils.MakeConsolePreloads(ctx),
//	}
//	console, err := console.New(config)
//	if err != nil {
//		utils.Fatalf("Failed to start the JavaScript console: %v", err)
//	}
//	defer console.Stop(false)
//
//	// Evaluate each of the specified JavaScript files
//	for _, file := range ctx.Args() {
//		if err = console.Execute(file); err != nil {
//			utils.Fatalf("Failed to execute %s: %v", file, err)
//		}
//	}
//	// Wait for pending callbacks, but stop for Ctrl-C.
//	abort := make(chan os.Signal, 1)
//	signal.Notify(abort, os.Interrupt)
//
//	go func() {
//		<-abort
//		os.Exit(0)
//	}()
//	console.Stop(true)
//
//	return nil
//}
