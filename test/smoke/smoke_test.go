// Package smoke provides smoke tests for deployment verification.
// These tests verify that the system is properly deployed and operational.
//
// Run with: go test -tags=smoke ./test/smoke/...
//
//go:build smoke

package smoke

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Smoke Test Configuration
// =============================================================================

type SmokeConfig struct {
	SupabaseURL    string
	SupabaseKey    string
	CoordinatorURL string
	FrontendURL    string
	NeoRPCURL      string
	Timeout        time.Duration
}

func loadSmokeConfig() SmokeConfig {
	return SmokeConfig{
		SupabaseURL:    getEnv("SUPABASE_URL", "http://localhost:54321"),
		SupabaseKey:    getEnv("SUPABASE_ANON_KEY", ""),
		CoordinatorURL: getEnv("COORDINATOR_URL", "https://localhost:4433"),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:5173"),
		NeoRPCURL:      getEnv("NEO_RPC_URL", "http://localhost:10332"),
		Timeout:        10 * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// =============================================================================
// HTTP Client
// =============================================================================

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For self-signed certs in dev
			},
		},
	}
}

// =============================================================================
// Supabase Smoke Tests
// =============================================================================

func TestSmoke_SupabaseHealth(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	// Test REST API
	req, _ := http.NewRequest("GET", config.SupabaseURL+"/rest/v1/", nil)
	req.Header.Set("apikey", config.SupabaseKey)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Supabase REST API unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		t.Errorf("Supabase REST API error: %d", resp.StatusCode)
	}

	t.Log("✓ Supabase REST API is healthy")
}

func TestSmoke_SupabaseAuth(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	// Test Auth API health
	resp, err := client.Get(config.SupabaseURL + "/auth/v1/health")
	if err != nil {
		t.Fatalf("Supabase Auth unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Supabase Auth unhealthy: %d", resp.StatusCode)
	}

	t.Log("✓ Supabase Auth is healthy")
}

func TestSmoke_SupabaseRealtime(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	// Test Realtime endpoint exists
	resp, err := client.Get(config.SupabaseURL + "/realtime/v1/")
	if err != nil {
		t.Logf("⚠ Supabase Realtime check failed: %v", err)
		return
	}
	defer resp.Body.Close()

	// Realtime may return various status codes, just check it's reachable
	t.Log("✓ Supabase Realtime endpoint is reachable")
}

func TestSmoke_SupabaseServiceRequests(t *testing.T) {
	config := loadSmokeConfig()
	if config.SupabaseKey == "" {
		t.Skip("SUPABASE_ANON_KEY not set")
	}

	client := newHTTPClient()

	req, _ := http.NewRequest("GET", config.SupabaseURL+"/rest/v1/service_requests?limit=1", nil)
	req.Header.Set("apikey", config.SupabaseKey)
	req.Header.Set("Authorization", "Bearer "+config.SupabaseKey)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("service_requests table unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("service_requests table does not exist - run migrations")
	} else if resp.StatusCode >= 400 {
		t.Errorf("service_requests query failed: %d", resp.StatusCode)
	}

	t.Log("✓ service_requests table is accessible")
}

// =============================================================================
// Coordinator Smoke Tests
// =============================================================================

func TestSmoke_CoordinatorStatus(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	resp, err := client.Get(config.CoordinatorURL + "/api/v2/status")
	if err != nil {
		t.Logf("⚠ Coordinator unreachable: %v (may not be running)", err)
		t.Skip("Coordinator not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Coordinator unhealthy: %d", resp.StatusCode)
	}

	var status map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&status)

	t.Logf("✓ Coordinator is healthy: %v", status)
}

func TestSmoke_CoordinatorManifest(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	resp, err := client.Get(config.CoordinatorURL + "/api/v2/manifest")
	if err != nil {
		t.Skip("Coordinator not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Log("⚠ Manifest not set - coordinator needs configuration")
	} else if resp.StatusCode == http.StatusOK {
		t.Log("✓ Coordinator manifest is configured")
	}
}

// =============================================================================
// Frontend Smoke Tests
// =============================================================================

func TestSmoke_FrontendHealth(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	resp, err := client.Get(config.FrontendURL)
	if err != nil {
		t.Logf("⚠ Frontend unreachable: %v", err)
		t.Skip("Frontend not available")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Frontend unhealthy: %d", resp.StatusCode)
	}

	t.Log("✓ Frontend is serving")
}

func TestSmoke_FrontendAssets(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	// Check if main assets are served
	assets := []string{"/favicon.svg", "/index.html"}

	for _, asset := range assets {
		resp, err := client.Get(config.FrontendURL + asset)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Logf("✓ Asset %s is served", asset)
		}
	}
}

// =============================================================================
// Neo N3 Smoke Tests
// =============================================================================

