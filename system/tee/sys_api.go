// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// sys.* APIs - Secure System APIs for JavaScript in Enclave
//
// These APIs are exposed to JavaScript code running inside the TEE enclave.
// They provide secure access to system functions through the ECALL/OCALL bridge.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Enclave (Trusted)                                │
//	│  ┌─────────────────────────────────────────────────────────────────────┐ │
//	│  │                    V8 JavaScript Engine                              │ │
//	│  │  ┌─────────────────────────────────────────────────────────────────┐ │ │
//	│  │  │  User JS Code                                                    │ │ │
//	│  │  │    const result = await sys.http.fetch(url);                     │ │ │
//	│  │  │    const secret = sys.secrets.get("api_key");                    │ │ │
//	│  │  │    sys.proof.sign(data);                                         │ │ │
//	│  │  └──────────────────────────┬──────────────────────────────────────┘ │ │
//	│  │                             │ sys.* calls                            │ │
//	│  │  ┌──────────────────────────▼──────────────────────────────────────┐ │ │
//	│  │  │  SysAPI Bridge (this file)                                       │ │ │
//	│  │  │    - sys.http    (HTTP requests via OCALL)                       │ │ │
//	│  │  │    - sys.secrets (Secret management)                             │ │ │
//	│  │  │    - sys.crypto  (Cryptographic operations)                      │ │ │
//	│  │  │    - sys.proof   (Proof generation)                              │ │ │
//	│  │  │    - sys.storage (Sealed storage)                                │ │ │
//	│  │  │    - sys.chain   (Blockchain interactions via OCALL)             │ │ │
//	│  │  └──────────────────────────┬──────────────────────────────────────┘ │ │
//	│  └─────────────────────────────│────────────────────────────────────────┘ │
//	│                                │ ECALL/OCALL                              │
//	└────────────────────────────────│──────────────────────────────────────────┘
//	                                 │
//	┌────────────────────────────────▼──────────────────────────────────────────┐
//	│                      Go Service Engine (Untrusted)                         │
//	│  - HTTP Client                                                             │
//	│  - Database Access                                                         │
//	│  - Blockchain RPC                                                          │
//	└────────────────────────────────────────────────────────────────────────────┘
package tee

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/system/tee/types"
)

// Type aliases for backward compatibility - use types from shared package
type (
	HTTPRequest      = types.HTTPRequest
	HTTPResponse     = types.HTTPResponse
	ChainCallRequest  = types.ChainCallRequest
	ChainCallResponse = types.ChainCallResponse
	ChainTxRequest    = types.ChainTxRequest
	ChainTxResponse   = types.ChainTxResponse
	ChainBlock        = types.ChainBlock
	ChainTransaction  = types.ChainTransaction
	ExecutionProof    = types.ExecutionProof
	OCALLType         = types.OCALLType
	OCALLRequest      = types.OCALLRequest
	OCALLResponse     = types.OCALLResponse
	ECALLType         = types.ECALLType
	ECALLRequest      = types.ECALLRequest
	ECALLResponse     = types.ECALLResponse
)

// Re-export constants from types package
const (
	OCALLTypeHTTP     = types.OCALLTypeHTTP
	OCALLTypeChainRPC = types.OCALLTypeChainRPC
	OCALLTypeChainTx  = types.OCALLTypeChainTx
	OCALLTypeStorage  = types.OCALLTypeStorage
	OCALLTypeLog      = types.OCALLTypeLog

	ECALLTypeExecute     = types.ECALLTypeExecute
	ECALLTypeGetSecret   = types.ECALLTypeGetSecret
	ECALLTypeSetSecret   = types.ECALLTypeSetSecret
	ECALLTypeAttestation = types.ECALLTypeAttestation
	ECALLTypeHealth      = types.ECALLTypeHealth
)

