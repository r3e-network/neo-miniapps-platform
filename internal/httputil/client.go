// Package httputil provides HTTP client utilities for service-to-service communication.
package httputil

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

// =============================================================================
// Service Client
// =============================================================================

// ServiceClient provides authenticated HTTP client for service-to-service calls.
// It automatically attaches service tokens and user IDs to requests.
type ServiceClient struct {
	httpClient     *http.Client
	tokenGenerator *serviceauth.ServiceTokenGenerator
	baseURL        string
	maxRetries     int
}

// ServiceClientConfig configures the service client.
type ServiceClientConfig struct {
	PrivateKey *rsa.PrivateKey
	ServiceID  string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// NewServiceClient creates a new authenticated service client.
func NewServiceClient(cfg ServiceClientConfig) *ServiceClient {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}

	var tokenGen *serviceauth.ServiceTokenGenerator
	if cfg.PrivateKey != nil && cfg.ServiceID != "" {
		tokenGen = serviceauth.NewServiceTokenGenerator(cfg.PrivateKey, cfg.ServiceID, time.Hour)
	}

	return &ServiceClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		tokenGenerator: tokenGen,
		baseURL:        cfg.BaseURL,
		maxRetries:     maxRetries,
	}
}

// Do executes an HTTP request with automatic service authentication.
// It attaches X-Service-Token and X-User-ID headers from context.
func (c *ServiceClient) Do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.doWithRetry(ctx, method, path, body, 0)
}

// doWithRetry executes request with retry logic for transient auth failures.
func (c *ServiceClient) doWithRetry(ctx context.Context, method, path string, body interface{}, attempt int) (*http.Response, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type for JSON requests
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Attach service token
	if c.tokenGenerator != nil {
		token, tokenErr := c.tokenGenerator.GenerateToken()
		if tokenErr != nil {
			return nil, fmt.Errorf("failed to generate service token: %w", tokenErr)
		}
		req.Header.Set(serviceauth.ServiceTokenHeader, token)
	}

	// Attach user ID from context
	if userID := serviceauth.GetUserID(ctx); userID != "" {
		req.Header.Set(serviceauth.UserIDHeader, userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Retry on transient auth failures
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && attempt < c.maxRetries {
		resp.Body.Close()
		return c.doWithRetry(ctx, method, path, body, attempt+1)
	}

	return resp, nil
}

// Get performs a GET request.
func (c *ServiceClient) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request with JSON body.
func (c *ServiceClient) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, path, body)
}

// Put performs a PUT request with JSON body.
func (c *ServiceClient) Put(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, path, body)
}

// Delete performs a DELETE request.
func (c *ServiceClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, path, nil)
}

// DecodeResponse decodes a JSON response into the target struct.
func DecodeResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, truncated, err := ReadAllWithLimit(resp.Body, 64<<10)
		if err != nil {
			return fmt.Errorf("read error response body: %w", err)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, msg)
	}

	if target == nil {
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<20)); err != nil {
			return fmt.Errorf("discard response body: %w", err)
		}
		return nil
	}

	body, err := ReadAllStrict(resp.Body, 8<<20)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
