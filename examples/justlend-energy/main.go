package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"

	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/contract"

	"github.com/fbsobreira/gotron-examples/utils"
)

const (
	// JustLend EnergyRental proxy contract on mainnet
	EnergyRentalContract = "TU2MJ5Veik1LRAgjeSzEdvmDYx7mefJZvd"

	// Resource types
	ResourceBandwidth = 0
	ResourceEnergy    = 1
)

func main() {
	var (
		action       string
		receiver     string
		renter       string
		amountTRX    float64
		resourceType int
		node         string
		dryRun       bool
	)

	flag.StringVar(&action, "action", "simulate-rent", "Action: simulate-rent, simulate-return, rent, return, info, rates")
	flag.StringVar(&receiver, "receiver", "", "Receiver address (base58)")
	flag.StringVar(&renter, "renter", "", "Renter/caller address (base58)")
	flag.Float64Var(&amountTRX, "amount", 0, "Amount in TRX")
	flag.IntVar(&resourceType, "resource", ResourceEnergy, "Resource type: 0=bandwidth, 1=energy")
	flag.StringVar(&node, "node", "", "gRPC node (default: grpc.trongrid.io:50051)")
	flag.BoolVar(&dryRun, "dryrun", false, "Sign transaction but do not broadcast")
	flag.Parse()

	switch action {
	case "simulate-rent", "simulate-return", "rent", "return":
		if receiver == "" {
			log.Fatalf("action %s requires -receiver", action)
		}
		if amountTRX <= 0 {
			log.Fatalf("action %s requires -amount > 0", action)
		}
	case "rates":
		if amountTRX <= 0 {
			log.Fatal("action rates requires -amount > 0")
		}
	case "info":
		if receiver == "" {
			log.Fatal("action info requires -receiver")
		}
	}

	if renter == "" {
		renter = receiver
	}

	conn := utils.NewGRPCClient(node)
	ctx := context.Background()

	switch action {
	case "simulate-rent":
		simulateRent(ctx, conn, renter, receiver, trxToSun(amountTRX), resourceType)
	case "simulate-return":
		simulateReturn(ctx, conn, renter, receiver, trxToSun(amountTRX), resourceType)
	case "rent":
		executeRent(ctx, conn, receiver, trxToSun(amountTRX), resourceType, dryRun)
	case "return":
		executeReturn(ctx, conn, receiver, trxToSun(amountTRX), resourceType, dryRun)
	case "info":
		queryRentalInfo(ctx, conn, renter, receiver, resourceType)
	case "rates":
		queryRates(ctx, conn, trxToSun(amountTRX), resourceType)
	default:
		log.Fatalf("unknown action: %s", action)
	}
}

func trxToSun(trx float64) int64 {
	sun := math.Round(trx * float64(utils.SunPerTRX))
	if sun <= 0 || sun > math.MaxInt64 {
		log.Fatalf("amount %.6f TRX is out of range", trx)
	}
	return int64(sun)
}

// rentParams builds the JSON parameter string for rent/return operations.
func rentParams(receiver string, amountSun int64, resourceType int) string {
	return fmt.Sprintf(`["%s", "%d", "%d"]`, receiver, amountSun, resourceType)
}

