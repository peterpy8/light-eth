package node

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

const (
	DefaultIPCSocket = "siotchain.ipc"  // Default (relative) name of the IPC RPC socket
	DefaultHTTPHost  = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort  = 8800        // Default TCP port for the HTTP RPC server
	DefaultWSHost    = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort    = 8800        // Default TCP port for the websocket RPC server
)

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Siotchain")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "Siotchain")
		} else {
			return filepath.Join(home, ".siotchain")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
