//go:build ignore

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

const defaultRPC = "https://testnet1.neo.coz.io:443"

func main() {
	wif := strings.TrimSpace(os.Getenv("NEO_TESTNET_WIF"))
	if wif == "" {
		fmt.Println("NEO_TESTNET_WIF environment variable not set")
		os.Exit(1)
	}

	miniappHashRaw := resolveMiniAppHash()
	if miniappHashRaw == "" {
		fmt.Println("MiniApp contract hash not set (MINIAPP_CONSUMER_HASH or CONTRACT_MINIAPP_CONSUMER_HASH)")
		os.Exit(1)
	}

	appID := strings.TrimSpace(os.Getenv("MINIAPP_APP_ID"))
	if appID == "" {
		appID = "com.test.consumer"
	}

	rpcURL := strings.TrimSpace(os.Getenv("NEO_RPC_URL"))
	if rpcURL == "" {
		rpcURL = defaultRPC
	}

	privateKey, err := keys.NewPrivateKeyFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid WIF: %v\n", err)
		os.Exit(1)
	}

	miniappHash, err := parseHash160(miniappHashRaw)
	if err != nil {
		fmt.Printf("Invalid MiniApp contract hash: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		fmt.Printf("Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	acc := wallet.NewAccountFromPrivateKey(privateKey)
	act, err := actor.NewSimple(client, acc)
	if err != nil {
		fmt.Printf("Failed to create actor: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== MiniApp RNG Request ===")
	fmt.Printf("RPC: %s\n", rpcURL)
	fmt.Printf("Caller: %s\n", address.Uint160ToString(privateKey.GetScriptHash()))
	fmt.Printf("MiniApp: 0x%s\n", miniappHash.StringLE())
	fmt.Printf("App ID: %s\n", appID)

	testResult, err := act.Call(miniappHash, "requestRng", appID)
	if err != nil {
		fmt.Printf("Test invoke failed: %v\n", err)
		os.Exit(1)
	}
	if testResult.State != "HALT" {
		fmt.Printf("Test invoke failed: %s (fault: %s)\n", testResult.State, testResult.FaultException)
		os.Exit(1)
	}

	txHash, vub, err := act.SendCall(miniappHash, "requestRng", appID)
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

	requestID := extractRequestID(appLog)
	if requestID != "" {
		fmt.Printf("✅ ServiceRequested request_id: %s\n", requestID)
	} else {
		fmt.Println("⚠️  ServiceRequested event not found in application log")
	}

	if requestID != "" && parseEnvBool("MINIAPP_WAIT_CALLBACK") {
		timeout := parseEnvDuration("MINIAPP_CALLBACK_TIMEOUT_SECONDS", 180*time.Second)
		fmt.Printf("Waiting for callback (timeout: %s)...\n", timeout)
		record, err := waitForCallback(ctx, act, miniappHash, requestID, timeout)
		if err != nil {
			fmt.Printf("❌ Callback wait failed: %v\n", err)
			os.Exit(1)
		}
		printCallback(record)
	}
}

func resolveMiniAppHash() string {
	for _, key := range []string{
		"MINIAPP_CONSUMER_HASH",
		"MINIAPP_CONTRACT_HASH",
		"CONTRACT_MINIAPP_CONSUMER_HASH",
	} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			return raw
		}
	}
	return ""
}

func parseHash160(raw string) (util.Uint160, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "0x")
	return util.Uint160DecodeStringLE(raw)
}

func waitForAppLog(ctx context.Context, client *rpcclient.Client, txHash util.Uint256) (*result.ApplicationLog, error) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(2 * time.Minute)

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

type callbackRecord struct {
	RequestID   string
	AppID       string
	ServiceType string
	Success     bool
	Result      []byte
	Error       string
	Timestamp   *big.Int
}

func waitForCallback(ctx context.Context, act *actor.Actor, contract util.Uint160, requestID string, timeout time.Duration) (*callbackRecord, error) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for callback")
		case <-ticker.C:
			record, err := fetchCallback(ctx, act, contract)
			if err != nil {
				continue
			}
			if record != nil && record.RequestID == requestID {
				return record, nil
			}
		}
	}
}

func fetchCallback(ctx context.Context, act *actor.Actor, contract util.Uint160) (*callbackRecord, error) {
	result, err := act.Call(contract, "getLastCallback")
	if err != nil {
		return nil, err
	}
	if result.State != "HALT" || len(result.Stack) == 0 {
		return nil, fmt.Errorf("callback call failed: %s", result.State)
	}

	items, ok := result.Stack[0].Value().([]stackitem.Item)
	if !ok || len(items) < 7 {
		return nil, fmt.Errorf("unexpected callback payload")
	}

	reqID, err := items[0].TryInteger()
	if err != nil {
		return nil, fmt.Errorf("callback request id invalid")
	}
	appID, err := itemToString(items[1])
	if err != nil {
		return nil, err
	}
	serviceType, err := itemToString(items[2])
	if err != nil {
		return nil, err
	}
	success, err := items[3].TryBool()
	if err != nil {
		return nil, fmt.Errorf("callback success invalid")
	}
	resultBytes, err := items[4].TryBytes()
	if err != nil {
		resultBytes = nil
	}
	errorMsg, err := itemToString(items[5])
	if err != nil {
		errorMsg = ""
	}
	timestamp, err := items[6].TryInteger()
	if err != nil {
		timestamp = big.NewInt(0)
	}

	return &callbackRecord{
		RequestID:   reqID.String(),
		AppID:       appID,
		ServiceType: serviceType,
		Success:     success,
		Result:      resultBytes,
		Error:       errorMsg,
		Timestamp:   timestamp,
	}, nil
}

func itemToString(item stackitem.Item) (string, error) {
	bytes, err := item.TryBytes()
	if err != nil {
		return "", fmt.Errorf("callback string invalid")
	}
	return string(bytes), nil
}

func printCallback(record *callbackRecord) {
	if record == nil {
		return
	}

	resultHex := ""
	if len(record.Result) > 0 {
		resultHex = hex.EncodeToString(record.Result)
	}

	fmt.Println("=== Callback Received ===")
	fmt.Printf("Request ID: %s\n", record.RequestID)
	fmt.Printf("App ID: %s\n", record.AppID)
	fmt.Printf("Service: %s\n", record.ServiceType)
	fmt.Printf("Success: %t\n", record.Success)
	if record.Error != "" {
		fmt.Printf("Error: %s\n", record.Error)
	}
	if resultHex != "" {
		fmt.Printf("Result (hex): %s\n", resultHex)
	}
	if record.Timestamp != nil {
		fmt.Printf("Timestamp: %s\n", record.Timestamp.String())
	}
}

func extractRequestID(appLog *result.ApplicationLog) string {
	if appLog == nil {
		return ""
	}

	for _, exec := range appLog.Executions {
		if !exec.VMState.HasFlag(1) {
			continue
		}
		for _, evt := range exec.Events {
			if evt.Name != "ServiceRequested" {
				continue
			}
			if evt.Item == nil {
				continue
			}
			items, ok := evt.Item.Value().([]stackitem.Item)
			if !ok || len(items) == 0 {
				continue
			}
			if reqID, ok := items[0].Value().(*big.Int); ok {
				return reqID.String()
			}
		}
	}
	return ""
}

func parseEnvBool(key string) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err == nil {
		return parsed
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil {
		return seconds
	}
	return fallback
}
