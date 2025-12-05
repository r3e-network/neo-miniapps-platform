// Package network provides secure HTTP networking inside the TEE enclave.
package network

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/types"
)

// VaultProvider provides access to secrets.
type VaultProvider interface {
	Use(ctx context.Context, namespace, name string, fn types.SecretConsumer) error
}

// Config holds network configuration.
type Config struct {
	Runtime        enclave.Runtime
	Vault          VaultProvider
	DefaultTimeout time.Duration
	MaxBodySize    int64
	AllowedHosts   []string
	// PinnedCerts maps host -> SHA256 fingerprint (raw bytes) of the leaf cert
	PinnedCerts map[string][]byte
}

// Client implements types.SecureNetwork.
type Client struct {
	mu             sync.RWMutex
	runtime        enclave.Runtime
	vault          VaultProvider
	httpClient     *http.Client
	defaultTimeout time.Duration
	maxBodySize    int64
	allowedHosts   map[string]struct{}
	pinnedCerts    map[string][]byte
}

// New creates a new secure network client.
func New(cfg Config) (*Client, error) {
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}

	timeout := cfg.DefaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxBody := cfg.MaxBodySize
	if maxBody == 0 {
		maxBody = 10 * 1024 * 1024 // 10MB default
	}

	hostSet := make(map[string]struct{})
	for _, h := range cfg.AllowedHosts {
		if h == "" {
			continue
		}
		hostSet[h] = struct{}{}
	}

	pins := make(map[string][]byte)
	for host, fp := range cfg.PinnedCerts {
		if host == "" || len(fp) == 0 {
			continue
		}
		pins[host] = fp
	}

	return &Client{
		runtime:        cfg.Runtime,
		vault:          cfg.Vault,
		defaultTimeout: timeout,
		maxBodySize:    maxBody,
		allowedHosts:   hostSet,
		pinnedCerts:    pins,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					VerifyConnection: func(cs tls.ConnectionState) error {
						return ensurePinned(cs.ServerName, cs.PeerCertificates, pins)
					},
				},
			},
		},
	}, nil
}

// Fetch performs an HTTP request with TLS inside the enclave.
func (c *Client) Fetch(ctx context.Context, req types.SecureHTTPRequest) (*types.SecureHTTPResponse, error) {
	return c.doRequest(ctx, req, nil)
}

// FetchWithSecret performs HTTP with secret-based auth.
func (c *Client) FetchWithSecret(ctx context.Context, req types.SecureHTTPRequest, namespace, secretName string, authType types.AuthType) (*types.SecureHTTPResponse, error) {
	if c.vault == nil {
		return nil, fmt.Errorf("vault not configured")
	}

	var resp *types.SecureHTTPResponse
	var fetchErr error

	err := c.vault.Use(ctx, namespace, secretName, func(secret []byte) error {
		// Inject auth inside enclave
		authReq := c.injectAuth(req, secret, authType)
		resp, fetchErr = c.doRequest(ctx, authReq, secret)
		return fetchErr
	})

	if err != nil {
		return nil, err
	}

	return resp, fetchErr
}

// RPC performs a JSON-RPC call.
func (c *Client) RPC(ctx context.Context, endpoint, method string, params any) (json.RawMessage, error) {
	return c.doRPC(ctx, endpoint, method, params, nil)
}

// RPCWithSecret performs RPC with secret-based auth.
func (c *Client) RPCWithSecret(ctx context.Context, endpoint, method string, params any, namespace, secretName string) (json.RawMessage, error) {
	if c.vault == nil {
		return nil, fmt.Errorf("vault not configured")
	}

	var result json.RawMessage
	var rpcErr error

	err := c.vault.Use(ctx, namespace, secretName, func(secret []byte) error {
		result, rpcErr = c.doRPC(ctx, endpoint, method, params, secret)
		return rpcErr
	})

	if err != nil {
		return nil, err
	}

	return result, rpcErr
}

// doRequest performs the actual HTTP request.
func (c *Client) doRequest(ctx context.Context, req types.SecureHTTPRequest, _ []byte) (*types.SecureHTTPResponse, error) {
	timeout := req.Timeout
	if timeout == 0 {
		timeout = c.defaultTimeout
	}

	if err := c.ensureAllowedHost(req.URL); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create HTTP request
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request (TLS handled inside enclave)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with limit
	limitedReader := io.LimitReader(resp.Body, c.maxBodySize)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Convert headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &types.SecureHTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       respBody,
	}, nil
}

// ensureAllowedHost enforces host allowlist (if configured).
func (c *Client) ensureAllowedHost(rawURL string) error {
	if len(c.allowedHosts) == 0 {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("invalid url host")
	}

	if _, ok := c.allowedHosts[host]; !ok {
		return fmt.Errorf("host not allowed: %s", host)
	}
	return nil
}

// ensurePinned enforces certificate pinning for a host if a pin exists.
// Peer certificates are provided in order: leaf first.
func ensurePinned(host string, peerCerts []*x509.Certificate, pins map[string][]byte) error {
	if len(pins) == 0 || host == "" {
		return nil
	}
	expected, ok := pins[host]
	if !ok {
		return nil
	}
	if len(peerCerts) == 0 || peerCerts[0] == nil {
		return fmt.Errorf("no peer certificate presented for host %s", host)
	}
	fp := sha256.Sum256(peerCerts[0].Raw)
	if !equalFingerprint(fp[:], expected) {
		return fmt.Errorf("pinned certificate mismatch for host %s (got %s)", host, hex.EncodeToString(fp[:]))
	}
	return nil
}

func equalFingerprint(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// injectAuth injects authentication into the request.
func (c *Client) injectAuth(req types.SecureHTTPRequest, secret []byte, authType types.AuthType) types.SecureHTTPRequest {
	// Clone headers
	headers := make(map[string]string)
	for k, v := range req.Headers {
		headers[k] = v
	}

	switch authType {
	case types.AuthTypeBearer:
		headers["Authorization"] = "Bearer " + string(secret)
	case types.AuthTypeBasic:
		headers["Authorization"] = "Basic " + string(secret)
	case types.AuthTypeAPIKey:
		headers["X-API-Key"] = string(secret)
	case types.AuthTypeCustom:
		// Custom auth - secret contains the full header value
		headers["Authorization"] = string(secret)
	}

	return types.SecureHTTPRequest{
		Method:  req.Method,
		URL:     req.URL,
		Headers: headers,
		Body:    req.Body,
		Timeout: req.Timeout,
	}
}

// doRPC performs a JSON-RPC call.
func (c *Client) doRPC(ctx context.Context, endpoint, method string, params any, secret []byte) (json.RawMessage, error) {
	// Build JSON-RPC request
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add auth if secret provided
	if secret != nil {
		headers["Authorization"] = "Bearer " + string(secret)
	}

	req := types.SecureHTTPRequest{
		Method:  "POST",
		URL:     endpoint,
		Headers: headers,
		Body:    body,
	}

	resp, err := c.doRequest(ctx, req, secret)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC error: status %d", resp.StatusCode)
	}

	// Parse JSON-RPC response
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// Zero zeros sensitive data.
func (c *Client) Zero() {
	// Nothing to zero - secrets are handled via vault callbacks
}
