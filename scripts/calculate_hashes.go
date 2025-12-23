//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
)

const (
	rpcURL   = "https://testnet1.neo.coz.io:443"
	buildDir = "contracts/build"
)

var contracts = []string{"PaymentHub", "Governance", "PriceFeed", "RandomnessLog", "AppRegistry", "AutomationAnchor"}

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	// Parse private key to get deployer account hash
	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}
	deployerHash := privateKey.GetScriptHash()

	// Create RPC client
	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Contract Hash Calculation ===")
	fmt.Printf("Deployer: %s\n\n", deployerHash.StringLE())

	for _, name := range contracts {
		nefPath := filepath.Join(buildDir, name+".nef")
		manifestPath := filepath.Join(buildDir, name+".manifest.json")

		// Load NEF
		nefData, err := os.ReadFile(nefPath)
		if err != nil {
			fmt.Printf("%s: NEF not found\n", name)
			continue
		}
		nefFile, err := nef.FileFromBytes(nefData)
		if err != nil {
			fmt.Printf("%s: Invalid NEF: %v\n", name, err)
			continue
		}

		// Load manifest
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			fmt.Printf("%s: Manifest not found\n", name)
			continue
		}
		var m manifest.Manifest
		if err := json.Unmarshal(manifestData, &m); err != nil {
			fmt.Printf("%s: Invalid manifest: %v\n", name, err)
			continue
		}

		// Calculate expected contract hash
		contractHash := state.CreateContractHash(deployerHash, nefFile.Checksum, m.Name)
		hashStr := "0x" + contractHash.StringLE()

		// Verify on chain
		contractState, err := client.GetContractStateByHash(contractHash)
		status := "NOT DEPLOYED"
		if err == nil && contractState != nil {
			status = "DEPLOYED ✅"
		}

		fmt.Printf("%-20s %s  [%s]\n", name+":", hashStr, status)
	}
}
