// pq-migrate authorizes a TIP-899 post-quantum address on an existing ECDSA
// account by rewriting its owner and active permissions
// (AccountPermissionUpdateContract). The account keeps its address and
// assets. The update itself is signed by the current ECDSA key.
//
// The PQ key is read from its keystore: that proves you hold it, and tells
// which scheme it uses, so a key the node cannot verify is refused. Only its
// public key and address are used.
//
// Dry run by default: it prints the permission update without broadcasting.
//
// Experimental: TIP-899 is a draft and is live only on the Nile testnet.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/fbsobreira/gotron-sdk/pkg/txcore"

	"github.com/fbsobreira/gotron-examples/utils"
)

const (
	// ModeEither lists both keys with threshold 1: either key alone can sign.
	ModeEither = "either"
	// ModeBoth lists both keys with threshold 2: every transaction needs both.
	ModeBoth = "both"
	// ModePQOnly lists only the PQ key: the ECDSA key loses all authority.
	ModePQOnly = "pq-only"
)

func main() {
	var (
		keystorePath string
		mode         string
		node         string
		operations   string
		confirm      bool
	)

	flag.StringVar(&keystorePath, "keystore", os.Getenv("PQ_KEYSTORE"), "Keystore of the PQ key to authorize (default: $PQ_KEYSTORE)")
	flag.StringVar(&mode, "mode", ModeEither, "Permission layout: either (ECDSA or PQ), both (ECDSA and PQ), pq-only")
	flag.StringVar(&node, "node", utils.NileNode, "gRPC node")
	flag.StringVar(&operations, "operations", "", "Active operations as a hex bitmap, only for an account with no stored permissions")
	flag.BoolVar(&confirm, "confirm", false, "Sign and broadcast the update (default: dry run)")
	flag.Parse()

	if keystorePath == "" {
		log.Fatal("-keystore or PQ_KEYSTORE is required (create one with go run ./examples/pq-keys)")
	}

	// Only the public part of the PQ key is needed; wipe the secret right away.
	pqKey := utils.LoadPQKeystore(keystorePath)
	pqScheme := pqKey.Scheme()
	pqAddress := pqKey.Public().Address().String()
	pqKey.Destroy()

	// The current ECDSA key (TRON_PRIVATE_KEY) owns the account and signs the update.
	s := utils.LoadSigner()
	owner := s.Address().String()

	keys, threshold := permissionKeys(mode, owner, pqAddress)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn := utils.NewGRPCClient(node)
	defer conn.Stop()

	caps, err := conn.GetPQCapabilitiesCtx(ctx)
	if err != nil {
		log.Fatalf("PQ capability check failed: %v", err)
	}
	ecdsaAlone := keys[owner] >= threshold
	if !caps.Allows(pqScheme) {
		// The node would refuse every signature from this key.
		log.Fatalf("%s does not accept %s signatures; refusing to authorize this key", node, pqScheme)
	}

	acc, err := conn.GetAccountCtx(ctx, owner)
	if err != nil {
		log.Fatalf("account lookup for %s failed: %v (the account must exist and hold the permission-update fee; Nile faucet: https://nileex.io/join/getJoinPage)", owner, err)
	}
	checkAccount(acc, owner)
	if !ecdsaAlone && !listsKey(acc.GetOwnerPermission(), pqAddress) {
		// Removing ECDSA authority is only safe once the PQ key is known to work.
		log.Fatalf("mode %q removes the ECDSA key's sole authority; first run -mode %s -confirm, "+
			"send a PQ-signed transaction from the account (examples/pq-transfer -from), then re-run", mode, ModeEither)
	}

	fmt.Println("=== PQ Permission Migration ===")
	fmt.Printf("Account:  %s (balance %s)\n", owner, utils.FormatTRX(acc.GetBalance()))
	fmt.Printf("PQ key:   %s (%s)\n", pqAddress, pqScheme)
	fmt.Printf("Node PQ:  FN_DSA_512=%v ML_DSA_44=%v\n", caps.FNDSA512, caps.MLDSA44)
	if fee, ok := chainParam(ctx, conn, "getUpdateAccountPermissionFee"); ok {
		fmt.Printf("Fee:      %s\n", utils.FormatTRX(fee))
	}
	fmt.Println()
	fmt.Println("Current permissions:")
	printPermissions(acc)

	// The update replaces every permission, so start from the current ones:
	// that keeps the active permission's name and operations.
	ownerPerm, _, actives, err := client.PermissionUpdateMaps(acc)
	switch {
	case errors.Is(err, client.ErrNoStoredPermissions):
		ownerPerm, actives = permissionsFromFlag(operations)
	case err != nil:
		log.Fatalf("read current permissions: %v", err)
	case operations != "":
		log.Fatal("-operations is only for an account with no stored permissions; this account's active operations are kept")
	}

	// The active permission gets the same keys as owner: an ECDSA-only active
	// permission would still allow transfers with the ECDSA key alone.
	ownerPerm["threshold"], ownerPerm["keys"] = threshold, keys
	actives[0]["threshold"], actives[0]["keys"] = threshold, keys

	ext, err := conn.UpdateAccountPermissionCtx(ctx, owner, ownerPerm, nil, actives)
	if err != nil {
		log.Fatalf("build permission update failed: %v", err)
	}

	update := new(core.AccountPermissionUpdateContract)
	if err := ext.GetTransaction().GetRawData().GetContract()[0].GetParameter().UnmarshalTo(update); err != nil {
		log.Fatalf("decode permission update failed: %v", err)
	}
	fmt.Println()
	fmt.Println("New permissions (from the built transaction):")
	printPermission("owner", update.GetOwner())
	for _, p := range update.GetActives() {
		printPermission("active", p)
	}
	fmt.Println()
	for _, p := range acc.GetActivePermission() {
		for _, k := range p.GetKeys() {
			if a := address.Address(k.GetAddress()).String(); keys[a] == 0 {
				fmt.Printf("Notice: active key %s is removed by this update.\n", a)
			}
		}
	}
	explainMode(mode, ecdsaAlone)

	if !confirm {
		fmt.Println()
		fmt.Printf("Draft ID: %x (a -confirm run builds a new transaction)\n", ext.GetTxid())
		fmt.Println("Mode:     DRY RUN (not signed, not broadcast). Re-run with -confirm to apply.")
		return
	}

	receipt, err := txcore.SendAndConfirm(ctx, conn, s, ext.GetTransaction(), 3*time.Second)
	if err != nil {
		log.Fatalf("permission update failed: %v", err)
	}
	if receipt.Error != "" {
		log.Fatalf("permission update rejected: %s (txid %s)", receipt.Error, receipt.TxID)
	}
	fmt.Println()
	fmt.Printf("TxID:     %s\n", receipt.TxID)
	fmt.Printf("Block:    %d\n", receipt.BlockNumber)
	fmt.Printf("Fee:      %s\n", utils.FormatTRX(receipt.Fee))

	after, err := conn.GetAccountCtx(ctx, owner)
	if err != nil {
		log.Fatalf("account lookup failed: %v", err)
	}
	fmt.Println()
	fmt.Println("Permissions on chain:")
	printPermissions(after)
}

