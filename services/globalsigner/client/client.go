// Package client provides a client for interacting with the GlobalSigner service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a client for the GlobalSigner service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	serviceID  string
}

// Config holds client configuration.
type Config struct {
	BaseURL   string
	ServiceID string
	Timeout   time.Duration
}

// New creates a new GlobalSigner client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL:   cfg.BaseURL,
		serviceID: cfg.ServiceID,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// =============================================================================
// Request/Response Types
// =============================================================================

// SignRequest is a request for domain-separated signing.
type SignRequest struct {
	Domain     string `json:"domain"`
	Data       string `json:"data"` // hex-encoded
	KeyVersion string `json:"key_version,omitempty"`
}

// SignResponse is the response from signing.
type SignResponse struct {
	Signature  string `json:"signature"` // hex-encoded
	KeyVersion string `json:"key_version"`
	PubKeyHex  string `json:"pubkey_hex"`
}

// DeriveRequest is a request for key derivation.
type DeriveRequest struct {
	Domain     string `json:"domain"`
	Path       string `json:"path"`
	KeyVersion string `json:"key_version,omitempty"`
}

// DeriveResponse is the response from key derivation.
type DeriveResponse struct {
	PubKeyHex  string `json:"pubkey_hex"`
	KeyVersion string `json:"key_version"`
}

// AttestationResponse is the attestation for a key.
type AttestationResponse struct {
	KeyVersion string `json:"key_version"`
	PubKeyHex  string `json:"pubkey_hex"`
	PubKeyHash string `json:"pubkey_hash"`
	Quote      string `json:"quote,omitempty"`
	MRENCLAVE  string `json:"mrenclave,omitempty"`
	MRSIGNER   string `json:"mrsigner,omitempty"`
	ProdID     uint16 `json:"prod_id,omitempty"`
	ISVSVN     uint16 `json:"isvsvn,omitempty"`
	Timestamp  string `json:"timestamp"`
	Simulated  bool   `json:"simulated"`
}

// KeyVersion represents a key version.
type KeyVersion struct {
	Version       string     `json:"version"`
	Status        string     `json:"status"`
	PubKeyHex     string     `json:"pubkey_hex"`
	PubKeyHash    string     `json:"pubkey_hash"`
	CreatedAt     time.Time  `json:"created_at"`
	ActivatedAt   *time.Time `json:"activated_at,omitempty"`
	OverlapEndsAt *time.Time `json:"overlap_ends_at,omitempty"`
}

// =============================================================================
// API Methods
// =============================================================================

// Sign performs domain-separated signing.
func (c *Client) Sign(ctx context.Context, req *SignRequest) (*SignResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sign", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s - %s", resp.Status, string(respBody))
	}

	var result SignResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// Derive performs deterministic key derivation.
func (c *Client) Derive(ctx context.Context, req *DeriveRequest) (*DeriveResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/derive", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result DeriveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetAttestation gets the attestation for the active key.
func (c *Client) GetAttestation(ctx context.Context) (*AttestationResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/attestation", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result AttestationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListKeys lists all key versions.
func (c *Client) ListKeys(ctx context.Context) ([]KeyVersion, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/keys", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result struct {
		ActiveVersion string       `json:"active_version"`
		KeyVersions   []KeyVersion `json:"key_versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.KeyVersions, nil
}

// Health checks if GlobalSigner is healthy.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}

	return nil
}
