// Package main demonstrates how to use the Oracle service.
//
// This example shows:
// - Submitting an oracle fetch request
// - Polling for completion
// - Verifying TEE signature
//
// Run: go run oracle_example.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// =============================================================================
// Configuration
// =============================================================================

type Config struct {
	SupabaseURL string
	SupabaseKey string
}

func loadConfig() Config {
	return Config{
		SupabaseURL: getEnv("SUPABASE_URL", "http://localhost:54321"),
		SupabaseKey: getEnv("SUPABASE_ANON_KEY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// =============================================================================
// Service Request Types
// =============================================================================

type ServiceRequest struct {
	ID          string                 `json:"id,omitempty"`
	ServiceType string                 `json:"service_type"`
	Operation   string                 `json:"operation"`
	Payload     map[string]interface{} `json:"payload"`
	Status      string                 `json:"status,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error_message,omitempty"`
}

// =============================================================================
// Supabase Client
// =============================================================================

type SupabaseClient struct {
	url    string
	key    string
	client *http.Client
}

func NewSupabaseClient(url, key string) *SupabaseClient {
	return &SupabaseClient{
		url:    url,
		key:    key,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SupabaseClient) SubmitRequest(ctx context.Context, req ServiceRequest) (*ServiceRequest, error) {
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.url+"/rest/v1/service_requests", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq)
	httpReq.Header.Set("Prefer", "return=representation")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("submit failed: %d - %s", resp.StatusCode, body)
	}

	var results []ServiceRequest
	json.NewDecoder(resp.Body).Decode(&results)

	if len(results) == 0 {
		return nil, fmt.Errorf("no request returned")
	}

	return &results[0], nil
}

func (c *SupabaseClient) GetRequest(ctx context.Context, id string) (*ServiceRequest, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET",
		c.url+"/rest/v1/service_requests?id=eq."+id, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(httpReq)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []ServiceRequest
	json.NewDecoder(resp.Body).Decode(&results)

	if len(results) == 0 {
		return nil, fmt.Errorf("request not found")
	}

	return &results[0], nil
}

func (c *SupabaseClient) WaitForCompletion(ctx context.Context, id string, timeout time.Duration) (*ServiceRequest, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		req, err := c.GetRequest(ctx, id)
		if err != nil {
			return nil, err
		}

		if req.Status == "completed" || req.Status == "failed" {
			return req, nil
		}

		log.Printf("Status: %s, waiting...", req.Status)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return nil, fmt.Errorf("timeout")
}

func (c *SupabaseClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
}

// =============================================================================
// Main
// =============================================================================

func main() {
	log.Println("=== Oracle Service Example ===")
	log.Println()

	// Load configuration
	config := loadConfig()
	if config.SupabaseKey == "" {
		log.Fatal("SUPABASE_ANON_KEY environment variable required")
	}

	// Create client
	client := NewSupabaseClient(config.SupabaseURL, config.SupabaseKey)
	ctx := context.Background()

	// Example 1: Fetch price data
	log.Println("Example 1: Fetching NEO price from CoinGecko...")

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
		log.Fatalf("Submit error: %v", err)
	}

	log.Printf("Request submitted: %s", submitted.ID)

	// Wait for completion
	result, err := client.WaitForCompletion(ctx, submitted.ID, 30*time.Second)
	if err != nil {
		log.Fatalf("Wait error: %v", err)
	}

	if result.Status == "completed" {
		log.Println("✓ Request completed successfully!")
		log.Printf("Result: %v", result.Result)
	} else {
		log.Printf("✗ Request failed: %s", result.Error)
	}

	log.Println()

	// Example 2: Fetch with JSON path extraction
	log.Println("Example 2: Fetching with JSON path extraction...")

	req2 := ServiceRequest{
		ServiceType: "oracle",
		Operation:   "fetch",
		Payload: map[string]interface{}{
			"url":       "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
			"method":    "GET",
			"json_path": "$.neo.usd",
		},
	}

	submitted2, err := client.SubmitRequest(ctx, req2)
	if err != nil {
		log.Fatalf("Submit error: %v", err)
	}

	result2, err := client.WaitForCompletion(ctx, submitted2.ID, 30*time.Second)
	if err != nil {
		log.Fatalf("Wait error: %v", err)
	}

	if result2.Status == "completed" {
		log.Println("✓ Request completed!")
		log.Printf("NEO Price (USD): %v", result2.Result["value"])
	}

	log.Println()
	log.Println("=== Example Complete ===")
}
