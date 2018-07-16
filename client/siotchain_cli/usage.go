// Contains the siotchain cmd usage template and generator.

package main

import (
	"io"

	"github.com/ethereum/go-ethereum/client/utils"
	"gopkg.in/urfave/cli.v1"
)

// AppHelpTemplate is the test template for the default, global app help topic.
var AppHelpTemplate = `NAME:
   {{.App.Name}} - {{.App.Usage}}

USAGE:
   {{.App.HelpName}} [options]{{if .App.Commands}} cmd [cmd options]{{end}} {{if .App.ArgsUsage}}{{.App.ArgsUsage}}{{else}}[arguments...]{{end}}
   {{if .App.Version}}
VERSION:
   {{.App.Version}}
   {{end}}{{if len .App.Authors}}
AUTHOR(S):
   {{range .App.Authors}}{{ . }}{{end}}
   {{end}}{{if .App.Commands}}
COMMANDS:
   help, h  Shows a list of commands or help for one cmd
   {{end}}
SIOTCHAIN-CLI OPTIONS:
  --rpcport value			HTTP-RPC server listening port (default: 8800)
  --request value			Request for JSON RPC call, if no request specified, will go into the interactive mode
REQUESTS SUPPORTED IN INTERACTIVE MODE:
	getNodeInfo					Get information of the node
	getAccounts					Get the address lists of all wallet of the node
	getNewAccount [password]					Create a new account with password
	unlockAccount [account addr] [password]					Unlock an account with password
	getBalance [account addr]					Get the current balance of the account
	connectPeer [peer url]					Connect to a peer (siot://[peerid]@127.0.0.1:10000)
	getPeers					Get id lists of all connected peers
	setMiner [account addr]					Set an account as miner
	startMine					Start mining	
	stopMine					Stop mining
	sentAsset [sender addr] [receiver addr] [value]					Send transaction from one account to another with value set
`

// flagGroup is a collection of flags belonging to a single topic.
type flagGroup struct {
	Name  string
	Flags []cli.Flag
}

// AppHelpFlagGroups is the application flags, grouped by functionality.
var AppHelpFlagGroups = []flagGroup{
	{
		Name: "SIOTCHAIN-CLI",
		Flags: []cli.Flag{
			utils.RPCPortFlag,
			utils.RequestFlag,
		},
	},
}

func init() {
	// Override the default app help template
	cli.AppHelpTemplate = AppHelpTemplate

	// Define a one shot struct to pass to the usage template
	type helpData struct {
		App        interface{}
		FlagGroups []flagGroup
	}
	// Override the default app help printer, but only for the global app help
	originalHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, tmpl string, data interface{}) {
		if tmpl == AppHelpTemplate {
			// Iterate over all the flags and add any uncategorized ones
			categorized := make(map[string]struct{})
			for _, group := range AppHelpFlagGroups {
				for _, flag := range group.Flags {
					categorized[flag.String()] = struct{}{}
				}
			}
			uncategorized := []cli.Flag{}
			if len(uncategorized) > 0 {
				// Append all ungategorized options to the misc group
				miscs := len(AppHelpFlagGroups[len(AppHelpFlagGroups)-1].Flags)
				AppHelpFlagGroups[len(AppHelpFlagGroups)-1].Flags = append(AppHelpFlagGroups[len(AppHelpFlagGroups)-1].Flags, uncategorized...)

				// Make sure they are removed afterwards
				defer func() {
					AppHelpFlagGroups[len(AppHelpFlagGroups)-1].Flags = AppHelpFlagGroups[len(AppHelpFlagGroups)-1].Flags[:miscs]
				}()
			}
			// Render out custom usage screen
			originalHelpPrinter(w, tmpl, helpData{data, AppHelpFlagGroups})
		} else {
			originalHelpPrinter(w, tmpl, AppHelpFlagGroups)
		}
	}
}
