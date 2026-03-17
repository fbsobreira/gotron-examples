package main

import (
	"fmt"

	"github.com/fbsobreira/gotron-examples/utils"
)

func main() {
	signer := utils.LoadSigner()
	fmt.Println(signer.Address)
}
