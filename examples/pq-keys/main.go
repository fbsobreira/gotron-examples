// pq-keys generates a TIP-899 post-quantum key, saves it as an encrypted
// wallet-cli compatible keystore, and reloads it to check the round trip.
//
// Experimental: TIP-899 is a draft and is live only on the Nile testnet.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fbsobreira/gotron-sdk/pkg/keystore"
	"github.com/fbsobreira/gotron-sdk/pkg/pqc"

	"github.com/fbsobreira/gotron-examples/utils"
)

func main() {
	var (
		schemeName string
		out        string
	)

	flag.StringVar(&schemeName, "scheme", "ml-dsa-44", "Scheme: ml-dsa-44 (pure Go) or fn-dsa-512 (needs -tags falcon; the one enabled on Nile)")
	flag.StringVar(&out, "out", "pq-keystore.json", "Keystore file to write (never overwritten)")
	flag.Parse()

	scheme := utils.ParsePQScheme(schemeName)
	utils.RequirePQScheme(scheme)

	if _, err := os.Stat(out); err == nil {
		log.Fatalf("%s already exists; refusing to overwrite", out)
	}

	key, err := pqc.GenerateKey(scheme, rand.Reader)
	if err != nil {
		log.Fatalf("key generation failed: %v", err)
	}
	defer key.Destroy()

	priv := key.Bytes()
	privSize := len(priv)
	clear(priv)

	fmt.Println("=== New PQ Key ===")
	fmt.Printf("Scheme:      %s\n", key.Scheme())
	fmt.Printf("Address:     %s\n", key.Public().Address())
	fmt.Printf("Public key:  %d bytes\n", len(key.Public().Bytes()))
	fmt.Printf("Private key: %d bytes\n", privSize)
	fmt.Println()

	// Encrypt with the standard scrypt cost, as wallet-cli does.
	password := utils.ReadPassphrase(true)
	data, err := pqc.EncryptKeystore(key, password, keystore.StandardScryptN, keystore.StandardScryptP)
	if err != nil {
		log.Fatalf("keystore encryption failed: %v", err)
	}
	// O_EXCL: fail rather than overwrite a file created since the check above.
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		log.Fatalf("failed to create keystore: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		log.Fatalf("failed to write keystore: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("failed to close keystore: %v", err)
	}

	// Reload from disk and confirm it is the same key.
	data, err = os.ReadFile(out)
	if err != nil {
		log.Fatalf("failed to read keystore: %v", err)
	}
	loaded, err := pqc.DecryptKeystore(data, password)
	if err != nil {
		log.Fatalf("keystore decryption failed: %v", err)
	}
	defer loaded.Destroy()
	if !loaded.Public().Equal(key.Public()) {
		log.Fatal("round trip failed: reloaded key differs from the generated one")
	}

	fmt.Println("=== Keystore ===")
	fmt.Printf("File:        %s (%d bytes, mode 0600)\n", out, len(data))
	fmt.Printf("Reloaded:    %s (matches)\n", loaded.Public().Address())
	fmt.Println()
	fmt.Println("Keep the keystore and its passphrase safe: anyone with both controls this address.")
	fmt.Println("Fund the address from the Nile faucet (https://nileex.io/join/getJoinPage) before sending from it.")
}
