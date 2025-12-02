package tee

import (
	"context"
	"fmt"
)

// SGX Bridge Layer
//
// This file defines the interface for bridging to Rust SGX SDK.
// In simulation mode, these are no-ops.
// In hardware mode, these call into Rust via CGO.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                      Go Service Layer                        │
//	│  ┌─────────────────────────────────────────────────────────┐ │
//	│  │                    TEE Engine (Go)                       │ │
//	│  │  - Secret Manager                                        │ │
//	│  │  - Script Engine (V8)                                    │ │
//	│  │  - Attestation                                           │ │
//	│  └────────────────────────┬────────────────────────────────┘ │
//	│                           │ CGO                              │
//	│  ┌────────────────────────▼────────────────────────────────┐ │
//	│  │                  Rust SGX Bridge                         │ │
//	│  │  - Enclave lifecycle                                     │ │
//	│  │  - Sealing/Unsealing                                     │ │
//	│  │  - Remote Attestation                                    │ │
//	│  └────────────────────────┬────────────────────────────────┘ │
//	└───────────────────────────│─────────────────────────────────┘
//	                            │
//	┌───────────────────────────▼─────────────────────────────────┐
//	│                    SGX Enclave (Rust)                        │
//	│  ┌─────────────────────────────────────────────────────────┐ │
//	│  │                 Occlum OS + Node.js                      │ │
//	│  │  - V8 JavaScript Engine                                  │ │
//	│  │  - Sealed Storage                                        │ │
//	│  │  - Crypto Operations                                     │ │
//	│  └─────────────────────────────────────────────────────────┘ │
//	└─────────────────────────────────────────────────────────────┘

// SGXBridge defines the interface for Rust SGX SDK operations.
// This will be implemented via CGO when hardware mode is enabled.
type SGXBridge interface {
	// Enclave Lifecycle
	InitializeEnclave(config SGXEnclaveConfig) error
	DestroyEnclave() error
	GetEnclaveStatus() (*SGXEnclaveStatus, error)

	// Sealing Operations (using enclave sealing key)
	SealData(plaintext []byte, additionalData []byte) ([]byte, error)
	UnsealData(sealed []byte, additionalData []byte) ([]byte, error)

	// Remote Attestation
	GenerateQuote(reportData []byte) (*SGXQuote, error)
	VerifyQuote(quote *SGXQuote) (*SGXQuoteVerification, error)

	// Crypto Operations (inside enclave)
	GenerateKeyPair(keyType SGXKeyType) (*SGXKeyPair, error)
	Sign(keyID string, data []byte) ([]byte, error)
	Verify(keyID string, data []byte, signature []byte) (bool, error)
	Encrypt(keyID string, plaintext []byte) ([]byte, error)
	Decrypt(keyID string, ciphertext []byte) ([]byte, error)

	// Script Execution (via Occlum + Node.js)
	ExecuteScript(req SGXScriptRequest) (*SGXScriptResult, error)
}

// SGXEnclaveConfig configures the SGX enclave.
type SGXEnclaveConfig struct {
	// EnclavePath is the path to the signed enclave binary
	EnclavePath string `json:"enclave_path"`

	// Debug enables debug mode (not for production)
	Debug bool `json:"debug"`

	// HeapSize in bytes
	HeapSize uint64 `json:"heap_size"`

	// StackSize in bytes
	StackSize uint64 `json:"stack_size"`

	// ThreadCount for the enclave
	ThreadCount uint32 `json:"thread_count"`

	// OcclumConfig for the Occlum OS
	OcclumConfig *OcclumConfig `json:"occlum_config,omitempty"`
}

// OcclumConfig configures the Occlum OS within the enclave.
type OcclumConfig struct {
	// ImagePath is the path to the Occlum image
	ImagePath string `json:"image_path"`

	// KernelHeapSize in bytes
	KernelHeapSize uint64 `json:"kernel_heap_size"`

	// UserSpaceSize in bytes
	UserSpaceSize uint64 `json:"user_space_size"`

	// MaxNumProcesses
	MaxNumProcesses uint32 `json:"max_num_processes"`

	// NodeJSPath within the Occlum image
	NodeJSPath string `json:"nodejs_path"`
}