// simulateRent calls rentResource via a read-only constant call (no fees).
func simulateRent(ctx context.Context, conn *client.GrpcClient, from, receiver string, amountSun int64, resourceType int) {
	fmt.Println("=== Simulate Rent Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	// Note: constant call sends msg.value=0, so JustLend's security deposit
	// check will always revert. This only validates ABI encoding and gas estimation.
	result, err := contract.New(conn, EnergyRentalContract).
		From(from).
		Method("rentResource(address,uint256,uint256)").
		Params(rentParams(receiver, amountSun, resourceType)).
		Call(ctx)
	if err != nil {
		log.Fatalf("simulate rent failed: %v", err)
	}

	fmt.Printf("Energy Used: %d\n", result.EnergyUsed)
	if len(result.RawResults) > 0 {
		fmt.Printf("Output: %x\n", result.RawResults[0])
	}
	fmt.Println("Note: revert is expected — constant call sends 0 TRX; use -action rent -dryrun to validate signing.")
}

// simulateReturn calls returnResource via a read-only constant call (no fees).
func simulateReturn(ctx context.Context, conn *client.GrpcClient, from, receiver string, amountSun int64, resourceType int) {
	fmt.Println("=== Simulate Return Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	result, err := contract.New(conn, EnergyRentalContract).
		From(from).
		Method("returnResource(address,uint256,uint256)").
		Params(rentParams(receiver, amountSun, resourceType)).
		Call(ctx)
	if err != nil {
		log.Fatalf("simulate return failed: %v", err)
	}

	fmt.Printf("Energy Used: %d\n", result.EnergyUsed)
	if len(result.RawResults) > 0 {
		fmt.Printf("Output: %x\n", result.RawResults[0])
	}
}

// executeRent builds, signs, and optionally broadcasts a rentResource transaction.
func executeRent(ctx context.Context, conn *client.GrpcClient, receiver string, amountSun int64, resourceType int, dryRun bool) {
	signer := utils.LoadSigner()
	from := signer.Address().String()

	fmt.Println("=== Execute Rent Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	call := contract.New(conn, EnergyRentalContract).
		From(from).
		Method("rentResource(address,uint256,uint256)").
		Params(rentParams(receiver, amountSun, resourceType)).
		WithFeeLimit(100_000_000).
		WithCallValue(amountSun) // security deposit in sun

	if dryRun {
		txExt, err := call.Build(ctx)
		if err != nil {
			log.Fatalf("build rent tx failed: %v", err)
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
		log.Fatalf("execute rent failed: %v", err)
	}

	fmt.Printf("TxID:   %s\n", receipt.TxID)
	fmt.Printf("Broadcast: SUCCESS (check explorer for execution result)\n")
}

// executeReturn builds, signs, and optionally broadcasts a returnResource transaction.
func executeReturn(ctx context.Context, conn *client.GrpcClient, receiver string, amountSun int64, resourceType int, dryRun bool) {
	signer := utils.LoadSigner()
	from := signer.Address().String()

	fmt.Println("=== Execute Return Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	call := contract.New(conn, EnergyRentalContract).
		From(from).
		Method("returnResource(address,uint256,uint256)").
		Params(rentParams(receiver, amountSun, resourceType)).
		WithFeeLimit(100_000_000)

	if dryRun {
		txExt, err := call.Build(ctx)
		if err != nil {
			log.Fatalf("build return tx failed: %v", err)
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
		log.Fatalf("execute return failed: %v", err)
	}

	fmt.Printf("TxID:   %s\n", receipt.TxID)
	fmt.Printf("Broadcast: SUCCESS (check explorer for execution result)\n")
}

// queryRentalInfo fetches rental details for a renter/receiver pair.
func queryRentalInfo(ctx context.Context, conn *client.GrpcClient, renter, receiver string, resourceType int) {
	fmt.Println("=== Rental Info ===")
	fmt.Printf("Renter:   %s\n", renter)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`["%s", "%s", "%d"]`, renter, receiver, resourceType)

	result, err := contract.New(conn, EnergyRentalContract).
		Method("rentals(address,address,uint256)").
		Params(params).
		Call(ctx)
	if err != nil {
		log.Fatalf("query rentals failed: %v", err)
	}

	if len(result.RawResults) > 0 && len(result.RawResults[0]) >= 96 {
		data := result.RawResults[0]
		// ABI tuple: (uint256 amount, uint256 deposit, uint256 rentIndex)
		amount := new(big.Int).SetBytes(data[0:32])
		deposit := new(big.Int).SetBytes(data[32:64])
		rentIdx := new(big.Int).SetBytes(data[64:96])
		if amount.IsInt64() {
			fmt.Printf("Amount:           %s\n", utils.FormatTRX(amount.Int64()))
		} else {
			fmt.Printf("Amount:           %s sun\n", amount.String())
		}
		if deposit.IsInt64() {
			fmt.Printf("Security Deposit: %s\n", utils.FormatTRX(deposit.Int64()))
		} else {
			fmt.Printf("Security Deposit: %s sun\n", deposit.String())
		}
		fmt.Printf("Rent Index:       %s\n", rentIdx.String())
	} else {
		fmt.Println("No active rental found")
	}
}

// queryRates fetches the current rental rate for a given amount and resource type.
func queryRates(ctx context.Context, conn *client.GrpcClient, amountSun int64, resourceType int) {
	fmt.Println("=== Rental Rates ===")
	fmt.Printf("Amount:   %s\n", utils.FormatTRX(amountSun))
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	// Query _rentalRate
	rateResult, err := contract.New(conn, EnergyRentalContract).
		Method("_rentalRate(uint256,uint256)").
		Params(fmt.Sprintf(`["%d", "%d"]`, amountSun, resourceType)).
		Call(ctx)
	if err != nil {
		log.Fatalf("query rental rate failed: %v", err)
	}

	if len(rateResult.RawResults) > 0 {
		rate := new(big.Int).SetBytes(rateResult.RawResults[0])
		fmt.Printf("Rental Rate:    %s (per second, scaled 1e18)\n", rate.String())
	}

	// Query _liquidateRate
	liqResult, err := contract.New(conn, EnergyRentalContract).
		Method("_liquidateRate(uint256)").
		Params(fmt.Sprintf(`["%d"]`, resourceType)).
		Call(ctx)
	if err != nil {
		log.Fatalf("query liquidate rate failed: %v", err)
	}

	if len(liqResult.RawResults) > 0 {
		rate := new(big.Int).SetBytes(liqResult.RawResults[0])
		fmt.Printf("Liquidate Rate: %s (per second, scaled 1e18)\n", rate.String())
	}

	// Query totalRent
	totalResult, err := contract.New(conn, EnergyRentalContract).
		Method("totalRent()").
		Call(ctx)
	if err != nil {
		log.Fatalf("query total rent failed: %v", err)
	}

	if len(totalResult.RawResults) > 0 {
		total := new(big.Int).SetBytes(totalResult.RawResults[0])
		if total.IsInt64() {
			fmt.Printf("Total Rent:     %s\n", utils.FormatTRX(total.Int64()))
		} else {
			fmt.Printf("Total Rent:     %s sun\n", total.String())
		}
	}
}

func resourceName(rt int) string {
	switch rt {
	case ResourceBandwidth:
		return "Bandwidth (0)"
	case ResourceEnergy:
		return "Energy (1)"
	default:
		return fmt.Sprintf("Unknown (%d)", rt)
	}
}
