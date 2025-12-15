package secretstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/runtime"
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
	// CallerServiceID is optionally propagated in X-Service-ID for development
	// environments where verified mTLS identity is not available.
	CallerServiceID string
	// MaxBodyBytes caps response bodies to prevent memory exhaustion.
	MaxBodyBytes int64
}

// Client fetches decrypted secrets from the NeoStore service over HTTP/mTLS.
type Client struct {
	baseURL         string
	httpClient      *http.Client
	callerServiceID string
	maxBodyBytes    int64
}

// New creates a new NeoStore client.
func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("secretstore: BaseURL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("secretstore: BaseURL must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("secretstore: BaseURL scheme must be http or https")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("secretstore: BaseURL must not include user info")
	}
	if runtime.StrictIdentityMode() && parsed.Scheme != "https" {
		return nil, fmt.Errorf("secretstore: BaseURL must use https in strict identity mode")
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	if client.Timeout == 0 {
		client.Timeout = defaultTimeout
	}

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodySize
	}

	return &Client{
		baseURL:         baseURL,
		httpClient:      client,
		callerServiceID: strings.TrimSpace(cfg.CallerServiceID),
		maxBodyBytes:    maxBodyBytes,
	}, nil
}

// GetSecret returns the decrypted secret value for a user.
func (c *Client) GetSecret(ctx context.Context, userID, name string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("secretstore: client is nil")
	}
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("secretstore: userID is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("secretstore: secret name is required")
	}

	endpoint := c.baseURL + "/secrets/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("secretstore: create request: %w", err)
	}

	req.Header.Set(serviceauth.UserIDHeader, userID)
	if c.callerServiceID != "" {
		req.Header.Set(serviceauth.ServiceIDHeader, c.callerServiceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("secretstore: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := ""
		if readErr == nil {
			msg = strings.TrimSpace(string(body))
		}
		if msg != "" {
			return "", fmt.Errorf("secretstore: %s: %s", resp.Status, msg)
		}
		return "", fmt.Errorf("secretstore: %s", resp.Status)
	}

	var out struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, c.maxBodyBytes))
	if err := dec.Decode(&out); err != nil {
		return "", fmt.Errorf("secretstore: decode response: %w", err)
	}

	return out.Value, nil
}
