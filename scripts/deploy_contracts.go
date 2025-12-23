//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/management"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const (
	rpcURL       = "https://testnet1.neo.coz.io:443"
	buildDir     = "contracts/build"
	configFile   = "deploy/config/testnet_contracts.json"
	networkMagic = 894710606
)

var contractsToDeploy = []string{"Governance", "AppRegistry"}

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	// Parse private key
	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deployer address: %s\n", address.Uint160ToString(privateKey.GetScriptHash()))

	// Create RPC client
	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	// Get network magic
	version, err := client.GetVersion()
	if err != nil {
		fmt.Printf("Failed to get version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Connected to network: %d\n", version.Protocol.Network)

	// Create in-memory wallet account
	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "deployer"

	// Create actor for signing transactions
	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	// Create management client for deployment
	mgmt := management.New(act)

	// Load existing config
	config := loadConfig(configFile)

	fmt.Println("\n=== Deploying Contracts ===")

	for _, name := range contractsToDeploy {
		fmt.Printf("\n--- %s ---\n", name)

		// Check if already deployed
		if existing, ok := config.Contracts[name]; ok && existing.Hash != "" && existing.Hash != "null" {
			fmt.Printf("Already deployed at: %s\n", existing.Hash)
			continue
		}

		// Load NEF and manifest
		nefPath := filepath.Join(buildDir, name+".nef")
		manifestPath := filepath.Join(buildDir, name+".manifest.json")

		nefFile, err := loadNEF(nefPath)
		if err != nil {
			fmt.Printf("Failed to load NEF: %v\n", err)
			continue
		}

		m, err := loadManifest(manifestPath)
		if err != nil {
			fmt.Printf("Failed to load manifest: %v\n", err)
			continue
		}

		fmt.Printf("Deploying %s...\n", name)

		// Deploy contract
		txHash, vub, err := mgmt.Deploy(nefFile, m, nil)
		if err != nil {
			fmt.Printf("Failed to deploy: %v\n", err)
			continue
		}

		fmt.Printf("Transaction sent: %s\n", txHash.StringLE())
		fmt.Printf("Valid until block: %d\n", vub)

		// Wait for transaction to be included
		fmt.Println("Waiting for confirmation...")
		contractHash, err := waitForDeployment(ctx, client, txHash, vub)
		if err != nil {
			fmt.Printf("Deployment failed: %v\n", err)
			continue
		}

		fmt.Printf("✅ Contract deployed at: %s\n", contractHash)

		// Update config
		config.Contracts[name] = &ContractInfo{
			Name:       name,
			Hash:       contractHash,
			Version:    "1.0.0",
			DeployedAt: time.Now().UTC().Format(time.RFC3339),
			Network:    "testnet",
			Status:     "deployed",
		}
	}

	// Save updated config
	if err := saveConfig(configFile, config); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
	} else {
		fmt.Println("\n✅ Configuration updated")
	}

	fmt.Println("\n=== Deployment Complete ===")
}

func loadNEF(path string) (*nef.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := nef.FileFromBytes(data)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func loadManifest(path string) (*manifest.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func waitForDeployment(ctx context.Context, client *rpcclient.Client, txHash util.Uint256, vub uint32) (string, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			// Check if transaction is included
			appLog, err := client.GetApplicationLog(txHash, nil)
			if err != nil {
				// Transaction not yet included
				continue
			}

			if len(appLog.Executions) == 0 {
				continue
			}

			exec := appLog.Executions[0]
			if exec.VMState.HasFlag(1) { // HALT
				// Get contract hash from stack - deploy returns the contract state
				if len(exec.Stack) > 0 {
					item := exec.Stack[0]
					// The deploy returns a struct, try to extract hash
					if arr, ok := item.Value().([]interface{}); ok && len(arr) > 0 {
						// First element should be the contract hash
						if hashItem, ok := arr[0].([]byte); ok {
							return fmt.Sprintf("0x%x", hashItem), nil
						}
					}
					// Try direct byte array
					if bs, ok := item.Value().([]byte); ok {
						return fmt.Sprintf("0x%x", bs), nil
					}
				}
				// Fallback: get from notifications
				for _, notif := range exec.Events {
					if notif.Name == "Deploy" && len(notif.Item.Value().([]interface{})) > 0 {
						if hash, ok := notif.Item.Value().([]interface{})[0].([]byte); ok {
							return fmt.Sprintf("0x%x", hash), nil
						}
					}
				}
				return "deployed-check-explorer", nil
			} else {
				return "", fmt.Errorf("transaction failed: %s", exec.FaultException)
			}
		}
	}
}

type ContractInfo struct {
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Version    string `json:"version,omitempty"`
	DeployedAt string `json:"deployed_at,omitempty"`
	Network    string `json:"network,omitempty"`
	Status     string `json:"status,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type Config struct {
	Network         string                   `json:"network"`
	UpdatedAt       string                   `json:"updated_at"`
	RPCEndpoints    []string                 `json:"rpc_endpoints,omitempty"`
	NetworkMagic    int                      `json:"network_magic,omitempty"`
	Contracts       map[string]*ContractInfo `json:"contracts"`
	LegacyContracts map[string]interface{}   `json:"legacy_contracts,omitempty"`
	Deployer        map[string]string        `json:"deployer,omitempty"`
}

func loadConfig(path string) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{
			Network:   "testnet",
			Contracts: make(map[string]*ContractInfo),
		}
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return &Config{
			Network:   "testnet",
			Contracts: make(map[string]*ContractInfo),
		}
	}

	if config.Contracts == nil {
		config.Contracts = make(map[string]*ContractInfo)
	}

	return &config
}

func saveConfig(path string, config *Config) error {
	config.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
