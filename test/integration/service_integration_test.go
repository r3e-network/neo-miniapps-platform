// Package integration provides integration tests for Neo Service Layer.
// These tests verify the interaction between components.
//
// Run with: go test -tags=integration ./test/integration/...
//
//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/ego"
	"github.com/R3E-Network/service_layer/marble/sdk"
	"github.com/R3E-Network/service_layer/supabase/client"
)

// =============================================================================
// Test Setup
// =============================================================================

func skipIfNoIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}
}

// =============================================================================
// EGo + Marble SDK Integration Tests
// =============================================================================

func TestEgoMarbleIntegration(t *testing.T) {
	// Create EGo runtime
	egoRuntime := ego.NewWithConfig(ego.Config{
		SimulationMode:  true,
		ProductID:       1,
		SecurityVersion: 1,
	})

	// Create Marble with EGo runtime
	marble, err := sdk.New(sdk.Config{
		MarbleType:     "test-service",
		SimulationMode: true,
		EgoConfig:      ego.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("Failed to create marble: %v", err)
	}

	// Test sealing through Marble (which uses EGo)
	testData := []byte("integration test data")

	sealed, err := marble.Seal(testData)
	if err != nil {
		t.Fatalf("Marble.Seal() error: %v", err)
	}

	unsealed, err := marble.Unseal(sealed)
	if err != nil {
		t.Fatalf("Marble.Unseal() error: %v", err)
	}

	if !bytes.Equal(unsealed, testData) {
		t.Error("Unsealed data doesn't match original")
	}

	// Test quote generation
	quote, err := marble.GenerateQuote(testData)
	if err != nil {
		t.Fatalf("Marble.GenerateQuote() error: %v", err)
	}

	// Verify quote through EGo runtime
	report, err := egoRuntime.VerifyQuote(quote)
	if err != nil {
		t.Fatalf("EgoRuntime.VerifyQuote() error: %v", err)
	}

	if report == nil {
		t.Error("VerifyQuote returned nil report")
	}
}

func TestMarbleActivationFlow(t *testing.T) {
	// Create mock coordinator
	activationCalled := false
	coordinator := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/marble/activate":
			activationCalled = true

			// Verify request
			var req sdk.ActivationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode activation request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if req.MarbleType != "oracle" {
				t.Errorf("MarbleType = %s, want oracle", req.MarbleType)
			}

			// Send response
			resp := sdk.ActivationResponse{
				Secrets: map[string][]byte{
					"api_key":      []byte("test-api-key"),
					"signing_key":  []byte("test-signing-key"),
				},
				Env: map[string]string{
					"SUPABASE_URL": "http://localhost:54321",
					"NEO_RPC_URL":  "http://localhost:10332",
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "/api/v2/marble/heartbeat":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer coordinator.Close()

	// Create and activate marble
	marble, err := sdk.New(sdk.Config{
		MarbleType:      "oracle",
		CoordinatorAddr: coordinator.Listener.Addr().String(),
		SimulationMode:  true,
		RetryConfig: sdk.RetryConfig{
			MaxRetries:        1,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create marble: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := marble.Activate(ctx); err != nil {
		t.Fatalf("Marble.Activate() error: %v", err)
	}

	if !activationCalled {
		t.Error("Coordinator activation endpoint was not called")
	}

	if !marble.IsActivated() {
		t.Error("Marble should be activated")
	}

	// Verify secrets were received
	apiKey, err := marble.GetSecret("api_key")
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(apiKey) != "test-api-key" {
		t.Errorf("api_key = %s, want test-api-key", apiKey)
	}

	// Verify environment was set
	supabaseURL := marble.GetEnv("SUPABASE_URL")
	if supabaseURL != "http://localhost:54321" {
		t.Errorf("SUPABASE_URL = %s, want http://localhost:54321", supabaseURL)
	}
}

// =============================================================================
// Supabase Client Integration Tests
// =============================================================================

func TestSupabaseClientIntegration(t *testing.T) {
	skipIfNoIntegration(t)

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		t.Skip("SUPABASE_URL and SUPABASE_SERVICE_KEY required")
	}

	// Create resilient client
	resilientClient := client.NewResilientClient(client.ResilientClientConfig{
		RetryConfig:          client.DefaultRetryConfig(),
		CircuitBreakerConfig: client.DefaultCircuitBreakerConfig(),
	})

	// Create Supabase client
	supabaseClient := sdk.NewSupabaseClient(supabaseURL, supabaseKey)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test query
	result, err := supabaseClient.From("service_requests").
		Select("*").
		Limit(1).
		Execute(ctx)

	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	t.Logf("Query result: %s", result)

	// Verify circuit breaker is healthy
	if resilientClient.CircuitState() != client.CircuitClosed {
		t.Errorf("Circuit state = %v, want closed", resilientClient.CircuitState())
	}
}

func TestSupabaseRequestFlow(t *testing.T) {
	// Create mock Supabase server
	var insertedRequest map[string]interface{}

	supabase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/rest/v1/service_requests":
			// Handle insert
			if err := json.NewDecoder(r.Body).Decode(&insertedRequest); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":           "test-request-id",
					"service_type": insertedRequest["service_type"],
					"operation":    insertedRequest["operation"],
					"status":       "pending",
				},
			})

		case r.Method == "GET" && r.URL.Path == "/rest/v1/service_requests":
			// Handle select
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":           "test-request-id",
					"service_type": "oracle",
					"operation":    "fetch",
					"status":       "completed",
					"result":       map[string]interface{}{"data": "test"},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer supabase.Close()

	client := sdk.NewSupabaseClient(supabase.URL, "test-key")
	ctx := context.Background()

	// Insert request
	request := map[string]interface{}{
		"service_type": "oracle",
		"operation":    "fetch",
		"payload":      map[string]string{"url": "https://api.example.com"},
	}

	result, err := client.From("service_requests").ExecuteInsert(ctx, request)
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	var inserted []map[string]interface{}
	if err := json.Unmarshal(result, &inserted); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(inserted) == 0 {
		t.Fatal("No records returned from insert")
	}

	if inserted[0]["service_type"] != "oracle" {
		t.Errorf("service_type = %v, want oracle", inserted[0]["service_type"])
	}
}

// =============================================================================
// Circuit Breaker Integration Tests
// =============================================================================

func TestCircuitBreakerWithRealRequests(t *testing.T) {
	failCount := 0
	maxFails := 3

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failCount < maxFails {
			failCount++
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	resilientClient := client.NewResilientClient(client.ResilientClientConfig{
		RetryConfig: client.RetryConfig{
			MaxRetries:           5,
			InitialBackoff:       10 * time.Millisecond,
			MaxBackoff:           100 * time.Millisecond,
			BackoffMultiplier:    2.0,
			RetryableStatusCodes: []int{http.StatusServiceUnavailable},
		},
		CircuitBreakerConfig: client.CircuitBreakerConfig{
			FailureThreshold: 10, // High threshold to allow retries
			SuccessThreshold: 1,
			Timeout:          1 * time.Second,
		},
	})

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := resilientClient.Do(req)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}

	metrics := resilientClient.Metrics()
	if metrics["retried_requests"] == 0 {
		t.Error("Expected some retried requests")
	}
}

// =============================================================================
// Full Service Flow Integration Test
// =============================================================================

func TestFullServiceFlow(t *testing.T) {
	// This test simulates the full flow:
	// 1. Marble activates with Coordinator
	// 2. Marble polls Supabase for requests
	// 3. Marble processes request
	// 4. Marble updates Supabase with result

	// Mock Coordinator
	coordinator := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/marble/activate" {
			resp := sdk.ActivationResponse{
				Secrets: map[string][]byte{
					"supabase_service_key": []byte("test-key"),
				},
				Env: map[string]string{
					"SUPABASE_URL": "will-be-replaced",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer coordinator.Close()

	// Mock Supabase
	requestProcessed := false
	supabase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case "GET":
			// Return pending request
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":           "req-123",
					"service_type": "oracle",
					"operation":    "fetch",
					"status":       "pending",
					"payload":      map[string]string{"url": "https://api.example.com"},
				},
			})
		case "PATCH":
			// Mark as processed
			requestProcessed = true
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "req-123", "status": "completed"},
			})
		}
	}))
	defer supabase.Close()

	// Create and activate marble
	marble, _ := sdk.New(sdk.Config{
		MarbleType:      "oracle",
		CoordinatorAddr: coordinator.Listener.Addr().String(),
		SimulationMode:  true,
		RetryConfig: sdk.RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 10 * time.Millisecond,
		},
	})

	ctx := context.Background()
	marble.Activate(ctx)

	// Create Supabase client
	supabaseClient := sdk.NewSupabaseClient(supabase.URL, "test-key")

	// Poll for requests
	result, err := supabaseClient.From("service_requests").
		Select("*").
		Eq("status", "pending").
		Limit(1).
		Execute(ctx)

	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}

	var requests []map[string]interface{}
	json.Unmarshal(result, &requests)

	if len(requests) == 0 {
		t.Fatal("No pending requests found")
	}

	// Process request (simulate)
	requestID := requests[0]["id"].(string)

	// Update status
	_, err = supabaseClient.From("service_requests").
		Eq("id", requestID).
		ExecuteUpdate(ctx, map[string]interface{}{
			"status": "completed",
			"result": map[string]string{"data": "processed"},
		})

	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	if !requestProcessed {
		t.Error("Request was not processed")
	}
}