// SysAPI defines the secure system APIs available to JavaScript in the enclave.
// These APIs are injected into the V8 runtime as the global `sys` object.
type SysAPI interface {
	// HTTP provides secure HTTP client functionality via OCALL.
	HTTP() SysHTTP

	// Secrets provides access to encrypted secrets.
	Secrets() SysSecrets

	// Crypto provides cryptographic operations.
	Crypto() SysCrypto

	// Proof provides proof generation and verification.
	Proof() SysProof

	// Storage provides sealed persistent storage.
	Storage() SysStorage

	// Chain provides blockchain interaction capabilities.
	Chain() SysChain

	// Log provides secure logging.
	Log() SysLog
}

// SysHTTP provides HTTP client functionality.
// All requests go through OCALL to the untrusted Go layer.
type SysHTTP interface {
	// Fetch performs an HTTP request and returns the response.
	// This is the primary method for external data access.
	Fetch(ctx context.Context, req HTTPRequest) (*HTTPResponse, error)

	// Get is a convenience method for GET requests.
	Get(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error)

	// Post is a convenience method for POST requests.
	Post(ctx context.Context, url string, body []byte, headers map[string]string) (*HTTPResponse, error)
}

// SysSecrets provides access to encrypted secrets.
type SysSecrets interface {
	// Get retrieves a secret by name.
	Get(ctx context.Context, name string) (string, error)

	// GetMultiple retrieves multiple secrets at once.
	GetMultiple(ctx context.Context, names []string) (map[string]string, error)

	// Set stores a secret (only available to authorized services).
	Set(ctx context.Context, name string, value string) error

	// Delete removes a secret.
	Delete(ctx context.Context, name string) error

	// List returns all secret names (not values).
	List(ctx context.Context) ([]string, error)
}

// SysCrypto provides cryptographic operations.
type SysCrypto interface {
	// Hash computes a cryptographic hash.
	Hash(algorithm string, data []byte) ([]byte, error)

	// Sign signs data using the enclave's signing key.
	Sign(data []byte) ([]byte, error)

	// Verify verifies a signature.
	Verify(data []byte, signature []byte, publicKey []byte) (bool, error)

	// Encrypt encrypts data using the specified key.
	Encrypt(keyID string, plaintext []byte) ([]byte, error)

	// Decrypt decrypts data using the specified key.
	Decrypt(keyID string, ciphertext []byte) ([]byte, error)

	// GenerateKey generates a new key pair.
	GenerateKey(keyType string) (*KeyPair, error)

	// RandomBytes generates cryptographically secure random bytes.
	RandomBytes(length int) ([]byte, error)
}

// KeyPair is an alias to types.KeyPair
type KeyPair = types.KeyPair

// SysProof provides proof generation and verification.
type SysProof interface {
	// GenerateProof generates a proof of execution.
	GenerateProof(ctx context.Context, data []byte) (*ExecutionProof, error)

	// VerifyProof verifies an execution proof.
	VerifyProof(ctx context.Context, proof *ExecutionProof) (bool, error)

	// GetAttestation returns the current TEE attestation.
	GetAttestation(ctx context.Context) (*AttestationReport, error)
}


// SysStorage provides sealed persistent storage.
type SysStorage interface {
	// Get retrieves a value by key.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with the given key.
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a value.
	Delete(ctx context.Context, key string) error

	// List returns all keys with the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)
}

// SysChain provides blockchain interaction capabilities.
type SysChain interface {
	// Call performs a read-only contract call.
	Call(ctx context.Context, req ChainCallRequest) (*ChainCallResponse, error)

	// SendTransaction sends a signed transaction (via OCALL).
	SendTransaction(ctx context.Context, req ChainTxRequest) (*ChainTxResponse, error)

	// GetBlock retrieves block information.
	GetBlock(ctx context.Context, chain string, blockNumber int64) (*ChainBlock, error)

	// GetTransaction retrieves transaction information.
	GetTransaction(ctx context.Context, chain string, txHash string) (*ChainTransaction, error)
}


// SysLog provides secure logging.
type SysLog interface {
	// Debug logs a debug message.
	Debug(msg string, args ...any)

	// Info logs an info message.
	Info(msg string, args ...any)

	// Warn logs a warning message.
	Warn(msg string, args ...any)

	// Error logs an error message.
	Error(msg string, args ...any)
}

