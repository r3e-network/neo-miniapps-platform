package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/tee/types"
)

// Client is the main Supabase client.
// API keys are stored in TEE vault and injected into requests inside the enclave.
type Client struct {
	config    Config
	trustRoot types.TrustRoot

	// Derived values
	baseURL     string
	restURL     string
	authURL     string
	storageURL  string
	realtimeURL string
	allowedHosts map[string]struct{}

	// Sub-clients
	auth     *AuthClient
	database *DatabaseClient
	storage  *StorageClient
	realtime *RealtimeClient
}

// New creates a new Supabase client backed by TEE.
func New(trustRoot types.TrustRoot, cfg Config) (*Client, error) {
	if trustRoot == nil {
		return nil, fmt.Errorf("trustRoot is required")
	}
	if cfg.ProjectURL == "" {
		return nil, fmt.Errorf("project URL is required")
	}
	if cfg.AnonKeySecret == "" {
		return nil, fmt.Errorf("anon key secret is required")
	}

	// Parse and validate URL
	baseURL := strings.TrimRight(cfg.ProjectURL, "/")
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid project URL: %w", err)
	}

	// Build allowed hosts
	allowedHosts := make(map[string]struct{})
	if len(cfg.AllowedHosts) == 0 {
		allowedHosts[parsedURL.Hostname()] = struct{}{}
	} else {
		for _, h := range cfg.AllowedHosts {
			if h != "" {
				allowedHosts[h] = struct{}{}
			}
		}
	}

	// Set default timeout
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	c := &Client{
		config:       cfg,
		trustRoot:    trustRoot,
		baseURL:      baseURL,
		restURL:      baseURL + "/rest/v1",
		authURL:      baseURL + "/auth/v1",
		storageURL:   baseURL + "/storage/v1",
		realtimeURL:  strings.Replace(baseURL, "https://", "wss://", 1) + "/realtime/v1",
		allowedHosts: allowedHosts,
	}

	// Initialize sub-clients
	c.auth = &AuthClient{client: c}
	c.database = &DatabaseClient{client: c}
	c.storage = &StorageClient{client: c}
	c.realtime = &RealtimeClient{client: c}

	return c, nil
}

// Auth returns the auth client.
func (c *Client) Auth() *AuthClient {
	return c.auth
}

// Database returns the database client.
func (c *Client) Database() *DatabaseClient {
	return c.database
}

// Storage returns the storage client.
func (c *Client) Storage() *StorageClient {
	return c.storage
}

// Realtime returns the realtime client.
func (c *Client) Realtime() *RealtimeClient {
	return c.realtime
}

// =============================================================================
// Internal HTTP Methods
// =============================================================================

// request performs an HTTP request with the anon key.
func (c *Client) request(ctx context.Context, method, urlPath string, body []byte, headers map[string]string) ([]byte, int, error) {
	return c.requestWithSecret(ctx, method, urlPath, body, headers, c.config.AnonKeySecret)
}

// requestWithServiceKey performs an HTTP request with the service role key.
func (c *Client) requestWithServiceKey(ctx context.Context, method, urlPath string, body []byte, headers map[string]string) ([]byte, int, error) {
	if c.config.ServiceKeySecret == "" {
		return nil, 0, fmt.Errorf("service key secret not configured")
	}
	return c.requestWithSecret(ctx, method, urlPath, body, headers, c.config.ServiceKeySecret)
}

// requestWithSecret performs an HTTP request with a specific secret.
func (c *Client) requestWithSecret(ctx context.Context, method, urlPath string, body []byte, headers map[string]string, secretName string) ([]byte, int, error) {
	// Validate URL
	if err := c.validateURL(urlPath); err != nil {
		return nil, 0, err
	}

	// Build headers
	reqHeaders := c.buildHeaders(headers)

	// Create request
	req := types.SecureHTTPRequest{
		Method:  method,
		URL:     urlPath,
		Headers: reqHeaders,
		Body:    body,
		Timeout: c.config.Timeout,
	}

	// Perform request with secret (API key injected inside enclave)
	resp, err := c.trustRoot.Network().FetchWithSecret(ctx, req, "supabase", secretName, types.AuthTypeBearer)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}

	return resp.Body, resp.StatusCode, nil
}

// requestWithToken performs an HTTP request with a user's access token.
func (c *Client) requestWithToken(ctx context.Context, method, urlPath string, body []byte, headers map[string]string, accessToken string) ([]byte, int, error) {
	// Validate URL
	if err := c.validateURL(urlPath); err != nil {
		return nil, 0, err
	}

	// Build headers with token
	reqHeaders := c.buildHeaders(headers)
	reqHeaders["Authorization"] = "Bearer " + accessToken

	// Create request
	req := types.SecureHTTPRequest{
		Method:  method,
		URL:     urlPath,
		Headers: reqHeaders,
		Body:    body,
		Timeout: c.config.Timeout,
	}

	// Perform request (no secret needed, token already in header)
	resp, err := c.trustRoot.Network().Fetch(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}

	return resp.Body, resp.StatusCode, nil
}

// buildHeaders builds request headers.
func (c *Client) buildHeaders(extra map[string]string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}

	// Add default headers
	for k, v := range c.config.DefaultHeaders {
		headers[k] = v
	}

	// Add extra headers
	for k, v := range extra {
		headers[k] = v
	}

	return headers
}

// validateURL validates that the URL is allowed.
func (c *Client) validateURL(rawURL string) error {
	if len(c.allowedHosts) == 0 {
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid URL host")
	}

	if _, ok := c.allowedHosts[host]; !ok {
		return fmt.Errorf("host not allowed: %s", host)
	}

	return nil
}

// parseError parses an error response.
func parseError(body []byte, statusCode int) error {
	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details"`
		Hint    string `json:"hint"`
		Error   string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return &Error{
			Code:       "unknown",
			Message:    string(body),
			StatusCode: statusCode,
		}
	}

	msg := errResp.Message
	if msg == "" {
		msg = errResp.Error
	}
	if msg == "" {
		msg = errResp.ErrorDescription
	}

	return &Error{
		Code:       errResp.Code,
		Message:    msg,
		Details:    errResp.Details,
		Hint:       errResp.Hint,
		StatusCode: statusCode,
	}
}
