// Package sdk provides the Enclave SDK for Go/JS script execution within TEE.
// This SDK enables scripts to interact with the enclave environment securely,
// including secret key operations, permission verification, transaction signing,
// and secure HTTPS connections.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/system/os/tee/types"
)

// Type aliases for backward compatibility - use types from shared package
type (
	Secret            = types.Secret
	SecretType        = types.SecretType
	Permission        = types.Permission
	Role              = types.Role
	KeyType           = types.KeyType
	KeyCurve          = types.KeyCurve
	KeyInfo           = types.KeyInfo
	HTTPRequest       = types.HTTPRequest
	HTTPResponse      = types.HTTPResponse
	AttestationReport = types.AttestationReport
	EnclaveInfo       = types.EnclaveInfo
)

// Re-export constants from types package
const (
	SecretTypeGeneric     = types.SecretTypeGeneric
	SecretTypeAPIKey      = types.SecretTypeAPIKey
	SecretTypePrivateKey  = types.SecretTypePrivateKey
	SecretTypeCertificate = types.SecretTypeCertificate
	SecretTypePassword    = types.SecretTypePassword
	SecretTypeToken       = types.SecretTypeToken

	KeyTypeECDSA   = types.KeyTypeECDSA
	KeyTypeEd25519 = types.KeyTypeEd25519
	KeyTypeRSA     = types.KeyTypeRSA
	KeyTypeAES     = types.KeyTypeAES

	KeyCurveP256      = types.KeyCurveP256
	KeyCurveP384      = types.KeyCurveP384
	KeyCurveSecp256k1 = types.KeyCurveSecp256k1

	RoleAdmin            = types.RoleAdmin
	RoleScheduler        = types.RoleScheduler
	RoleOracleRunner     = types.RoleOracleRunner
	RoleRandomnessRunner = types.RoleRandomnessRunner
	RoleJamRunner        = types.RoleJamRunner
	RoleDataFeedSigner   = types.RoleDataFeedSigner
	RoleServiceRunner    = types.RoleServiceRunner
)

// ============================================================
// Error Definitions
// ============================================================

var (
	ErrSecretNotFound      = errors.New("secret not found")
	ErrSecretExists        = errors.New("secret already exists")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrKeyNotFound         = errors.New("key not found")
	ErrInvalidKey          = errors.New("invalid key")
	ErrEnclaveSealFailed   = errors.New("enclave seal failed")
	ErrEnclaveUnsealFailed = errors.New("enclave unseal failed")
	ErrAttestationFailed   = errors.New("attestation failed")
	ErrHTTPRequestFailed   = errors.New("HTTP request failed")
	ErrTimeout             = errors.New("operation timeout")
)

// ============================================================
// Core Interfaces
// ============================================================

// EnclaveSDK is the main interface for scripts to interact with the enclave.
type EnclaveSDK interface {
	// Secret Management
	Secrets() SecretsManager

	// Key Management
	Keys() KeyManager

	// Permission Verification
	Permissions() PermissionManager

	// Transaction Signing
	Signer() TransactionSigner

	// Secure HTTP Client
	HTTP() SecureHTTPClient

	// Attestation
	Attestation() AttestationProvider

	// Context for the current execution
	Context() ExecutionContext
}

// ExecutionContext provides context about the current script execution.
type ExecutionContext interface {
	// RequestID returns the current request identifier
	RequestID() string

	// ServiceID returns the service package identifier
	ServiceID() string

	// CallerID returns the caller's account/contract identifier
	CallerID() string

	// Deadline returns the execution deadline
	Deadline() time.Time

	// Metadata returns execution metadata
	Metadata() map[string]string
}

// ============================================================
// Secrets Manager Interface
// ============================================================

