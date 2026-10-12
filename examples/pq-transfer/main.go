// pq-transfer sends TRX on the Nile testnet from an address controlled by an
// FN-DSA-512 post-quantum key. The transaction carries the PQ public key and
// signature in pq_auth_sig instead of an ECDSA signature.
//
// By default it sends from the key's own address. With -from it sends from an
// account whose owner permission lists the PQ key (see examples/pq-migrate).
//
// Experimental: TIP-899 is a draft and is live only on the Nile testnet.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/fbsobreira/gotron-sdk/pkg/signer"
	"github.com/fbsobreira/gotron-sdk/pkg/txbuilder"

	"github.com/fbsobreira/gotron-examples/utils"
)

func main() {
	var (
		keystorePath string
		from         string
		to           string
		amount       int64
		node         string
		dryRun       bool
	)

	flag.StringVar(&keystorePath, "keystore", os.Getenv("PQ_KEYSTORE"), "PQ keystore file (default: $PQ_KEYSTORE)")
	flag.StringVar(&from, "from", "", "Sending account (default: the PQ key's own address)")
	flag.StringVar(&to, "to", "", "Recipient address (base58)")
	flag.Int64Var(&amount, "amount", 1_000_000, "Amount in sun (1 TRX = 1,000,000 sun)")
	flag.StringVar(&node, "node", utils.NileNode, "gRPC node")
	flag.BoolVar(&dryRun, "dryrun", false, "Sign transaction but do not broadcast")
	flag.Parse()

	if keystorePath == "" {
		log.Fatal("-keystore or PQ_KEYSTORE is required (create one with go run ./examples/pq-keys)")
	}
	if to == "" {
		log.Fatal("-to is required")
	}
	if amount <= 0 {
		log.Fatal("-amount must be > 0")
	}

	key := utils.LoadPQKeystore(keystorePath)
	defer key.Destroy()
	// Fail before any network call if this build cannot sign with the key.
	utils.RequirePQScheme(key.Scheme())

	s, err := signer.NewPQSigner(key)
	if err != nil {
		log.Fatalf("failed to create PQ signer: %v", err)
	}
	if from == "" {
		from = s.Address().String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn := utils.NewGRPCClient(node)
	defer conn.Stop()

	// A new PQ address does not exist on chain until something is sent to it,
	// and the SDK reports that as client.ErrAccountNotFound.
	acc, err := conn.GetAccountCtx(ctx, from)
	if err != nil && !errors.Is(err, client.ErrAccountNotFound) {
		log.Fatalf("account lookup failed: %v", err)
	}
	if acc.GetBalance() == 0 {
		log.Fatalf("%s has no TRX on this network. Fund it from the Nile faucet first: https://nileex.io/join/getJoinPage\n"+
			"Each PQ transfer burns about 1.7 TRX of bandwidth unless the account has staked bandwidth.", from)
	}

	fmt.Println("=== PQ Transfer ===")
	fmt.Printf("Scheme:  %s\n", key.Scheme())
	fmt.Printf("PQ key:  %s\n", s.Address())
	fmt.Printf("From:    %s (balance %s)\n", from, utils.FormatTRX(acc.GetBalance()))
	fmt.Printf("To:      %s\n", to)
	fmt.Printf("Amount:  %s (%d sun)\n", utils.FormatTRX(amount), amount)
	fmt.Println()

	tx := txbuilder.New(conn).Transfer(from, to, amount)

	if dryRun {
		// Send checks the scheme itself; a dry run has to ask explicitly.
		allowed, err := conn.PQSchemeAllowedCtx(ctx, key.Scheme())
		if err != nil {
			log.Fatalf("PQ capability check failed: %v", err)
		}
		if !allowed {
			log.Fatalf("%s is not enabled on %s", key.Scheme(), node)
		}
		signed, err := tx.Sign(ctx, s)
		if err != nil {
			log.Fatalf("signing failed: %v", err)
		}
		printPQAuthSig(signed)
		fmt.Println("Mode:    DRY RUN (not broadcast)")
		return
	}

	// SendAndConfirm refuses before signing (txcore.ErrPQSchemeDisabled) if
	// the node has the scheme switched off.
	receipt, err := tx.SendAndConfirm(ctx, s)
	if err != nil {
		log.Fatalf("transfer failed: %v", err)
	}
	if receipt.Error != "" {
		log.Fatalf("transfer rejected: %s (txid %s)", receipt.Error, receipt.TxID)
	}

	fmt.Printf("TxID:    %s\n", receipt.TxID)
	fmt.Printf("Block:   %d\n", receipt.BlockNumber)
	fmt.Printf("Fee:     %s\n", utils.FormatTRX(receipt.Fee))

	// Read the transaction back to show what the node stored.
	onChain, err := conn.GetTransactionByIDCtx(ctx, receipt.TxID)
	if err != nil {
		log.Fatalf("fetch transaction failed: %v", err)
	}
	printPQAuthSig(onChain)
}

func printPQAuthSig(tx *core.Transaction) {
	fmt.Printf("ECDSA signatures: %d\n", len(tx.GetSignature()))
	for i, p := range tx.GetPqAuthSig() {
		fmt.Printf("pq_auth_sig[%d]:   %s, public key %d bytes, signature %d bytes\n",
			i, p.GetScheme(), len(p.GetPublicKey()), len(p.GetSignature()))
	}
}
