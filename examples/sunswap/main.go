package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/contract"

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
	flag.Float64Var(&slippage, "slippage", 0.5, "Slippage tolerance in percent (0-100)")
	flag.StringVar(&node, "node", "", "gRPC node (default: grpc.trongrid.io:50051)")
	flag.BoolVar(&dryRun, "dryrun", false, "Sign transaction but do not broadcast")
	flag.Parse()

	if slippage < 0 || slippage > 100 {
		log.Fatalf("slippage must be between 0 and 100, got %.2f", slippage)
	}

	conn := utils.NewGRPCClient(node)
	ctx := context.Background()

	amount, ok := new(big.Int).SetString(amountIn, 10)
	if !ok {
		log.Fatalf("invalid amount: %s", amountIn)
	}

	switch action {
	case "quote":
		getQuote(ctx, conn, tokenIn, tokenOut, amount)
	case "simulate-swap":
		if from == "" {
			log.Fatal("required: -from for simulate-swap")
		}
		simulateSwap(ctx, conn, from, tokenIn, tokenOut, amount, slippage)
	case "swap":
		executeSwap(ctx, conn, tokenIn, tokenOut, amount, slippage, dryRun)
	case "pair":
		getPair(ctx, conn, tokenIn, tokenOut)
	default:
		log.Fatalf("unknown action: %s", action)
	}
}

// getQuote calls getAmountsOut on the router to get a price quote.
func getQuote(ctx context.Context, conn *client.GrpcClient, tokenIn, tokenOut string, amountIn *big.Int) {
	fmt.Println("=== SunSwap V2 Quote ===")
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Println()

	result, err := contract.New(conn, SunSwapV2Router).
		Method("getAmountsOut(uint256,address[])").
		Params(fmt.Sprintf(`["%s", ["%s", "%s"]]`, amountIn.String(), tokenIn, tokenOut)).
		Call(ctx)
	if err != nil {
		log.Fatalf("getAmountsOut failed: %v", err)
	}

	decodeAmountsOutput(result.RawResults)
}

// simulateSwap simulates the swap via a read-only constant call.
// For WTRX input (TRX->Token), uses swapExactETHForTokens; otherwise swapExactTokensForTokens.
// Note: the WTRX path will revert because constant calls cannot send TRX (msg.value=0).
func simulateSwap(ctx context.Context, conn *client.GrpcClient, from, tokenIn, tokenOut string, amountIn *big.Int, slippage float64) {
	fmt.Println("=== Simulate Swap ===")
	fmt.Printf("From:      %s\n", from)
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Printf("Slippage:  %.1f%%\n", slippage)
	fmt.Println()

	// Get quote first
	quoteResult, err := contract.New(conn, SunSwapV2Router).
		Method("getAmountsOut(uint256,address[])").
		Params(fmt.Sprintf(`["%s", ["%s", "%s"]]`, amountIn.String(), tokenIn, tokenOut)).
		Call(ctx)
	if err != nil {
		log.Fatalf("quote failed: %v", err)
	}

	expectedOut := decodeLastAmount(quoteResult.RawResults)
	if expectedOut == nil {
		log.Fatal("could not decode quote output")
	}

	minOut := applySlippage(expectedOut, slippage)
	fmt.Printf("Expected Out: %s\n", expectedOut.String())
	fmt.Printf("Min Out:      %s (after %.1f%% slippage)\n", minOut.String(), slippage)

	deadline := time.Now().Add(20 * time.Minute).Unix()

	var call *contract.ContractCall
	if tokenIn == WTRX {
		// TRX->Token: swapExactETHForTokens (amountIn sent as msg.value, not a param)
		call = contract.New(conn, SunSwapV2Router).
			From(from).
			Method("swapExactETHForTokens(uint256,address[],address,uint256)").
			Params(fmt.Sprintf(`["%s", ["%s", "%s"], "%s", "%d"]`,
				minOut.String(), tokenIn, tokenOut, from, deadline))
		fmt.Println("Note: WTRX path uses swapExactETHForTokens; simulation will revert (msg.value=0 in constant call).")
	} else {
		// Token->Token: swapExactTokensForTokens
		call = contract.New(conn, SunSwapV2Router).
			From(from).
			Method("swapExactTokensForTokens(uint256,uint256,address[],address,uint256)").
			Params(fmt.Sprintf(`["%s", "%s", ["%s", "%s"], "%s", "%d"]`,
				amountIn.String(), minOut.String(), tokenIn, tokenOut, from, deadline))
	}

	result, err := call.Call(ctx)
	if err != nil {
		log.Fatalf("simulate swap failed: %v", err)
	}

	fmt.Printf("Energy Used: %d\n", result.EnergyUsed)
	if len(result.RawResults) > 0 {
		fmt.Printf("Output: %x\n", result.RawResults[0])
	}
}

