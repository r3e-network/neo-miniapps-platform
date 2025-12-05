// Package oracle provides the Oracle service.
package oracle

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected operations for the Oracle service.
type Enclave struct {
	*base.BaseEnclave

	keyHandle os.KeyHandle
}

// NewEnclave creates a new Oracle enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}

	// Derive signing key for this service
	handle, err := e.DeriveKey(ctx, "signing")
	if err != nil {
		return fmt.Errorf("derive signing key: %w", err)
	}
	e.keyHandle = handle

	e.Logger().Info("oracle enclave initialized with signing key")
	return nil
}

// SecureFetch performs a secure HTTP fetch inside the TEE.
func (e *Enclave) SecureFetch(ctx context.Context, req *FetchRequest) (*FetchResponse, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Build HTTP request
	httpReq := os.HTTPRequest{
		Method:  req.Method,
		URL:     req.URL,
		Headers: req.Headers,
		Body:    req.Body,
		Timeout: 30 * time.Second,
	}

	var resp *os.HTTPResponse
	var err error

	// Fetch with or without secret
	if req.SecretName != "" {
		resp, err = e.FetchWithSecret(ctx, httpReq, req.SecretName, req.AuthType)
	} else {
		resp, err = e.OS().Network().Fetch(ctx, httpReq)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Create response
	fetchResp := &FetchResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
		Timestamp:  time.Now(),
	}

	// Sign the response for integrity
	signature, err := e.signResponse(ctx, fetchResp)
	if err != nil {
		e.Logger().Warn("failed to sign response", "error", err)
	} else {
		fetchResp.Signature = signature
	}

	return fetchResp, nil
}

// signResponse signs the response data.
func (e *Enclave) signResponse(ctx context.Context, resp *FetchResponse) ([]byte, error) {
	// Create hash of response data
	h := sha256.New()
	h.Write(resp.Body)
	h.Write([]byte(resp.Timestamp.Format(time.RFC3339)))
	hash := h.Sum(nil)

	// Sign with enclave key
	return e.Sign(ctx, e.keyHandle, hash)
}

// VerifyResponse verifies a response signature.
func (e *Enclave) VerifyResponse(ctx context.Context, resp *FetchResponse) (bool, error) {
	if !e.IsReady() {
		return false, fmt.Errorf("enclave not ready")
	}

	if len(resp.Signature) == 0 {
		return false, fmt.Errorf("no signature")
	}

	// Recreate hash
	h := sha256.New()
	h.Write(resp.Body)
	h.Write([]byte(resp.Timestamp.Format(time.RFC3339)))
	hash := h.Sum(nil)

	// Get public key
	pubKey, err := e.OS().Keys().GetPublicKey(ctx, e.keyHandle)
	if err != nil {
		return false, fmt.Errorf("get public key: %w", err)
	}

	// Verify signature
	return e.OS().Keys().Verify(ctx, pubKey, hash, resp.Signature)
}

// GetPublicKey returns the enclave's public signing key.
func (e *Enclave) GetPublicKey(ctx context.Context) ([]byte, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	return e.OS().Keys().GetPublicKey(ctx, e.keyHandle)
}

// GetAddress returns the enclave's blockchain address.
func (e *Enclave) GetAddress(ctx context.Context, chain os.ChainType) (string, error) {
	if !e.IsReady() {
		return "", fmt.Errorf("enclave not ready")
	}

	return e.OS().Keys().GetAddress(ctx, e.keyHandle, chain)
}

// GenerateAttestation generates an attestation quote.
func (e *Enclave) GenerateAttestation(ctx context.Context, userData []byte) (*os.Quote, error) {
	return e.GenerateQuote(ctx, userData)
}

// ExecuteScript executes a script inside the enclave.
func (e *Enclave) ExecuteScript(ctx context.Context, script, entryPoint string, input map[string]any) (*os.ComputeResult, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	req := os.ComputeRequest{
		Script:     script,
		EntryPoint: entryPoint,
		Input:      input,
		Timeout:    30 * time.Second,
	}

	return e.Execute(ctx, req)
}

// ExecuteScriptWithSecrets executes a script with access to secrets.
func (e *Enclave) ExecuteScriptWithSecrets(ctx context.Context, script, entryPoint string, input map[string]any, secretNames []string) (*os.ComputeResult, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	req := os.ComputeRequest{
		Script:     script,
		EntryPoint: entryPoint,
		Input:      input,
		Timeout:    30 * time.Second,
	}

	return e.ExecuteWithSecrets(ctx, req, secretNames)
}

// ExtractJSONPath extracts data from JSON using a JSONPath expression.
func (e *Enclave) ExtractJSONPath(ctx context.Context, data []byte, jsonPath string) ([]byte, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	// Simple JSONPath extraction - for production, use a proper JSONPath library
	// This is a placeholder implementation
	// TODO: Implement proper JSONPath extraction using a library like github.com/PaesslerAG/jsonpath

	// For now, return the raw data if no path specified
	if jsonPath == "" || jsonPath == "$" {
		return data, nil
	}

	// Basic implementation: just return the data
	// In production, this should parse the JSON and extract the value at the path
	return data, nil
}

// SignData signs arbitrary data with the enclave's signing key.
func (e *Enclave) SignData(ctx context.Context, data []byte) ([]byte, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}

	// Create hash of data
	h := sha256.New()
	h.Write(data)
	hash := h.Sum(nil)

	// Sign with enclave key
	return e.Sign(ctx, e.keyHandle, hash)
}
