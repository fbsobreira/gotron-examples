package utils

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
)

// PrintTxResult prints the result of a TriggerConstantContract call,
// including revert reasons decoded from ABI-encoded error data.
func PrintTxResult(tx *api.TransactionExtention) {
	if tx.GetResult().GetCode() != 0 {
		fmt.Printf("Error: %s\n", string(tx.GetResult().GetMessage()))
		return
	}

	fmt.Println("Result: SUCCESS")

	if len(tx.GetConstantResult()) > 0 {
		data := tx.GetConstantResult()[0]
		fmt.Printf("Output[0]: %s\n", hex.EncodeToString(data))

		// Check for ABI-encoded revert reason: Error(string) selector = 0x08c379a0
		if len(data) >= 68 && hex.EncodeToString(data[:4]) == "08c379a0" {
			strLen := new(big.Int).SetBytes(data[36:68]).Int64()
			if int64(len(data)) >= 68+strLen {
				fmt.Printf("Revert:    %s\n", string(data[68:68+strLen]))
			}
		} else if len(data) == 32 {
			val := new(big.Int).SetBytes(data)
			fmt.Printf("Decoded:   %s\n", val.String())
		}

		for i, r := range tx.GetConstantResult()[1:] {
			fmt.Printf("Output[%d]: %s\n", i+1, hex.EncodeToString(r))
		}
	}

	if len(tx.GetTxid()) > 0 {
		fmt.Printf("TxID:      %s\n", hex.EncodeToString(tx.GetTxid()))
	}

	if energyUsed := tx.GetEnergyUsed(); energyUsed > 0 {
		fmt.Printf("Energy:    %d\n", energyUsed)
	}
}
