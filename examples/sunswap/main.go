package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"

	"github.com/fbsobreira/gotron-examples/utils"
)

const (
	// SunSwap V2 Router on mainnet
	SunSwapV2Router = "TXF1xDbVGdxFGbovmmmXvBGu8ZiE3Lq4mR"

	// Common TRC20 tokens
	WTRX = "TNUC9Qb1rRpS5CbWLmNMxXBjyFoydXjWFR" // Wrapped TRX, 6 decimals
	USDT = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t" // Tether USD, 6 decimals
	USDD = "TPYmHEhy5n8TCEfYGqW2rPxsghSfzghPDn" // USDD, 18 decimals
)

func main() {
	var (
		action   string
		from     string
		tokenIn  string
		tokenOut string
		amountIn string
		slippage float64
		node     string
		dryRun   bool
	)

	flag.StringVar(&action, "action", "quote", "Action: quote, simulate-swap, swap, pair")
	flag.StringVar(&from, "from", "", "Caller address (base58, only for simulate-swap)")
	flag.StringVar(&tokenIn, "token-in", WTRX, "Input token address")
	flag.StringVar(&tokenOut, "token-out", USDT, "Output token address")
	flag.StringVar(&amountIn, "amount", "1000000", "Input amount in token smallest unit (sun for TRX)")
	flag.Float64Var(&slippage, "slippage", 0.5, "Slippage tolerance in percent (0–100)")
	flag.StringVar(&node, "node", "", "gRPC node (default: grpc.trongrid.io:50051)")
	flag.BoolVar(&dryRun, "dryrun", false, "Sign transaction but do not broadcast")
	flag.Parse()

	if slippage < 0 || slippage > 100 {
		log.Fatalf("slippage must be between 0 and 100, got %.2f", slippage)
	}

	conn := utils.NewGRPCClient(node)

	amount, ok := new(big.Int).SetString(amountIn, 10)
	if !ok {
		log.Fatalf("invalid amount: %s", amountIn)
	}

	switch action {
	case "quote":
		getQuote(conn, tokenIn, tokenOut, amount)
	case "simulate-swap":
		if from == "" {
			log.Fatal("required: -from for simulate-swap")
		}
		simulateSwap(conn, from, tokenIn, tokenOut, amount, slippage)
	case "swap":
		executeSwap(conn, tokenIn, tokenOut, amount, slippage, dryRun)
	case "pair":
		getPair(conn, tokenIn, tokenOut)
	default:
		log.Fatalf("unknown action: %s", action)
	}
}

// getQuote calls getAmountsOut on the router to get a price quote.
func getQuote(conn *client.GrpcClient, tokenIn, tokenOut string, amountIn *big.Int) {
	fmt.Println("=== SunSwap V2 Quote ===")
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Println()

	params := fmt.Sprintf(`[{"uint256":"%s"},{"address[]":["%s","%s"]}]`,
		amountIn.String(), tokenIn, tokenOut)

	tx, err := conn.TriggerConstantContract(
		"",
		SunSwapV2Router,
		"getAmountsOut(uint256,address[])",
		params,
	)
	if err != nil {
		log.Fatalf("getAmountsOut failed: %v", err)
	}

	if tx.GetResult().GetCode() != 0 {
		fmt.Printf("Error: %s\n", string(tx.GetResult().GetMessage()))
		return
	}

	decodeAmountsOutput(tx)
}

