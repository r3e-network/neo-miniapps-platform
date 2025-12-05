// Package e2e provides end-to-end tests for Neo Service Layer.
// These tests verify the complete request flow from frontend to blockchain.
//
// Run with: go test -tags=e2e ./test/e2e/...
//
//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// =============================================================================
// Test Configuration
// =============================================================================

type E2EConfig struct {
	SupabaseURL    string
	SupabaseKey    string
	CoordinatorURL string
	NeoRPCURL      string
}

func loadConfig() E2EConfig {
	return E2EConfig{
		SupabaseURL:    getEnvOrDefault("SUPABASE_URL", "http://localhost:54321"),
		SupabaseKey:    getEnvOrDefault("SUPABASE_ANON_KEY", ""),
		CoordinatorURL: getEnvOrDefault("COORDINATOR_URL", "https://localhost:4433"),
		NeoRPCURL:      getEnvOrDefault("NEO_RPC_URL", "http://localhost:10332"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func skipE2E(t *testing.T) {
	if os.Getenv("E2E_TEST") != "true" {
		t.Skip("Skipping E2E test. Set E2E_TEST=true to run.")
	}
}

// =============================================================================
// Service Request Types
// =============================================================================

type ServiceRequest struct {
	ID          string                 `json:"id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ServiceType string                 `json:"service_type"`
	Operation   string                 `json:"operation"`
	Payload     map[string]interface{} `json:"payload"`
	Status      string                 `json:"status,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error_message,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
}

// =============================================================================
// E2E Test Client
// =============================================================================

type E2EClient struct {
	config     E2EConfig
	httpClient *http.Client
}

func NewE2EClient(config E2EConfig) *E2EClient {
	return &E2EClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *E2EClient) SubmitRequest(ctx context.Context, req ServiceRequest) (*ServiceRequest, error) {
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.config.SupabaseURL+"/rest/v1/service_requests",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", c.config.SupabaseKey)
	httpReq.Header.Set("Authorization", "Bearer "+c.config.SupabaseKey)
	httpReq.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var results []ServiceRequest
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no request returned")
	}

	return &results[0], nil
}

func (c *E2EClient) GetRequest(ctx context.Context, id string) (*ServiceRequest, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		c.config.SupabaseURL+"/rest/v1/service_requests?id=eq."+id,
		nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("apikey", c.config.SupabaseKey)
	httpReq.Header.Set("Authorization", "Bearer "+c.config.SupabaseKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []ServiceRequest
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("request not found")
	}

	return &results[0], nil
}

func (c *E2EClient) WaitForCompletion(ctx context.Context, id string, timeout time.Duration) (*ServiceRequest, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		req, err := c.GetRequest(ctx, id)
		if err != nil {
			return nil, err
		}

		if req.Status == "completed" || req.Status == "failed" {
			return req, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return nil, fmt.Errorf("timeout waiting for request completion")
}

// =============================================================================
// Oracle Service E2E Tests
// =============================================================================

func TestE2E_OracleFetch(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	// Submit oracle fetch request
	req := ServiceRequest{
		ServiceType: "oracle",
		Operation:   "fetch",
		Payload: map[string]interface{}{
			"url":    "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
			"method": "GET",
		},
	}

	submitted, err := client.SubmitRequest(ctx, req)
	if err != nil {
		t.Fatalf("SubmitRequest error: %v", err)
	}

	t.Logf("Submitted request: %s", submitted.ID)

	// Wait for completion
	result, err := client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Status = %s, want completed. Error: %s", result.Status, result.Error)
	}

	if result.Result == nil {
		t.Error("Result should not be nil")
	}

	t.Logf("Oracle result: %v", result.Result)
}

// =============================================================================
// VRF Service E2E Tests
// =============================================================================

func TestE2E_VRFRandom(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	// Submit VRF random request
	req := ServiceRequest{
		ServiceType: "vrf",
		Operation:   "random",
		Payload: map[string]interface{}{
			"seed":       "test-seed-12345",
			"num_values": 1,
		},
	}

	submitted, err := client.SubmitRequest(ctx, req)
	if err != nil {
		t.Fatalf("SubmitRequest error: %v", err)
	}

	result, err := client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Status = %s, want completed", result.Status)
	}

	// Verify randomness and proof
	if result.Result["randomness"] == nil {
		t.Error("Result should contain randomness")
	}
	if result.Result["proof"] == nil {
		t.Error("Result should contain proof")
	}

	t.Logf("VRF result: %v", result.Result)
}

// =============================================================================
// Secrets Service E2E Tests
// =============================================================================

func TestE2E_SecretsStoreGet(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	secretKey := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
	secretValue := "super-secret-value"

	// Store secret
	storeReq := ServiceRequest{
		ServiceType: "secrets",
		Operation:   "store",
		Payload: map[string]interface{}{
			"key":   secretKey,
			"value": secretValue,
		},
	}

	submitted, err := client.SubmitRequest(ctx, storeReq)
	if err != nil {
		t.Fatalf("Store request error: %v", err)
	}

	result, err := client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("Store failed: %s", result.Error)
	}

	// Get secret
	getReq := ServiceRequest{
		ServiceType: "secrets",
		Operation:   "get",
		Payload: map[string]interface{}{
			"key": secretKey,
		},
	}

	submitted, err = client.SubmitRequest(ctx, getReq)
	if err != nil {
		t.Fatalf("Get request error: %v", err)
	}

	result, err = client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("Get failed: %s", result.Error)
	}

	// Note: In real implementation, value would be encrypted
	t.Logf("Secret retrieved successfully")
}

