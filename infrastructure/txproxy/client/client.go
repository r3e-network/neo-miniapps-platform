// Package client provides an HTTP client for the TxProxy service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	slhttputil "github.com/R3E-Network/service_layer/infrastructure/httputil"
	"github.com/R3E-Network/service_layer/infrastructure/serviceauth"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"
)

// Client is an HTTP client for interacting with TxProxy over the MarbleRun mesh.
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
	// (caller identity is enforced by MarbleRun mTLS), but it is still useful for
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

// New creates a new TxProxy client.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	forceTimeout := cfg.Timeout != 0

	baseURL, _, err := slhttputil.NormalizeServiceBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("txproxy: %w", err)
	}

	httpClient := slhttputil.CopyHTTPClientWithTimeout(cfg.HTTPClient, timeout, forceTimeout)

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodySize
	}

	return &Client{
		baseURL:      baseURL,
		serviceID:    strings.TrimSpace(cfg.ServiceID),
		httpClient:   httpClient,
		maxBodyBytes: maxBodyBytes,
	}, nil
}

// Invoke calls TxProxy /invoke.
func (c *Client) Invoke(ctx context.Context, req *txproxytypes.InvokeRequest) (*txproxytypes.InvokeResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("txproxy: client is nil")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("txproxy: http client not configured")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/invoke", bytes.NewReader(body))
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

	var result txproxytypes.InvokeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
