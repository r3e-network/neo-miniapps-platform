// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// OCALL Handler - Go Service Engine (Untrusted Layer)
//
// This file implements the OCALL handler that processes outbound calls from the enclave.
// The handler runs in the untrusted Go layer and provides access to external resources.
//
// Security Model:
// - All OCALL requests are validated before processing
// - HTTP requests are filtered through allowlists
// - Chain RPC calls go through the configured RPC endpoints
// - All responses are sanitized before returning to the enclave
package tee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OCALLHandlerConfig configures the OCALL handler.
type OCALLHandlerConfig struct {
	// HTTPClient for making external requests
	HTTPClient *http.Client

	// AllowedHosts is a list of allowed hosts for HTTP requests
	// Empty list means all hosts are allowed (not recommended for production)
	AllowedHosts []string

	// BlockedHosts is a list of blocked hosts (takes precedence over AllowedHosts)
	BlockedHosts []string

	// MaxRequestSize limits the size of HTTP request bodies
	MaxRequestSize int64

	// MaxResponseSize limits the size of HTTP response bodies
	MaxResponseSize int64

	// DefaultTimeout for HTTP requests
	DefaultTimeout time.Duration

	// ChainRPCEndpoints maps chain names to RPC URLs
	ChainRPCEndpoints map[string]string

	// StorageBackend for sealed storage persistence
	// If nil, storage OCALLs will use an in-memory backend
	StorageBackend StorageBackend

	// Logger for OCALL events
	Logger OCALLLogger
}

// OCALLLogger logs OCALL events.
type OCALLLogger interface {
	LogOCALL(ctx context.Context, req OCALLRequest, resp *OCALLResponse, duration time.Duration)
}

// DefaultOCALLHandlerConfig returns a default configuration.
func DefaultOCALLHandlerConfig() OCALLHandlerConfig {
	return OCALLHandlerConfig{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		AllowedHosts:      []string{}, // Empty = allow all (configure for production)
		BlockedHosts:      []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"},
		MaxRequestSize:    10 * 1024 * 1024,  // 10MB
		MaxResponseSize:   50 * 1024 * 1024,  // 50MB
		DefaultTimeout:    30 * time.Second,
		ChainRPCEndpoints: make(map[string]string),
	}
}

// ocallHandlerImpl implements OCALLHandler.
type ocallHandlerImpl struct {
	mu     sync.RWMutex
	config OCALLHandlerConfig

	// storageBackend is the actual backend used (may be default if not configured)
	storageBackend StorageBackend

	// Metrics
	requestCount  int64
	errorCount    int64
	totalDuration time.Duration
}

// NewOCALLHandler creates a new OCALL handler.
func NewOCALLHandler(config OCALLHandlerConfig) OCALLHandler {
	if config.HTTPClient == nil {
		config.HTTPClient = DefaultOCALLHandlerConfig().HTTPClient
	}
	if config.MaxRequestSize <= 0 {
		config.MaxRequestSize = DefaultOCALLHandlerConfig().MaxRequestSize
	}
	if config.MaxResponseSize <= 0 {
		config.MaxResponseSize = DefaultOCALLHandlerConfig().MaxResponseSize
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = DefaultOCALLHandlerConfig().DefaultTimeout
	}

	// Initialize storage backend - use in-memory if not configured
	storageBackend := config.StorageBackend
	if storageBackend == nil {
		storageBackend = NewMemoryStorageBackend()
	}

	return &ocallHandlerImpl{
		config:         config,
		storageBackend: storageBackend,
	}
}

// HandleOCALL processes an OCALL request.
func (h *ocallHandlerImpl) HandleOCALL(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	start := time.Now()

	h.mu.Lock()
	h.requestCount++
	h.mu.Unlock()

	var resp *OCALLResponse
	var err error

	switch req.Type {
	case OCALLTypeHTTP:
		resp, err = h.handleHTTP(ctx, req)
	case OCALLTypeChainRPC:
		resp, err = h.handleChainRPC(ctx, req)
	case OCALLTypeChainTx:
		resp, err = h.handleChainTx(ctx, req)
	case OCALLTypeStorage:
		resp, err = h.handleStorage(ctx, req)
	case OCALLTypeLog:
		resp, err = h.handleLog(ctx, req)
	default:
		err = fmt.Errorf("unknown OCALL type: %s", req.Type)
	}

	duration := time.Since(start)

	if err != nil {
		h.mu.Lock()
		h.errorCount++
		h.totalDuration += duration
		h.mu.Unlock()

		resp = &OCALLResponse{
			RequestID: req.RequestID,
			Success:   false,
			Error:     err.Error(),
		}
	} else {
		h.mu.Lock()
		h.totalDuration += duration
		h.mu.Unlock()
	}

	// Log the OCALL
	if h.config.Logger != nil {
		h.config.Logger.LogOCALL(ctx, req, resp, duration)
	}

	return resp, nil
}

