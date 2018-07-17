package debug

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
	"gopkg.in/urfave/cli.v1"
)

var (
	verbosityFlag = cli.GenericFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=core, 5=debug, 6=detail",
		Value: glog.GetVerbosity(),
	}
	vmoduleFlag = cli.GenericFlag{
		Name:  "vmodule",
		Usage: "Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. siot/*=6,p2p=5)",
		Value: glog.GetVModule(),
	}
	backtraceAtFlag = cli.GenericFlag{
		Name:  "backtrace",
		Usage: "Request a stack trace at a specific logging statement (e.g. \"block.go:271\")",
		Value: glog.GetTraceLocation(),
	}
	pprofFlag = cli.BoolFlag{
		Name:  "pprof",
		Usage: "Enable the pprof HTTP server",
	}
	pprofPortFlag = cli.IntFlag{
		Name:  "pprofport",
		Usage: "pprof HTTP server listening port",
		Value: 6060,
	}
	pprofAddrFlag = cli.StringFlag{
		Name:  "pprofaddr",
		Usage: "pprof HTTP server listening interface",
		Value: "127.0.0.1",
	}
	memprofilerateFlag = cli.IntFlag{
		Name:  "memprofilerate",
		Usage: "Turn on memory profiling with the given rate",
		Value: runtime.MemProfileRate,
	}
	blockprofilerateFlag = cli.IntFlag{
		Name:  "blockprofilerate",
		Usage: "Turn on block profiling with the given rate",
	}
	cpuprofileFlag = cli.StringFlag{
		Name:  "cpuprofile",
		Usage: "Write CPU profile to the given file",
	}
	traceFlag = cli.StringFlag{
		Name:  "trace",
		Usage: "Write execution trace to the given file",
	}
)

// Flags holds all cmd-line flags required for debugging.
var Flags = []cli.Flag{
	verbosityFlag, vmoduleFlag, backtraceAtFlag,
	pprofFlag, pprofAddrFlag, pprofPortFlag,
	memprofilerateFlag, blockprofilerateFlag, cpuprofileFlag, traceFlag,
}

// Setup initializes profiling and logging based on the CLI flags.
// It should be called as early as possible in the program.
func Setup(ctx *cli.Context) error {
	// logging
	glog.CopyStandardLogTo("INFO")
	glog.SetToStderr(true)

	// profiling, tracing
	runtime.MemProfileRate = ctx.GlobalInt(memprofilerateFlag.Name)
	Handler.SetBlockProfileRate(ctx.GlobalInt(blockprofilerateFlag.Name))
	if traceFile := ctx.GlobalString(traceFlag.Name); traceFile != "" {
		if err := Handler.StartGoTrace(traceFile); err != nil {
			return err
		}
	}
	if cpuFile := ctx.GlobalString(cpuprofileFlag.Name); cpuFile != "" {
		if err := Handler.StartCPUProfile(cpuFile); err != nil {
			return err
		}
	}

	// pprof server
	if ctx.GlobalBool(pprofFlag.Name) {
		address := fmt.Sprintf("%s:%d", ctx.GlobalString(pprofAddrFlag.Name), ctx.GlobalInt(pprofPortFlag.Name))
		go func() {
			glog.V(logger.Info).Infof("starting pprof server at http://%s/debug/pprof", address)
			glog.Errorln(http.ListenAndServe(address, nil))
		}()
	}
	return nil
}

// Exit stops all running profiles, flushing their output to the
// respective file.
func Exit() {
	Handler.StopCPUProfile()
	Handler.StopGoTrace()
}