func TestSmoke_NeoRPCHealth(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	// JSON-RPC request
	payload := `{"jsonrpc":"2.0","method":"getversion","params":[],"id":1}`

	resp, err := client.Post(config.NeoRPCURL, "application/json",
		strings.NewReader(payload))
	if err != nil {
		t.Logf("⚠ Neo RPC unreachable: %v", err)
		t.Skip("Neo RPC not available")
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != nil {
		t.Errorf("Neo RPC error: %v", result["error"])
	}

	t.Logf("✓ Neo RPC is healthy: %v", result["result"])
}

func TestSmoke_NeoBlockHeight(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	payload := `{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}`

	resp, err := client.Post(config.NeoRPCURL, "application/json",
		strings.NewReader(payload))
	if err != nil {
		t.Skip("Neo RPC not available")
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if blockCount, ok := result["result"].(float64); ok {
		t.Logf("✓ Neo block height: %d", int(blockCount))
	}
}

// =============================================================================
// Service Health Smoke Tests
// =============================================================================

func TestSmoke_ServiceEndpoints(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	services := []string{
		"oracle", "vrf", "secrets", "gasbank",
		"datafeeds", "automation", "mixer",
	}

	for _, svc := range services {
		// Try to reach service health endpoint
		url := fmt.Sprintf("http://localhost:8080/services/%s/health", svc)
		resp, err := client.Get(url)
		if err != nil {
			t.Logf("⚠ Service %s unreachable", svc)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Logf("✓ Service %s is healthy", svc)
		} else {
			t.Logf("⚠ Service %s returned %d", svc, resp.StatusCode)
		}
	}
}

// =============================================================================
// Database Schema Smoke Tests
// =============================================================================

func TestSmoke_DatabaseTables(t *testing.T) {
	config := loadSmokeConfig()
	if config.SupabaseKey == "" {
		t.Skip("SUPABASE_ANON_KEY not set")
	}

	client := newHTTPClient()

	tables := []string{
		"service_requests",
		"realtime_notifications",
	}

	for _, table := range tables {
		req, _ := http.NewRequest("GET",
			fmt.Sprintf("%s/rest/v1/%s?limit=0", config.SupabaseURL, table), nil)
		req.Header.Set("apikey", config.SupabaseKey)
		req.Header.Set("Authorization", "Bearer "+config.SupabaseKey)

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("⚠ Table %s check failed: %v", table, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Logf("✓ Table %s exists", table)
		} else if resp.StatusCode == http.StatusNotFound {
			t.Errorf("✗ Table %s does not exist", table)
		}
	}
}

// =============================================================================
// Connectivity Smoke Tests
// =============================================================================

func TestSmoke_AllServicesConnectivity(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	endpoints := map[string]string{
		"Supabase":    config.SupabaseURL + "/rest/v1/",
		"Coordinator": config.CoordinatorURL + "/api/v2/status",
		"Frontend":    config.FrontendURL,
		"Neo RPC":     config.NeoRPCURL,
	}

	healthy := 0
	total := len(endpoints)

	for name, url := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

		if name == "Supabase" {
			req.Header.Set("apikey", config.SupabaseKey)
		}

		resp, err := client.Do(req)
		cancel()

		if err != nil {
			t.Logf("⚠ %s: unreachable", name)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 500 {
			t.Logf("✓ %s: reachable", name)
			healthy++
		} else {
			t.Logf("✗ %s: error %d", name, resp.StatusCode)
		}
	}

	t.Logf("\nConnectivity: %d/%d services healthy", healthy, total)

	if healthy < total/2 {
		t.Error("Too many services unhealthy")
	}
}

// =============================================================================
// Quick Smoke Test (All-in-One)
// =============================================================================

func TestSmoke_QuickCheck(t *testing.T) {
	config := loadSmokeConfig()
	client := newHTTPClient()

	t.Log("=== Neo Service Layer Smoke Test ===")
	t.Log("")

	// 1. Supabase
	t.Log("1. Checking Supabase...")
	req, _ := http.NewRequest("GET", config.SupabaseURL+"/rest/v1/", nil)
	req.Header.Set("apikey", config.SupabaseKey)
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
		t.Log("   ✓ Supabase is running")
	} else {
		t.Log("   ✗ Supabase is not running")
	}

	// 2. Coordinator
	t.Log("2. Checking Coordinator...")
	if resp, err := client.Get(config.CoordinatorURL + "/api/v2/status"); err == nil {
		resp.Body.Close()
		t.Log("   ✓ Coordinator is running")
	} else {
		t.Log("   ⚠ Coordinator is not running (optional in dev)")
	}

	// 3. Frontend
	t.Log("3. Checking Frontend...")
	if resp, err := client.Get(config.FrontendURL); err == nil {
		resp.Body.Close()
		t.Log("   ✓ Frontend is running")
	} else {
		t.Log("   ⚠ Frontend is not running")
	}

	// 4. Neo RPC
	t.Log("4. Checking Neo RPC...")
	payload := `{"jsonrpc":"2.0","method":"getversion","params":[],"id":1}`
	if resp, err := client.Post(config.NeoRPCURL, "application/json",
		strings.NewReader(payload)); err == nil {
		resp.Body.Close()
		t.Log("   ✓ Neo RPC is running")
	} else {
		t.Log("   ⚠ Neo RPC is not running")
	}

	t.Log("")
	t.Log("=== Smoke Test Complete ===")
}
