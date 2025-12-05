// Package supabase provides a thin TEE-friendly Supabase REST client.
// It keeps API keys in the enclave via SecretsAPI and uses ServiceOS Network
// so TLS/auth happen inside the TEE.
package supabase

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	serviceos "github.com/R3E-Network/service_layer/platform/os"
)

// Config configures the Supabase client.
// APIKeySecret is the secret name (in the service namespace) holding the API key.
type Config struct {
	ProjectURL   string
	APIKeySecret string // optional if APIKey is provided
	APIKey       string // optional inline key; prefer APIKeySecret when available
	// Optional additional headers to send on every request.
	DefaultHeaders map[string]string
	// Optional explicit allowlist; if empty, derived from ProjectURL host.
	AllowedHosts []string
}

// Client performs Supabase REST calls using the TEE-backed Network + Secrets APIs.
type Client struct {
	os      serviceos.ServiceOS
	cfg     Config
	prefix  string
	headers map[string]string
	allowed map[string]struct{}
	useSecret bool
	apiKey    string
}

// New creates a Supabase client.
func New(os serviceos.ServiceOS, cfg Config) (*Client, error) {
	if os == nil {
		return nil, fmt.Errorf("serviceOS is required")
	}
	if cfg.ProjectURL == "" {
		return nil, fmt.Errorf("project URL is required")
	}
	if cfg.APIKeySecret == "" && cfg.APIKey == "" {
		return nil, fmt.Errorf("api key secret or api key is required")
	}
	trimmed := strings.TrimRight(cfg.ProjectURL, "/")

	allowed := make(map[string]struct{})
	if len(cfg.AllowedHosts) == 0 {
		if u, err := url.Parse(cfg.ProjectURL); err == nil && u.Hostname() != "" {
			allowed[u.Hostname()] = struct{}{}
		}
	} else {
		for _, h := range cfg.AllowedHosts {
			if h != "" {
				allowed[h] = struct{}{}
			}
		}
	}

	return &Client{
		os:     os,
		cfg:    cfg,
		prefix: trimmed + "/rest/v1",
		headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		allowed:   allowed,
		useSecret: cfg.APIKeySecret != "",
		apiKey:    cfg.APIKey,
	}, nil
}

// Select performs a GET on a table with an optional query string (already encoded).
func (c *Client) Select(ctx context.Context, table string, query string) (*serviceos.HTTPResponse, error) {
	if table == "" {
		return nil, fmt.Errorf("table is required")
	}
	path := c.prefix + "/" + url.PathEscape(table)
	if query != "" {
		path += "?" + query
	}
	req := serviceos.HTTPRequest{
		Method:  "GET",
		URL:     path,
		Headers: c.mergeHeaders(map[string]string{"Accept": "application/json"}),
	}
	return c.requestWithKey(ctx, req)
}

// Insert performs a POST insert into a table.
func (c *Client) Insert(ctx context.Context, table string, body []byte) (*serviceos.HTTPResponse, error) {
	if table == "" {
		return nil, fmt.Errorf("table is required")
	}
	req := serviceos.HTTPRequest{
		Method: "POST",
		URL:    c.prefix + "/" + url.PathEscape(table),
		Headers: c.mergeHeaders(map[string]string{
			"Prefer": "return=representation",
		}),
		Body: body,
	}
	return c.requestWithKey(ctx, req)
}

// requestWithKey injects the API key inside the enclave and performs the request.
func (c *Client) requestWithKey(ctx context.Context, req serviceos.HTTPRequest) (*serviceos.HTTPResponse, error) {
	if err := c.ensureAllowed(req.URL); err != nil {
		return nil, err
	}
	if c.useSecret {
		return c.os.Network().FetchWithSecret(ctx, req, c.cfg.APIKeySecret, serviceos.AuthBearer)
	}
	// Inline API key path: add headers inside the enclave and use Fetch
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	if _, ok := req.Headers["apikey"]; !ok {
		req.Headers["apikey"] = c.apiKey
	}
	if _, ok := req.Headers["Authorization"]; !ok {
		req.Headers["Authorization"] = "Bearer " + c.apiKey
	}
	return c.os.Network().Fetch(ctx, req)
}

func (c *Client) mergeHeaders(h map[string]string) map[string]string {
	merged := make(map[string]string, len(c.cfg.DefaultHeaders)+len(c.headers)+len(h))
	for k, v := range c.headers {
		if v != "" {
			merged[k] = v
		}
	}
	for k, v := range c.cfg.DefaultHeaders {
		if v != "" {
			merged[k] = v
		}
	}
	for k, v := range h {
		if v != "" {
			merged[k] = v
		}
	}
	return merged
}

func (c *Client) ensureAllowed(rawURL string) error {
	if len(c.allowed) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid url host")
	}
	if _, ok := c.allowed[host]; !ok {
		return fmt.Errorf("host not allowed for supabase: %s", host)
	}
	return nil
}
