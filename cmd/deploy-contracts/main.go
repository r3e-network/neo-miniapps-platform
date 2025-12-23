// Command deploy-contracts provides a comprehensive CLI for managing Neo N3 smart contracts.
// It supports checking status, deploying, updating, and managing contract addresses.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/deploy/testnet"
	"github.com/R3E-Network/service_layer/infrastructure/chain"
)

var platformContracts = []string{
	"PaymentHub",
	"Governance",
	"PriceFeed",
	"RandomnessLog",
	"AppRegistry",
	"AutomationAnchor",
}

func main() {
	// Subcommands
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)
	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	exportCmd := flag.NewFlagSet("export", flag.ExitOnError)

	// Common flags
	rpcURL := "https://testnet1.neo.coz.io:443"
	configFile := "deploy/config/testnet_contracts.json"
	buildDir := "contracts/build"

	// Status flags
	statusRPC := statusCmd.String("rpc", rpcURL, "Neo N3 RPC URL")
	statusConfig := statusCmd.String("config", configFile, "Contract config file")

	// Deploy flags
	deployRPC := deployCmd.String("rpc", rpcURL, "Neo N3 RPC URL")
	deployConfig := deployCmd.String("config", configFile, "Contract config file")
	deployBuild := deployCmd.String("build", buildDir, "Contract build directory")
	deployContract := deployCmd.String("contract", "", "Specific contract to deploy (or 'all')")
	deployDryRun := deployCmd.Bool("dry-run", true, "Simulate deployment without executing")

	// Update flags
	updateRPC := updateCmd.String("rpc", rpcURL, "Neo N3 RPC URL")
	updateConfig := updateCmd.String("config", configFile, "Contract config file")
	updateBuild := updateCmd.String("build", buildDir, "Contract build directory")
	updateContract := updateCmd.String("contract", "", "Contract to update")

	// Verify flags
	verifyRPC := verifyCmd.String("rpc", rpcURL, "Neo N3 RPC URL")
	verifyConfig := verifyCmd.String("config", configFile, "Contract config file")

	// Export flags
	exportConfig := exportCmd.String("config", configFile, "Contract config file")
	exportFormat := exportCmd.String("format", "env", "Export format: env, json, or dotenv")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "status":
		statusCmd.Parse(os.Args[2:])
		runStatus(*statusRPC, *statusConfig)
	case "deploy":
		deployCmd.Parse(os.Args[2:])
		runDeploy(*deployRPC, *deployConfig, *deployBuild, *deployContract, *deployDryRun)
	case "update":
		updateCmd.Parse(os.Args[2:])
		runUpdate(*updateRPC, *updateConfig, *updateBuild, *updateContract)
	case "verify":
		verifyCmd.Parse(os.Args[2:])
		runVerify(*verifyRPC, *verifyConfig)
	case "export":
		exportCmd.Parse(os.Args[2:])
		runExport(*exportConfig, *exportFormat)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Neo N3 Contract Deployment Tool

Usage:
  deploy-contracts <command> [options]

Commands:
  status    Check deployment status of all contracts
  deploy    Deploy contracts to the network
  update    Update an existing contract
  verify    Verify contracts are callable
  export    Export contract addresses

