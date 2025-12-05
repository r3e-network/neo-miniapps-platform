// Package enclave provides the enclave runtime abstraction.
package enclave

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/R3E-Network/service_layer/tee/types"
)

// Mode specifies the enclave operation mode.
type Mode string

const (
	ModeSimulation Mode = "simulation"
	ModeHardware   Mode = "hardware"
)

// Config holds enclave configuration.
type Config struct {
	Mode           Mode
	EnclaveID      string
	SealingKeyPath string
	DebugMode      bool
}

// Runtime provides the enclave runtime abstraction.
type Runtime interface {
	// Lifecycle
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Health(ctx context.Context) error

	// Identity
	EnclaveID() string
	Mode() Mode

	// Cryptographic operations
	Seal(plaintext []byte) ([]byte, error)
	Unseal(ciphertext []byte) ([]byte, error)
	GenerateRandom(size int) ([]byte, error)

	// Measurements
	GetMeasurement() ([]byte, error)
	GetSignerMeasurement() ([]byte, error)
}

// runtimeImpl implements Runtime.
type runtimeImpl struct {
	mu         sync.RWMutex
	config     Config
	sealingKey []byte
	ready      bool
}

// New creates a new enclave runtime.
func New(cfg Config) (Runtime, error) {
	if cfg.EnclaveID == "" {
		return nil, fmt.Errorf("enclave_id is required")
	}

	return &runtimeImpl{
		config: cfg,
	}, nil
}

// Initialize initializes the enclave runtime.
func (r *runtimeImpl) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ready {
		return nil
	}

	// Initialize sealing key
	if err := r.initSealingKey(); err != nil {
		return fmt.Errorf("init sealing key: %w", err)
	}

	r.ready = true
	return nil
}

// initSealingKey initializes or loads the sealing key.
func (r *runtimeImpl) initSealingKey() error {
	if r.config.Mode == ModeHardware {
		// In hardware mode, derive from SGX sealing key
		r.sealingKey = r.deriveSGXSealingKey()
		return nil
	}

	// Simulation mode: load from file or generate
	if r.config.SealingKeyPath != "" {
		key, err := os.ReadFile(r.config.SealingKeyPath)
		if err == nil && len(key) == 32 {
			r.sealingKey = key
			return nil
		}
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate sealing key: %w", err)
	}
	r.sealingKey = key

	// Persist if path specified
	if r.config.SealingKeyPath != "" {
		if err := os.WriteFile(r.config.SealingKeyPath, key, 0600); err != nil {
			return fmt.Errorf("save sealing key: %w", err)
		}
	}

	return nil
}

// deriveSGXSealingKey derives sealing key from SGX (placeholder).
func (r *runtimeImpl) deriveSGXSealingKey() []byte {
	h := sha256.New()
	h.Write([]byte("SGX_SEALING_KEY"))
	h.Write([]byte(r.config.EnclaveID))
	return h.Sum(nil)
}

// Shutdown shuts down the enclave runtime.
func (r *runtimeImpl) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Zero sealing key
	if r.sealingKey != nil {
		ZeroBytes(r.sealingKey)
		r.sealingKey = nil
	}

	r.ready = false
	return nil
}

// Health checks if the runtime is healthy.
func (r *runtimeImpl) Health(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return types.ErrEnclaveNotReady
	}
	return nil
}

// EnclaveID returns the enclave identifier.
func (r *runtimeImpl) EnclaveID() string {
	return r.config.EnclaveID
}

// Mode returns the enclave mode.
func (r *runtimeImpl) Mode() Mode {
	return r.config.Mode
}

// Seal encrypts data using the enclave's sealing key.
func (r *runtimeImpl) Seal(plaintext []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return nil, types.ErrEnclaveNotReady
	}

	block, err := aes.NewCipher(r.sealingKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Unseal decrypts data using the enclave's sealing key.
func (r *runtimeImpl) Unseal(ciphertext []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return nil, types.ErrEnclaveNotReady
	}

	block, err := aes.NewCipher(r.sealingKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// GenerateRandom generates cryptographically secure random bytes.
func (r *runtimeImpl) GenerateRandom(size int) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return nil, types.ErrEnclaveNotReady
	}

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate random: %w", err)
	}
	return buf, nil
}

// GetMeasurement returns the enclave measurement (MRENCLAVE).
func (r *runtimeImpl) GetMeasurement() ([]byte, error) {
	h := sha256.New()
	h.Write([]byte("MRENCLAVE"))
	h.Write([]byte(r.config.EnclaveID))
	return h.Sum(nil), nil
}

// GetSignerMeasurement returns the signer measurement (MRSIGNER).
func (r *runtimeImpl) GetSignerMeasurement() ([]byte, error) {
	h := sha256.New()
	h.Write([]byte("MRSIGNER"))
	h.Write([]byte("R3E-Network"))
	return h.Sum(nil), nil
}

// =============================================================================
// Utility Functions
// =============================================================================

// ZeroBytes securely zeros a byte slice.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// SecureBuffer is a buffer that zeros itself when done.
type SecureBuffer struct {
	data []byte
}

// NewSecureBuffer creates a new secure buffer.
func NewSecureBuffer(size int) *SecureBuffer {
	return &SecureBuffer{data: make([]byte, size)}
}

// Data returns the buffer data.
func (b *SecureBuffer) Data() []byte {
	return b.data
}

// Zero zeros the buffer.
func (b *SecureBuffer) Zero() {
	ZeroBytes(b.data)
}
