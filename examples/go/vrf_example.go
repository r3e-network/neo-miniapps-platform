// Package main demonstrates how to use the VRF (Verifiable Random Function) service.
//
// This example shows:
// - Generating verifiable random numbers
// - Verifying VRF proofs
// - Using randomness for fair selection
//
// Run: go run vrf_example.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

// =============================================================================
// Configuration
// =============================================================================

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// =============================================================================
// Types
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
// Client
// =============================================================================

type Client struct {
	url    string
	key    string
	client *http.Client
}

func NewClient(url, key string) *Client {
	return &Client{
		url:    url,
		key:    key,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Submit(ctx context.Context, req ServiceRequest) (*ServiceRequest, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST",
		c.url+"/rest/v1/service_requests", bytes.NewReader(body))

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", c.key)
	httpReq.Header.Set("Authorization", "Bearer "+c.key)
	httpReq.Header.Set("Prefer", "return=representation")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed: %s", body)
	}

	var results []ServiceRequest
	json.NewDecoder(resp.Body).Decode(&results)
	return &results[0], nil
}

func (c *Client) Wait(ctx context.Context, id string) (*ServiceRequest, error) {
	for i := 0; i < 30; i++ {
		httpReq, _ := http.NewRequestWithContext(ctx, "GET",
			c.url+"/rest/v1/service_requests?id=eq."+id, nil)
		httpReq.Header.Set("apikey", c.key)
		httpReq.Header.Set("Authorization", "Bearer "+c.key)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, err
		}

		var results []ServiceRequest
		json.NewDecoder(resp.Body).Decode(&results)
		resp.Body.Close()

		if len(results) > 0 && (results[0].Status == "completed" || results[0].Status == "failed") {
			return &results[0], nil
		}

		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("timeout")
}

// =============================================================================
// Main
// =============================================================================

func main() {
	log.Println("=== VRF Service Example ===")
	log.Println()

	url := getEnv("SUPABASE_URL", "http://localhost:54321")
	key := getEnv("SUPABASE_ANON_KEY", "")
	if key == "" {
		log.Fatal("SUPABASE_ANON_KEY required")
	}

	client := NewClient(url, key)
	ctx := context.Background()

	// Example 1: Generate random number
	log.Println("Example 1: Generating verifiable random number...")

	req := ServiceRequest{
		ServiceType: "vrf",
		Operation:   "random",
		Payload: map[string]interface{}{
			"seed": "lottery-round-12345",
		},
	}

	submitted, err := client.Submit(ctx, req)
	if err != nil {
		log.Fatalf("Submit error: %v", err)
	}

	log.Printf("Request ID: %s", submitted.ID)

	result, err := client.Wait(ctx, submitted.ID)
	if err != nil {
		log.Fatalf("Wait error: %v", err)
	}

	if result.Status == "completed" {
		log.Println("✓ Random number generated!")
		log.Printf("  Randomness: %v", result.Result["randomness"])
		log.Printf("  Proof: %v", result.Result["proof"])
	} else {
		log.Printf("✗ Failed: %s", result.Error)
	}

	log.Println()

	// Example 2: Fair lottery selection
	log.Println("Example 2: Fair lottery winner selection...")

	participants := []string{"Alice", "Bob", "Charlie", "Diana", "Eve"}
	log.Printf("Participants: %v", participants)

	req2 := ServiceRequest{
		ServiceType: "vrf",
		Operation:   "random",
		Payload: map[string]interface{}{
			"seed": fmt.Sprintf("lottery-%d", time.Now().UnixNano()),
		},
	}

	submitted2, _ := client.Submit(ctx, req2)
	result2, _ := client.Wait(ctx, submitted2.ID)

	if result2.Status == "completed" {
		// Use randomness to select winner
		if randomHex, ok := result2.Result["randomness"].(string); ok {
			randomBig := new(big.Int)
			randomBig.SetString(randomHex, 16)

			winnerIndex := new(big.Int).Mod(randomBig, big.NewInt(int64(len(participants))))
			winner := participants[winnerIndex.Int64()]

			log.Printf("✓ Winner: %s (index %d)", winner, winnerIndex.Int64())
			log.Printf("  Proof available for verification")
		}
	}

	log.Println()

	// Example 3: Multiple random values
	log.Println("Example 3: Generating multiple random values...")

	req3 := ServiceRequest{
		ServiceType: "vrf",
		Operation:   "random",
		Payload: map[string]interface{}{
			"seed":       "multi-random-seed",
			"num_values": 5,
		},
	}

	submitted3, _ := client.Submit(ctx, req3)
	result3, _ := client.Wait(ctx, submitted3.ID)

	if result3.Status == "completed" {
		log.Println("✓ Multiple random values generated!")
		if values, ok := result3.Result["values"].([]interface{}); ok {
			for i, v := range values {
				log.Printf("  Value %d: %v", i+1, v)
			}
		}
	}

	log.Println()
	log.Println("=== Example Complete ===")
}