// =============================================================================
// ECALL/OCALL Bridge Types - defined in types/ package, re-exported above
// =============================================================================

// =============================================================================
// OCALL Handler Interface
// =============================================================================

// OCALLHandler handles outbound calls from the enclave.
// This is implemented by the Go Service Engine (untrusted layer).
type OCALLHandler interface {
	// HandleOCALL processes an OCALL request and returns a response.
	HandleOCALL(ctx context.Context, req OCALLRequest) (*OCALLResponse, error)
}

// =============================================================================
// Default SysAPI Implementation (Simulation Mode)
// =============================================================================

// sysAPIImpl is the default implementation of SysAPI for simulation mode.
type sysAPIImpl struct {
	serviceID    string
	accountID    string
	secretVault  SecretVault
	ocallHandler OCALLHandler
	logs         []string
}

// NewSysAPI creates a new SysAPI instance for a specific execution context.
func NewSysAPI(serviceID, accountID string, vault SecretVault, handler OCALLHandler) SysAPI {
	return &sysAPIImpl{
		serviceID:    serviceID,
		accountID:    accountID,
		secretVault:  vault,
		ocallHandler: handler,
		logs:         make([]string, 0),
	}
}

func (s *sysAPIImpl) HTTP() SysHTTP {
	return &sysHTTPImpl{handler: s.ocallHandler}
}

func (s *sysAPIImpl) Secrets() SysSecrets {
	return &sysSecretsImpl{
		serviceID: s.serviceID,
		accountID: s.accountID,
		vault:     s.secretVault,
	}
}

func (s *sysAPIImpl) Crypto() SysCrypto {
	return NewSysCrypto()
}

func (s *sysAPIImpl) Proof() SysProof {
	return NewSysProof("enclave-"+s.serviceID, nil)
}

func (s *sysAPIImpl) Storage() SysStorage {
	return newSysStorage("enclave-"+s.serviceID, s.ocallHandler)
}

func (s *sysAPIImpl) Chain() SysChain {
	return &sysChainImpl{handler: s.ocallHandler}
}

func (s *sysAPIImpl) Log() SysLog {
	return &sysLogImpl{logs: &s.logs}
}

// GetLogs returns all captured logs.
func (s *sysAPIImpl) GetLogs() []string {
	return s.logs
}

// =============================================================================
// SysHTTP Implementation
// =============================================================================

type sysHTTPImpl struct {
	handler OCALLHandler
}

