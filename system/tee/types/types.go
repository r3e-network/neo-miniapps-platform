// Package types provides shared type definitions for the TEE subsystem.
// These types are used across the TEE engine, SDK, and service packages.
package types

import (
	"crypto/ecdsa"
	"encoding/json"
	"time"
)

// ============================================================
// HTTP Types
// ============================================================

// HTTPRequest represents an HTTP request.
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
}

// HTTPResponse represents an HTTP response.
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// ============================================================
// Attestation Types
// ============================================================

// AttestationReport represents a TEE attestation report.
type AttestationReport struct {
	// EnclaveID is the unique identifier of this enclave instance
	EnclaveID string `json:"enclave_id"`

	// Quote is the SGX quote (or simulation placeholder)
	Quote []byte `json:"quote,omitempty"`

	// ReportData contains user-provided data included in the report
	ReportData []byte `json:"report_data,omitempty"`

	// MREnclave measurement
	MREnclave string `json:"mr_enclave,omitempty"`

	// MRSigner measurement
	MRSigner string `json:"mr_signer,omitempty"`

	// Mode indicates if running in simulation or hardware mode
	Mode EnclaveMode `json:"mode"`

	// Timestamp when the attestation was generated
	Timestamp time.Time `json:"timestamp"`

	// Signature over the report
	Signature []byte `json:"signature,omitempty"`

	// PublicKey used for signing
	PublicKey []byte `json:"public_key,omitempty"`

	// ProductID for SGX
	ProductID uint16 `json:"product_id,omitempty"`

	// SecurityVer for SGX
	SecurityVer uint16 `json:"security_ver,omitempty"`
}

// EnclaveMode indicates the TEE operation mode.
type EnclaveMode string

const (
	EnclaveModeSimulation EnclaveMode = "simulation"
	EnclaveModeHardware   EnclaveMode = "hardware"
)

// EnclaveInfo contains information about the enclave.
type EnclaveInfo struct {
	EnclaveID   string `json:"enclave_id"`
	Version     string `json:"version"`
	MREnclave   []byte `json:"mr_enclave"`
	MRSigner    []byte `json:"mr_signer"`
	ProductID   uint16 `json:"product_id"`
	SecurityVer uint16 `json:"security_ver"`
	Debug       bool   `json:"debug"`
}

// ============================================================
// Secret Types
// ============================================================

// Secret represents a secret stored in the enclave.
type Secret struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Value       []byte            `json:"-"` // Never serialized
	Type        SecretType        `json:"type"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Permissions []Permission      `json:"permissions"`
}

// SecretType defines the type of secret.
type SecretType string

const (
	SecretTypeGeneric     SecretType = "generic"
	SecretTypeAPIKey      SecretType = "api_key"
	SecretTypePrivateKey  SecretType = "private_key"
	SecretTypeCertificate SecretType = "certificate"
	SecretTypePassword    SecretType = "password"
	SecretTypeToken       SecretType = "token"
)

// Permission represents a permission grant.
type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Scope    string   `json:"scope,omitempty"`
}

// Role represents a role assignment.
type Role string

const (
	RoleAdmin            Role = "admin"
	RoleScheduler        Role = "scheduler"
	RoleOracleRunner     Role = "oracle_runner"
	RoleRandomnessRunner Role = "randomness_runner"
	RoleJamRunner        Role = "jam_runner"
	RoleDataFeedSigner   Role = "data_feed_signer"
	RoleServiceRunner    Role = "service_runner"
)

// ============================================================
// Key Types
// ============================================================

// KeyType defines the type of cryptographic key.
type KeyType string

const (
	KeyTypeECDSA   KeyType = "ecdsa"
	KeyTypeEd25519 KeyType = "ed25519"
	KeyTypeRSA     KeyType = "rsa"
	KeyTypeAES     KeyType = "aes"
)

// KeyCurve defines the elliptic curve for ECDSA keys.
type KeyCurve string

const (
	KeyCurveP256      KeyCurve = "P-256"
	KeyCurveP384      KeyCurve = "P-384"
	KeyCurveSecp256k1 KeyCurve = "secp256k1"
)

// KeyInfo contains metadata about a key.
type KeyInfo struct {
	ID        string    `json:"id"`
	Type      KeyType   `json:"type"`
	Curve     KeyCurve  `json:"curve,omitempty"`
	PublicKey []byte    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
	ParentID  string    `json:"parent_id,omitempty"` // For derived keys
	Path      string    `json:"path,omitempty"`      // HD derivation path
}

// KeyPair represents a cryptographic key pair.
type KeyPair struct {
	KeyID     string `json:"key_id"`
	KeyType   string `json:"key_type"`
	PublicKey []byte `json:"public_key"`
	// Private key never leaves the enclave
}

// ============================================================
// Signing Types
// ============================================================

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

// VerifyRequest is the request to verify a signature.
type VerifyRequest struct {
	PublicKey []byte `json:"public_key"`
	Data      []byte `json:"data"`
	Signature []byte `json:"signature"`
	HashAlg   string `json:"hash_alg,omitempty"`
}

// ============================================================
// Chain Types
// ============================================================

// ChainCallRequest represents a contract call request.
type ChainCallRequest struct {
	Chain    string          `json:"chain"`
	Contract string          `json:"contract"`
	Method   string          `json:"method"`
	Args     []any           `json:"args,omitempty"`
	ABI      json.RawMessage `json:"abi,omitempty"`
}

// ChainCallResponse represents a contract call response.
type ChainCallResponse struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

// ChainTxRequest represents a transaction request.
type ChainTxRequest struct {
	Chain    string `json:"chain"`
	To       string `json:"to"`
	Value    string `json:"value,omitempty"`
	Data     []byte `json:"data,omitempty"`
	GasLimit uint64 `json:"gas_limit,omitempty"`
	Nonce    uint64 `json:"nonce,omitempty"`
}

// ChainTxResponse represents a transaction response.
type ChainTxResponse struct {
	TxHash string `json:"tx_hash"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ChainBlock represents block information.