// executeSwap builds a swap transaction, signs it, and optionally broadcasts.
func executeSwap(ctx context.Context, conn *client.GrpcClient, tokenIn, tokenOut string, amountIn *big.Int, slippage float64, dryRun bool) {
	signer := utils.LoadSigner()
	from := signer.Address().String()

	fmt.Println("=== Execute Swap ===")
	fmt.Printf("From:      %s\n", from)
	fmt.Printf("Token In:  %s\n", tokenIn)
	fmt.Printf("Token Out: %s\n", tokenOut)
	fmt.Printf("Amount In: %s\n", amountIn.String())
	fmt.Printf("Slippage:  %.1f%%\n", slippage)
	fmt.Println()

	// Get quote first
	quoteResult, err := contract.New(conn, SunSwapV2Router).
		Method("getAmountsOut(uint256,address[])").
		Params(fmt.Sprintf(`["%s", ["%s", "%s"]]`, amountIn.String(), tokenIn, tokenOut)).
		Call(ctx)
	if err != nil {
		log.Fatalf("quote failed: %v", err)
	}

	expectedOut := decodeLastAmount(quoteResult.RawResults)
	if expectedOut == nil {
		log.Fatal("could not decode quote output")
	}

	minOut := applySlippage(expectedOut, slippage)
	fmt.Printf("Expected Out: %s\n", expectedOut.String())
	fmt.Printf("Min Out:      %s (after %.1f%% slippage)\n", minOut.String(), slippage)

	deadline := time.Now().Add(20 * time.Minute).Unix()

	var call *contract.ContractCall
	if tokenIn == WTRX {
		// TRX->Token: amountIn is sent as native TRX (callValue), not an ABI param
		if !amountIn.IsInt64() {
			log.Fatal("amount is too large to use as TRX callValue (exceeds int64)")
		}
		call = contract.New(conn, SunSwapV2Router).
			From(from).
			Method("swapExactETHForTokens(uint256,address[],address,uint256)").
			Params(fmt.Sprintf(`["%s", ["%s", "%s"], "%s", "%d"]`,
				minOut.String(), tokenIn, tokenOut, from, deadline)).
			WithFeeLimit(150_000_000).
			WithCallValue(amountIn.Int64())
	} else {
		// Token->Token: no callValue needed
		call = contract.New(conn, SunSwapV2Router).
			From(from).
			Method("swapExactTokensForTokens(uint256,uint256,address[],address,uint256)").
			Params(fmt.Sprintf(`["%s", "%s", ["%s", "%s"], "%s", "%d"]`,
				amountIn.String(), minOut.String(), tokenIn, tokenOut, from, deadline)).
			WithFeeLimit(150_000_000)
	}

	if dryRun {
		txExt, err := call.Build(ctx)
		if err != nil {
			log.Fatalf("build swap tx failed: %v", err)
		}
		signed, err := signer.Sign(txExt.GetTransaction())
		if err != nil {
			log.Fatalf("signing failed: %v", err)
		}
		fmt.Printf("TxID:   %x\n", txExt.GetTxid())
		fmt.Printf("Signed: yes (%d signature(s))\n", len(signed.GetSignature()))
		fmt.Println("Mode:   DRY RUN (not broadcast)")
		return
	}

	receipt, err := call.Send(ctx, signer)
	if err != nil {
		log.Fatalf("execute swap failed: %v", err)
	}

	fmt.Printf("TxID:   %s\n", receipt.TxID)
	fmt.Printf("Broadcast: SUCCESS (check explorer for execution result)\n")
}

// getPair looks up the pair contract address for two tokens.
func getPair(ctx context.Context, conn *client.GrpcClient, tokenA, tokenB string) {
	fmt.Println("=== Get Pair ===")
	fmt.Printf("Token A: %s\n", tokenA)
	fmt.Printf("Token B: %s\n", tokenB)
	fmt.Println()

	result, err := contract.New(conn, SunSwapV2Router).
		Method("getPairOffChain(address,address)").
		Params(fmt.Sprintf(`["%s", "%s"]`, tokenA, tokenB)).
		Call(ctx)
	if err != nil {
		log.Fatalf("getPairOffChain failed: %v", err)
	}

	if len(result.RawResults) > 0 && len(result.RawResults[0]) >= 32 {
		data := result.RawResults[0]
		// ABI address: 32-byte word, last 20 bytes. TRON adds 0x41 prefix.
		hexAddr := "41" + hex.EncodeToString(data[12:32])
		addr, err := address.HexToAddress(hexAddr)
		if err != nil {
			log.Fatalf("invalid pair address: %v", err)
		}
		fmt.Printf("Pair: %s\n", addr.String())
	}
}

func decodeAmountsOutput(rawResults [][]byte) {
	if len(rawResults) == 0 {
		fmt.Println("No output")
		return
	}

	data := rawResults[0]
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

func decodeLastAmount(rawResults [][]byte) *big.Int {
	if len(rawResults) == 0 {
		return nil
	}

	data := rawResults[0]
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