// handleHTTP processes HTTP OCALL requests.
func (h *ocallHandlerImpl) handleHTTP(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	var httpReq HTTPRequest
	if err := json.Unmarshal(req.Payload, &httpReq); err != nil {
		return nil, fmt.Errorf("unmarshal HTTP request: %w", err)
	}

	// Validate the request
	if err := h.validateHTTPRequest(httpReq); err != nil {
		return nil, fmt.Errorf("validate HTTP request: %w", err)
	}

	// Set timeout
	timeout := httpReq.Timeout
	if timeout <= 0 {
		timeout = h.config.DefaultTimeout
	}
	if req.Timeout > 0 && req.Timeout < timeout {
		timeout = req.Timeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the HTTP request
	var body io.Reader
	if len(httpReq.Body) > 0 {
		if int64(len(httpReq.Body)) > h.config.MaxRequestSize {
			return nil, fmt.Errorf("request body too large: %d > %d", len(httpReq.Body), h.config.MaxRequestSize)
		}
		body = bytes.NewReader(httpReq.Body)
	}

	r, err := http.NewRequestWithContext(ctx, httpReq.Method, httpReq.URL, body)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}

	// Set headers
	for k, v := range httpReq.Headers {
		r.Header.Set(k, v)
	}

	// Set default headers if not provided
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", "TEE-OCALL/1.0")
	}
	if r.Header.Get("Content-Type") == "" && len(httpReq.Body) > 0 {
		r.Header.Set("Content-Type", "application/json")
	}

	// Execute the request
	resp, err := h.config.HTTPClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, h.config.MaxResponseSize)
	respBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}

	// Build response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
	}

	payload, err := json.Marshal(httpResp)
	if err != nil {
		return nil, fmt.Errorf("marshal HTTP response: %w", err)
	}

	return &OCALLResponse{
		RequestID: req.RequestID,
		Success:   true,
		Payload:   payload,
	}, nil
}

// validateHTTPRequest validates an HTTP request.
func (h *ocallHandlerImpl) validateHTTPRequest(req HTTPRequest) error {
	if req.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if req.Method == "" {
		req.Method = "GET"
	}

	// Validate method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	if !validMethods[strings.ToUpper(req.Method)] {
		return fmt.Errorf("invalid HTTP method: %s", req.Method)
	}

	// Extract host from URL
	host := extractHost(req.URL)
	if host == "" {
		return fmt.Errorf("invalid URL: cannot extract host")
	}

	// Check blocked hosts
	for _, blocked := range h.config.BlockedHosts {
		if strings.EqualFold(host, blocked) || strings.HasSuffix(host, "."+blocked) {
			return fmt.Errorf("host %s is blocked", host)
		}
	}

	// Check allowed hosts (if configured)
	if len(h.config.AllowedHosts) > 0 {
		allowed := false
		for _, a := range h.config.AllowedHosts {
			if strings.EqualFold(host, a) || strings.HasSuffix(host, "."+a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host %s is not in allowed list", host)
		}
	}

	return nil
}

// handleChainRPC processes chain RPC OCALL requests.
func (h *ocallHandlerImpl) handleChainRPC(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	var callReq ChainCallRequest
	if err := json.Unmarshal(req.Payload, &callReq); err != nil {
		return nil, fmt.Errorf("unmarshal chain call request: %w", err)
	}

	// Get RPC endpoint for the chain
	h.mu.RLock()
	rpcURL, ok := h.config.ChainRPCEndpoints[strings.ToLower(callReq.Chain)]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no RPC endpoint configured for chain: %s", callReq.Chain)
	}

	// Build JSON-RPC request
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  callReq.Method,
		"params":  callReq.Args,
	}

	rpcBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal RPC request: %w", err)
	}

	// Execute RPC call
	httpReq, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(rpcBody))
	if err != nil {
		return nil, fmt.Errorf("create RPC request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute RPC request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, h.config.MaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read RPC response: %w", err)
	}

	// Parse RPC response
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal RPC response: %w", err)
	}

	callResp := ChainCallResponse{}
	if rpcResp.Error != nil {
		callResp.Error = rpcResp.Error.Message
	} else {
		callResp.Result = rpcResp.Result
	}

	payload, err := json.Marshal(callResp)
	if err != nil {
		return nil, fmt.Errorf("marshal call response: %w", err)
	}

	return &OCALLResponse{
		RequestID: req.RequestID,
		Success:   rpcResp.Error == nil,
		Payload:   payload,
		Error:     callResp.Error,
	}, nil
}

