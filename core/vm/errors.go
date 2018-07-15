package vm

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/params"
)

var OutOfGasError = errors.New("Out of gas")
var CodeStoreOutOfGasError = errors.New("Contract creation code storage out of gas")
var DepthError = fmt.Errorf("Max call depth exceeded (%d)", params.CallCreateDepth)
var TraceLimitReachedError = errors.New("The number of logs reached the specified limit")
