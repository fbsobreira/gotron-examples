package utils

import (
	"log"
	"os"

	"github.com/fbsobreira/gotron-sdk/pkg/keys"
	"github.com/fbsobreira/gotron-sdk/pkg/signer"
)

// LoadSigner loads a private key from TRON_PRIVATE_KEY environment variable
// and returns a signer.Signer suitable for the fluent builder API.
func LoadSigner() signer.Signer {
	hexKey := os.Getenv("TRON_PRIVATE_KEY")
	if hexKey == "" {
		log.Fatal("TRON_PRIVATE_KEY environment variable is required")
	}
	key, err := keys.GetPrivateKeyFromHex(hexKey)
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}
	s, err := signer.NewPrivateKeySignerFromBTCEC(key)
	if err != nil {
		log.Fatalf("failed to create signer: %v", err)
	}
	return s
}