type ChainBlock struct {
	Number    int64  `json:"number"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
	TxCount   int    `json:"tx_count"`
}

// ChainTransaction represents transaction information.
type ChainTransaction struct {
	Hash        string `json:"hash"`
	BlockNumber int64  `json:"block_number"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Status      string `json:"status"`
}

// ============================================================
// Execution Types
// ============================================================

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
)

// ExecutionProof represents a proof of execution in the TEE.
type ExecutionProof struct {
	// ProofID is a unique identifier for this proof.
	ProofID string `json:"proof_id"`

	// EnclaveID identifies the enclave that generated the proof.
	EnclaveID string `json:"enclave_id"`

	// InputHash is the hash of the input data.
	InputHash string `json:"input_hash"`

	// OutputHash is the hash of the output data.
	OutputHash string `json:"output_hash"`

	// Timestamp when the proof was generated.
	Timestamp time.Time `json:"timestamp"`

	// Signature over the proof data.
	Signature []byte `json:"signature"`

	// AttestationQuote is the SGX quote (optional).
	AttestationQuote []byte `json:"attestation_quote,omitempty"`
}

// ============================================================
// ECALL/OCALL Types
// ============================================================

// OCALLType defines the type of OCALL (outbound call from enclave).
type OCALLType string

const (
	OCALLTypeHTTP     OCALLType = "http"
	OCALLTypeChainRPC OCALLType = "chain_rpc"
	OCALLTypeChainTx  OCALLType = "chain_tx"
	OCALLTypeStorage  OCALLType = "storage"
	OCALLTypeLog      OCALLType = "log"
)

// OCALLRequest represents an outbound call from the enclave.
type OCALLRequest struct {
	Type      OCALLType       `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
	Timeout   time.Duration   `json:"timeout,omitempty"`
}

// OCALLResponse represents the response to an OCALL.
type OCALLResponse struct {
	RequestID string          `json:"request_id"`
	Success   bool            `json:"success"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// ECALLType defines the type of ECALL (inbound call to enclave).
type ECALLType string

const (
	ECALLTypeExecute     ECALLType = "execute"
	ECALLTypeGetSecret   ECALLType = "get_secret"
	ECALLTypeSetSecret   ECALLType = "set_secret"
	ECALLTypeAttestation ECALLType = "attestation"
	ECALLTypeHealth      ECALLType = "health"
)

// ECALLRequest represents an inbound call to the enclave.
type ECALLRequest struct {
	Type      ECALLType       `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

// ECALLResponse represents the response from an ECALL.
type ECALLResponse struct {
	RequestID string          `json:"request_id"`
	Success   bool            `json:"success"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
	Proof     *ExecutionProof `json:"proof,omitempty"`
}

// ============================================================
// Signer Interface (for cross-package use)
// ============================================================

// Signer provides signing capabilities.
type Signer interface {
	// Sign signs data and returns the signature.
	Sign(data []byte) ([]byte, error)

	// GetPublicKey returns the public key.
	GetPublicKey() []byte

	// GetSigningKey returns the ECDSA private key (internal use).
	GetSigningKey() *ecdsa.PrivateKey
}
