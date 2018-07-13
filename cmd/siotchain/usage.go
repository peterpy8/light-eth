// Contains the siotchain command usage template and generator.

package main

import (
	"io"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"gopkg.in/urfave/cli.v1"
)

// AppHelpTemplate is the test template for the default, global app help topic.
var AppHelpTemplate = `NAME:
   {{.App.Name}} - {{.App.Usage}}

USAGE:
   {{.App.HelpName}} [options]{{if .App.Commands}} command [command options]{{end}} {{if .App.ArgsUsage}}{{.App.ArgsUsage}}{{else}}[arguments...]{{end}}
   {{if .App.Version}}
VERSION:
   {{.App.Version}}
   {{end}}{{if len .App.Authors}}
AUTHOR(S):
   {{range .App.Authors}}{{ . }}{{end}}
   {{end}}{{if .App.Commands}}
COMMANDS:
   init     Initialize a new genesis block
   help, h  Shows a list of commands or help for one command
   {{end}}
SIOTCHAIN OPTIONS:
  --dir value			  			 Target directory to save the databases and account keystore (default: "/home/vivid/.siotchain")
  --networkport value       Network listening port (default: 10000)
  --rpc                     Enable the HTTP-RPC server
  --rpcport value           HTTP-RPC server listening port (default: 8800)
  --chainnetwork value      Network identifier (default: 9876)
`

	// flagGroup is a collection of flags belonging to a single topic.
	type flagGroup struct {
		Name  string
		Flags []cli.Flag
	}

	// AppHelpFlagGroups is the application flags, grouped by functionality.
	var AppHelpFlagGroups = []flagGroup{
		{
			Name: "SIOTCHAIN",
			Flags: []cli.Flag{
				utils.DataDirFlag,
				utils.ListenPortFlag,
				utils.RPCEnabledFlag,
				utils.RPCPortFlag,
				utils.NetworkIdFlag,
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
