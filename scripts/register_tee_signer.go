//go:build ignore

// Script to register TEE signer in AppRegistry for v2.0 contracts

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const rpcURL = "https://testnet1.neo.coz.io:443"

// Contract addresses (v2.0 platform contracts)
var contracts = map[string]string{
	"AppRegistry":      "0x79d16bee03122e992bb80c478ad4ed405f33bc7f",
	"PriceFeed":        "0xc5d9117d255054489d1cf59b2c1d188c01bc9954",
	"RandomnessLog":    "0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39",
	"PaymentHub":       "0x45777109546ceaacfbeed9336d695bb8b8bd77ca",
	"AutomationAnchor": "0x1c888d699ce76b0824028af310d90c3c18adeab5",
}

func main() {
	wif := os.Getenv("NEO_TESTNET_WIF")
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}

	deployerHash := privateKey.GetScriptHash()
	deployerAddr := address.Uint160ToString(deployerHash)
	pubKey := privateKey.PublicKey()

	fmt.Printf("Deployer: %s\n", deployerAddr)
	fmt.Printf("Deployer Hash (LE): %s\n", deployerHash.StringLE())
	fmt.Printf("Public Key: %s\n", pubKey.StringCompressed())

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "deployer"

	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	appRegistryHash, _ := parseContractHash(contracts["AppRegistry"])

	// Step 1: Check if already registered as TEE signer
	fmt.Println("\n=== Checking TEE Signer Registration ===")
	result, err := act.Call(appRegistryHash, "isTeeSigner", deployerHash)
	if err != nil {
		fmt.Printf("Failed to check TEE signer: %v\n", err)
	} else if result.State == "HALT" && len(result.Stack) > 0 {
		if b, ok := result.Stack[0].Value().(bool); ok && b {
			fmt.Println("Already registered as TEE signer")
		} else {
			fmt.Println("Not registered as TEE signer, registering...")
			if err := registerTeeSigner(ctx, client, act, appRegistryHash, deployerHash); err != nil {
				fmt.Printf("Failed to register TEE signer: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("TEE signer registered")
		}
	}

	// Step 2: Verify registration
	fmt.Println("\n=== Verifying Registration ===")
	result, err = act.Call(appRegistryHash, "isTeeSigner", deployerHash)
	if err != nil {
		fmt.Printf("Failed to verify TEE signer: %v\n", err)
	} else if result.State == "HALT" && len(result.Stack) > 0 {
		if b, ok := result.Stack[0].Value().(bool); ok && b {
			fmt.Println("Verified: TEE signer is registered")
		} else {
			fmt.Println("Warning: TEE signer registration may have failed")
		}
	}

	fmt.Println("\n=== Done ===")
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func registerTeeSigner(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHash util.Uint160, signerHash util.Uint160) error {
	// First, test invoke - registerTeeSigner only takes Hash160
	testResult, err := act.Call(contractHash, "registerTeeSigner", signerHash)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}

	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}

	fmt.Printf("Test invoke succeeded, GAS: %s\n", testResult.GasConsumed)

	// Send actual transaction
	txHash, vub, err := act.SendCall(contractHash, "registerTeeSigner", signerHash)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("Transaction sent: %s (valid until block %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func waitForTx(ctx context.Context, client *rpcclient.Client, txHash util.Uint256) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(2 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			appLog, err := client.GetApplicationLog(txHash, nil)
			if err != nil {
				continue
			}

			if len(appLog.Executions) == 0 {
				continue
			}

			exec := appLog.Executions[0]
			if exec.VMState.HasFlag(1) { // HALT
				return nil
			}
			return fmt.Errorf("transaction failed: %s", exec.FaultException)
		}
	}
}
