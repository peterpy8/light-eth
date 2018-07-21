package localEnv

import (
	"errors"
	"fmt"

	"github.com/siotchain/siot/configure"
)

var OutOfGasError = errors.New("Out of gas")
var CodeStoreOutOfGasError = errors.New("ExternalLogic creation code storage out of gas")
var DepthError = fmt.Errorf("Max call depth exceeded (%d)", configure.CallCreateDepth)
var TraceLimitReachedError = errors.New("The number of logs reached the specified limit")
