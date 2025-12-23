//go:build ignore

// Script to configure PaymentHub for all 7 MiniApps

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

// Contract addresses
var contracts = map[string]string{
	"PaymentHub": "0x45777109546ceaacfbeed9336d695bb8b8bd77ca",
}

// All 7 MiniApps to configure
var miniApps = []struct {
	AppID       string
	Name        string
	Category    string
	Description string
}{
	{"builtin-lottery", "Neo Lottery", "gaming", "Decentralized lottery with provably fair randomness"},
	{"builtin-coin-flip", "Neo Coin Flip", "gaming", "50/50 coin flip game"},
	{"builtin-dice-game", "Neo Dice", "gaming", "Roll dice and win up to 6x"},
	{"builtin-scratch-card", "Neo Scratch Cards", "gaming", "Instant win scratch cards"},
	{"builtin-prediction-market", "Neo Predictions", "defi", "Bet on price movements"},
	{"builtin-flashloan", "Neo FlashLoan", "defi", "Instant borrow and repay"},
	{"builtin-price-ticker", "Neo Price Ticker", "defi", "Real-time price feeds"},
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
	fmt.Printf("Deployer: %s\n", deployerAddr)
	fmt.Printf("Deployer Hash (LE): %s\n", deployerHash.StringLE())

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

	paymentHubHash, _ := parseContractHash(contracts["PaymentHub"])

	fmt.Println("\n=== Configuring PaymentHub for All 7 MiniApps ===")

	for _, app := range miniApps {
		fmt.Printf("\n--- Configuring %s (%s) ---\n", app.Name, app.AppID)

		// Check if already configured
		result, err := act.Call(paymentHubHash, "getApp", app.AppID)
		if err == nil && result.State == "HALT" && len(result.Stack) > 0 {
			// Check if owner is set (non-null)
			if result.Stack[0].Type().String() == "Array" {
				arr := result.Stack[0].Value()
				if arr != nil {
					fmt.Printf("✅ %s already configured\n", app.AppID)
					continue
				}
			}
		}

		// Configure the app
		if err := configureApp(ctx, client, act, paymentHubHash, app.AppID, deployerHash); err != nil {
			fmt.Printf("❌ Failed to configure %s: %v\n", app.AppID, err)
		} else {
			fmt.Printf("✅ %s configured successfully\n", app.AppID)
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Println("\n=== PaymentHub Configuration Complete ===")
	fmt.Printf("Configured %d MiniApps\n", len(miniApps))
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(hashStr, "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func configureApp(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHash util.Uint160, appID string, owner util.Uint160) error {
	// ConfigureApp(appId, owner, recipients[], sharesBps[], enabled)
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