// permissionKeys returns the key weights and threshold for a mode.
func permissionKeys(mode, ecdsa, pq string) (map[string]int64, int64) {
	switch mode {
	case ModeEither:
		return map[string]int64{ecdsa: 1, pq: 1}, 1
	case ModeBoth:
		return map[string]int64{ecdsa: 1, pq: 1}, 2
	case ModePQOnly:
		return map[string]int64{pq: 1}, 1
	default:
		log.Fatalf("unknown -mode %q (want %s, %s or %s)", mode, ModeEither, ModeBoth, ModePQOnly)
		return nil, 0
	}
}

// checkAccount refuses account shapes this example would silently break.
func checkAccount(acc *core.Account, owner string) {
	if acc.GetIsWitness() || acc.GetWitnessPermission() != nil {
		log.Fatal("witness accounts are not supported here: the update must also carry the witness permission, which takes exactly one key")
	}
	if len(acc.GetActivePermission()) > 1 {
		log.Fatal("account has several active permissions; this example would replace them with one")
	}
	// The ECDSA key must be able to sign the update under the current owner
	// permission, stored or implicit (the account's own key, threshold 1).
	op, err := client.SigningPermission(acc, 0)
	if err != nil {
		log.Fatalf("owner permission: %v", err)
	}
	var weight int64
	for _, k := range op.GetKeys() {
		if address.Address(k.GetAddress()).String() == owner {
			weight += k.GetWeight()
		}
	}
	if weight < op.GetThreshold() {
		log.Fatalf("TRON_PRIVATE_KEY alone cannot sign for the current owner permission (weight %d < threshold %d)", weight, op.GetThreshold())
	}
}