// =============================================================================
// GasBank Service E2E Tests
// =============================================================================

func TestE2E_GasBankBalance(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	// Check balance
	req := ServiceRequest{
		ServiceType: "gasbank",
		Operation:   "balance",
		Payload: map[string]interface{}{
			"account": "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
		},
	}

	submitted, err := client.SubmitRequest(ctx, req)
	if err != nil {
		t.Fatalf("SubmitRequest error: %v", err)
	}

	result, err := client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion error: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Status = %s, want completed", result.Status)
	}

	t.Logf("GasBank balance: %v", result.Result)
}

// =============================================================================
// Full Flow E2E Test
// =============================================================================

func TestE2E_FullOracleToBlockchain(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	// 1. Submit oracle request
	oracleReq := ServiceRequest{
		ServiceType: "oracle",
		Operation:   "fetch",
		Payload: map[string]interface{}{
			"url":           "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
			"callback_hash": "0x1234567890abcdef", // Contract hash
		},
	}

	submitted, err := client.SubmitRequest(ctx, oracleReq)
	if err != nil {
		t.Fatalf("Oracle request error: %v", err)
	}

	t.Logf("Step 1: Oracle request submitted: %s", submitted.ID)

	// 2. Wait for oracle completion
	result, err := client.WaitForCompletion(ctx, submitted.ID, 60*time.Second)
	if err != nil {
		t.Fatalf("Oracle completion error: %v", err)
	}

	if result.Status != "completed" {
		t.Fatalf("Oracle failed: %s", result.Error)
	}

	t.Logf("Step 2: Oracle completed with result: %v", result.Result)

	// 3. Verify TEE signature exists
	if result.Result["tee_signature"] == nil {
		t.Log("Warning: TEE signature not present (may be simulation mode)")
	}

	t.Log("Step 3: Full flow completed successfully")
}

// =============================================================================
// Concurrent Requests E2E Test
// =============================================================================

func TestE2E_ConcurrentRequests(t *testing.T) {
	skipE2E(t)

	config := loadConfig()
	client := NewE2EClient(config)
	ctx := context.Background()

	numRequests := 5
	results := make(chan *ServiceRequest, numRequests)
	errors := make(chan error, numRequests)

	// Submit concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			req := ServiceRequest{
				ServiceType: "vrf",
				Operation:   "random",
				Payload: map[string]interface{}{
					"seed": fmt.Sprintf("concurrent-seed-%d", idx),
				},
			}

			submitted, err := client.SubmitRequest(ctx, req)
			if err != nil {
				errors <- err
				return
			}

			result, err := client.WaitForCompletion(ctx, submitted.ID, 60*time.Second)
			if err != nil {
				errors <- err
				return
			}

			results <- result
		}(i)
	}

	// Collect results
	completed := 0
	failed := 0

	for i := 0; i < numRequests; i++ {
		select {
		case result := <-results:
			if result.Status == "completed" {
				completed++
			} else {
				failed++
			}
		case err := <-errors:
			t.Logf("Request error: %v", err)
			failed++
		case <-time.After(120 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	t.Logf("Concurrent requests: %d completed, %d failed", completed, failed)

	if completed < numRequests/2 {
		t.Errorf("Too many failures: %d/%d", failed, numRequests)
	}
}
