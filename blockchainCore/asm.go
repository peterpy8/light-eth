package blockchainCore

import (
	"fmt"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/blockchainCore/localEnv"
)

func Disassemble(script []byte) (asm []string) {
	pc := new(big.Int)
	for {
		if pc.Cmp(big.NewInt(int64(len(script)))) >= 0 {
			return
		}

		// Get the memory location of pc
		val := script[pc.Int64()]
		// Get the opcode (it must be an opcode!)
		op := localEnv.OpCode(val)

		asm = append(asm, fmt.Sprintf("%04v: %v", pc, op))

		switch op {
		case localEnv.PUSH1, localEnv.PUSH2, localEnv.PUSH3, localEnv.PUSH4, localEnv.PUSH5, localEnv.PUSH6, localEnv.PUSH7, localEnv.PUSH8,
			localEnv.PUSH9, localEnv.PUSH10, localEnv.PUSH11, localEnv.PUSH12, localEnv.PUSH13, localEnv.PUSH14, localEnv.PUSH15,
			localEnv.PUSH16, localEnv.PUSH17, localEnv.PUSH18, localEnv.PUSH19, localEnv.PUSH20, localEnv.PUSH21, localEnv.PUSH22,
			localEnv.PUSH23, localEnv.PUSH24, localEnv.PUSH25, localEnv.PUSH26, localEnv.PUSH27, localEnv.PUSH28, localEnv.PUSH29,
			localEnv.PUSH30, localEnv.PUSH31, localEnv.PUSH32:
			pc.Add(pc, helper.Big1)
			a := int64(op) - int64(localEnv.PUSH1) + 1
			if int(pc.Int64()+a) > len(script) {
				return
			}

			data := script[pc.Int64() : pc.Int64()+a]
			if len(data) == 0 {
				data = []byte{0}
			}
			asm = append(asm, fmt.Sprintf("%04v: 0x%x", pc, data))

			pc.Add(pc, big.NewInt(a-1))
		}

		pc.Add(pc, helper.Big1)
	}
}
