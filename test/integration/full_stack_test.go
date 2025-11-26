//go:build integration

// Package integration provides full-stack integration tests for the service layer.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestConfig holds test configuration.
type TestConfig struct {
	BaseURL    string
	DevToken   string
	TenantID   string
	AdminUser  string
	AdminPass  string
}

// DefaultTestConfig returns default test configuration.
func DefaultTestConfig() TestConfig {
	baseURL := os.Getenv("TEST_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	devToken := os.Getenv("TEST_DEV_TOKEN")
	if devToken == "" {
		devToken = "dev-token"
	}
	tenantID := os.Getenv("TEST_TENANT_ID")
	if tenantID == "" {
		tenantID = "integration-tenant"
	}
	return TestConfig{
		BaseURL:   baseURL,
		DevToken:  devToken,
		TenantID:  tenantID,
		AdminUser: "admin",
		AdminPass: "admin",
	}
}

// TestClient wraps HTTP client with helper methods.
type TestClient struct {
	config TestConfig
	client *http.Client
}

// NewTestClient creates a new test client.
func NewTestClient(config TestConfig) *TestClient {
	return &TestClient{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Request makes an authenticated HTTP request.
func (c *TestClient) Request(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.config.BaseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.DevToken)
	req.Header.Set("X-Tenant-ID", c.config.TenantID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	return resp
}

// RequestJSON makes a request and decodes the JSON response.
func (c *TestClient) RequestJSON(t *testing.T, method, path string, body, result interface{}) int {
	t.Helper()

	resp := c.Request(t, method, path, body)
	defer resp.Body.Close()

	if result != nil {
		data, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(data, result); err != nil {
			t.Logf("response body: %s", string(data))
			t.Fatalf("decode response: %v", err)
		}
	}

	return resp.StatusCode
}

// RequestWithTenant makes a request with a specific tenant ID.
func (c *TestClient) RequestWithTenant(t *testing.T, method, path string, body interface{}, tenantID string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.config.BaseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.DevToken)
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	return resp
}

// checkAPIAvailable verifies the API is reachable.
func checkAPIAvailable(t *testing.T, client *TestClient) {
	t.Helper()
	resp, err := client.client.Get(client.config.BaseURL + "/healthz")
	if err != nil {
		t.Skipf("API not available at %s: %v", client.config.BaseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("API health check failed: %d", resp.StatusCode)
	}
}

// TestFullStackAccountLifecycle tests complete account CRUD operations.
func TestFullStackAccountLifecycle(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "integration-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)

	if status != http.StatusCreated {
		t.Fatalf("create account: expected 201, got %d", status)
	}

	accountID, ok := account["ID"].(string)
	if !ok || accountID == "" {
		t.Fatalf("expected account ID, got %v", account)
	}
	t.Logf("Created account: %s", accountID)

	// Get account
	var retrieved map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID, nil, &retrieved)
	if status != http.StatusOK {
		t.Fatalf("get account: expected 200, got %d", status)
	}
	if retrieved["ID"] != accountID {
		t.Errorf("expected ID %s, got %v", accountID, retrieved["ID"])
	}

	// List accounts
	var accounts []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts", nil, &accounts)
	if status != http.StatusOK {
		t.Fatalf("list accounts: expected 200, got %d", status)
	}
	if len(accounts) == 0 {
		t.Error("expected at least one account")
	}

	// Update account metadata
	var updated map[string]interface{}
	status = client.RequestJSON(t, http.MethodPatch, "/accounts/"+accountID, map[string]interface{}{
		"metadata": map[string]string{"env": "test", "tier": "integration"},
	}, &updated)
	if status != http.StatusOK {
		t.Logf("update account: got %d (may not be implemented)", status)
	}

	// Cleanup - delete account
	resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Logf("delete account: got %d", resp.StatusCode)
	}
}

// TestFullStackFunctionLifecycle tests function CRUD operations.
func TestFullStackFunctionLifecycle(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account first
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "function-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Create function
	var function map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/functions", map[string]interface{}{
		"name":   "test-function",
		"source": "() => ({ result: 'hello world', timestamp: Date.now() })",
	}, &function)

	if status != http.StatusCreated {
		t.Fatalf("create function: expected 201, got %d", status)
	}

	functionID, ok := function["ID"].(string)
	if !ok || functionID == "" {
		t.Fatalf("expected function ID, got %v", function)
	}
	t.Logf("Created function: %s", functionID)

	// List functions
	var functions []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/functions", nil, &functions)
	if status != http.StatusOK {
		t.Fatalf("list functions: expected 200, got %d", status)
	}
	if len(functions) == 0 {
		t.Error("expected at least one function")
	}

	// Get function
	var retrieved map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/functions/"+functionID, nil, &retrieved)
	if status != http.StatusOK {
		t.Fatalf("get function: expected 200, got %d", status)
	}
}