func (h *sysHTTPImpl) Fetch(ctx context.Context, req HTTPRequest) (*HTTPResponse, error) {
	if h.handler == nil {
		return nil, fmt.Errorf("OCALL handler not available")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeHTTP,
		RequestID: generateRequestID(),
		Payload:   payload,
		Timeout:   req.Timeout,
	}

	resp, err := h.handler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, fmt.Errorf("OCALL failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("HTTP request failed: %s", resp.Error)
	}

	var httpResp HTTPResponse
	if err := json.Unmarshal(resp.Payload, &httpResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &httpResp, nil
}

func (h *sysHTTPImpl) Get(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error) {
	return h.Fetch(ctx, HTTPRequest{
		Method:  "GET",
		URL:     url,
		Headers: headers,
	})
}

func (h *sysHTTPImpl) Post(ctx context.Context, url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	return h.Fetch(ctx, HTTPRequest{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    body,
	})
}

// =============================================================================
// SysSecrets Implementation
// =============================================================================

type sysSecretsImpl struct {
	serviceID string
	accountID string
	vault     SecretVault
}

func (s *sysSecretsImpl) Get(ctx context.Context, name string) (string, error) {
	if s.vault == nil {
		return "", fmt.Errorf("secret vault not available")
	}
	value, err := s.vault.GetSecret(ctx, s.serviceID, s.accountID, name)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (s *sysSecretsImpl) GetMultiple(ctx context.Context, names []string) (map[string]string, error) {
	if s.vault == nil {
		return nil, fmt.Errorf("secret vault not available")
	}
	return s.vault.GetSecrets(ctx, s.serviceID, s.accountID, names)
}

func (s *sysSecretsImpl) Set(ctx context.Context, name string, value string) error {
	if s.vault == nil {
		return fmt.Errorf("secret vault not available")
	}
	return s.vault.StoreSecret(ctx, s.serviceID, s.accountID, name, []byte(value))
}

func (s *sysSecretsImpl) Delete(ctx context.Context, name string) error {
	if s.vault == nil {
		return fmt.Errorf("secret vault not available")
	}
	return s.vault.DeleteSecret(ctx, s.serviceID, s.accountID, name)
}

func (s *sysSecretsImpl) List(ctx context.Context) ([]string, error) {
	if s.vault == nil {
		return nil, fmt.Errorf("secret vault not available")
	}
	return s.vault.ListSecrets(ctx, s.serviceID, s.accountID)
}

// =============================================================================
// SysCrypto Implementation - see sys_crypto.go for full implementation
// =============================================================================

// =============================================================================
// SysProof Implementation - see sys_proof.go for full implementation
// =============================================================================

// =============================================================================
// SysStorage Implementation
// =============================================================================

// Note: The actual SysStorage implementation is in sys_storage.go
// This file only contains the factory function to create it.

// newSysStorage creates a new sealed storage instance for the given enclave.
func newSysStorage(enclaveID string, handler OCALLHandler) SysStorage {
	config := DefaultSealedStorageConfig(enclaveID, handler)
	storage, err := NewSealedStorage(config)
	if err != nil {
		// Fall back to a no-op implementation if initialization fails
		return &noopStorageImpl{}
	}
	return storage
}

// noopStorageImpl is a fallback implementation when sealed storage fails to initialize.
type noopStorageImpl struct{}

func (s *noopStorageImpl) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, fmt.Errorf("storage not available")
}

func (s *noopStorageImpl) Set(ctx context.Context, key string, value []byte) error {
	return fmt.Errorf("storage not available")
}

func (s *noopStorageImpl) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("storage not available")
}

func (s *noopStorageImpl) List(ctx context.Context, prefix string) ([]string, error) {
	return nil, fmt.Errorf("storage not available")
}

// =============================================================================
// SysChain Implementation
// =============================================================================

type sysChainImpl struct {
	handler OCALLHandler
}