// simulateSwap simulates the swap via TriggerConstantContract.
// For WTRX input (TRX→Token), uses swapExactETHForTokens; otherwise swapExactTokensForTokens.
// Note: the WTRX path will revert because TriggerConstantContract cannot send TRX (msg.value=0).
func simulateSwap(conn *client.GrpcClient, from, tokenIn, tokenOut string, amountIn *big.Int, slippage float64) {
	fmt.Println("=== Simulate Swap ===")
	fmt.Printf("From:      %s\n", from)
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Printf("Slippage:  %.1f%%\n", slippage)
	fmt.Println()

	quoteParams := fmt.Sprintf(`[{"uint256":"%s"},{"address[]":["%s","%s"]}]`,
		amountIn.String(), tokenIn, tokenOut)

	quoteTx, err := conn.TriggerConstantContract(
		"",
		SunSwapV2Router,
		"getAmountsOut(uint256,address[])",
		quoteParams,
	)
	if err != nil {
		log.Fatalf("quote failed: %v", err)
	}

	expectedOut := decodeLastAmount(quoteTx)
	if expectedOut == nil {
		log.Fatal("could not decode quote output")
	}

	minOut := applySlippage(expectedOut, slippage)
	fmt.Printf("Expected Out: %s\n", expectedOut.String())
	fmt.Printf("Min Out:      %s (after %.1f%% slippage)\n", minOut.String(), slippage)

	deadline := time.Now().Add(20 * time.Minute).Unix()

	var method string
	var params string
	if tokenIn == WTRX {
		// TRX→Token: swapExactETHForTokens (amountIn sent as msg.value, not a param)
		method = "swapExactETHForTokens(uint256,address[],address,uint256)"
		params = fmt.Sprintf(`[{"uint256":"%s"},{"address[]":["%s","%s"]},{"address":"%s"},{"uint256":"%d"}]`,
			minOut.String(), tokenIn, tokenOut, from, deadline)
		fmt.Println("Note: WTRX path uses swapExactETHForTokens; simulation will revert (msg.value=0 in constant call).")
	} else {
		// Token→Token: swapExactTokensForTokens
		method = "swapExactTokensForTokens(uint256,uint256,address[],address,uint256)"
		params = fmt.Sprintf(`[{"uint256":"%s"},{"uint256":"%s"},{"address[]":["%s","%s"]},{"address":"%s"},{"uint256":"%d"}]`,
			amountIn.String(), minOut.String(), tokenIn, tokenOut, from, deadline)
	}

	tx, err := conn.TriggerConstantContract(from, SunSwapV2Router, method, params)
	if err != nil {
		log.Fatalf("simulate swap failed: %v", err)
	}

	utils.PrintTxResult(tx)
}

// executeSwap builds a swap transaction, signs it, and optionally broadcasts.
func executeSwap(conn *client.GrpcClient, tokenIn, tokenOut string, amountIn *big.Int, slippage float64, dryRun bool) {
	signer := utils.LoadSigner()

	fmt.Println("=== Execute Swap ===")
	fmt.Printf("From:      %s\n", signer.Address)
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Printf("Slippage:  %.1f%%\n", slippage)
	fmt.Println()

	quoteParams := fmt.Sprintf(`[{"uint256":"%s"},{"address[]":["%s","%s"]}]`,
		amountIn.String(), tokenIn, tokenOut)

	quoteTx, err := conn.TriggerConstantContract(
		"",
		SunSwapV2Router,
		"getAmountsOut(uint256,address[])",
		quoteParams,
	)
	if err != nil {
		log.Fatalf("quote failed: %v", err)
	}

	expectedOut := decodeLastAmount(quoteTx)
	if expectedOut == nil {
		log.Fatal("could not decode quote output")
	}

	minOut := applySlippage(expectedOut, slippage)
	fmt.Printf("Expected Out: %s\n", expectedOut.String())
	fmt.Printf("Min Out:      %s (after %.1f%% slippage)\n", minOut.String(), slippage)

	deadline := time.Now().Add(20 * time.Minute).Unix()

	var callValue int64
	var method, params string
	if tokenIn == WTRX {
		// TRX→Token: amountIn is sent as native TRX (callValue), not an ABI param
		if !amountIn.IsInt64() {
			log.Fatal("amount is too large to use as TRX callValue (exceeds int64)")
		}
		callValue = amountIn.Int64()
		method = "swapExactETHForTokens(uint256,address[],address,uint256)"
		params = fmt.Sprintf(`[{"uint256":"%s"},{"address[]":["%s","%s"]},{"address":"%s"},{"uint256":"%d"}]`,
			minOut.String(), tokenIn, tokenOut, signer.Address, deadline)
	} else {
		// Token→Token: no callValue needed
		method = "swapExactTokensForTokens(uint256,uint256,address[],address,uint256)"
		params = fmt.Sprintf(`[{"uint256":"%s"},{"uint256":"%s"},{"address[]":["%s","%s"]},{"address":"%s"},{"uint256":"%d"}]`,
			amountIn.String(), minOut.String(), tokenIn, tokenOut, signer.Address, deadline)
	}

	tx, err := conn.TriggerContract(
		signer.Address,
		SunSwapV2Router,
		method,
		params,
		150_000_000, // feeLimit: 150 TRX
		callValue,
		"",
		0,
	)
	if err != nil {
		log.Fatalf("execute swap failed: %v", err)
	}

	signer.SignAndBroadcast(conn, tx, dryRun)
}

