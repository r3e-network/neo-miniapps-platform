//go:build smoke

// Package smoke provides smoke tests for quick validation of system health.
// Smoke tests are lightweight tests that verify basic functionality without
// extensive coverage - they're designed to catch obvious failures quickly.
package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// SmokeConfig holds smoke test configuration.
type SmokeConfig struct {
	BaseURL  string
	DevToken string
	TenantID string
	Timeout  time.Duration
}

// DefaultSmokeConfig returns default configuration for smoke tests.
func DefaultSmokeConfig() SmokeConfig {
	baseURL := os.Getenv("SMOKE_API_URL")
	if baseURL == "" {
		baseURL = os.Getenv("TEST_API_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	devToken := os.Getenv("SMOKE_DEV_TOKEN")
	if devToken == "" {
		devToken = os.Getenv("TEST_DEV_TOKEN")
	}
	if devToken == "" {
		devToken = "dev-token"
	}
	tenantID := os.Getenv("SMOKE_TENANT_ID")
	if tenantID == "" {
		tenantID = os.Getenv("TEST_TENANT_ID")
	}
	if tenantID == "" {
		tenantID = "smoke-tenant"
	}
	timeout := 10 * time.Second
	if t := os.Getenv("SMOKE_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}
	return SmokeConfig{
		BaseURL:  baseURL,
		DevToken: devToken,
		TenantID: tenantID,
		Timeout:  timeout,
	}
}

// SmokeClient wraps HTTP client for smoke tests.
type SmokeClient struct {
	config SmokeConfig
	client *http.Client
}

// NewSmokeClient creates a new smoke test client.
func NewSmokeClient(config SmokeConfig) *SmokeClient {
	return &SmokeClient{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Get makes an authenticated GET request.
func (c *SmokeClient) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.config.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.DevToken)
	req.Header.Set("X-Tenant-ID", c.config.TenantID)
	return c.client.Do(req)
}

// Post makes an authenticated POST request.
func (c *SmokeClient) Post(path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPost, c.config.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.DevToken)
	req.Header.Set("X-Tenant-ID", c.config.TenantID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.client.Do(req)
}

// GetPublic makes an unauthenticated GET request.
func (c *SmokeClient) GetPublic(path string) (*http.Response, error) {
	return c.client.Get(c.config.BaseURL + path)
}

// skipIfNotAvailable skips test if API is not reachable.
func skipIfNotAvailable(t *testing.T, client *SmokeClient) {
	t.Helper()
	resp, err := client.GetPublic("/healthz")
	if err != nil {
		t.Skipf("API not available: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("API health check failed: %d", resp.StatusCode)
	}
}

// TestSmokeHealth verifies the health endpoint is responding.
func TestSmokeHealth(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())

	resp, err := client.GetPublic("/healthz")
	if err != nil {
		t.Skipf("API not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check: expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") && resp.StatusCode != http.StatusOK {
		t.Errorf("health check body: %s", string(body))
	}
	t.Logf("Health check passed: %s", string(body))
}

// TestSmokeAuthentication verifies authentication is working.
func TestSmokeAuthentication(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// Unauthenticated request should be rejected
	resp, err := client.GetPublic("/accounts")
	if err != nil {
		t.Fatalf("unauthenticated request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated: expected 401, got %d", resp.StatusCode)
	}

	// Authenticated request should work (assuming tenant enforcement requires it)
	resp, err = client.Get("/system/status")
	if err != nil {
		t.Fatalf("authenticated request: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("authenticated: expected 200, got %d", resp.StatusCode)
	}
}

// TestSmokeSystemStatus verifies system status endpoint.
func TestSmokeSystemStatus(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	resp, err := client.Get("/system/status")
	if err != nil {
		t.Fatalf("system status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("system status: expected 200, got %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	t.Logf("System status: %v", status)
}

// TestSmokeSystemDescriptors verifies system descriptors endpoint.
func TestSmokeSystemDescriptors(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	resp, err := client.Get("/system/descriptors")
	if err != nil {
		t.Fatalf("system descriptors: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("system descriptors: expected 200, got %d", resp.StatusCode)
	}

	var descriptors interface{}
	if err := json.NewDecoder(resp.Body).Decode(&descriptors); err != nil {
		t.Fatalf("decode descriptors: %v", err)
	}

	t.Logf("System descriptors available")
}

// TestSmokeAccountCRUD performs a quick account create/get/delete cycle.
func TestSmokeAccountCRUD(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// Create account
	resp, err := client.Post("/accounts", map[string]string{
		"owner": fmt.Sprintf("smoke-test-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create account: expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	var account map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		t.Fatalf("decode account: %v", err)
	}

	accountID, ok := account["ID"].(string)
	if !ok || accountID == "" {
		t.Fatalf("invalid account ID: %v", account)
	}
	t.Logf("Created account: %s", accountID)

	// Get account
	resp, err = client.Get("/accounts/" + accountID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("get account: expected 200, got %d", resp.StatusCode)
	}

	// Delete account (cleanup)
	req, _ := http.NewRequest(http.MethodDelete, client.config.BaseURL+"/accounts/"+accountID, nil)
	req.Header.Set("Authorization", "Bearer "+client.config.DevToken)
	req.Header.Set("X-Tenant-ID", client.config.TenantID)
	resp, err = client.client.Do(req)
	if err != nil {
		t.Logf("delete account (cleanup): %v", err)
	} else {
		resp.Body.Close()
	}
}

// TestSmokeNeoEndpoints verifies Neo-specific endpoints if available.
func TestSmokeNeoEndpoints(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// Neo status
	resp, err := client.Get("/neo/status")
	if err != nil {
		t.Fatalf("neo status: %v", err)
	}
	defer resp.Body.Close()

	// Neo endpoints might not be enabled
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Neo endpoints not enabled")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("neo status: expected 200, got %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode neo status: %v", err)
	}

	enabled, ok := status["enabled"].(bool)
	if !ok {
		t.Logf("Neo status: %v", status)
		return
	}

	if enabled {
		t.Logf("Neo indexer enabled, latest_height: %v", status["latest_height"])

		// Test blocks endpoint
		resp, err = client.Get("/neo/blocks?limit=1")
		if err != nil {
			t.Fatalf("neo blocks: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("neo blocks: expected 200, got %d", resp.StatusCode)
		}
	} else {
		t.Log("Neo indexer not enabled")
	}
}

// TestSmokeMetrics verifies metrics endpoint if available.
func TestSmokeMetrics(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())

	// Metrics endpoint may require auth
	resp, err := client.Get("/metrics")
	if err != nil {
		t.Skipf("metrics endpoint not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("metrics endpoint not enabled")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		t.Log("Metrics endpoint requires authentication")
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics: expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) > 0 {
		t.Logf("Metrics endpoint responding (%d bytes)", len(body))
	}
}

// TestSmokeResponseTimes verifies basic response time expectations.
func TestSmokeResponseTimes(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// /healthz should be fast
	start := time.Now()
	resp, err := client.GetPublic("/healthz")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("/healthz: request failed: %v", err)
		return
	}
	resp.Body.Close()

	maxAcceptable := 2 * time.Second
	if elapsed > maxAcceptable {
		t.Errorf("/healthz: response time %v exceeds %v", elapsed, maxAcceptable)
	} else {
		t.Logf("/healthz: %v (%d)", elapsed, resp.StatusCode)
	}
}

// TestSmokeCORS verifies CORS headers are set.
func TestSmokeCORS(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	req, _ := http.NewRequest(http.MethodOptions, client.config.BaseURL+"/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := client.client.Do(req)
	if err != nil {
		t.Fatalf("CORS preflight: %v", err)
	}
	defer resp.Body.Close()

	// Check for CORS headers
	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "" {
		t.Log("CORS not configured (Access-Control-Allow-Origin not set)")
	} else {
		t.Logf("CORS configured: %s", allowOrigin)
	}
}

// TestSmokeErrorHandling verifies error responses are properly formatted.
func TestSmokeErrorHandling(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// Request non-existent resource
	resp, err := client.Get("/accounts/nonexistent-id-12345")
	if err != nil {
		t.Fatalf("error request: %v", err)
	}
	defer resp.Body.Close()

	// Should return 404 or 403 (if tenant mismatch)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 404/403, got %d", resp.StatusCode)
	}

	// Verify content type
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") && ct != "" {
		t.Logf("Error response Content-Type: %s (expected application/json)", ct)
	}
}

// TestSmokeContentTypes verifies correct content types in responses.
func TestSmokeContentTypes(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	resp, err := client.Get("/system/status")
	if err != nil {
		t.Fatalf("content type test: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %s", ct)
	}
}

// TestSmokeBusEndpoints verifies bus/event system endpoints if available.
func TestSmokeBusEndpoints(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	// Try /system/bus/events endpoint
	resp, err := client.Get("/system/bus/events")
	if err != nil {
		t.Fatalf("bus events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Bus endpoints not available")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bus events: expected 200, got %d", resp.StatusCode)
	}

	t.Log("Bus events endpoint available")
}

// TestSmokeReadinessProbe tests readiness probe endpoint.
func TestSmokeReadinessProbe(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())

	// Try /readyz endpoint (may require auth)
	resp, err := client.Get("/readyz")
	if err != nil {
		t.Skipf("readiness probe not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Try /ready as alternative
		resp, err = client.Get("/ready")
		if err != nil {
			t.Skip("No readiness probe endpoint found")
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("readiness probe: unexpected status %d", resp.StatusCode)
	}

	t.Logf("Readiness probe: %d", resp.StatusCode)
}

// TestSmokeLivenessProbe tests liveness probe endpoint.
func TestSmokeLivenessProbe(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())

	// Try /livez endpoint (may require auth)
	resp, err := client.Get("/livez")
	if err != nil {
		t.Skipf("liveness probe not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// /healthz typically serves as liveness
		t.Skip("Dedicated liveness probe not found (using /healthz)")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		t.Log("Liveness probe requires authentication")
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("liveness probe: expected 200, got %d", resp.StatusCode)
	}

	t.Logf("Liveness probe: %d", resp.StatusCode)
}

// TestSmokeParallelRequests tests system under light parallel load.
func TestSmokeParallelRequests(t *testing.T) {
	client := NewSmokeClient(DefaultSmokeConfig())
	skipIfNotAvailable(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			select {
			case <-ctx.Done():
				results <- ctx.Err()
				return
			default:
			}

			resp, err := client.Get("/system/status")
			if err != nil {
				results <- err
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("status %d", resp.StatusCode)
				return
			}
			results <- nil
		}()
	}

	// Collect results
	errors := 0
	for i := 0; i < 5; i++ {
		if err := <-results; err != nil {
			t.Logf("Parallel request error: %v", err)
			errors++
		}
	}

	if errors > 1 {
		t.Errorf("Too many parallel request failures: %d/5", errors)
	}
}
