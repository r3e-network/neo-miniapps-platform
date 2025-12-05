//go:build integration
// +build integration

// Package contracts provides integration tests for Service Layer contracts.
package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// DeploymentResult represents a deployed contract.
type DeploymentResult struct {
	Contract   string    `json:"contract"`
	ScriptHash string    `json:"script_hash"`
	TxHash     string    `json:"tx_hash"`
	Network    string    `json:"network"`
	DeployedAt time.Time `json:"deployed_at"`
}

// TestConfig holds test configuration.
type TestConfig struct {
	NeoExpressPath string
	RPCEndpoint    string
	Deployments    map[string]string // contract name -> script hash
}

var testConfig *TestConfig

func TestMain(m *testing.M) {
	// Setup
	config, err := setupTestEnvironment()
	if err != nil {
		fmt.Printf("Failed to setup test environment: %v\n", err)
		os.Exit(1)
	}
	testConfig = config

	// Run tests
	code := m.Run()

	// Cleanup (optional - keep neo-express running for debugging)
	// cleanupTestEnvironment()

	os.Exit(code)
}

func setupTestEnvironment() (*TestConfig, error) {
	config := &TestConfig{
		NeoExpressPath: "test/neo-express/default.neo-express",
		RPCEndpoint:    "http://localhost:50012",
		Deployments:    make(map[string]string),
	}

	// Load deployment results
	data, err := os.ReadFile("contracts/build/deployment-neo-express.json")
	if err != nil {
		return nil, fmt.Errorf("deployment results not found: %v", err)
	}

	var results []DeploymentResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("failed to parse deployment results: %v", err)
	}

	for _, r := range results {
		config.Deployments[r.Contract] = r.ScriptHash
	}

	return config, nil
}

// =============================================================================
// Gateway Contract Tests
// =============================================================================

func TestGatewayDeployed(t *testing.T) {
	hash, ok := testConfig.Deployments["ServiceLayerGateway"]
	if !ok || hash == "" {
		t.Fatal("ServiceLayerGateway not deployed")
	}
	t.Logf("Gateway deployed at: %s", hash)
}

func TestGatewayGetAdmin(t *testing.T) {
	hash := testConfig.Deployments["ServiceLayerGateway"]
	if hash == "" {
		t.Skip("Gateway not deployed")
	}

	result, err := invokeRead(hash, "getAdmin")
	if err != nil {
		t.Fatalf("Failed to get admin: %v", err)
	}

	t.Logf("Admin address: %s", result)
}

func TestGatewayGetMinGas(t *testing.T) {
	hash := testConfig.Deployments["ServiceLayerGateway"]
	if hash == "" {
		t.Skip("Gateway not deployed")
	}

	result, err := invokeRead(hash, "getMinGas")
	if err != nil {
		t.Fatalf("Failed to get min gas: %v", err)
	}

	t.Logf("Min gas: %s", result)
}

func TestGatewayIsPaused(t *testing.T) {
	hash := testConfig.Deployments["ServiceLayerGateway"]
	if hash == "" {
		t.Skip("Gateway not deployed")
	}

	result, err := invokeRead(hash, "isPaused")
	if err != nil {
		t.Fatalf("Failed to check paused: %v", err)
	}

	if result != "false" && result != "0" {
		t.Errorf("Expected contract to not be paused, got: %s", result)
	}
}

// =============================================================================
// Oracle Service Tests
// =============================================================================

func TestOracleServiceDeployed(t *testing.T) {
	hash, ok := testConfig.Deployments["OracleService"]
	if !ok || hash == "" {
		t.Fatal("OracleService not deployed")
	}
	t.Logf("OracleService deployed at: %s", hash)
}

func TestOracleServiceRegistered(t *testing.T) {
	gatewayHash := testConfig.Deployments["ServiceLayerGateway"]
	oracleHash := testConfig.Deployments["OracleService"]

	if gatewayHash == "" || oracleHash == "" {
		t.Skip("Contracts not deployed")
	}

	result, err := invokeRead(gatewayHash, "getServiceContract", "oracle")
	if err != nil {
		t.Fatalf("Failed to get service contract: %v", err)
	}

	if result != oracleHash {
		t.Errorf("Oracle not registered correctly. Expected %s, got %s", oracleHash, result)
	}
}

// =============================================================================
// VRF Service Tests
// =============================================================================

