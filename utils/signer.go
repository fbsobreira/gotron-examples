package utils

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"
	"github.com/fbsobreira/gotron-sdk/pkg/keys"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
)

// Signer holds a loaded private key and provides signing utilities.
// The key is stored as hex to avoid importing btcec in the public surface;
// gotron-sdk's keys package handles all key operations internally.
type Signer struct {
	Address string
	hexKey  string
}

// LoadSigner loads a private key from TRON_PRIVATE_KEY environment variable.
func LoadSigner() *Signer {
	hexKey := os.Getenv("TRON_PRIVATE_KEY")
	if hexKey == "" {
		log.Fatal("TRON_PRIVATE_KEY environment variable is required")
	}
	// Validate key and derive address at load time.
	key, err := keys.GetPrivateKeyFromHex(hexKey)
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}
	addr := address.BTCECPubkeyToAddress(key.PubKey()).String()
	return &Signer{Address: addr, hexKey: hexKey}
}

// Sign signs a transaction and returns the signed transaction.
func (s *Signer) Sign(tx *core.Transaction) *core.Transaction {
	key, err := keys.GetPrivateKeyFromHex(s.hexKey)
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}
	signed, err := transaction.SignTransaction(tx, key)
	if err != nil {
		log.Fatalf("signing failed: %v", err)
	}
	return signed
}

// SignAndBroadcast signs a transaction and broadcasts it.
// If dryRun is true, it signs but does not broadcast.
func (s *Signer) SignAndBroadcast(conn *client.GrpcClient, tx *api.TransactionExtention, dryRun bool) {
	if tx.GetResult().GetCode() != 0 {
		log.Fatalf("transaction creation failed: %s", string(tx.GetResult().GetMessage()))
	}

	signedTx := s.Sign(tx.GetTransaction())

	txID := hex.EncodeToString(tx.GetTxid())
	fmt.Printf("TxID:   %s\n", txID)
	fmt.Printf("Signed: yes (%d signature(s))\n", len(signedTx.GetSignature()))

	if dryRun {
		fmt.Println("Mode:   DRY RUN (not broadcast)")
		return
	}

	fmt.Println("Broadcasting...")
	result, err := conn.Broadcast(signedTx)
	if err != nil {
		log.Fatalf("broadcast failed: %v", err)
	}

	fmt.Printf("Result: %s\n", result.GetCode().String())
	if result.GetCode() != api.Return_SUCCESS {
		fmt.Printf("Message: %s\n", string(result.GetMessage()))
	}
}
