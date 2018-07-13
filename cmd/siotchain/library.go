// Contains a simple library definition to allow creating a Siot instance from
// straight C code.

package main

import "C"
import (
	"fmt"
	"os"
	"strings"
)

//export doRun
func doRun(args *C.char) C.int {
	// This is equivalent to siotchain.main, just modified to handle the function arg passing
	if err := app.Run(strings.Split("siotchain "+C.GoString(args), " ")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -1
	}
	return 0
}
