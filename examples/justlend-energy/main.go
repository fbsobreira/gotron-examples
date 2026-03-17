package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"

	"github.com/fbsobreira/gotron-sdk/pkg/client"

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

	// Validate required flags per action
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

	switch action {
	case "simulate-rent":
		simulateRent(conn, renter, receiver, trxToSun(amountTRX), resourceType)
	case "simulate-return":
		simulateReturn(conn, renter, receiver, trxToSun(amountTRX), resourceType)
	case "rent":
		executeRent(conn, receiver, trxToSun(amountTRX), resourceType, dryRun)
	case "return":
		executeReturn(conn, receiver, trxToSun(amountTRX), resourceType, dryRun)
	case "info":
		queryRentalInfo(conn, renter, receiver, resourceType)
	case "rates":
		queryRates(conn, trxToSun(amountTRX), resourceType)
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

// simulateRent calls rentResource via TriggerConstantContract (read-only simulation)
func simulateRent(conn *client.GrpcClient, from, receiver string, amountSun int64, resourceType int) {
	fmt.Println("=== Simulate Rent Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"uint256":"%d"},{"uint256":"%d"}]`,
		receiver, amountSun, resourceType)

	// Note: TriggerConstantContract sends msg.value=0, so JustLend's security deposit
	// check will always revert here. This only validates ABI encoding and gas estimation.
	tx, err := conn.TriggerConstantContract(
		from,
		EnergyRentalContract,
		"rentResource(address,uint256,uint256)",
		params,
	)
	if err != nil {
		log.Fatalf("simulate rent failed: %v", err)
	}

	utils.PrintTxResult(tx)
	fmt.Println("Note: revert is expected — TriggerConstantContract sends 0 TRX; use -action rent -dryrun to validate signing.")
}

// simulateReturn calls returnResource via TriggerConstantContract (read-only simulation)
func simulateReturn(conn *client.GrpcClient, from, receiver string, amountSun int64, resourceType int) {
	fmt.Println("=== Simulate Return Resource ===")
	fmt.Printf("From:     %s\n", from)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"uint256":"%d"},{"uint256":"%d"}]`,
		receiver, amountSun, resourceType)

	tx, err := conn.TriggerConstantContract(
		from,
		EnergyRentalContract,
		"returnResource(address,uint256,uint256)",
		params,
	)
	if err != nil {
		log.Fatalf("simulate return failed: %v", err)
	}

	utils.PrintTxResult(tx)
}

// executeRent calls rentResource via TriggerContract, signs, and optionally broadcasts.
func executeRent(conn *client.GrpcClient, receiver string, amountSun int64, resourceType int, dryRun bool) {
	signer := utils.LoadSigner()

	fmt.Println("=== Execute Rent Resource ===")
	fmt.Printf("From:     %s\n", signer.Address)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"uint256":"%d"},{"uint256":"%d"}]`,
		receiver, amountSun, resourceType)

	tx, err := conn.TriggerContract(
		signer.Address,
		EnergyRentalContract,
		"rentResource(address,uint256,uint256)",
		params,
		100_000_000, // feeLimit: 100 TRX
		amountSun,   // callValue: security deposit in sun
		"",
		0,
	)
	if err != nil {
		log.Fatalf("execute rent failed: %v", err)
	}

	signer.SignAndBroadcast(conn, tx, dryRun)
}

// executeReturn calls returnResource via TriggerContract, signs, and optionally broadcasts.
func executeReturn(conn *client.GrpcClient, receiver string, amountSun int64, resourceType int, dryRun bool) {
	signer := utils.LoadSigner()

	fmt.Println("=== Execute Return Resource ===")
	fmt.Printf("From:     %s\n", signer.Address)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Amount:   %s (%d sun)\n", utils.FormatTRX(amountSun), amountSun)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"uint256":"%d"},{"uint256":"%d"}]`,
		receiver, amountSun, resourceType)

	tx, err := conn.TriggerContract(
		signer.Address,
		EnergyRentalContract,
		"returnResource(address,uint256,uint256)",
		params,
		100_000_000, // feeLimit: 100 TRX
		0,
		"",
		0,
	)
	if err != nil {
		log.Fatalf("execute return failed: %v", err)
	}

	signer.SignAndBroadcast(conn, tx, dryRun)
}

