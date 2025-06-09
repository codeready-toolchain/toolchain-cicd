package main

import (
	"fmt"

	"github.com/codeready-toolchain/pairing/dummy"
)

func main() {
	result := dummy.Add(2, 3)
	fmt.Printf("2 + 3 = %d\n", result)
}
