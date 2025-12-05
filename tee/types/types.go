// Package types defines all interfaces and types for the TEE (Trusted Execution Environment).
// This is the foundation layer - all types are defined here to avoid circular dependencies.
//
// Architecture:
//
//	TEE is the trust root of the entire system. All sensitive operations
//	(secrets, keys, network with credentials) happen inside the enclave.
//	Data NEVER leaves the enclave in plaintext.
package types

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// =============================================================================
// Core Errors
// =============================================================================

var (
	ErrEnclaveNotReady     = errors.New("enclave not ready")
	ErrSecretNotFound      = errors.New("secret not found")
	ErrSecretAccessDenied  = errors.New("secret access denied")
	ErrKeyNotFound         = errors.New("key not found")
	ErrInvalidKeyHandle    = errors.New("invalid key handle")
	ErrAttestationFailed   = errors.New("attestation failed")
	ErrNetworkNotAllowed   = errors.New("network request not allowed")
	ErrComputeTimeout      = errors.New("compute timeout")
	ErrMemoryLimitExceeded = errors.New("memory limit exceeded")
)

// =============================================================================
// Enclave Mode
// =============================================================================

// EnclaveMode specifies the TEE operation mode.
type EnclaveMode string

const (
	EnclaveModeSimulation EnclaveMode = "simulation"
	EnclaveModeHardware   EnclaveMode = "hardware"
)

// =============================================================================
// TrustRoot - The Foundation Interface
// =============================================================================

// TrustRoot is the foundation of all secure operations.
// It provides access to all TEE capabilities.
type TrustRoot interface {
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error

	// Mode
	Mode() EnclaveMode

	// Core capabilities
	Vault() SecureVault
	Network() SecureNetwork
	Keys() KeyManager
	Compute() ConfidentialCompute
	Attestation() Attestor
	Neo() NeoSigner // Neo N3 blockchain operations
}

// =============================================================================
// SecureVault - Secret Management
// =============================================================================

// SecretConsumer is called with the secret value inside the enclave.
// The secret is ONLY available inside this callback and is zeroed after.
type SecretConsumer func(secret []byte) error

// MultiSecretConsumer is called with multiple secrets inside the enclave.
type MultiSecretConsumer func(secrets map[string][]byte) error

// SecureVault manages secrets inside the TEE enclave.
// Secrets are encrypted at rest and NEVER leave the enclave in plaintext.
type SecureVault interface {
	// Store encrypts and stores a secret.
	Store(ctx context.Context, namespace, name string, value []byte) error

	// Use executes a function with access to a secret.
	// The secret is ONLY available inside the callback.
	// This is the ONLY way to access secret values - they are never returned.
	Use(ctx context.Context, namespace, name string, fn SecretConsumer) error

	// UseMultiple executes a function with access to multiple secrets.
	UseMultiple(ctx context.Context, refs []SecretRef, fn MultiSecretConsumer) error

	// Delete removes a secret.
	Delete(ctx context.Context, namespace, name string) error

	// List returns secret names (not values) in a namespace.
	List(ctx context.Context, namespace string) ([]string, error)

	// Exists checks if a secret exists.
	Exists(ctx context.Context, namespace, name string) (bool, error)
}

// SecretRef references a secret to be made available during computation.
type SecretRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Alias     string `json:"alias,omitempty"` // Optional alias for the secret
}

// =============================================================================
// SecureNetwork - Networking with TLS Inside Enclave
// =============================================================================

// SecureNetwork provides networking with TLS termination inside the enclave.
// Credentials are injected inside the enclave and never leave.
type SecureNetwork interface {
	// Fetch performs an HTTP request with TLS inside the enclave.
	Fetch(ctx context.Context, req SecureHTTPRequest) (*SecureHTTPResponse, error)

	// FetchWithSecret performs HTTP with secret-based auth.
	// The secret is retrieved from vault and injected inside the enclave.
	FetchWithSecret(ctx context.Context, req SecureHTTPRequest, namespace, secretName string, authType AuthType) (*SecureHTTPResponse, error)

	// RPC performs a JSON-RPC call.
	RPC(ctx context.Context, endpoint, method string, params any) (json.RawMessage, error)

	// RPCWithSecret performs RPC with secret-based auth.
	RPCWithSecret(ctx context.Context, endpoint, method string, params any, namespace, secretName string) (json.RawMessage, error)
}

