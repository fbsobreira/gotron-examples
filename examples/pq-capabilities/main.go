// pq-capabilities prints which TIP-899 post-quantum signature schemes a node
// accepts, and which ones this binary can sign with.
//
// Experimental: TIP-899 is a draft and is live only on the Nile testnet.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/pqc"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"

	"github.com/fbsobreira/gotron-examples/utils"
)

func main() {
	var node string
	flag.StringVar(&node, "node", "", "Query only this gRPC node (default: Nile and mainnet)")
	flag.Parse()

	nodes := []string{utils.NileNode, utils.DefaultNode}
	if node != "" {
		nodes = []string{node}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== Node PQ Capabilities ===")
	for _, n := range nodes {
		conn := utils.NewGRPCClient(n)
		caps, err := conn.GetPQCapabilitiesCtx(ctx)
		conn.Stop()
		if err != nil {
			log.Fatalf("query %s failed: %v", n, err)
		}
		fmt.Printf("%s\n", n)
		fmt.Printf("  FN_DSA_512: %v\n", caps.FNDSA512)
		fmt.Printf("  ML_DSA_44:  %v\n", caps.MLDSA44)
	}

	// Which schemes this binary can sign with. FN-DSA-512 needs -tags falcon.
	fmt.Println()
	fmt.Println("=== Local Signing Support ===")
	for _, s := range []core.PQScheme{core.PQScheme_FN_DSA_512, core.PQScheme_ML_DSA_44} {
		if err := pqc.SchemeAvailable(s); err != nil {
			fmt.Printf("%-11s no (%v)\n", s.String()+":", err)
			continue
		}
		fmt.Printf("%-11s yes\n", s.String()+":")
	}
}
