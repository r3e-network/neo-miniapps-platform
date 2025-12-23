//go:build ignore

// Script to set up contracts for simulation

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

// Contract addresses (new v2.0 platform contracts)
var contracts = map[string]string{
	"PriceFeed":        "0xc5d9117d255054489d1cf59b2c1d188c01bc9954",
	"RandomnessLog":    "0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39",
	"PaymentHub":       "0x45777109546ceaacfbeed9336d695bb8b8bd77ca",
	"AutomationAnchor": "0x1c888d699ce76b0824028af310d90c3c18adeab5",
}

// MiniApps to configure
var miniApps = []string{"builtin-lottery", "builtin-coin-flip", "builtin-dice-game"}

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
	fmt.Printf("Deployer: %s\n", deployerAddr)
	fmt.Printf("Deployer Hash (LE): %s\n", deployerHash.StringLE())

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	// Create wallet account
	acc := wallet.NewAccountFromPrivateKey(privateKey)
	acc.Label = "deployer"

	// Create actor with CalledByEntry scope
	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Setting Up Contracts ===")

	// 1. Set updater for PriceFeed
	fmt.Println("\n--- PriceFeed: SetUpdater ---")
	if err := setUpdater(ctx, client, act, contracts["PriceFeed"], deployerHash); err != nil {
		fmt.Printf("PriceFeed SetUpdater failed: %v\n", err)
	} else {
		fmt.Println("✅ PriceFeed updater set")
	}
	time.Sleep(2 * time.Second)

	// 2. Set updater for RandomnessLog
	fmt.Println("\n--- RandomnessLog: SetUpdater ---")
	if err := setUpdater(ctx, client, act, contracts["RandomnessLog"], deployerHash); err != nil {
		fmt.Printf("RandomnessLog SetUpdater failed: %v\n", err)
	} else {
		fmt.Println("✅ RandomnessLog updater set")
	}
	time.Sleep(2 * time.Second)

	// 3. Configure MiniApps in PaymentHub
	fmt.Println("\n--- PaymentHub: ConfigureApp ---")
	for _, appID := range miniApps {
		if err := configureApp(ctx, client, act, contracts["PaymentHub"], appID, deployerHash); err != nil {
			fmt.Printf("PaymentHub ConfigureApp(%s) failed: %v\n", appID, err)
		} else {
			fmt.Printf("✅ PaymentHub app configured: %s\n", appID)
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== Contract Setup Complete ===")
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func setUpdater(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHashStr string, updater util.Uint160) error {
	contractHash, err := parseContractHash(contractHashStr)
	if err != nil {
		return fmt.Errorf("parse contract hash: %w", err)
	}

	// First, test invoke to check if it will succeed
	// Pass the UInt160 directly - neo-go will convert it properly
	testResult, err := act.Call(contractHash, "setUpdater", updater)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}

	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}

	fmt.Printf("Test invoke succeeded, GAS: %s\n", testResult.GasConsumed)

	// Now send the actual transaction
	txHash, vub, err := act.SendCall(contractHash, "setUpdater", updater)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("Transaction sent: %s (valid until block %d)\n", txHash.StringLE(), vub)

	return waitForTx(ctx, client, txHash)
}

func configureApp(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHashStr string, appID string, owner util.Uint160) error {
	contractHash, err := parseContractHash(contractHashStr)
	if err != nil {
		return fmt.Errorf("parse contract hash: %w", err)
	}

	// ConfigureApp(appId, owner, recipients[], sharesBps[], enabled)
	// Pass arrays directly
	recipients := []util.Uint160{owner}
	sharesBps := []int64{10000} // 100% to owner

	// First, test invoke
	testResult, err := act.Call(contractHash, "configureApp", appID, owner, recipients, sharesBps, true)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}

	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}

	fmt.Printf("Test invoke succeeded, GAS: %s\n", testResult.GasConsumed)

	// Send actual transaction
	txHash, vub, err := act.SendCall(contractHash, "configureApp", appID, owner, recipients, sharesBps, true)
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
				continue // Not yet included
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
