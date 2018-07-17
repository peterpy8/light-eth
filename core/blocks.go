package core

import "github.com/siotchain/siot/helper"

// Set of manually tracked bad hashes (usually hard forks)
var BadHashes = map[helper.Hash]bool{
	helper.HexToHash("05bef30ef572270f654746da22639a7a0c97dd97a7050b9e252391996aaeb689"): true,
}