// getPair looks up the pair contract address for two tokens.
func getPair(conn *client.GrpcClient, tokenA, tokenB string) {
	fmt.Println("=== Get Pair ===")
	fmt.Printf("Token A: %s\n", tokenA)
	fmt.Printf("Token B: %s\n", tokenB)
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"address":"%s"}]`, tokenA, tokenB)

	tx, err := conn.TriggerConstantContract(
		"",
		SunSwapV2Router,
		"getPairOffChain(address,address)",
		params,
	)
	if err != nil {
		log.Fatalf("getPairOffChain failed: %v", err)
	}

	if len(tx.GetConstantResult()) > 0 && len(tx.GetConstantResult()[0]) >= 32 {
		data := tx.GetConstantResult()[0]
		// ABI address: 32-byte word, last 20 bytes. TRON adds 0x41 prefix.
		hexAddr := "41" + hex.EncodeToString(data[12:32])
		addr := address.HexToAddress(hexAddr)
		fmt.Printf("Pair: %s\n", addr.String())
	}
}

func decodeAmountsOutput(tx *api.TransactionExtention) {
	if len(tx.GetConstantResult()) == 0 {
		fmt.Println("No output")
		return
	}

	data := tx.GetConstantResult()[0]
	fmt.Printf("Raw output: %s\n", hex.EncodeToString(data))

	// ABI-encoded dynamic array: offset(32) + length(32) + elements(32 each)
	if len(data) < 64 {
		return
	}

	arrayLen := new(big.Int).SetBytes(data[32:64]).Int64()
	fmt.Printf("Path length: %d\n", arrayLen)

	for i := int64(0); i < arrayLen && 64+((i+1)*32) <= int64(len(data)); i++ {
		start := 64 + (i * 32)
		val := new(big.Int).SetBytes(data[start : start+32])
		fmt.Printf("  Amount[%d]: %s\n", i, val.String())
	}
}

func decodeLastAmount(tx *api.TransactionExtention) *big.Int {
	if len(tx.GetConstantResult()) == 0 {
		return nil
	}

	data := tx.GetConstantResult()[0]
	if len(data) < 64 {
		return nil
	}

	arrayLen := new(big.Int).SetBytes(data[32:64]).Int64()
	if arrayLen == 0 {
		return nil
	}

	lastIdx := arrayLen - 1
	start := 64 + (lastIdx * 32)
	if start+32 > int64(len(data)) {
		return nil
	}

	return new(big.Int).SetBytes(data[start : start+32])
}

func applySlippage(amount *big.Int, slippagePct float64) *big.Int {
	// minOut = amount * (10000 - slippageBps) / 10000
	bps := int64(slippagePct * 100)
	factor := big.NewInt(10000 - bps)
	result := new(big.Int).Mul(amount, factor)
	result.Div(result, big.NewInt(10000))
	return result
}
