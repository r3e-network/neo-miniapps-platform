package contract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
)

func TestPlatformContractsNeoExpressSmoke(t *testing.T) {
	SkipIfNoNeoExpress(t)
	SkipIfNoCompiledContracts(t)

	if testing.Short() {
		t.Skip("skipping neo-express platform contract test in short mode")
	}

	// Ensure artifacts exist (contract build should already have run).
	for _, name := range []string{
		"PaymentHub",
		"Governance",
		"PriceFeed",
		"RandomnessLog",
		"AppRegistry",
		"AutomationAnchor",
	} {
		if _, _, err := FindContractArtifacts(name); err != nil {
			t.Fatalf("missing contract artifacts for %s: %v", name, err)
		}
	}

	nx := NewNeoExpress(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := nx.Start(ctx); err != nil {
		t.Fatalf("neo-express Start: %v", err)
	}
	t.Cleanup(func() { _ = nx.Stop() })

	account := "genesis"

	genesisScriptHash, err := nx.GetWalletScriptHash(account)
	if err != nil {
		t.Fatalf("GetWalletScriptHash(%s): %v", account, err)
	}

	deploy := func(name string) *chain.DeployedContract {
		nefPath, _, err := FindContractArtifacts(name)
		if err != nil {
			t.Fatalf("FindContractArtifacts(%s): %v", name, err)
		}
		contract, err := nx.Deploy(nefPath, "", account)
		if err != nil {
			t.Fatalf("deploy %s: %v", name, err)
		}
		if contract.Hash == "" {
			t.Fatalf("deploy %s: empty hash", name)
		}
		t.Logf("Deployed %s at %s", name, contract.Hash)
		return contract
	}

	paymentHub := deploy("PaymentHub")
	governance := deploy("Governance")
	priceFeed := deploy("PriceFeed")
	randomnessLog := deploy("RandomnessLog")
	appRegistry := deploy("AppRegistry")
	automationAnchor := deploy("AutomationAnchor")

	// Helpers for reading integer stack results.
	stackInt64 := func(label string, result *chain.InvokeResult) int64 {
		t.Helper()
		if result == nil || len(result.Stack) == 0 {
			t.Fatalf("%s: empty stack", label)
		}
		if result.Stack[0].Type != "Integer" {
			t.Fatalf("%s: expected Integer stack item, got %s", label, result.Stack[0].Type)
		}
		var raw string
		if err := json.Unmarshal(result.Stack[0].Value, &raw); err != nil {
			t.Fatalf("%s: parse integer value: %v", label, err)
		}
		val, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			t.Fatalf("%s: parse int64: %v", label, err)
		}
		return val
	}

	stackBool := func(label string, result *chain.InvokeResult) bool {
		t.Helper()
		if result == nil || len(result.Stack) == 0 {
			t.Fatalf("%s: empty stack", label)
		}
		if result.Stack[0].Type != "Boolean" {
			t.Fatalf("%s: expected Boolean stack item, got %s", label, result.Stack[0].Type)
		}
		var value bool
		if err := json.Unmarshal(result.Stack[0].Value, &value); err != nil {
			t.Fatalf("%s: parse bool value: %v", label, err)
		}
		return value
	}

	stackArray := func(label string, result *chain.InvokeResult) []chain.StackItem {
		t.Helper()
		if result == nil || len(result.Stack) == 0 {
			t.Fatalf("%s: empty stack", label)
		}
		items, err := chain.ParseArray(result.Stack[0])
		if err != nil {
			t.Fatalf("%s: parse array: %v", label, err)
		}
		return items
	}

	// Basic admin reads (smoke). Note: neo-devpack exports ABI method names in lowerCamelCase.
	if _, err := nx.InvokeWithAccountResults(paymentHub.Hash, "admin", account); err != nil {
		t.Fatalf("PaymentHub.admin: %v", err)
	}
	if _, err := nx.InvokeWithAccountResults(governance.Hash, "admin", account); err != nil {
		t.Fatalf("Governance.admin: %v", err)
	}

	// PriceFeed updater flow.
	if _, err := nx.InvokeWithAccount(priceFeed.Hash, "setUpdater", account, map[string]any{
		"type":  "Hash160",
		"value": genesisScriptHash,
	}); err != nil {
		t.Fatalf("PriceFeed.setUpdater: %v", err)
	}

	attestationHash := "0x" + strings.Repeat("11", 32)
	ts := time.Now().Unix()
	if _, err := nx.InvokeWithAccount(priceFeed.Hash, "update", account,
		"BTC-USD", int64(1), int64(100000000), ts, attestationHash, int64(1),
	); err != nil {
		t.Fatalf("PriceFeed.update: %v", err)
	}
	latestPrice, err := nx.InvokeWithAccountResults(priceFeed.Hash, "getLatest", account, "BTC-USD")
	if err != nil {
		t.Fatalf("PriceFeed.getLatest: %v", err)
	}
	latestItems := stackArray("PriceFeed.getLatest", latestPrice)
	if len(latestItems) < 5 {
		t.Fatalf("PriceFeed.getLatest: expected 5 items, got %d", len(latestItems))
	}
	roundID, err := chain.ParseInteger(latestItems[0])
	if err != nil {
		t.Fatalf("PriceFeed.getLatest: parse round_id: %v", err)
	}
	priceValue, err := chain.ParseInteger(latestItems[1])
	if err != nil {
		t.Fatalf("PriceFeed.getLatest: parse price: %v", err)
	}
	if roundID.Int64() != 1 {
		t.Fatalf("PriceFeed.getLatest: round_id = %s, want 1", roundID.String())
	}
	if priceValue.Int64() != 100000000 {
		t.Fatalf("PriceFeed.getLatest: price = %s, want 100000000", priceValue.String())
	}

	// round_id must be monotonic
	if _, err := nx.InvokeWithAccount(priceFeed.Hash, "update", account,
		"BTC-USD", int64(1), int64(100000001), ts, attestationHash, int64(1),
	); err == nil {
		t.Fatalf("PriceFeed.update: expected non-monotonic round_id rejection")
	}

	// RandomnessLog updater flow.
	if _, err := nx.InvokeWithAccount(randomnessLog.Hash, "setUpdater", account, map[string]any{
		"type":  "Hash160",
		"value": genesisScriptHash,
	}); err != nil {
		t.Fatalf("RandomnessLog.setUpdater: %v", err)
	}

	recordID := "req-1"
	randomness := "0x" + strings.Repeat("bb", 32)
	if _, err := nx.InvokeWithAccount(randomnessLog.Hash, "record", account,
		recordID, randomness, attestationHash, ts,
	); err != nil {
		t.Fatalf("RandomnessLog.record: %v", err)
	}
	recordRes, err := nx.InvokeWithAccountResults(randomnessLog.Hash, "get", account, recordID)
	if err != nil {
		t.Fatalf("RandomnessLog.get: %v", err)
	}
	recordItems := stackArray("RandomnessLog.get", recordRes)
	if len(recordItems) < 4 {
		t.Fatalf("RandomnessLog.get: expected 4 items, got %d", len(recordItems))
	}
	if got, err := chain.ParseByteArray(recordItems[1]); err != nil {
		t.Fatalf("RandomnessLog.get: parse randomness: %v", err)
	} else if "0x"+hex.EncodeToString(got) != strings.ToLower(randomness) {
		t.Fatalf("RandomnessLog.get: randomness mismatch")
	}

	// AppRegistry register/get.
	appID := "app-1"
	appIDBytes := hexBytes(appID)
	manifestHash := "0x" + strings.Repeat("cc", 32)
	entryURL := hexBytes("https://example.com/app")
	developerPubKey := "0x" + strings.Repeat("dd", 33)

	if _, err := nx.InvokeWithAccount(appRegistry.Hash, "register", account,
		appIDBytes, manifestHash, entryURL, developerPubKey,
	); err != nil {
		t.Fatalf("AppRegistry.register: %v", err)
	}
	appInfo, err := nx.InvokeWithAccountResults(appRegistry.Hash, "getApp", account, appIDBytes)
	if err != nil {
		t.Fatalf("AppRegistry.getApp: %v", err)
	}
	appItems := stackArray("AppRegistry.getApp", appInfo)
	if len(appItems) < 7 {
		t.Fatalf("AppRegistry.getApp: expected 7 items, got %d", len(appItems))
	}
	status, err := chain.ParseInteger(appItems[5])
	if err != nil {
		t.Fatalf("AppRegistry.getApp: parse status: %v", err)
	}
	if status.Int64() != 0 {
		t.Fatalf("AppRegistry.getApp: status = %s, want 0 (Pending)", status.String())
	}

	if _, err := nx.InvokeWithAccount(appRegistry.Hash, "setStatus", account, appIDBytes, int64(1)); err != nil {
		t.Fatalf("AppRegistry.setStatus: %v", err)
	}
	appInfo2, err := nx.InvokeWithAccountResults(appRegistry.Hash, "getApp", account, appIDBytes)
	if err != nil {
		t.Fatalf("AppRegistry.getApp(after setStatus): %v", err)
	}
	appItems2 := stackArray("AppRegistry.getApp(after setStatus)", appInfo2)
	status2, err := chain.ParseInteger(appItems2[5])
	if err != nil {
		t.Fatalf("AppRegistry.getApp(after setStatus): parse status: %v", err)
	}
	if status2.Int64() != 1 {
		t.Fatalf("AppRegistry.getApp(after setStatus): status = %s, want 1 (Approved)", status2.String())
	}

	// AutomationAnchor task registry + anti-replay.
	if _, err := nx.InvokeWithAccount(automationAnchor.Hash, "setUpdater", account, map[string]any{
		"type":  "Hash160",
		"value": genesisScriptHash,
	}); err != nil {
		t.Fatalf("AutomationAnchor.setUpdater: %v", err)
	}

	taskID := hexBytes("task-1")
	emptyBytes := "0x"

	if _, err := nx.InvokeWithAccount(automationAnchor.Hash, "registerTask", account,
		taskID, priceFeed.Hash, "update", emptyBytes, int64(0), true,
	); err != nil {
		t.Fatalf("AutomationAnchor.registerTask: %v", err)
	}

	txHashBytes := "0x" + strings.Repeat("ee", 32)
	usedBefore, err := nx.InvokeWithAccountResults(automationAnchor.Hash, "isNonceUsed", account, taskID, int64(1))
	if err != nil {
		t.Fatalf("AutomationAnchor.isNonceUsed(before): %v", err)
	}
	if stackBool("AutomationAnchor.isNonceUsed(before)", usedBefore) {
		t.Fatalf("AutomationAnchor.isNonceUsed(before): expected false")
	}

	if _, err := nx.InvokeWithAccount(automationAnchor.Hash, "markExecuted", account,
		taskID, int64(1), txHashBytes,
	); err != nil {
		t.Fatalf("AutomationAnchor.markExecuted: %v", err)
	}

	usedAfter, err := nx.InvokeWithAccountResults(automationAnchor.Hash, "isNonceUsed", account, taskID, int64(1))
	if err != nil {
		t.Fatalf("AutomationAnchor.isNonceUsed(after): %v", err)
	}
	if !stackBool("AutomationAnchor.isNonceUsed(after)", usedAfter) {
		t.Fatalf("AutomationAnchor.isNonceUsed(after): expected true")
	}

	if _, err := nx.InvokeWithAccount(automationAnchor.Hash, "markExecuted", account,
		taskID, int64(1), txHashBytes,
	); err == nil {
		t.Fatalf("AutomationAnchor.markExecuted: expected nonce reuse rejection")
	} else {
		t.Logf("AutomationAnchor.markExecuted duplicate nonce rejected: %v", err)
	}

	// =========================================================================
	// PaymentHub basic workflow (GAS-only)
	// =========================================================================

	recipient := genesisScriptHash
	recipients := []string{recipient}
	shares := []int64{10000}

	if _, err := nx.InvokeWithAccount(paymentHub.Hash, "configureApp", account,
		appID,
		map[string]any{"type": "Hash160", "value": genesisScriptHash},
		recipients,
		shares,
		true,
	); err != nil {
		t.Fatalf("PaymentHub.configureApp: %v", err)
	}

	amount := int64(100000000) // 1 GAS (GAS has 8 decimals)
	if _, err := nx.InvokeWithAccount(paymentHub.Hash, "pay", account, appID, amount, "test payment"); err != nil {
		t.Fatalf("PaymentHub.pay: %v", err)
	}

	balBefore, err := nx.InvokeWithAccountResults(paymentHub.Hash, "getAppBalance", account, appID)
	if err != nil {
		t.Fatalf("PaymentHub.getAppBalance: %v", err)
	}
	if got := stackInt64("PaymentHub.getAppBalance", balBefore); got < amount {
		t.Fatalf("PaymentHub.getAppBalance: expected >= %d, got %d", amount, got)
	}

	if _, err := nx.InvokeWithAccount(paymentHub.Hash, "withdraw", account, appID); err != nil {
		t.Fatalf("PaymentHub.withdraw: %v", err)
	}

	balAfter, err := nx.InvokeWithAccountResults(paymentHub.Hash, "getAppBalance", account, appID)
	if err != nil {
		t.Fatalf("PaymentHub.getAppBalance(after): %v", err)
	}
	if got := stackInt64("PaymentHub.getAppBalance(after)", balAfter); got != 0 {
		t.Fatalf("PaymentHub.getAppBalance(after): expected 0, got %d", got)
	}

	// =========================================================================
	// Governance basic workflow (NEO-only)
	// =========================================================================

	if _, err := nx.InvokeWithAccount(governance.Hash, "stake", account, int64(10)); err != nil {
		t.Fatalf("Governance.stake: %v", err)
	}

	stakeRes, err := nx.InvokeWithAccountResults(governance.Hash, "getStake", account, map[string]any{
		"type":  "Hash160",
		"value": genesisScriptHash,
	})
	if err != nil {
		t.Fatalf("Governance.getStake: %v", err)
	}
	if got := stackInt64("Governance.getStake", stakeRes); got < 10 {
		t.Fatalf("Governance.getStake: expected >= 10, got %d", got)
	}

	proposalID := "proposal-1"
	if _, err := nx.InvokeWithAccount(governance.Hash, "createProposal", account,
		proposalID, "test proposal", int64(0), int64(10_000_000_000_000),
	); err != nil {
		t.Fatalf("Governance.createProposal: %v", err)
	}

	if _, err := nx.InvokeWithAccount(governance.Hash, "vote", account, proposalID, true, int64(5)); err != nil {
		t.Fatalf("Governance.vote: %v", err)
	}

	if _, err := nx.InvokeWithAccount(governance.Hash, "unstake", account, int64(1)); err != nil {
		t.Fatalf("Governance.unstake: %v", err)
	}
}

func hexBytes(value string) string {
	return "0x" + hex.EncodeToString([]byte(value))
}