func (c *sysChainImpl) Call(ctx context.Context, req ChainCallRequest) (*ChainCallResponse, error) {
	if c.handler == nil {
		return nil, fmt.Errorf("OCALL handler not available")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeChainRPC,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := c.handler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, fmt.Errorf("OCALL failed: %w", err)
	}

	if !resp.Success {
		return &ChainCallResponse{Error: resp.Error}, nil
	}

	var callResp ChainCallResponse
	if err := json.Unmarshal(resp.Payload, &callResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &callResp, nil
}

func (c *sysChainImpl) SendTransaction(ctx context.Context, req ChainTxRequest) (*ChainTxResponse, error) {
	if c.handler == nil {
		return nil, fmt.Errorf("OCALL handler not available")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeChainTx,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := c.handler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, fmt.Errorf("OCALL failed: %w", err)
	}

	if !resp.Success {
		return &ChainTxResponse{Status: "failed", Error: resp.Error}, nil
	}

	var txResp ChainTxResponse
	if err := json.Unmarshal(resp.Payload, &txResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &txResp, nil
}

func (c *sysChainImpl) GetBlock(ctx context.Context, chain string, blockNumber int64) (*ChainBlock, error) {
	if c.handler == nil {
		return nil, fmt.Errorf("OCALL handler not available")
	}

	// Build the RPC request for getting block
	callReq := ChainCallRequest{
		Chain:  chain,
		Method: "eth_getBlockByNumber",
		Args:   []any{fmt.Sprintf("0x%x", blockNumber), false},
	}

	payload, err := json.Marshal(callReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeChainRPC,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := c.handler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, fmt.Errorf("OCALL failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("get block failed: %s", resp.Error)
	}

	// Parse the RPC response
	var callResp ChainCallResponse
	if err := json.Unmarshal(resp.Payload, &callResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if callResp.Error != "" {
		return nil, fmt.Errorf("RPC error: %s", callResp.Error)
	}

	// Parse block data from result
	var blockData struct {
		Number       string `json:"number"`
		Hash         string `json:"hash"`
		Timestamp    string `json:"timestamp"`
		Transactions []any  `json:"transactions"`
	}
	if err := json.Unmarshal(callResp.Result, &blockData); err != nil {
		return nil, fmt.Errorf("unmarshal block: %w", err)
	}

	// Parse hex values
	number := parseHexInt64(blockData.Number)
	timestamp := parseHexInt64(blockData.Timestamp)

	return &ChainBlock{
		Number:    number,
		Hash:      blockData.Hash,
		Timestamp: timestamp,
		TxCount:   len(blockData.Transactions),
	}, nil
}

func (c *sysChainImpl) GetTransaction(ctx context.Context, chain string, txHash string) (*ChainTransaction, error) {
	if c.handler == nil {
		return nil, fmt.Errorf("OCALL handler not available")
	}

	// Build the RPC request for getting transaction
	callReq := ChainCallRequest{
		Chain:  chain,
		Method: "eth_getTransactionByHash",
		Args:   []any{txHash},
	}

	payload, err := json.Marshal(callReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeChainRPC,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := c.handler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, fmt.Errorf("OCALL failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("get transaction failed: %s", resp.Error)
	}

	// Parse the RPC response
	var callResp ChainCallResponse
	if err := json.Unmarshal(resp.Payload, &callResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if callResp.Error != "" {
		return nil, fmt.Errorf("RPC error: %s", callResp.Error)
	}

	// Parse transaction data from result
	var txData struct {
		Hash        string `json:"hash"`
		BlockNumber string `json:"blockNumber"`
		From        string `json:"from"`
		To          string `json:"to"`
		Value       string `json:"value"`
	}
	if err := json.Unmarshal(callResp.Result, &txData); err != nil {
		return nil, fmt.Errorf("unmarshal transaction: %w", err)
	}

	blockNumber := parseHexInt64(txData.BlockNumber)

	// Determine status (if blockNumber is set, tx is confirmed)
	status := "pending"
	if blockNumber > 0 {
		status = "confirmed"
	}

	return &ChainTransaction{
		Hash:        txData.Hash,
		BlockNumber: blockNumber,
		From:        txData.From,
		To:          txData.To,
		Value:       txData.Value,
		Status:      status,
	}, nil
}

// parseHexInt64 parses a hex string (with or without 0x prefix) to int64.
func parseHexInt64(s string) int64 {
	if s == "" || s == "0x" {
		return 0
	}
	// Remove 0x prefix if present
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	var result int64
	for _, c := range s {
		result *= 16
		switch {
		case c >= '0' && c <= '9':
			result += int64(c - '0')
		case c >= 'a' && c <= 'f':
			result += int64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			result += int64(c - 'A' + 10)
		}
	}
	return result
}

// =============================================================================
// SysLog Implementation
// =============================================================================

type sysLogImpl struct {
	logs *[]string
}

func (l *sysLogImpl) Debug(msg string, args ...any) {
	*l.logs = append(*l.logs, fmt.Sprintf("[DEBUG] "+msg, args...))
}

func (l *sysLogImpl) Info(msg string, args ...any) {
	*l.logs = append(*l.logs, fmt.Sprintf("[INFO] "+msg, args...))
}

func (l *sysLogImpl) Warn(msg string, args ...any) {
	*l.logs = append(*l.logs, fmt.Sprintf("[WARN] "+msg, args...))
}

func (l *sysLogImpl) Error(msg string, args ...any) {
	*l.logs = append(*l.logs, fmt.Sprintf("[ERROR] "+msg, args...))
}

// =============================================================================
// Helpers
// =============================================================================

var requestCounter int64

func generateRequestID() string {
	requestCounter++
	return fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), requestCounter)
}
