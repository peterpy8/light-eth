// Package utils contains internal helper functions for Siotchain commands.
package utils

import (
	"fmt"
	"runtime"

	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/common/rlp"
)

const (
	VersionMajor = 1        // Major version component of the current release
	VersionMinor = 0        // Minor version component of the current release
	VersionPatch = 0        // Patch version component of the current release
)

// Version holds the textual version string.
var Version = func() string {
	v := fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
	return v
}()

// MakeDefaultExtraData returns the default Siotchain block extra data blob.
func MakeDefaultExtraData(clientIdentifier string) []byte {
	var clientInfo = struct {
		Version   uint
		Name      string
		GoVersion string
		Os        string
	}{uint(VersionMajor<<16 | VersionMinor<<8 | VersionPatch), clientIdentifier, runtime.Version(), runtime.GOOS}
	extra, err := rlp.EncodeToBytes(clientInfo)
	if err != nil {
		glog.V(logger.Warn).Infoln("error setting canonical miner information:", err)
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize.Uint64() {
		glog.V(logger.Warn).Infoln("error setting canonical miner information: extra exceeds", params.MaximumExtraDataSize)
		glog.V(logger.Debug).Infof("extra: %x\n", extra)
		return nil
	}
	return extra
}