Examples:
  # Check contract status
  deploy-contracts status

  # Simulate deployment (dry-run)
  deploy-contracts deploy --contract=PaymentHub --dry-run

  # Deploy all contracts
  deploy-contracts deploy --contract=all --dry-run=false

  # Export addresses as environment variables
  deploy-contracts export --format=env

  # Verify contracts are callable
  deploy-contracts verify`)
}

func runStatus(rpcURL, configFile string) {
	log.Println("=== Contract Deployment Status ===")
	log.Printf("RPC: %s", rpcURL)
	log.Printf("Config: %s", configFile)

	// Load registry
	registry := chain.NewContractRegistry("testnet", filepath.Dir(configFile))
	if err := registry.LoadFromFile(configFile); err != nil {
		log.Printf("Warning: Could not load config file: %v", err)
	}
	registry.LoadFromEnv()

	// Create deployer for RPC calls
	deployer, err := testnet.NewDeployer(rpcURL)
	if err != nil {
		log.Printf("Warning: Could not create deployer: %v", err)
	} else {
		blockCount, _ := deployer.GetBlockCount()
		gasBalance, _ := deployer.GetGASBalanceFloat()
		log.Printf("Block height: %d", blockCount)
		log.Printf("Deployer GAS: %.4f", gasBalance)
	}

	log.Println("\n=== Platform Contracts ===")
	fmt.Printf("%-20s %-50s %-12s\n", "Contract", "Hash", "Status")
	fmt.Println(strings.Repeat("-", 85))

	for _, name := range platformContracts {
		info := registry.Get(name)
		hash := "-"
		status := "NOT DEPLOYED"

		if info != nil && info.Hash != "" {
			hash = info.Hash
			status = "DEPLOYED"

			// Verify on chain if deployer available
			if deployer != nil {
				state, err := deployer.GetContractState(info.Hash)
				if err != nil {
					status = "ERROR"
				} else if state != nil {
					status = "VERIFIED"
				}
			}
		}

		fmt.Printf("%-20s %-50s %-12s\n", name, hash, status)
	}

	// Show missing contracts
	missing := registry.Validate()
	if len(missing) > 0 {
		log.Printf("\n⚠️  Missing contracts: %s", strings.Join(missing, ", "))
	} else {
		log.Println("\n✅ All platform contracts are deployed")
	}
}

func runDeploy(rpcURL, configFile, buildDir, contractName string, dryRun bool) {
	if contractName == "" {
		log.Fatal("--contract flag is required (use contract name or 'all')")
	}

	log.Println("=== Contract Deployment ===")
	log.Printf("RPC: %s", rpcURL)
	log.Printf("Build dir: %s", buildDir)
	log.Printf("Dry run: %v", dryRun)

	deployer, err := testnet.NewDeployer(rpcURL)
	if err != nil {
		log.Fatalf("Failed to create deployer: %v", err)
	}

	log.Printf("Deployer: %s", deployer.GetAddress())

	gasBalance, err := deployer.GetGASBalanceFloat()
	if err != nil {
		log.Printf("Warning: Could not get GAS balance: %v", err)
	} else {
		log.Printf("GAS balance: %.4f", gasBalance)
	}

	// Load registry
	registry := chain.NewContractRegistry("testnet", filepath.Dir(configFile))
	_ = registry.LoadFromFile(configFile)

	// Determine contracts to deploy
	var toDeploy []string
	if contractName == "all" {
		toDeploy = platformContracts
	} else {
		toDeploy = []string{contractName}
	}

	var totalGas float64
	results := make(map[string]*chain.DeployedContract)

	for _, name := range toDeploy {
		nefPath := filepath.Join(buildDir, name+".nef")
		manifestPath := filepath.Join(buildDir, name+".manifest.json")

		if _, err := os.Stat(nefPath); os.IsNotExist(err) {
			log.Printf("⚠️  %s: Not built (missing %s)", name, nefPath)
			continue
		}

		log.Printf("\n--- %s ---", name)

		// Check if already deployed
		existing := registry.Get(name)
		if existing != nil && existing.Hash != "" {
			log.Printf("Already deployed at: %s", existing.Hash)
			if !dryRun {
				log.Printf("Skipping (use 'update' command to upgrade)")
				continue
			}
		}

		// Simulate deployment
		deployed, err := deployer.DeployContract(nefPath, manifestPath)
		if err != nil {
			log.Printf("❌ Simulation failed: %v", err)
			continue
		}

		gasFloat := parseGas(deployed.GasConsumed)
		totalGas += gasFloat
		log.Printf("Expected hash: %s", deployed.Hash)
		log.Printf("Estimated GAS: %.4f", gasFloat)

		if dryRun {
			log.Printf("✅ Simulation passed (dry-run)")
		} else {
			log.Printf("⚠️  Actual deployment requires neo-go CLI:")
			log.Printf("   neo-go contract deploy -i %s -m %s -r %s -w wallet.json", nefPath, manifestPath, rpcURL)
		}

		results[name] = deployed
	}

	log.Printf("\n=== Summary ===")
	log.Printf("Total estimated GAS: %.4f", totalGas)
	log.Printf("Available GAS: %.4f", gasBalance)

	if totalGas > gasBalance {
		log.Printf("⚠️  Insufficient GAS (need %.4f more)", totalGas-gasBalance)
	}

	if dryRun {
		log.Println("\nTo deploy for real, run with --dry-run=false")
		log.Println("Note: Actual deployment requires manual signing with neo-go CLI")
	}
}

func runUpdate(rpcURL, configFile, buildDir, contractName string) {
	if contractName == "" {
		log.Fatal("--contract flag is required")
	}

	log.Println("=== Contract Update ===")
	log.Printf("Contract: %s", contractName)
	log.Printf("RPC: %s", rpcURL)

	// Load registry
	registry := chain.NewContractRegistry("testnet", filepath.Dir(configFile))
	_ = registry.LoadFromFile(configFile)
	registry.LoadFromEnv()

	info := registry.Get(contractName)
	if info == nil || info.Hash == "" {
		log.Fatalf("Contract %s not found in registry", contractName)
	}

	log.Printf("Current hash: %s", info.Hash)

	nefPath := filepath.Join(buildDir, contractName+".nef")
	manifestPath := filepath.Join(buildDir, contractName+".manifest.json")

	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		log.Fatalf("Contract not built: %s", nefPath)
	}

	log.Println("\nTo update the contract, use neo-go CLI:")
	log.Printf("neo-go contract update -i %s -m %s -r %s -w wallet.json --hash %s", nefPath, manifestPath, rpcURL, info.Hash)
	log.Println("\nNote: Update requires admin signature")
}

func runVerify(rpcURL, configFile string) {
	log.Println("=== Contract Verification ===")
	log.Printf("RPC: %s", rpcURL)

	// Load registry
	registry := chain.NewContractRegistry("testnet", filepath.Dir(configFile))
	_ = registry.LoadFromFile(configFile)
	registry.LoadFromEnv()

	deployer, err := testnet.NewDeployer(rpcURL)
	if err != nil {
		log.Fatalf("Failed to create deployer: %v", err)
	}

	ctx := context.Background()
	client, err := chain.NewClient(chain.Config{
		RPCURL:    rpcURL,
		NetworkID: 894710606,
	})
	if err != nil {
		log.Fatalf("Failed to create chain client: %v", err)
	}

	log.Println("\n=== Verifying Contracts ===")

	for _, name := range platformContracts {
		info := registry.Get(name)
		if info == nil || info.Hash == "" {
			log.Printf("%-20s: NOT DEPLOYED", name)
			continue
		}

		// Check contract state
		state, err := deployer.GetContractState(info.Hash)
		if err != nil {
			log.Printf("%-20s: ERROR - %v", name, err)
			continue
		}

		if state == nil {
			log.Printf("%-20s: NOT FOUND ON CHAIN", name)
			continue
		}

		// Try to invoke a read-only method
		var testMethod string
		switch name {
		case "PriceFeed":
			testMethod = "getLatest"
		case "RandomnessLog":
			testMethod = "getUpdater"
		case "AutomationAnchor":
			testMethod = "getUpdater"
		default:
			testMethod = ""
		}

		if testMethod != "" {
			_, invokeErr := client.Call(ctx, "invokefunction", []interface{}{info.Hash, testMethod, []interface{}{}})
			if invokeErr != nil {
				log.Printf("%-20s: DEPLOYED (invoke test failed: %v)", name, invokeErr)
			} else {
				log.Printf("%-20s: ✅ VERIFIED", name)
			}
		} else {
			log.Printf("%-20s: DEPLOYED (no test method)", name)
		}
	}
}

func runExport(configFile, format string) {
	// Load registry
	registry := chain.NewContractRegistry("testnet", filepath.Dir(configFile))
	_ = registry.LoadFromFile(configFile)
	registry.LoadFromEnv()

	switch format {
	case "env":
		fmt.Println(registry.GenerateEnvExports())
	case "json":
		addresses := registry.GetAddresses()
		data, _ := json.MarshalIndent(addresses, "", "  ")
		fmt.Println(string(data))
	case "dotenv":
		fmt.Println("# Neo N3 Contract Addresses")
		fmt.Println("# Generated at:", time.Now().UTC().Format(time.RFC3339))
		fmt.Println(registry.GenerateEnvExports())
	default:
		log.Fatalf("Unknown format: %s (use env, json, or dotenv)", format)
	}
}

func parseGas(gasConsumed string) float64 {
	var gas int64
	if _, err := fmt.Sscanf(gasConsumed, "%d", &gas); err != nil {
		return 0
	}
	return float64(gas) / 1e8
}