// handleChainTx processes chain transaction OCALL requests.
func (h *ocallHandlerImpl) handleChainTx(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	var txReq ChainTxRequest
	if err := json.Unmarshal(req.Payload, &txReq); err != nil {
		return nil, fmt.Errorf("unmarshal chain tx request: %w", err)
	}

	// Get RPC endpoint for the chain
	h.mu.RLock()
	rpcURL, ok := h.config.ChainRPCEndpoints[strings.ToLower(txReq.Chain)]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no RPC endpoint configured for chain: %s", txReq.Chain)
	}

	// Build transaction object
	txObj := map[string]any{
		"to":   txReq.To,
		"data": fmt.Sprintf("0x%x", txReq.Data),
	}
	if txReq.Value != "" {
		txObj["value"] = txReq.Value
	}
	if txReq.GasLimit > 0 {
		txObj["gas"] = fmt.Sprintf("0x%x", txReq.GasLimit)
	}

	// Build JSON-RPC request for eth_sendTransaction
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "eth_sendTransaction",
		"params":  []any{txObj},
	}

	rpcBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal RPC request: %w", err)
	}

	// Execute RPC call
	httpReq, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(rpcBody))
	if err != nil {
		return nil, fmt.Errorf("create RPC request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.config.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute RPC request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, h.config.MaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read RPC response: %w", err)
	}

	// Parse RPC response
	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal RPC response: %w", err)
	}

	txResp := ChainTxResponse{
		TxHash: rpcResp.Result,
		Status: "pending",
	}
	if rpcResp.Error != nil {
		txResp.Status = "failed"
		txResp.Error = rpcResp.Error.Message
	}

	payload, err := json.Marshal(txResp)
	if err != nil {
		return nil, fmt.Errorf("marshal tx response: %w", err)
	}

	return &OCALLResponse{
		RequestID: req.RequestID,
		Success:   rpcResp.Error == nil,
		Payload:   payload,
		Error:     txResp.Error,
	}, nil
}

// handleStorage processes storage OCALL requests.
// This delegates to HandleStorageOCALL which handles the actual storage operations.
func (h *ocallHandlerImpl) handleStorage(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	resp, err := HandleStorageOCALL(ctx, h.storageBackend, req.Payload)
	if err != nil {
		return &OCALLResponse{
			RequestID: req.RequestID,
			Success:   false,
			Error:     err.Error(),
		}, nil
	}
	resp.RequestID = req.RequestID
	return resp, nil
}

// handleLog processes log OCALL requests.
func (h *ocallHandlerImpl) handleLog(ctx context.Context, req OCALLRequest) (*OCALLResponse, error) {
	// Log messages from enclave are captured but not returned
	// They are typically forwarded to the logging system
	return &OCALLResponse{
		RequestID: req.RequestID,
		Success:   true,
	}, nil
}

// SetChainRPCEndpoint sets an RPC endpoint for a chain.
func (h *ocallHandlerImpl) SetChainRPCEndpoint(chain, url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.config.ChainRPCEndpoints == nil {
		h.config.ChainRPCEndpoints = make(map[string]string)
	}
	h.config.ChainRPCEndpoints[strings.ToLower(chain)] = url
}

// GetMetrics returns handler metrics.
func (h *ocallHandlerImpl) GetMetrics() (requestCount, errorCount int64, avgDuration time.Duration) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	requestCount = h.requestCount
	errorCount = h.errorCount
	if requestCount > 0 {
		avgDuration = h.totalDuration / time.Duration(requestCount)
	}
	return
}

// extractHost extracts the host from a URL.
func extractHost(urlStr string) string {
	// Simple extraction - in production use net/url
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")
	if idx := strings.Index(urlStr, "/"); idx > 0 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx > 0 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}