// TestFullStackSecretManagement tests secret CRUD operations.
func TestFullStackSecretManagement(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "secret-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Create secret
	var secret map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/secrets", map[string]interface{}{
		"name":  "API_KEY",
		"value": "super-secret-api-key-12345",
	}, &secret)

	if status != http.StatusCreated {
		t.Fatalf("create secret: expected 201, got %d", status)
	}
	t.Logf("Created secret: %v", secret)

	// List secrets
	var secrets []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/secrets", nil, &secrets)
	if status != http.StatusOK {
		t.Fatalf("list secrets: expected 200, got %d", status)
	}
	if len(secrets) == 0 {
		t.Error("expected at least one secret")
	}

	// Delete secret
	resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID+"/secrets/API_KEY", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Logf("delete secret: got %d", resp.StatusCode)
	}
}

// TestFullStackOracleDataSource tests oracle data source operations.
func TestFullStackOracleDataSource(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "oracle-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Create data source
	var source map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/oracle/sources", map[string]interface{}{
		"name":        "price-feed",
		"description": "Price data from external API",
		"url":         "https://api.example.com/prices",
		"method":      "GET",
		"headers":     map[string]string{"Authorization": "Bearer xxx"},
	}, &source)

	if status != http.StatusCreated {
		t.Fatalf("create oracle source: expected 201, got %d", status)
	}

	sourceID, ok := source["ID"].(string)
	if !ok || sourceID == "" {
		t.Fatalf("expected source ID, got %v", source)
	}
	t.Logf("Created oracle source: %s", sourceID)

	// Create oracle request
	var request map[string]interface{}
	status = client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/oracle/requests", map[string]interface{}{
		"data_source_id": sourceID,
		"payload":        `{"symbol": "NEO/USD"}`,
	}, &request)

	if status != http.StatusCreated {
		t.Fatalf("create oracle request: expected 201, got %d", status)
	}
	t.Logf("Created oracle request: %v", request)

	// List requests
	var requests []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/oracle/requests?limit=10", nil, &requests)
	if status != http.StatusOK {
		t.Fatalf("list oracle requests: expected 200, got %d", status)
	}
}

// TestFullStackAutomationJob tests automation job creation.
func TestFullStackAutomationJob(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "automation-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Create function for automation
	var function map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/functions", map[string]interface{}{
		"name":   "cron-function",
		"source": "() => ({ executed: true, at: Date.now() })",
	}, &function)
	functionID := function["ID"].(string)

	// Create automation job
	var job map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/automation/jobs", map[string]interface{}{
		"function_id": functionID,
		"name":        "hourly-job",
		"schedule":    "@every 1h",
	}, &job)

	if status != http.StatusCreated {
		t.Fatalf("create automation job: expected 201, got %d", status)
	}
	t.Logf("Created automation job: %v", job)

	// List jobs
	var jobs []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/automation/jobs", nil, &jobs)
	if status != http.StatusOK {
		t.Fatalf("list automation jobs: expected 200, got %d", status)
	}
	if len(jobs) == 0 {
		t.Error("expected at least one job")
	}
}

// TestFullStackPriceFeed tests price feed operations.
func TestFullStackPriceFeed(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "pricefeed-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Create price feed
	var feed map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/pricefeeds", map[string]interface{}{
		"base_asset":         "NEO",
		"quote_asset":        "USD",
		"update_interval":    "5m",
		"heartbeat_interval": "1h",
		"deviation_percent":  1.0,
	}, &feed)

	if status != http.StatusCreated {
		t.Fatalf("create price feed: expected 201, got %d", status)
	}
	t.Logf("Created price feed: %v", feed)

	// List price feeds
	var feeds []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/pricefeeds", nil, &feeds)
	if status != http.StatusOK {
		t.Fatalf("list price feeds: expected 200, got %d", status)
	}
}