// listsKey reports whether a permission lists addr as a key.
func listsKey(p *core.Permission, addr string) bool {
	for _, k := range p.GetKeys() {
		if address.Address(k.GetAddress()).String() == addr {
			return true
		}
	}
	return false
}

// explainMode prints what the new permissions mean for the ECDSA key.
func explainMode(mode string, ecdsaAlone bool) {
	switch {
	case ecdsaAlone:
		fmt.Println("Result: PQ key added, ECDSA key keeps full authority. Either key can sign alone.")
		fmt.Println("This is a staging step, NOT a migration: a quantum attacker who breaks ECDSA still")
		fmt.Println("controls the account. Use it to test PQ signing, then re-run with -mode both or pq-only.")
	case mode == ModeBoth:
		fmt.Println("Result: every transaction needs BOTH an ECDSA and a PQ signature (threshold 2).")
		fmt.Println("WARNING: if the PQ key is lost, or governance disables its scheme, the account is locked.")
		fmt.Println("pq-transfer adds only the PQ signature: the ECDSA co-signature needs another tool.")
	default:
		fmt.Println("!!! PQ-ONLY: the ECDSA key loses ALL authority over this account. !!!")
		fmt.Println("WARNING: if the PQ key is lost, or governance disables its scheme, the account is")
		fmt.Println("locked for good. Back up the PQ keystore and test with -mode either first.")
	}
}

func printPermissions(acc *core.Account) {
	if acc.GetOwnerPermission() == nil {
		fmt.Println("  owner:  (default) own address, weight 1, threshold 1")
	} else {
		printPermission("owner", acc.GetOwnerPermission())
	}
	for _, p := range acc.GetActivePermission() {
		printPermission("active", p)
	}
}

func printPermission(kind string, p *core.Permission) {
	fmt.Printf("  %-6s  id=%d name=%q threshold=%d\n", kind+":", p.GetId(), p.GetPermissionName(), p.GetThreshold())
	for _, k := range p.GetKeys() {
		fmt.Printf("          key %s weight %d\n", address.Address(k.GetAddress()), k.GetWeight())
	}
}

// permissionsFromFlag builds the owner and active permissions for an account
// that has none stored: one created before multi-sign, at genesis or by a
// contract. No API gives the network's default active operations, and
// proposals change them, so the user passes them as a hex bitmap.
func permissionsFromFlag(operations string) (map[string]interface{}, []map[string]interface{}) {
	if operations == "" {
		log.Fatal("account has no stored owner or active permission, so there are no operations to copy. " +
			"Pass -operations with the active permission's hex operations bitmap, copied from an account " +
			"created recently on the same network")
	}
	bitmap, err := hex.DecodeString(operations)
	if err != nil {
		log.Fatalf("-operations: %v", err)
	}
	ops, err := client.PermissionOperations(bitmap)
	if err != nil {
		log.Fatalf("-operations: %v", err)
	}
	if len(ops) == 0 {
		log.Fatal("-operations sets no operations")
	}
	return map[string]interface{}{}, []map[string]interface{}{{"name": "active", "operations": ops}}
}

// chainParam reads one dynamic chain parameter by key.
func chainParam(ctx context.Context, conn *client.GrpcClient, key string) (int64, bool) {
	params, err := conn.GetChainParametersCtx(ctx)
	if err != nil {
		return 0, false
	}
	for _, p := range params.GetChainParameter() {
		if p.GetKey() == key {
			return p.GetValue(), true
		}
	}
	return 0, false
}
