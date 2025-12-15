// Package client provides a client for interacting with the GlobalSigner service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	slhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

// Client is a client for the GlobalSigner service.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	serviceID    string
	maxBodyBytes int64
}

// Config holds client configuration.
type Config struct {
	BaseURL string
	// ServiceID identifies the caller. In strict identity mode this is redundant
	// (caller identity is enforced by MarbleRun mTLS), but it's still useful for
	// local development and debugging.
	ServiceID string
	Timeout   time.Duration
	// HTTPClient optionally overrides the client used to execute requests.
	// For MarbleRun mesh calls, prefer using `marble.Marble.HTTPClient()` so
	// requests are sent over verified mTLS.
	HTTPClient *http.Client
	// MaxBodyBytes caps responses to prevent memory exhaustion.
	MaxBodyBytes int64
}

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxBodySize = 1 << 20 // 1MiB
)

// New creates a new GlobalSigner client.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	baseURL, _, err := slhttputil.NormalizeServiceBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("globalsigner: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else {
		// Avoid mutating a caller-supplied client.
		copied := *client
		if copied.Timeout == 0 || cfg.Timeout != 0 {
			copied.Timeout = timeout
		}
		client = &copied
	}

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodySize
	}

	return &Client{
		baseURL:      baseURL,
		serviceID:    strings.TrimSpace(cfg.ServiceID),
		httpClient:   client,
		maxBodyBytes: maxBodyBytes,
	}, nil
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
	if c == nil {
		return nil, fmt.Errorf("globalsigner: client is nil")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("globalsigner: http client not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sign", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.serviceID != "" {
		httpReq.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("request failed: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return nil, fmt.Errorf("request failed: %s - %s", resp.Status, msg)
		}
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	respBody, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result SignResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// Derive performs deterministic key derivation.
func (c *Client) Derive(ctx context.Context, req *DeriveRequest) (*DeriveResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("globalsigner: client is nil")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("globalsigner: http client not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/derive", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.serviceID != "" {
		httpReq.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("request failed: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return nil, fmt.Errorf("request failed: %s - %s", resp.Status, msg)
		}
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result DeriveResponse
	if body, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	} else if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetAttestation gets the attestation for the active key.
func (c *Client) GetAttestation(ctx context.Context) (*AttestationResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("globalsigner: client is nil")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("globalsigner: http client not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/attestation", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.serviceID != "" {
		httpReq.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("request failed: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return nil, fmt.Errorf("request failed: %s - %s", resp.Status, msg)
		}
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result AttestationResponse
	if body, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	} else if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListKeys lists all key versions.
func (c *Client) ListKeys(ctx context.Context) ([]KeyVersion, error) {
	if c == nil {
		return nil, fmt.Errorf("globalsigner: client is nil")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("globalsigner: http client not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/keys", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.serviceID != "" {
		httpReq.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return nil, fmt.Errorf("request failed: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return nil, fmt.Errorf("request failed: %s - %s", resp.Status, msg)
		}
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result struct {
		ActiveVersion string       `json:"active_version"`
		KeyVersions   []KeyVersion `json:"key_versions"`
	}
	if body, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	} else if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.KeyVersions, nil
}

// Health checks if GlobalSigner is healthy.
func (c *Client) Health(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("globalsigner: client is nil")
	}
	if c.httpClient == nil {
		return fmt.Errorf("globalsigner: http client not configured")
	}

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
