//go:build ignore

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

const defaultRPC = "https://testnet1.neo.coz.io:443"

var miniApps = []struct {
	AppID string
}{
	{"builtin-lottery"},
	{"builtin-coin-flip"},
	{"builtin-dice-game"},
	{"builtin-scratch-card"},
	{"builtin-prediction-market"},
	{"builtin-flashloan"},
	{"builtin-price-ticker"},
}

func main() {
	wif := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}

	rpcURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	if rpcURL == "" {
		rpcURL = defaultRPC
	}

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

	deployerHash := privateKey.GetScriptHash()
	deployerAddr := address.Uint160ToString(deployerHash)
	fmt.Printf("Deployer: %s\n", deployerAddr)

	updaterHash := deployerHash
	if raw := strings.TrimSpace(os.Getenv("UPDATER_HASH")); raw != "" {
		if parsed, err := parseContractHash(raw); err != nil {
			fmt.Printf("Invalid UPDATER_HASH: %v\n", err)
			os.Exit(1)
		} else {
			updaterHash = parsed
		}
	} else if raw := strings.TrimSpace(os.Getenv("UPDATER_ADDRESS")); raw != "" {
		if parsed, err := address.StringToUint160(raw); err != nil {
			fmt.Printf("Invalid UPDATER_ADDRESS: %v\n", err)
			os.Exit(1)
		} else {
			updaterHash = parsed
		}
	}
	if updaterHash != deployerHash {
		fmt.Printf("Updater: %s\n", address.Uint160ToString(updaterHash))
	}

	fmt.Println("\n=== Setting Up Updaters ===")
	updaterTargets := map[string]string{
		"PriceFeed":           os.Getenv("CONTRACT_PRICEFEED_HASH"),
		"RandomnessLog":       os.Getenv("CONTRACT_RANDOMNESSLOG_HASH"),
		"AutomationAnchor":    os.Getenv("CONTRACT_AUTOMATIONANCHOR_HASH"),
		"ServiceLayerGateway": os.Getenv("CONTRACT_SERVICEGATEWAY_HASH"),
	}

	for name, hash := range updaterTargets {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			fmt.Printf("- %s: hash not set, skipping\n", name)
			continue
		}
		fmt.Printf("- %s: setUpdater\n", name)
		if err := setUpdater(ctx, client, act, hash, updaterHash); err != nil {
			fmt.Printf("  ❌ %s SetUpdater failed: %v\n", name, err)
		} else {
			fmt.Printf("  ✅ %s updater set\n", name)
		}
		time.Sleep(2 * time.Second)
	}

	paymentHubHash := strings.TrimSpace(os.Getenv("CONTRACT_PAYMENTHUB_HASH"))
	if paymentHubHash == "" {
		fmt.Println("\nPaymentHub hash not set; skipping app configuration")
		return
	}

	fmt.Println("\n=== Configuring PaymentHub MiniApps ===")
	paymentHub, err := parseContractHash(paymentHubHash)
	if err != nil {
		fmt.Printf("Invalid PaymentHub hash: %v\n", err)
		os.Exit(1)
	}

	for _, app := range miniApps {
		fmt.Printf("- %s\n", app.AppID)
		if alreadyConfigured(act, paymentHub, app.AppID) {
			fmt.Printf("  ✅ already configured\n")
			continue
		}

		if err := configureApp(ctx, client, act, paymentHub, app.AppID, deployerHash); err != nil {
			fmt.Printf("  ❌ configure failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ configured\n")
		}
		time.Sleep(2 * time.Second)
	}
}

func parseContractHash(hashStr string) (util.Uint160, error) {
	hashStr = strings.TrimPrefix(strings.TrimSpace(hashStr), "0x")
	return util.Uint160DecodeStringLE(hashStr)
}

func setUpdater(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHashStr string, updater util.Uint160) error {
	contractHash, err := parseContractHash(contractHashStr)
	if err != nil {
		return fmt.Errorf("parse contract hash: %w", err)
	}

	testResult, err := act.Call(contractHash, "setUpdater", updater)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}
	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}

	txHash, vub, err := act.SendCall(contractHash, "setUpdater", updater)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("  tx %s (vub %d)\n", txHash.StringLE(), vub)
	return waitForTx(ctx, client, txHash)
}

func alreadyConfigured(act *actor.Actor, contractHash util.Uint160, appID string) bool {
	result, err := act.Call(contractHash, "getApp", appID)
	if err != nil || result.State != "HALT" || len(result.Stack) == 0 {
		return false
	}
	if result.Stack[0].Type().String() != "Array" {
		return false
	}
	return result.Stack[0].Value() != nil
}

func configureApp(ctx context.Context, client *rpcclient.Client, act *actor.Actor, contractHash util.Uint160, appID string, owner util.Uint160) error {
	recipients := []util.Uint160{owner}
	sharesBps := []int64{10000}

	testResult, err := act.Call(contractHash, "configureApp", appID, owner, recipients, sharesBps, true)
	if err != nil {
		return fmt.Errorf("test invoke failed: %w", err)
	}
	if testResult.State != "HALT" {
		return fmt.Errorf("test invoke failed: %s (fault: %s)", testResult.State, testResult.FaultException)
	}

	txHash, vub, err := act.SendCall(contractHash, "configureApp", appID, owner, recipients, sharesBps, true)
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	fmt.Printf("  tx %s (vub %d)\n", txHash.StringLE(), vub)
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
			if exec.VMState.HasFlag(1) {
				return nil
			}
			return fmt.Errorf("transaction failed: %s", exec.FaultException)
		}
	}
}
