// Package fairy provides integration tests using Neo Fairy.
package fairy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	neofeeds "github.com/R3E-Network/service_layer/services/neofeeds/marble"
)

const (
	fairyRPCURL = "http://127.0.0.1:16868"
)

func skipIfNoFairy(t *testing.T) {
	t.Helper()
	client := NewClient(fairyRPCURL)
	if !client.IsAvailable() {
		t.Skip("Neo Fairy not available at", fairyRPCURL)
	}
}

func getContractPaths(t *testing.T) (nefPath, manifestPath string) {
	t.Helper()

	// Find contracts relative to test file
	testDir, _ := os.Getwd()
	root := filepath.Join(testDir, "..", "..")

	nefPath = filepath.Join(root, "contracts", "build", "NeoFeedsService.nef")
	manifestPath = filepath.Join(root, "contracts", "build", "NeoFeedsService.manifest.json")

	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		t.Skipf("Contract not found: %s", nefPath)
	}
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Skipf("Manifest not found: %s", manifestPath)
	}

	return nefPath, manifestPath
}

// TestFairyConnectivity tests basic connectivity to Fairy.
func TestFairyConnectivity(t *testing.T) {
	skipIfNoFairy(t)

	client := NewClient(fairyRPCURL)
	result, err := client.HelloFairy()
	if err != nil {
		t.Fatalf("HelloFairy: %v", err)
	}

	t.Logf("Fairy says: %+v", result)
}

// TestFairySessionManagement tests session creation and deletion.
func TestFairySessionManagement(t *testing.T) {
	skipIfNoFairy(t)

	client := NewClient(fairyRPCURL)

	// Create session
	sessionID, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Logf("Created session: %s", sessionID)

	// Delete session
	if err := client.DeleteSession(sessionID); err != nil {
		t.Errorf("DeleteSession: %v", err)
	}
}

// TestFairyVirtualDeploy tests virtual contract deployment.
func TestFairyVirtualDeploy(t *testing.T) {
	skipIfNoFairy(t)

	nefPath, manifestPath := getContractPaths(t)
	client := NewClient(fairyRPCURL)

	// Setup session with GAS for deployment (1000 GAS)
	sessionID, accountHash, err := client.SetupSessionWithGas(1000_00000000)
	if err != nil {
		t.Skipf("SetupSessionWithGas: %v", err)
	}
	defer client.DeleteSession(sessionID)
	t.Logf("Session: %s, Account: %s", sessionID, accountHash)

	// Deploy contract
	result, err := client.VirtualDeploy(sessionID, nefPath, manifestPath)
	if err != nil {
		t.Fatalf("VirtualDeploy: %v", err)
	}

	t.Logf("Contract deployed:")
	t.Logf("  Hash: %s", result.ContractHash)
	t.Logf("  Gas: %s", result.GasConsumed)
	t.Logf("  State: %s", result.State)

	if result.State != "HALT" {
		t.Errorf("expected HALT state, got %s", result.State)
	}
}

// TestNeoFeedsServiceWithFairy tests the NeoFeeds service with Fairy.
func TestNeoFeedsServiceWithFairy(t *testing.T) {
	skipIfNoFairy(t)

	nefPath, manifestPath := getContractPaths(t)
	client := NewClient(fairyRPCURL)

	// Setup session with GAS for deployment
	sessionID, _, err := client.SetupSessionWithGas(1000_00000000)
	if err != nil {
		t.Skipf("SetupSessionWithGas: %v", err)
	}
	defer client.DeleteSession(sessionID)

	// Deploy NeoFeedsService contract
	deployResult, err := client.VirtualDeploy(sessionID, nefPath, manifestPath)
	if err != nil {
		t.Fatalf("VirtualDeploy: %v", err)
	}
	contractHash := deployResult.ContractHash
	t.Logf("NeoFeedsService deployed: %s", contractHash)

	// Initialize neofeeds service to fetch prices
	m, _ := marble.New(marble.Config{MarbleType: "neofeeds"})
	m.SetTestSecret("NEOFEEDS_SIGNING_KEY", []byte("test-signing-key-32-bytes-long!!"))

	svc, err := neofeeds.New(&neofeeds.Config{
		Marble:      m,
		ArbitrumRPC: "https://arb1.arbitrum.io/rpc",
	})
	if err != nil {
		t.Fatalf("neofeeds.New: %v", err)
	}

	// Fetch BTC price from Chainlink
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	price, err := svc.GetPrice(ctx, "BTC/USD")
	if err != nil {
		t.Skipf("Price fetch failed (network): %v", err)
	}

	t.Logf("Fetched BTC/USD price: %d (decimals: %d)", price.Price, price.Decimals)
	t.Logf("Signature: %x", price.Signature)
	t.Logf("PublicKey: %x", price.PublicKey)

	// Call GetFeedConfig on the deployed contract
	invokeResult, err := client.InvokeFunctionWithSession(
		sessionID,
		false, // read-only
		contractHash,
		"getFeedConfig",
		[]interface{}{"BTC/USD"},
	)
	if err != nil {
		t.Logf("getFeedConfig failed (expected for new deploy): %v", err)
	} else {
		t.Logf("getFeedConfig result: %+v", invokeResult)
	}
}

// TestNeoFeedsPriceFlow tests the full price flow with Fairy.
func TestNeoFeedsPriceFlow(t *testing.T) {
	skipIfNoFairy(t)

	nefPath, manifestPath := getContractPaths(t)
	client := NewClient(fairyRPCURL)

	// Setup session with GAS
	sessionID, _, err := client.SetupSessionWithGas(1000_00000000)
	if err != nil {
		t.Skipf("SetupSessionWithGas: %v", err)
	}
	defer client.DeleteSession(sessionID)

	// Deploy contract
	deployResult, err := client.VirtualDeploy(sessionID, nefPath, manifestPath)
	if err != nil {
		t.Fatalf("VirtualDeploy: %v", err)
	}
	contractHash := deployResult.ContractHash
	t.Logf("Contract deployed: %s", contractHash)

	// Set virtual time (to avoid timestamp issues)
	now := uint64(time.Now().UnixMilli())
	if setTimeErr := client.SetTime(sessionID, now); setTimeErr != nil {
		t.Logf("SetTime: %v (might not be supported)", setTimeErr)
	}

	// Get admin address (should be the deployer)
	adminResult, err := client.InvokeFunctionWithSession(
		sessionID,
		false,
		contractHash,
		"admin",
		nil,
	)
	if err != nil {
		t.Fatalf("admin(): %v", err)
	}
	t.Logf("Admin result: %+v", adminResult)

	// Check if contract is paused
	pausedResult, err := client.InvokeFunctionWithSession(
		sessionID,
		false,
		contractHash,
		"paused",
		nil,
	)
	if err != nil {
		t.Fatalf("paused(): %v", err)
	}
	t.Logf("Paused result: %+v", pausedResult)
}

// BenchmarkFairyDeploy benchmarks contract deployment via Fairy.
func BenchmarkFairyDeploy(b *testing.B) {
	client := NewClient(fairyRPCURL)
	if !client.IsAvailable() {
		b.Skip("Neo Fairy not available")
	}

	testDir, _ := os.Getwd()
	root := filepath.Join(testDir, "..", "..")
	nefPath := filepath.Join(root, "contracts", "build", "NeoFeedsService.nef")
	manifestPath := filepath.Join(root, "contracts", "build", "NeoFeedsService.manifest.json")

	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		b.Skip("Contract not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessionID, _ := client.NewSession()
		client.VirtualDeploy(sessionID, nefPath, manifestPath)
		client.DeleteSession(sessionID)
	}
}
