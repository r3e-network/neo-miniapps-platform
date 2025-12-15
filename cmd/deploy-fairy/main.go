// Package main provides a tool to deploy and test Service Layer contracts using Fairy.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/test/fairy"
)

var coreContracts = []string{
	"ServiceLayerGateway",
	"NeoFeedsService",
	"VRFService",
	"NeoVaultService",
	"NeoFlowService",
}

// DeployedContract and DeploymentResult are imported from internal/chain package

func main() {
	fairyURL := flag.String("fairy", "http://127.0.0.1:16868", "Fairy RPC URL")
	buildDir := flag.String("build", "contracts/build", "Contract build directory")
	outputFile := flag.String("output", "deploy/config/fairy_contracts.json", "Output file for deployed contracts")
	gasAmount := flag.Int64("gas", 10000_00000000, "GAS amount to fund account (in fractions)")
	keepSession := flag.Bool("keep", false, "Keep session after deployment (don't delete)")
	flag.Parse()

	log.Println("=== Service Layer Contract Deployment (Fairy) ===")
	log.Printf("Fairy RPC: %s", *fairyURL)
	log.Printf("Build directory: %s", *buildDir)

	client := fairy.NewClient(*fairyURL)

	if !client.IsAvailable() {
		log.Fatal("Fairy not available. Start with: ./test/fairy/start-fairy.sh")
	}

	hello, err := client.HelloFairy()
	if err != nil {
		log.Fatalf("HelloFairy failed: %v", err)
	}
	log.Printf("Fairy status: %v", hello)

	sessionID, accountHash, err := client.SetupSessionWithGas(*gasAmount)
	if err != nil {
		log.Fatalf("SetupSessionWithGas failed: %v", err)
	}
	log.Printf("Session: %s", sessionID)
	log.Printf("Account: %s", accountHash)

	if !*keepSession {
		defer func() {
			if deleteErr := client.DeleteSession(sessionID); deleteErr != nil {
				log.Printf("Warning: DeleteSession failed: %v", deleteErr)
			} else {
				log.Println("Session deleted.")
			}
		}()
	}

	result := chain.DeploymentResult{
		SessionID: sessionID,
		Account:   accountHash,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	log.Println("\n=== Deploying Contracts ===")
	for _, name := range coreContracts {
		nefPath := filepath.Join(*buildDir, name+".nef")
		manifestPath := filepath.Join(*buildDir, name+".manifest.json")

		if _, statErr := os.Stat(nefPath); os.IsNotExist(statErr) {
			log.Printf("  Skipping %s (not built)", name)
			continue
		}

		log.Printf("Deploying %s...", name)
		deployed, deployErr := client.VirtualDeploy(sessionID, nefPath, manifestPath)
		if deployErr != nil {
			log.Printf("  ERROR: %v", deployErr)
			continue
		}

		contract := chain.DeployedContract{
			Name:        name,
			Hash:        deployed.ContractHash,
			GasConsumed: deployed.GasConsumed,
			State:       deployed.State,
		}
		result.Contracts = append(result.Contracts, contract)

		log.Printf("  Hash: %s", deployed.ContractHash)
		log.Printf("  Gas: %s", deployed.GasConsumed)
		log.Printf("  State: %s", deployed.State)

		if deployed.State != "HALT" {
			log.Printf("  WARNING: Deployment did not HALT!")
		}
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(*outputFile), 0o755); mkdirErr != nil {
		log.Printf("Warning: create output dir: %v", mkdirErr)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Warning: marshal output: %v", err)
		data = []byte("{}")
	}
	if err := os.WriteFile(*outputFile, data, 0o600); err != nil {
		log.Printf("Warning: write output: %v", err)
	}

	log.Println("\n=== Deployment Complete ===")
	fmt.Println(string(data))

	log.Println("\n=== Testing Contract Invocations ===")
	for _, c := range result.Contracts {
		if c.Hash == "" {
			continue
		}
		log.Printf("Testing %s (%s)...", c.Name, c.Hash)

		invokeResult, err := client.InvokeFunctionWithSession(
			sessionID,
			false,
			c.Hash,
			"admin",
			nil,
		)
		if err != nil {
			log.Printf("  admin() error: %v", err)
		} else {
			log.Printf("  admin() state: %s", invokeResult.State)
			if len(invokeResult.Stack) > 0 {
				log.Printf("  admin() result: %+v", invokeResult.Stack[0])
			}
		}

		pausedResult, err := client.InvokeFunctionWithSession(
			sessionID,
			false,
			c.Hash,
			"paused",
			nil,
		)
		if err != nil {
			log.Printf("  paused() error: %v", err)
		} else {
			log.Printf("  paused() state: %s", pausedResult.State)
			if len(pausedResult.Stack) > 0 {
				log.Printf("  paused() result: %+v", pausedResult.Stack[0])
			}
		}
	}

	if *keepSession {
		log.Printf("\n=== Session kept: %s ===", sessionID)
		log.Println("Use this session ID for further testing.")
	}
}
