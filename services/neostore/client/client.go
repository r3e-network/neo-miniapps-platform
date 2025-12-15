// Package client provides a client SDK for the NeoStore service.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	slhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

const (
	defaultTimeout     = 15 * time.Second
	defaultMaxBodySize = 1 << 20 // 1MiB
)

// Config configures the NeoStore client.
type Config struct {
	// BaseURL is the base URL of the NeoStore service (e.g. https://neostore:8087).
	BaseURL string
	// HTTPClient is used to execute requests. When nil, a default client with a
	// conservative timeout is used. For MarbleRun mesh calls, prefer using
	// `marble.Marble.HTTPClient()` so requests are sent over verified mTLS.
	HTTPClient *http.Client
	// ServiceID is optionally propagated in X-Service-ID for development
	// environments where verified mTLS identity is not available.
	ServiceID string
	// Timeout is used when HTTPClient is nil or has a zero timeout.
	Timeout time.Duration
	// MaxBodyBytes caps response bodies to prevent memory exhaustion.
	MaxBodyBytes int64
}

// Client fetches decrypted secrets from the NeoStore service over HTTP/mTLS.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	serviceID    string
	maxBodyBytes int64
}

// New creates a new NeoStore client.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	baseURL, _, err := slhttputil.NormalizeServiceBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("neostore: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else {
		// Avoid mutating a caller-supplied client (e.g., Marble mTLS client).
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
		httpClient:   client,
		serviceID:    strings.TrimSpace(cfg.ServiceID),
		maxBodyBytes: maxBodyBytes,
	}, nil
}

// GetSecret returns the decrypted secret value for a user.
func (c *Client) GetSecret(ctx context.Context, userID, name string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("neostore: client is nil")
	}
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("neostore: userID is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("neostore: secret name is required")
	}

	endpoint := c.baseURL + "/secrets/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("neostore: create request: %w", err)
	}

	req.Header.Set(serviceauth.UserIDHeader, userID)
	if c.serviceID != "" {
		req.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("neostore: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 4<<10)
		if readErr != nil {
			return "", fmt.Errorf("neostore: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return "", fmt.Errorf("neostore: %s: %s", resp.Status, msg)
		}
		return "", fmt.Errorf("neostore: %s", resp.Status)
	}

	respBody, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes)
	if err != nil {
		return "", fmt.Errorf("neostore: read response: %w", err)
	}

	var out struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("neostore: decode response: %w", err)
	}

	return out.Value, nil
}
