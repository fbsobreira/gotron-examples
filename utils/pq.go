package utils

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/fbsobreira/gotron-sdk/pkg/pqc"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"golang.org/x/term"
)

const (
	// NileNode is the Nile testnet gRPC endpoint, the only network with
	// TIP-899 post-quantum signatures enabled.
	NileNode = "grpc.nile.trongrid.io:50051"

	// PQPasswordEnv is read for the keystore passphrase before prompting.
	PQPasswordEnv = "PQ_KEYSTORE_PASSWORD"

	// FalconBuildHint explains how to build with FN-DSA-512 support.
	FalconBuildHint = "FN-DSA-512 needs liboqs and CGO: brew install liboqs pkgconf, " +
		"then run with CGO_ENABLED=1 go run -tags falcon ..."
)

// ParsePQScheme maps a CLI scheme name to its TIP-899 enum value.
func ParsePQScheme(name string) core.PQScheme {
	switch name {
	case "fn-dsa-512":
		return core.PQScheme_FN_DSA_512
	case "ml-dsa-44":
		return core.PQScheme_ML_DSA_44
	default:
		log.Fatalf("unknown scheme %q (want fn-dsa-512 or ml-dsa-44)", name)
		return core.PQScheme_UNKNOWN_PQ_SCHEME
	}
}

// RequirePQScheme exits with build instructions when this binary cannot
// sign with scheme (FN-DSA-512 without -tags falcon).
func RequirePQScheme(scheme core.PQScheme) {
	err := pqc.SchemeAvailable(scheme)
	if errors.Is(err, pqc.ErrFalconUnavailable) {
		log.Fatalf("%s is not available in this build.\n%s", scheme, FalconBuildHint)
	}
	if err != nil {
		log.Fatalf("%s is not available: %v", scheme, err)
	}
}

// ReadPassphrase returns the keystore passphrase from PQ_KEYSTORE_PASSWORD,
// or prompts for it on the terminal without echo. With confirm set, the
// prompt asks twice. Never take a passphrase as a CLI argument: it ends up
// in shell history and process listings.
func ReadPassphrase(confirm bool) string {
	if p := os.Getenv(PQPasswordEnv); p != "" {
		return p
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		log.Fatalf("no terminal to prompt on; set %s", PQPasswordEnv)
	}
	p := prompt("Keystore passphrase: ")
	if p == "" {
		log.Fatal("passphrase must not be empty")
	}
	if confirm && prompt("Repeat passphrase: ") != p {
		log.Fatal("passphrases do not match")
	}
	return p
}

func prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("failed to read passphrase: %v", err)
	}
	return string(b)
}

// LoadPQKeystore decrypts a wallet-cli compatible post-quantum keystore.
// The caller should Destroy the key when done.
func LoadPQKeystore(path string) pqc.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read keystore: %v", err)
	}
	key, err := pqc.DecryptKeystore(data, ReadPassphrase(false))
	if err != nil {
		log.Fatalf("failed to decrypt keystore: %v", err)
	}
	return key
}
