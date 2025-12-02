// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"time"
)

// secureHTTPClientImpl implements SecureHTTPClient interface.
type secureHTTPClientImpl struct {
	client    *http.Client
	tlsConfig *tls.Config
	certPool  *x509.CertPool
}

// NewSecureHTTPClient creates a new secure HTTP client instance.
func NewSecureHTTPClient(transport http.RoundTripper) SecureHTTPClient {
	certPool, _ := x509.SystemCertPool()
	if certPool == nil {
		certPool = x509.NewCertPool()
	}

	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	var httpTransport http.RoundTripper
	if transport != nil {
		httpTransport = transport
	} else {
		httpTransport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return &secureHTTPClientImpl{
		client: &http.Client{
			Transport: httpTransport,
			Timeout:   30 * time.Second,
		},
		tlsConfig: tlsConfig,
		certPool:  certPool,
	}
}

func (c *secureHTTPClientImpl) Get(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error) {
	req := &HTTPRequest{
		Method: http.MethodGet,
		URL:    url,
	}
	c.applyOptions(req, opts)
	return c.Do(ctx, req)
}

func (c *secureHTTPClientImpl) Post(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error) {
	req := &HTTPRequest{
		Method: http.MethodPost,
		URL:    url,
		Body:   body,
	}
	c.applyOptions(req, opts)
	return c.Do(ctx, req)
}

func (c *secureHTTPClientImpl) Put(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error) {
	req := &HTTPRequest{
		Method: http.MethodPut,
		URL:    url,
		Body:   body,
	}
	c.applyOptions(req, opts)
	return c.Do(ctx, req)
}

func (c *secureHTTPClientImpl) Delete(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error) {
	req := &HTTPRequest{
		Method: http.MethodDelete,
		URL:    url,
	}
	c.applyOptions(req, opts)
	return c.Do(ctx, req)
}

// applyOptions applies HTTP options to the request.
func (c *secureHTTPClientImpl) applyOptions(req *HTTPRequest, opts []HTTPOption) {
	options := &httpOptions{
		headers: make(map[string]string),
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Apply headers
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	for k, v := range options.headers {
		req.Headers[k] = v
	}

	// Apply timeout
	if options.timeout > 0 {
		req.Timeout = options.timeout
	}

	// Apply auth as headers
	if options.auth != nil {
		switch options.auth.Type {
		case "basic":
			// Basic auth will be applied in Do
		case "bearer":
			req.Headers["Authorization"] = "Bearer " + options.auth.Token
		case "api_key":
			header := options.auth.Header
			if header == "" {
				header = "X-API-Key"
			}
			req.Headers[header] = options.auth.Token
		}
	}
}

func (c *secureHTTPClientImpl) Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
	// Use request timeout or default
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create HTTP request
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set content type for POST/PUT if not specified
	if (req.Method == http.MethodPost || req.Method == http.MethodPut) && len(req.Body) > 0 {
		if httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}
	}

	// Create client with timeout
	client := &http.Client{
		Transport: c.client.Transport,
		Timeout:   timeout,
	}

	// Execute request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, ErrHTTPRequestFailed
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
	}, nil
}

func (c *secureHTTPClientImpl) SetTLSConfig(config *tls.Config) {
	c.tlsConfig = config
	c.client.Transport = &http.Transport{
		TLSClientConfig: config,
	}
}

func (c *secureHTTPClientImpl) AddTrustedCert(cert []byte) error {
	if c.certPool == nil {
		c.certPool = x509.NewCertPool()
	}

	if !c.certPool.AppendCertsFromPEM(cert) {
		return errors.New("failed to add certificate")
	}

	c.tlsConfig.RootCAs = c.certPool
	c.client.Transport = &http.Transport{
		TLSClientConfig: c.tlsConfig,
	}

	return nil
}