// SecureHTTPRequest represents an HTTP request to be made inside the enclave.
type SecureHTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
}

// SecureHTTPResponse represents an HTTP response received inside the enclave.
type SecureHTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// AuthType specifies how to use a secret for authentication.
type AuthType string

const (
	AuthTypeBearer AuthType = "bearer"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeCustom AuthType = "custom"
)

// =============================================================================
// KeyManager - HSM-like Key Management
// =============================================================================

// KeyHandle is an opaque reference to a key inside the enclave.
// The actual private key NEVER leaves the enclave.
type KeyHandle string

// ChainType specifies the blockchain type for address derivation.
type ChainType string

const (
	ChainTypeEthereum ChainType = "ethereum"
	ChainTypeNeo      ChainType = "neo"
	ChainTypeBitcoin  ChainType = "bitcoin"
)

// KeyManager provides HSM-like key management inside the enclave.
// Private keys NEVER leave the enclave - only handles and public keys are exported.
type KeyManager interface {
	// DeriveKey derives a key from the master seed using the given path.
	// Returns a handle to the key (the actual key stays in the enclave).
	DeriveKey(ctx context.Context, path string) (KeyHandle, error)

	// Sign signs data with a key (signing happens inside the enclave).
	Sign(ctx context.Context, handle KeyHandle, data []byte) ([]byte, error)

	// Verify verifies a signature.
	Verify(ctx context.Context, publicKey, data, signature []byte) (bool, error)

	// GetPublicKey returns the public key (safe to export).
	GetPublicKey(ctx context.Context, handle KeyHandle) ([]byte, error)

	// GetAddress returns the blockchain address for a key.
	GetAddress(ctx context.Context, handle KeyHandle, chain ChainType) (string, error)
}

// =============================================================================
// ConfidentialCompute - Script Execution with Secrets
// =============================================================================

// ComputeStatus represents the status of a computation.
type ComputeStatus string

const (
	ComputeStatusPending   ComputeStatus = "pending"
	ComputeStatusRunning   ComputeStatus = "running"
	ComputeStatusSucceeded ComputeStatus = "succeeded"
	ComputeStatusFailed    ComputeStatus = "failed"
	ComputeStatusTimeout   ComputeStatus = "timeout"
)

// ComputeRequest specifies what to execute inside the enclave.
type ComputeRequest struct {
	ServiceID  string         `json:"service_id"`
	Script     string         `json:"script"`
	EntryPoint string         `json:"entry_point"`
	Input      map[string]any `json:"input,omitempty"`
	Timeout    time.Duration  `json:"timeout,omitempty"`
}