func TestVRFServiceDeployed(t *testing.T) {
	hash, ok := testConfig.Deployments["VRFService"]
	if !ok || hash == "" {
		t.Fatal("VRFService not deployed")
	}
	t.Logf("VRFService deployed at: %s", hash)
}

func TestVRFServiceRegistered(t *testing.T) {
	gatewayHash := testConfig.Deployments["ServiceLayerGateway"]
	vrfHash := testConfig.Deployments["VRFService"]

	if gatewayHash == "" || vrfHash == "" {
		t.Skip("Contracts not deployed")
	}

	result, err := invokeRead(gatewayHash, "getServiceContract", "vrf")
	if err != nil {
		t.Fatalf("Failed to get service contract: %v", err)
	}

	if result != vrfHash {
		t.Errorf("VRF not registered correctly. Expected %s, got %s", vrfHash, result)
	}
}

// =============================================================================
// DataFeeds Service Tests
// =============================================================================

func TestDataFeedsServiceDeployed(t *testing.T) {
	hash, ok := testConfig.Deployments["DataFeedsService"]
	if !ok || hash == "" {
		t.Fatal("DataFeedsService not deployed")
	}
	t.Logf("DataFeedsService deployed at: %s", hash)
}

// =============================================================================
// GasBank Service Tests
// =============================================================================

func TestGasBankServiceDeployed(t *testing.T) {
	hash, ok := testConfig.Deployments["GasBankService"]
	if !ok || hash == "" {
		t.Fatal("GasBankService not deployed")
	}
	t.Logf("GasBankService deployed at: %s", hash)
}

// =============================================================================
// End-to-End Flow Tests
// =============================================================================

func TestE2E_GasDeposit(t *testing.T) {
	gatewayHash := testConfig.Deployments["ServiceLayerGateway"]
	if gatewayHash == "" {
		t.Skip("Gateway not deployed")
	}

	// This test would:
	// 1. Transfer GAS to gateway
	// 2. Verify balance increased
	// 3. Withdraw GAS
	// 4. Verify balance decreased

	t.Log("E2E Gas deposit test - requires wallet interaction")
}

func TestE2E_OracleRequest(t *testing.T) {
	gatewayHash := testConfig.Deployments["ServiceLayerGateway"]
	oracleHash := testConfig.Deployments["OracleService"]

	if gatewayHash == "" || oracleHash == "" {
		t.Skip("Contracts not deployed")
	}

	// This test would:
	// 1. Deploy a test consumer contract
	// 2. Deposit GAS for the consumer
	// 3. Make an oracle request
	// 4. Verify ServiceRequest event emitted
	// 5. Simulate TEE callback
	// 6. Verify callback received

	t.Log("E2E Oracle request test - requires full flow")
}

func TestE2E_VRFRequest(t *testing.T) {
	gatewayHash := testConfig.Deployments["ServiceLayerGateway"]
	vrfHash := testConfig.Deployments["VRFService"]

	if gatewayHash == "" || vrfHash == "" {
		t.Skip("Contracts not deployed")
	}

	// This test would:
	// 1. Deploy a test consumer contract
	// 2. Deposit GAS for the consumer
	// 3. Make a VRF request
	// 4. Verify ServiceRequest event emitted
	// 5. Simulate TEE callback with random value
	// 6. Verify callback received with valid proof

	t.Log("E2E VRF request test - requires full flow")
}

// =============================================================================
// Helper Functions
// =============================================================================

func invokeRead(contractHash, method string, args ...string) (string, error) {
	cmdArgs := []string{
		"contract", "invoke",
		contractHash, method,
	}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, "--input", testConfig.NeoExpressPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "neoxp", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, string(output))
	}

	return parseInvokeResult(string(output)), nil
}

func invokeWrite(contractHash, method, account string, args ...string) (string, error) {
	cmdArgs := []string{
		"contract", "invoke",
		contractHash, method,
	}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs,
		"--account", account,
		"--input", testConfig.NeoExpressPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "neoxp", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, string(output))
	}

	return parseTxHash(string(output)), nil
}

func parseInvokeResult(output string) string {
	// Parse result from neoxp output
	// This is simplified - actual parsing depends on output format
	return output
}

func parseTxHash(output string) string {
	// Parse transaction hash from output
	return output
}

func waitForTx(txHash string, timeout time.Duration) error {
	// Wait for transaction to be confirmed
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Check transaction status
		time.Sleep(time.Second)
	}
	return nil
}
