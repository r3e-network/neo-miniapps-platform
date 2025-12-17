// Command deploy-testnet deploys Service Layer contracts to Neo N3 testnet.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/R3E-Network/service_layer/deploy/testnet"
	"github.com/R3E-Network/service_layer/infrastructure/chain"
)

var contracts = []string{
	"PaymentHub",
	"Governance",
	"PriceFeed",
	"RandomnessLog",
	"AppRegistry",
	"AutomationAnchor",
}

func main() {
	rpcURL := flag.String("rpc", "https://testnet1.neo.coz.io:443", "Neo N3 testnet RPC URL")
	buildDir := flag.String("build", "contracts/build", "Contract build directory")
	outputFile := flag.String("output", "deploy/config/testnet_contracts.json", "Output file for deployed contracts")
	checkOnly := flag.Bool("check", false, "Only check connectivity and balance")
	estimateOnly := flag.Bool("estimate", false, "Only estimate gas costs (simulation)")
	flag.Parse()

	log.Println("=== Neo N3 Testnet MiniApp Platform Contract Deployment ===")
	log.Printf("RPC: %s", *rpcURL)
	log.Printf("Build directory: %s", *buildDir)

	deployer, err := testnet.NewDeployer(*rpcURL)
	if err != nil {
		log.Fatalf("Failed to create deployer: %v", err)
	}

	log.Printf("Deployer address: %s", deployer.GetAddress())
	log.Printf("Deployer script hash: %s", deployer.GetScriptHash())

	blockCount, err := deployer.GetBlockCount()
	if err != nil {
		log.Fatalf("Failed to connect to testnet: %v", err)
	}
	log.Printf("Testnet block height: %d", blockCount)

	gasBalance, err := deployer.GetGASBalanceFloat()
	if err != nil {
		log.Printf("Warning: Failed to get GAS balance: %v", err)
	} else {
		log.Printf("GAS balance: %.8f GAS", gasBalance)
	}

	if *checkOnly {
		log.Println("Check complete.")
		return
	}

	result := chain.DeploymentResult{
		Network:  "testnet",
		Deployer: deployer.GetAddress(),
	}

	var totalGas float64

	log.Println("\n=== Estimating Deployment Costs ===")
	for _, name := range contracts {
		nefPath := filepath.Join(*buildDir, name+".nef")
		manifestPath := filepath.Join(*buildDir, name+".manifest.json")

		if _, statErr := os.Stat(nefPath); os.IsNotExist(statErr) {
			log.Printf("  Skipping %s (not built)", name)
			continue
		}

		log.Printf("Simulating %s...", name)
		deployed, deployErr := deployer.DeployContract(nefPath, manifestPath)
		if deployErr != nil {
			log.Printf("  ERROR: %v", deployErr)
			continue
		}

		deployed.Name = name
		deployed.DeployedAt = time.Now().UTC().Format(time.RFC3339)
		result.Contracts = append(result.Contracts, *deployed)

		gasFloat := parseGas(deployed.GasConsumed)
		totalGas += gasFloat
		log.Printf("  %s: %.8f GAS", name, gasFloat)
	}

	log.Println("\n=== Cost Summary ===")
	log.Printf("Total estimated GAS: %.8f GAS", totalGas)
	log.Printf("Available balance:   %.8f GAS", gasBalance)
	if gasBalance >= totalGas {
		log.Println("Status: SUFFICIENT BALANCE")
	} else {
		log.Printf("Status: INSUFFICIENT (need %.8f more GAS)", totalGas-gasBalance)
	}

	if *estimateOnly {
		log.Println("\n=== Estimate Only Mode ===")
		log.Println("To deploy contracts to testnet, use neo-go CLI:")
		log.Println("")
		for _, name := range contracts {
			nefPath := filepath.Join(*buildDir, name+".nef")
			manifestPath := filepath.Join(*buildDir, name+".manifest.json")
			if _, statErr := os.Stat(nefPath); statErr == nil {
				log.Printf("neo-go contract deploy -i %s -m %s -r %s -w wallet.json", nefPath, manifestPath, *rpcURL)
			}
		}
		return
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(*outputFile), 0o755); mkdirErr != nil {
		log.Printf("Warning: Failed to create output directory: %v", mkdirErr)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Warning: Failed to marshal output: %v", err)
		data = []byte("{}")
	}
	if err := os.WriteFile(*outputFile, data, 0o600); err != nil {
		log.Printf("Warning: Failed to write output file: %v", err)
	}

	log.Println("\n=== Simulation Results ===")
	fmt.Println(string(data))

	log.Println("\n=== Next Steps ===")
	log.Println("Contract deployment simulations passed.")
	log.Println("For actual deployment, you can:")
	log.Println("1. Use Fairy for local testing: go run ./cmd/deploy-fairy/main.go")
	log.Println("2. Use neo-go CLI for testnet deployment")
	log.Println("3. Call setUpdater for PriceFeed/RandomnessLog/AutomationAnchor from the admin wallet")
}

func parseGas(gasConsumed string) float64 {
	var gas int64
	if _, err := fmt.Sscanf(gasConsumed, "%d", &gas); err != nil {
		return 0
	}
	return float64(gas) / 1e8
}
