package localEnv

import (
	"fmt"
	"math/big"
)

// stack is an object for basic stack operations. Items popped to the stack are
// expected to be changed and modified. stack does not take care of adding newly
// initialised objects.
type Stack struct {
	data []*big.Int
}

func newstack() *Stack {
	return &Stack{}
}

func (st *Stack) Data() []*big.Int {
	return st.data
}

func (st *Stack) push(d *big.Int) {
	// NOTE push limit (1024) is checked in baseCheck
	//stackItem := new(big.Int).Set(d)
	//st.data = append(st.data, stackItem)
	st.data = append(st.data, d)
}
func (st *Stack) pushN(ds ...*big.Int) {
	st.data = append(st.data, ds...)
}

func (st *Stack) pop() (ret *big.Int) {
	ret = st.data[len(st.data)-1]
	st.data = st.data[:len(st.data)-1]
	return
}

func (st *Stack) len() int {
	return len(st.data)
}

func (st *Stack) swap(n int) {
	st.data[st.len()-n], st.data[st.len()-1] = st.data[st.len()-1], st.data[st.len()-n]
}

func (st *Stack) dup(n int) {
	st.push(new(big.Int).Set(st.data[st.len()-n]))
}

func (st *Stack) peek() *big.Int {
	return st.data[st.len()-1]
}

func (st *Stack) require(n int) error {
	if st.len() < n {
		return fmt.Errorf("stack underflow (%d <=> %d)", len(st.data), n)
	}
	return nil
}

func (st *Stack) Print() {
	fmt.Println("### stack ###")
	if len(st.data) > 0 {
		for i, val := range st.data {
			fmt.Printf("%-3d  %v\n", i, val)
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("#############")
}