// queryRentalInfo fetches rental details for a renter/receiver pair.
func queryRentalInfo(conn *client.GrpcClient, renter, receiver string, resourceType int) {
	fmt.Println("=== Rental Info ===")
	fmt.Printf("Renter:   %s\n", renter)
	fmt.Printf("Receiver: %s\n", receiver)
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	params := fmt.Sprintf(`[{"address":"%s"},{"address":"%s"},{"uint256":"%d"}]`,
		renter, receiver, resourceType)

	tx, err := conn.TriggerConstantContract(
		"",
		EnergyRentalContract,
		"rentals(address,address,uint256)",
		params,
	)
	if err != nil {
		log.Fatalf("query rentals failed: %v", err)
	}

	if len(tx.GetConstantResult()) > 0 {
		result := tx.GetConstantResult()[0]
		if len(result) >= 96 {
			// ABI tuple: (uint256 amount, uint256 deposit, uint256 rentIndex)
			amount := new(big.Int).SetBytes(result[0:32])
			deposit := new(big.Int).SetBytes(result[32:64])
			rentIdx := new(big.Int).SetBytes(result[64:96])
			fmt.Printf("Amount:           %s\n", utils.FormatTRX(amount.Int64()))
			fmt.Printf("Security Deposit: %s\n", utils.FormatTRX(deposit.Int64()))
			fmt.Printf("Rent Index:       %s\n", rentIdx.String())
		} else {
			fmt.Println("No active rental found")
		}
	}
}

// queryRates fetches the current rental rate for a given amount and resource type.
func queryRates(conn *client.GrpcClient, amountSun int64, resourceType int) {
	fmt.Println("=== Rental Rates ===")
	fmt.Printf("Amount:   %s\n", utils.FormatTRX(amountSun))
	fmt.Printf("Resource: %s\n", resourceName(resourceType))
	fmt.Println()

	// Query _rentalRate
	params := fmt.Sprintf(`[{"uint256":"%d"},{"uint256":"%d"}]`, amountSun, resourceType)
	tx, err := conn.TriggerConstantContract(
		"",
		EnergyRentalContract,
		"_rentalRate(uint256,uint256)",
		params,
	)
	if err != nil {
		log.Fatalf("query rental rate failed: %v", err)
	}

	if len(tx.GetConstantResult()) > 0 {
		rate := new(big.Int).SetBytes(tx.GetConstantResult()[0])
		fmt.Printf("Rental Rate:    %s (per second, scaled 1e18)\n", rate.String())
	}

	// Query _liquidateRate
	liqParams := fmt.Sprintf(`[{"uint256":"%d"}]`, resourceType)
	tx2, err := conn.TriggerConstantContract(
		"",
		EnergyRentalContract,
		"_liquidateRate(uint256)",
		liqParams,
	)
	if err != nil {
		log.Fatalf("query liquidate rate failed: %v", err)
	}

	if len(tx2.GetConstantResult()) > 0 {
		rate := new(big.Int).SetBytes(tx2.GetConstantResult()[0])
		fmt.Printf("Liquidate Rate: %s (per second, scaled 1e18)\n", rate.String())
	}

	// Query totalRent
	tx3, err := conn.TriggerConstantContract(
		"",
		EnergyRentalContract,
		"totalRent()",
		"",
	)
	if err != nil {
		log.Fatalf("query total rent failed: %v", err)
	}

	if len(tx3.GetConstantResult()) > 0 {
		total := new(big.Int).SetBytes(tx3.GetConstantResult()[0])
		fmt.Printf("Total Rent:     %s\n", utils.FormatTRX(total.Int64()))
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