// SecretsManager handles secret key operations within the enclave.
type SecretsManager interface {
	// Add stores a new secret in the enclave's secure storage.
	// The secret is encrypted using the enclave's sealing key.
	Add(ctx context.Context, req *AddSecretRequest) (*AddSecretResponse, error)

	// Update modifies an existing secret.
	Update(ctx context.Context, req *UpdateSecretRequest) (*UpdateSecretResponse, error)

	// Delete removes a secret from secure storage.
	Delete(ctx context.Context, req *DeleteSecretRequest) error

	// Find retrieves a secret by name or pattern.
	Find(ctx context.Context, req *FindSecretRequest) (*FindSecretResponse, error)

	// Get retrieves a specific secret by ID.
	Get(ctx context.Context, secretID string) (*Secret, error)

	// List returns all secrets accessible to the caller.
	List(ctx context.Context, req *ListSecretsRequest) (*ListSecretsResponse, error)

	// Exists checks if a secret exists.
	Exists(ctx context.Context, secretID string) (bool, error)
}

// AddSecretRequest is the request to add a new secret.
type AddSecretRequest struct {
	Name        string            `json:"name"`
	Value       []byte            `json:"value"`
	Type        SecretType        `json:"type"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Permissions []Permission      `json:"permissions,omitempty"`
}

// AddSecretResponse is the response after adding a secret.
type AddSecretResponse struct {
	SecretID  string    `json:"secret_id"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateSecretRequest is the request to update a secret.
type UpdateSecretRequest struct {
	SecretID    string            `json:"secret_id"`
	Value       []byte            `json:"value,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Permissions []Permission      `json:"permissions,omitempty"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
}

// UpdateSecretResponse is the response after updating a secret.
type UpdateSecretResponse struct {
	SecretID  string    `json:"secret_id"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeleteSecretRequest is the request to delete a secret.
type DeleteSecretRequest struct {
	SecretID string `json:"secret_id"`
}

// FindSecretRequest is the request to find secrets.
type FindSecretRequest struct {
	NamePattern string            `json:"name_pattern,omitempty"`
	Type        SecretType        `json:"type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
}

// FindSecretResponse is the response containing found secrets.
type FindSecretResponse struct {
	Secrets    []*Secret `json:"secrets"`
	TotalCount int       `json:"total_count"`
}

// ListSecretsRequest is the request to list secrets.
type ListSecretsRequest struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ListSecretsResponse is the response containing listed secrets.
type ListSecretsResponse struct {
	Secrets    []*Secret `json:"secrets"`
	TotalCount int       `json:"total_count"`
}

// ============================================================
// Key Manager Interface
// ============================================================

// KeyManager handles cryptographic key operations within the enclave.
type KeyManager interface {
	// GenerateKey generates a new key pair within the enclave.
	GenerateKey(ctx context.Context, req *GenerateKeyRequest) (*GenerateKeyResponse, error)

	// ImportKey imports an existing key into the enclave.
	ImportKey(ctx context.Context, req *ImportKeyRequest) (*ImportKeyResponse, error)

	// ExportPublicKey exports the public key (private key never leaves enclave).
	ExportPublicKey(ctx context.Context, keyID string) ([]byte, error)

	// DeleteKey removes a key from the enclave.
	DeleteKey(ctx context.Context, keyID string) error

	// ListKeys returns all keys accessible to the caller.
	ListKeys(ctx context.Context) ([]*KeyInfo, error)

	// DeriveKey derives a child key using HD derivation.
	DeriveKey(ctx context.Context, req *DeriveKeyRequest) (*DeriveKeyResponse, error)
}

// GenerateKeyRequest is the request to generate a new key.
type GenerateKeyRequest struct {
	Type  KeyType  `json:"type"`
	Curve KeyCurve `json:"curve,omitempty"`
	Label string   `json:"label,omitempty"`
}

// GenerateKeyResponse is the response after generating a key.
type GenerateKeyResponse struct {
	KeyID     string    `json:"key_id"`
	PublicKey []byte    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
}

// ImportKeyRequest is the request to import a key.
type ImportKeyRequest struct {
	Type       KeyType  `json:"type"`
	Curve      KeyCurve `json:"curve,omitempty"`
	PrivateKey []byte   `json:"private_key"`
	Label      string   `json:"label,omitempty"`
}

// ImportKeyResponse is the response after importing a key.
type ImportKeyResponse struct {
	KeyID     string    `json:"key_id"`
	PublicKey []byte    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
}

// DeriveKeyRequest is the request to derive a child key.
type DeriveKeyRequest struct {
	ParentKeyID string `json:"parent_key_id"`
	Path        string `json:"path"` // BIP32/BIP44 path
	Label       string `json:"label,omitempty"`
}

// DeriveKeyResponse is the response after deriving a key.
type DeriveKeyResponse struct {
	KeyID     string    `json:"key_id"`
	PublicKey []byte    `json:"public_key"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Permission Manager Interface
// ============================================================

// PermissionManager handles permission verification within the enclave.
type PermissionManager interface {
	// Check verifies if the caller has the specified permission.
	Check(ctx context.Context, permission string) (bool, error)

	// CheckAll verifies if the caller has all specified permissions.
	CheckAll(ctx context.Context, permissions []string) (bool, error)

	// CheckAny verifies if the caller has any of the specified permissions.
	CheckAny(ctx context.Context, permissions []string) (bool, error)

	// GetCallerPermissions returns all permissions for the current caller.
	GetCallerPermissions(ctx context.Context) ([]Permission, error)

	// VerifyRole verifies if the caller has the specified role.
	VerifyRole(ctx context.Context, role Role) (bool, error)

	// GetCallerRoles returns all roles for the current caller.
	GetCallerRoles(ctx context.Context) ([]Role, error)
}

// ============================================================
// Transaction Signer Interface
// ============================================================

// TransactionSigner handles transaction signing within the enclave.
type TransactionSigner interface {
	// Sign signs arbitrary data with the specified key.
	Sign(ctx context.Context, req *SignRequest) (*SignResponse, error)

	// SignTransaction signs a blockchain transaction.
	SignTransaction(ctx context.Context, req *SignTransactionRequest) (*SignTransactionResponse, error)

	// SignMessage signs a message with EIP-191 or similar standard.
	SignMessage(ctx context.Context, req *SignMessageRequest) (*SignMessageResponse, error)

	// Verify verifies a signature.
	Verify(ctx context.Context, req *VerifyRequest) (bool, error)

	// GetSigningKey returns the public key for signing operations.
	GetSigningKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error)
}

// SignRequest is the request to sign data.
type SignRequest struct {
	KeyID   string `json:"key_id"`
	Data    []byte `json:"data"`
	HashAlg string `json:"hash_alg,omitempty"` // sha256, sha384, keccak256
}

// SignResponse is the response containing the signature.
type SignResponse struct {
	Signature []byte `json:"signature"`
	PublicKey []byte `json:"public_key"`
	Algorithm string `json:"algorithm"`
}

// SignTransactionRequest is the request to sign a transaction.
type SignTransactionRequest struct {
	KeyID       string `json:"key_id"`
	Transaction []byte `json:"transaction"` // Serialized transaction
	ChainID     string `json:"chain_id"`
	TxType      string `json:"tx_type"` // neo, ethereum, etc.
}

// SignTransactionResponse is the response containing the signed transaction.
type SignTransactionResponse struct {
	SignedTransaction []byte `json:"signed_transaction"`
	TxHash            []byte `json:"tx_hash"`
	Signature         []byte `json:"signature"`
}

// SignMessageRequest is the request to sign a message.
type SignMessageRequest struct {
	KeyID   string `json:"key_id"`
	Message []byte `json:"message"`
	Format  string `json:"format"` // eip191, eip712, raw
}

// SignMessageResponse is the response containing the signed message.
type SignMessageResponse struct {
	Signature   []byte `json:"signature"`
	MessageHash []byte `json:"message_hash"`
	PublicKey   []byte `json:"public_key"`
}

// VerifyRequest is the request to verify a signature.
type VerifyRequest struct {
	PublicKey []byte `json:"public_key"`
	Data      []byte `json:"data"`
	Signature []byte `json:"signature"`
	HashAlg   string `json:"hash_alg,omitempty"`
}

// ============================================================
// Secure HTTP Client Interface
// ============================================================

// SecureHTTPClient provides secure HTTPS connections from within the enclave.
type SecureHTTPClient interface {
	// Get performs a secure GET request.
	Get(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error)

	// Post performs a secure POST request.
	Post(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error)

	// Put performs a secure PUT request.
	Put(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error)

	// Delete performs a secure DELETE request.
	Delete(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error)

	// Do performs a custom HTTP request.
	Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

	// SetTLSConfig sets custom TLS configuration.
	SetTLSConfig(config *tls.Config)

	// AddTrustedCert adds a trusted certificate for TLS verification.
	AddTrustedCert(cert []byte) error
}

// HTTPOption is a functional option for HTTP requests.
type HTTPOption func(*httpOptions)

type httpOptions struct {
	headers map[string]string
	timeout time.Duration
	auth    *HTTPAuth
}

// HTTPAuth represents HTTP authentication.
type HTTPAuth struct {
	Type     string `json:"type"` // basic, bearer, api_key
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	Header   string `json:"header,omitempty"` // For api_key
}

// WithHeader adds a header to the request.
func WithHeader(key, value string) HTTPOption {
	return func(o *httpOptions) {
		if o.headers == nil {
			o.headers = make(map[string]string)
		}
		o.headers[key] = value
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) HTTPOption {
	return func(o *httpOptions) {
		o.timeout = d
	}
}

// WithAuth sets the authentication for the request.
func WithAuth(auth *HTTPAuth) HTTPOption {
	return func(o *httpOptions) {
		o.auth = auth
	}
}

// WithBearerToken sets bearer token authentication.
func WithBearerToken(token string) HTTPOption {
	return func(o *httpOptions) {
		o.auth = &HTTPAuth{Type: "bearer", Token: token}
	}
}

// WithAPIKey sets API key authentication.
func WithAPIKey(header, key string) HTTPOption {
	return func(o *httpOptions) {
		o.auth = &HTTPAuth{Type: "api_key", Header: header, Token: key}
	}
}

// ============================================================
// Attestation Provider Interface
// ============================================================

// AttestationProvider handles TEE attestation operations.
type AttestationProvider interface {
	// GenerateReport generates a TEE attestation report.
	GenerateReport(ctx context.Context, userData []byte) (*AttestationReport, error)

	// VerifyReport verifies a TEE attestation report.
	VerifyReport(ctx context.Context, report *AttestationReport) (bool, error)

	// GetEnclaveInfo returns information about the current enclave.
	GetEnclaveInfo(ctx context.Context) (*EnclaveInfo, error)

	// GetQuote generates a quote for remote attestation.
	GetQuote(ctx context.Context, reportData []byte) ([]byte, error)
}

// ============================================================
// SDK Factory
// ============================================================

// Config holds configuration for the Enclave SDK.
type Config struct {
	ServiceID     string
	RequestID     string
	CallerID      string
	Deadline      time.Time
	Metadata      map[string]string
	HTTPTransport http.RoundTripper
}

// New creates a new Enclave SDK instance.
// This is called by the enclave runtime when executing a script.
func New(cfg *Config) EnclaveSDK {
	return &enclaveSDK{
		config: cfg,
	}
}

// enclaveSDK is the default implementation of EnclaveSDK.
type enclaveSDK struct {
	config      *Config
	secrets     SecretsManager
	keys        KeyManager
	permissions PermissionManager
	signer      TransactionSigner
	http        SecureHTTPClient
	attestation AttestationProvider
}

func (s *enclaveSDK) Secrets() SecretsManager           { return s.secrets }
func (s *enclaveSDK) Keys() KeyManager                  { return s.keys }
func (s *enclaveSDK) Permissions() PermissionManager    { return s.permissions }
func (s *enclaveSDK) Signer() TransactionSigner         { return s.signer }
func (s *enclaveSDK) HTTP() SecureHTTPClient            { return s.http }
func (s *enclaveSDK) Attestation() AttestationProvider  { return s.attestation }
func (s *enclaveSDK) Context() ExecutionContext         { return &executionContext{config: s.config} }

type executionContext struct {
	config *Config
}

func (c *executionContext) RequestID() string          { return c.config.RequestID }
func (c *executionContext) ServiceID() string          { return c.config.ServiceID }
func (c *executionContext) CallerID() string           { return c.config.CallerID }
func (c *executionContext) Deadline() time.Time        { return c.config.Deadline }
func (c *executionContext) Metadata() map[string]string { return c.config.Metadata }