// ComputeResult contains the result of confidential computation.
type ComputeResult struct {
	Status      ComputeStatus  `json:"status"`
	Output      map[string]any `json:"output,omitempty"`
	Logs        []string       `json:"logs,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Duration    time.Duration  `json:"duration"`
}

// ConfidentialCompute executes code with data confidentiality guarantees.
type ConfidentialCompute interface {
	// Execute runs code inside the enclave.
	Execute(ctx context.Context, req ComputeRequest) (*ComputeResult, error)

	// ExecuteWithSecrets runs code with access to secrets.
	// Secrets are injected inside the enclave and zeroed after execution.
	ExecuteWithSecrets(ctx context.Context, req ComputeRequest, secretRefs []SecretRef) (*ComputeResult, error)
}

// =============================================================================
// Attestor - Remote Attestation
// =============================================================================

// Quote represents an SGX quote for remote attestation.
// Extended with MarbleRun-compatible fields for package verification.
type Quote struct {
	RawQuote  []byte    `json:"raw_quote"`
	UserData  []byte    `json:"user_data"`
	MREnclave string    `json:"mr_enclave"` // MRENCLAVE - hash of enclave code
	MRSigner  string    `json:"mr_signer"`  // MRSIGNER - hash of signing key
	Timestamp time.Time `json:"timestamp"`

	// MarbleRun-compatible fields for package verification
	ProductID       uint16 `json:"product_id"`       // Product ID from enclave
	SecurityVersion uint16 `json:"security_version"` // Security Version Number (SVN)
	TCBStatus       string `json:"tcb_status"`       // TCB status: UpToDate, SWHardeningNeeded, etc.
	Debug           bool   `json:"debug"`            // True if enclave is in debug mode
}

// QuoteVerification contains the result of quote verification.
type QuoteVerification struct {
	Valid      bool      `json:"valid"`
	MREnclave  string    `json:"mr_enclave"`
	MRSigner   string    `json:"mr_signer"`
	VerifiedAt time.Time `json:"verified_at"`
}

// AttestationReport contains enclave attestation information.
type AttestationReport struct {
	EnclaveID string    `json:"enclave_id"`
	Mode      string    `json:"mode"`
	MREnclave string    `json:"mr_enclave"`
	MRSigner  string    `json:"mr_signer"`
	Timestamp time.Time `json:"timestamp"`
}

// Attestor provides remote attestation capabilities.
type Attestor interface {
	// GenerateQuote generates a quote for remote attestation.
	GenerateQuote(ctx context.Context, userData []byte) (*Quote, error)

	// VerifyQuote verifies a quote.
	VerifyQuote(ctx context.Context, quote *Quote) (*QuoteVerification, error)

	// GetReport returns the current attestation report.
	GetReport(ctx context.Context) (*AttestationReport, error)
}

// =============================================================================
// NeoSigner - Neo N3 Blockchain Operations
// =============================================================================

// WalletHandle is an opaque reference to a Neo wallet inside the enclave.
// The actual private key NEVER leaves the enclave.
type WalletHandle string

// NeoSigner handles Neo N3 blockchain operations inside the TEE enclave.
// Private keys NEVER leave the enclave - all signing happens inside.
type NeoSigner interface {
	// CreateWallet creates a new Neo wallet inside the enclave.
	// Returns a handle to the wallet (the actual key stays in the enclave).
	CreateWallet(ctx context.Context, path string) (WalletHandle, error)

	// DeriveWallet derives a Neo wallet from a seed path.
	DeriveWallet(ctx context.Context, seedPath string) (WalletHandle, error)

	// ImportWIF imports a wallet from WIF (Wallet Import Format).
	// The WIF is decrypted inside the enclave and never leaves.
	ImportWIF(ctx context.Context, wif string) (WalletHandle, error)

	// GetAddress returns the Neo N3 address for a wallet handle.
	GetAddress(ctx context.Context, handle WalletHandle) (string, error)

	// GetPublicKey returns the compressed public key for a wallet handle.
	GetPublicKey(ctx context.Context, handle WalletHandle) ([]byte, error)

	// GetScriptHash returns the script hash for a wallet handle.
	GetScriptHash(ctx context.Context, handle WalletHandle) ([]byte, error)

	// SignData signs arbitrary data with a wallet.
	// Signing happens inside the enclave - private key never leaves.
	SignData(ctx context.Context, handle WalletHandle, data []byte) ([]byte, error)

	// SignTransaction signs a Neo N3 transaction hash.
	// The transaction is signed inside the enclave.
	SignTransaction(ctx context.Context, handle WalletHandle, txHash []byte) ([]byte, error)

	// VerifySignature verifies a signature against a public key.
	VerifySignature(ctx context.Context, publicKey, data, signature []byte) (bool, error)

	// ListWallets returns all wallet handles.
	ListWallets(ctx context.Context) ([]WalletHandle, error)

	// DeleteWallet removes a wallet from the enclave.
	DeleteWallet(ctx context.Context, handle WalletHandle) error
}

// =============================================================================
// TEE Error Type
// =============================================================================

// TEEError represents a TEE-specific error.
type TEEError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *TEEError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *TEEError) Unwrap() error {
	return e.Cause
}

// NewTEEError creates a new TEE error.
func NewTEEError(code, message string, cause error) *TEEError {
	return &TEEError{Code: code, Message: message, Cause: cause}
}
