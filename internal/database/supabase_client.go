// Package database provides Supabase database integration.
package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Client wraps the Supabase REST API client.
type Client struct {
	mu         sync.RWMutex
	url        string
	serviceKey string
	httpClient *http.Client
}

// Config holds database configuration.
type Config struct {
	URL        string
	ServiceKey string
}

// NewClient creates a new Supabase client.
func NewClient(cfg Config) (*Client, error) {
	url := cfg.URL
	if url == "" {
		url = os.Getenv("SUPABASE_URL")
	}

	key := cfg.ServiceKey
	if key == "" {
		key = os.Getenv("SUPABASE_SERVICE_KEY")
	}

	if url == "" {
		// Allow running without database in development/testing mode
		env := os.Getenv("MARBLE_ENV")
		if env == "development" || env == "testing" {
			url = "http://localhost:54321" // Mock URL for development
		} else {
			return nil, fmt.Errorf("SUPABASE_URL is required")
		}
	}

	return &Client{
		url:        url,
		serviceKey: key,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// request makes an HTTP request to the Supabase REST API.
func (c *Client) request(ctx context.Context, method, table string, body interface{}, query string) ([]byte, error) {
	url := fmt.Sprintf("%s/rest/v1/%s", c.url, table)
	if query != "" {
		url += "?" + query
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
