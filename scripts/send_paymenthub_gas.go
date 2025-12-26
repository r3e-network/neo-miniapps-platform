//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const (
	defaultRPC        = "https://testnet1.neo.coz.io:443"
	defaultPaymentHub = "0x0bb8f09e6d3611bc5c8adbd79ff8af1e34f73193"
	gasContractHashLE = "d2a4cff31913016155e38e474a2c06d08be276cf"
	defaultAppID      = "builtin-lottery"
	defaultAmount     = int64(100000) // 0.001 GAS (GAS has 8 decimals)
	txWaitTimeout     = 2 * time.Minute
)

func main() {
	wif := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	rpcURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	if rpcURL == "" {
		rpcURL = defaultRPC
	}

	paymentHubHash := strings.TrimSpace(os.Getenv("CONTRACT_PAYMENTHUB_HASH"))
	if paymentHubHash == "" {
		paymentHubHash = defaultPaymentHub
	}

	appID := strings.TrimSpace(os.Getenv("PAY_APP_ID"))
	if appID == "" {
		appID = defaultAppID
	}

	amount := defaultAmount
	if raw := strings.TrimSpace(os.Getenv("PAY_GAS_AMOUNT")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			fmt.Printf("Invalid PAY_GAS_AMOUNT: %s\n", raw)
			os.Exit(1)
		}
		amount = parsed
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}
	payerHash := privateKey.GetScriptHash()
	payerAddr := address.Uint160ToString(payerHash)

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	act, err := actor.New(client, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: acc.ScriptHash(),
			Scopes:  transaction.Global,
		},
		Account: acc,
	}})
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	paymentHub, err := parseHash160(paymentHubHash)
	if err != nil {
		fmt.Printf("Invalid PaymentHub hash: %v\n", err)
		os.Exit(1)
	}
	gasHash, _ := util.Uint160DecodeStringLE(gasContractHashLE)

	fmt.Println("=== PaymentHub GAS Transfer ===")
	fmt.Printf("RPC: %s\n", rpcURL)
	fmt.Printf("Payer: %s\n", payerAddr)
	fmt.Printf("PaymentHub: 0x%s\n", paymentHub.StringLE())
	fmt.Printf("App ID: %s\n", appID)
	fmt.Printf("Amount: %d (GAS fractions)\n", amount)

	testResult, err := act.Call(gasHash, "transfer", payerHash, paymentHub, amount, appID)
	if err != nil {
		fmt.Printf("Test invoke failed: %v\n", err)
		os.Exit(1)
	}
	if testResult.State != "HALT" {
		fmt.Printf("Test invoke failed: %s (fault: %s)\n", testResult.State, testResult.FaultException)
		os.Exit(1)
	}

	txHash, vub, err := act.SendCall(gasHash, "transfer", payerHash, paymentHub, amount, appID)
	if err != nil {
		fmt.Printf("Send failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Transaction sent: %s (vub %d)\n", txHash.StringLE(), vub)

	appLog, err := waitForAppLog(ctx, client, txHash)
	if err != nil {
		fmt.Printf("Failed to confirm tx: %v\n", err)
		os.Exit(1)
	}

	printPaymentEvent(appLog, paymentHub)
	printAppBalance(act, paymentHub, appID)
}

func parseHash160(raw string) (util.Uint160, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "0x")
	return util.Uint160DecodeStringLE(raw)
}

func waitForAppLog(ctx context.Context, client *rpcclient.Client, txHash util.Uint256) (*result.ApplicationLog, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(txWaitTimeout)

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for application log")
		case <-ticker.C:
			appLog, err := client.GetApplicationLog(txHash, nil)
			if err != nil {
				continue
			}
			if len(appLog.Executions) == 0 {
				continue
			}
			exec := appLog.Executions[0]
			if !exec.VMState.HasFlag(1) {
				return nil, fmt.Errorf("transaction failed: %s", exec.FaultException)
			}
			return appLog, nil
		}
	}
}

func printPaymentEvent(appLog *result.ApplicationLog, paymentHub util.Uint160) {
	if appLog == nil {
		return
	}
	for _, exec := range appLog.Executions {
		if !exec.VMState.HasFlag(1) {
			continue
		}
		for _, evt := range exec.Events {
			if evt.Name != "PaymentReceived" {
				continue
			}
			if evt.ScriptHash != paymentHub {
				continue
			}
			if evt.Item == nil {
				continue
			}
			items, ok := evt.Item.Value().([]stackitem.Item)
			if !ok || len(items) < 4 {
				continue
			}
			fmt.Println("✅ PaymentReceived event emitted")
			fmt.Printf("ReceiptID: %v\n", items[0].Value())
			fmt.Printf("AppID: %s\n", bytesToString(items[1]))
			fmt.Printf("Payer: %v\n", items[2].Value())
			fmt.Printf("Amount: %v\n", items[3].Value())
			return
		}
	}
	fmt.Println("⚠️  PaymentReceived event not found")
}

func printAppBalance(act *actor.Actor, paymentHub util.Uint160, appID string) {
	if act == nil {
		return
	}
	result, err := act.Call(paymentHub, "getAppBalance", appID)
	if err != nil {
		fmt.Printf("GetAppBalance error: %v\n", err)
		return
	}
	if result.State != "HALT" {
		fmt.Printf("GetAppBalance failed: %s\n", result.FaultException)
		return
	}
	if len(result.Stack) > 0 {
		fmt.Printf("App balance: %v\n", result.Stack[0].Value())
	}
}

func bytesToString(item stackitem.Item) string {
	if item == nil {
		return ""
	}
	switch v := item.Value().(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
