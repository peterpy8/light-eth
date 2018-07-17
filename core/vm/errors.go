package vm

import (
	"errors"
	"fmt"

	"github.com/siotchain/siot/params"
)

var OutOfGasError = errors.New("Out of gas")
var CodeStoreOutOfGasError = errors.New("ExternalLogic creation code storage out of gas")
var DepthError = fmt.Errorf("Max call depth exceeded (%d)", params.CallCreateDepth)
var TraceLimitReachedError = errors.New("The number of logs reached the specified limit")