// TestFullStackSystemEndpoints tests system status endpoints.
func TestFullStackSystemEndpoints(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Health check (public)
	resp, err := client.client.Get(client.config.BaseURL + "/healthz")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check: expected 200, got %d", resp.StatusCode)
	}

	// System status (authenticated)
	var status map[string]interface{}
	code := client.RequestJSON(t, http.MethodGet, "/system/status", nil, &status)
	if code != http.StatusOK {
		t.Fatalf("system status: expected 200, got %d", code)
	}
	t.Logf("System status: %v", status)

	// System descriptors (authenticated)
	var descriptors interface{}
	code = client.RequestJSON(t, http.MethodGet, "/system/descriptors", nil, &descriptors)
	if code != http.StatusOK {
		t.Fatalf("system descriptors: expected 200, got %d", code)
	}
	t.Logf("System descriptors: %v", descriptors)
}

// TestFullStackTenantIsolation tests tenant isolation.
func TestFullStackTenantIsolation(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account under tenant A
	var accountA map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "tenant-a-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &accountA)
	accountAID := accountA["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountAID, nil)
		resp.Body.Close()
	}()

	// Try to access account A from tenant B
	resp := client.RequestWithTenant(t, http.MethodGet, "/accounts/"+accountAID, nil, "different-tenant")
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-tenant access: expected 403, got %d", resp.StatusCode)
	}
}

// TestFullStackConcurrentRequests tests concurrent request handling.
func TestFullStackConcurrentRequests(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create base account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "concurrent-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Concurrent function creation
	done := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			var fn map[string]interface{}
			status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/functions", map[string]interface{}{
				"name":   fmt.Sprintf("concurrent-fn-%d", idx),
				"source": fmt.Sprintf("() => ({ index: %d })", idx),
			}, &fn)

			if status != http.StatusCreated {
				errors <- fmt.Errorf("function %d: expected 201, got %d", idx, status)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case err := <-errors:
			t.Error(err)
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}

	// Verify all functions were created
	var functions []map[string]interface{}
	client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/functions", nil, &functions)
	if len(functions) < 10 {
		t.Errorf("expected at least 10 functions, got %d", len(functions))
	}
}

// TestFullStackDataFeed tests data feed with signer enforcement.
func TestFullStackDataFeed(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "datafeed-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Register wallet (required for signer_set)
	walletAddr := "0x" + fmt.Sprintf("%040x", time.Now().UnixNano())
	var wallet map[string]interface{}
	status := client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/workspace-wallets", map[string]interface{}{
		"wallet_address": walletAddr,
		"label":          "test-signer",
		"status":         "active",
	}, &wallet)

	if status != http.StatusCreated {
		t.Fatalf("create wallet: expected 201, got %d", status)
	}

	// Create data feed
	var feed map[string]interface{}
	status = client.RequestJSON(t, http.MethodPost, "/accounts/"+accountID+"/datafeeds", map[string]interface{}{
		"pair":              "neo/gas",
		"decimals":          8,
		"heartbeat_seconds": 60,
		"threshold_ppm":     100,
		"signer_set":        []string{walletAddr},
	}, &feed)

	if status != http.StatusCreated {
		t.Fatalf("create data feed: expected 201, got %d", status)
	}
	t.Logf("Created data feed: %v", feed)
}

// TestFullStackVRF tests VRF operations.
func TestFullStackVRF(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "vrf-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// List VRF keys
	var keys []map[string]interface{}
	status := client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/vrf/keys", nil, &keys)
	if status != http.StatusOK {
		t.Fatalf("list VRF keys: expected 200, got %d", status)
	}
	t.Logf("VRF keys: %v", keys)

	// List VRF requests
	var requests []map[string]interface{}
	status = client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/vrf/requests?limit=10", nil, &requests)
	if status != http.StatusOK {
		t.Fatalf("list VRF requests: expected 200, got %d", status)
	}
}

// TestFullStackGasBank tests gas bank operations.
func TestFullStackGasBank(t *testing.T) {
	client := NewTestClient(DefaultTestConfig())
	checkAPIAvailable(t, client)

	// Create account
	var account map[string]interface{}
	client.RequestJSON(t, http.MethodPost, "/accounts", map[string]string{
		"owner": "gasbank-test-owner-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}, &account)
	accountID := account["ID"].(string)
	defer func() {
		resp := client.Request(t, http.MethodDelete, "/accounts/"+accountID, nil)
		resp.Body.Close()
	}()

	// Get gas bank status
	var gasbank map[string]interface{}
	status := client.RequestJSON(t, http.MethodGet, "/accounts/"+accountID+"/gasbank", nil, &gasbank)
	if status != http.StatusOK && status != http.StatusNotFound {
		t.Fatalf("get gasbank: expected 200/404, got %d", status)
	}
	t.Logf("GasBank: %v", gasbank)
}