// SGXEnclaveStatus represents the current enclave state.
type SGXEnclaveStatus struct {
	Initialized bool   `json:"initialized"`
	EnclaveID   uint64 `json:"enclave_id"`
	Debug       bool   `json:"debug"`
	HeapUsed    uint64 `json:"heap_used"`
	HeapFree    uint64 `json:"heap_free"`
}

// SGXQuote represents an SGX quote for remote attestation.
type SGXQuote struct {
	// Version of the quote structure
	Version uint16 `json:"version"`

	// SignType (EPID or ECDSA)
	SignType uint16 `json:"sign_type"`

	// EPID Group ID
	EPIDGroupID [4]byte `json:"epid_group_id"`

	// QE SVN (Quoting Enclave Security Version Number)
	QESVN uint16 `json:"qe_svn"`

	// PCE SVN (Provisioning Certification Enclave SVN)
	PCESVN uint16 `json:"pce_svn"`

	// MRENCLAVE (256-bit hash of enclave code and data)
	MREnclave [32]byte `json:"mr_enclave"`

	// MRSIGNER (256-bit hash of enclave signer's public key)
	MRSigner [32]byte `json:"mr_signer"`

	// ISV Product ID
	ISVProdID uint16 `json:"isv_prod_id"`

	// ISV SVN (Independent Software Vendor Security Version Number)
	ISVSVN uint16 `json:"isv_svn"`

	// Report Data (64 bytes of user data)
	ReportData [64]byte `json:"report_data"`

	// Signature over the quote
	Signature []byte `json:"signature"`

	// Raw quote bytes
	Raw []byte `json:"raw"`
}

// SGXQuoteVerification contains the result of quote verification.
type SGXQuoteVerification struct {
	Valid           bool   `json:"valid"`
	TrustedMREnclave bool   `json:"trusted_mr_enclave"`
	TrustedMRSigner  bool   `json:"trusted_mr_signer"`
	DebugEnclave    bool   `json:"debug_enclave"`
	OutOfDate       bool   `json:"out_of_date"`
	ConfigNeeded    bool   `json:"config_needed"`
	Message         string `json:"message,omitempty"`
}

// SGXKeyType defines the type of key to generate.
type SGXKeyType string

const (
	SGXKeyTypeECDSA   SGXKeyType = "ecdsa_p256"
	SGXKeyTypeRSA2048 SGXKeyType = "rsa_2048"
	SGXKeyTypeRSA4096 SGXKeyType = "rsa_4096"
	SGXKeyTypeAES128  SGXKeyType = "aes_128"
	SGXKeyTypeAES256  SGXKeyType = "aes_256"
)

// SGXKeyPair represents a key pair generated in the enclave.
type SGXKeyPair struct {
	KeyID     string     `json:"key_id"`
	KeyType   SGXKeyType `json:"key_type"`
	PublicKey []byte     `json:"public_key"`
	// Private key never leaves the enclave
}

// SGXScriptRequest is a request to execute a script in the enclave.
type SGXScriptRequest struct {
	Script      string            `json:"script"`
	EntryPoint  string            `json:"entry_point"`
	Input       []byte            `json:"input"`       // JSON-encoded input
	Secrets     map[string][]byte `json:"secrets"`     // Encrypted secrets
	MemoryLimit uint64            `json:"memory_limit"`
	TimeoutMS   uint64            `json:"timeout_ms"`
}

// SGXScriptResult is the result of script execution.
type SGXScriptResult struct {
	Output     []byte   `json:"output"`      // JSON-encoded output
	Logs       []string `json:"logs"`
	Error      string   `json:"error,omitempty"`
	MemoryUsed uint64   `json:"memory_used"`
	DurationMS uint64   `json:"duration_ms"`
}

// NoopSGXBridge is a no-op implementation for simulation mode.
type NoopSGXBridge struct{}

func NewNoopSGXBridge() SGXBridge {
	return &NoopSGXBridge{}
}

func (b *NoopSGXBridge) InitializeEnclave(config SGXEnclaveConfig) error {
	return fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) DestroyEnclave() error {
	return nil
}

func (b *NoopSGXBridge) GetEnclaveStatus() (*SGXEnclaveStatus, error) {
	return &SGXEnclaveStatus{Initialized: false}, nil
}

func (b *NoopSGXBridge) SealData(plaintext []byte, additionalData []byte) ([]byte, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) UnsealData(sealed []byte, additionalData []byte) ([]byte, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) GenerateQuote(reportData []byte) (*SGXQuote, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) VerifyQuote(quote *SGXQuote) (*SGXQuoteVerification, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) GenerateKeyPair(keyType SGXKeyType) (*SGXKeyPair, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) Sign(keyID string, data []byte) ([]byte, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) Verify(keyID string, data []byte, signature []byte) (bool, error) {
	return false, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) Encrypt(keyID string, plaintext []byte) ([]byte, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) Decrypt(keyID string, ciphertext []byte) ([]byte, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

func (b *NoopSGXBridge) ExecuteScript(req SGXScriptRequest) (*SGXScriptResult, error) {
	return nil, fmt.Errorf("SGX not available in simulation mode")
}

// HardwareSGXBridge will be implemented when hardware mode is enabled.
// It will use CGO to call into Rust SGX SDK.
//
// Build tags will be used to conditionally compile:
// - //go:build !sgx -> uses NoopSGXBridge
// - //go:build sgx  -> uses HardwareSGXBridge with CGO
//
// Example CGO integration (to be implemented):
//
// /*
// #cgo LDFLAGS: -L${SRCDIR}/rust_sgx/target/release -lsgx_bridge
// #include "sgx_bridge.h"
// */
// import "C"
//
// type HardwareSGXBridge struct {
//     enclaveID C.sgx_enclave_id_t
// }

// GetSGXBridge returns the appropriate SGX bridge based on build configuration.
func GetSGXBridge(mode EnclaveMode) SGXBridge {
	if mode == EnclaveModeHardware {
		// In the future, return HardwareSGXBridge when available
		return NewNoopSGXBridge()
	}
	return NewNoopSGXBridge()
}

// SGXHardwareRuntime implements EnclaveRuntime using actual SGX hardware.
// This is a placeholder for future implementation.
type SGXHardwareRuntime struct {
	bridge SGXBridge
	config SGXEnclaveConfig
}

func NewSGXHardwareRuntime(config SGXEnclaveConfig) (EnclaveRuntime, error) {
	return nil, fmt.Errorf("SGX hardware mode not yet implemented")
}

func (r *SGXHardwareRuntime) Initialize(ctx context.Context) error {
	return r.bridge.InitializeEnclave(r.config)
}

func (r *SGXHardwareRuntime) Shutdown(ctx context.Context) error {
	return r.bridge.DestroyEnclave()
}

func (r *SGXHardwareRuntime) Health(ctx context.Context) error {
	status, err := r.bridge.GetEnclaveStatus()
	if err != nil {
		return err
	}
	if !status.Initialized {
		return ErrEnclaveNotReady
	}
	return nil
}

func (r *SGXHardwareRuntime) Mode() EnclaveMode {
	return EnclaveModeHardware
}

func (r *SGXHardwareRuntime) SealData(ctx context.Context, data []byte) ([]byte, error) {
	return r.bridge.SealData(data, nil)
}

func (r *SGXHardwareRuntime) UnsealData(ctx context.Context, sealed []byte) ([]byte, error) {
	return r.bridge.UnsealData(sealed, nil)
}
