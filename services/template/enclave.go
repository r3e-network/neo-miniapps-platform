// Package template provides enclave operations for the template service.
package template

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
)

// Enclave handles TEE-protected operations for the template service.
// All sensitive operations happen inside the enclave.
type Enclave struct {
	mu        sync.RWMutex
	os        os.ServiceOS
	ready     bool
}

// NewEnclave creates a new enclave for the template service.
func NewEnclave(serviceOS os.ServiceOS) (*Enclave, error) {
	return &Enclave{
		os: serviceOS,
	}, nil
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.os.Logger().Info("initializing template enclave")

	// Load any sealed configuration
	if e.os.HasCapability(os.CapStorage) {
		if err := e.loadSealedConfig(ctx); err != nil {
			e.os.Logger().Warn("failed to load sealed config", "error", err)
		}
	}

	e.ready = true
	e.os.Logger().Info("template enclave initialized")
	return nil
}

// Shutdown shuts down the enclave.
func (e *Enclave) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.os.Logger().Info("shutting down template enclave")
	e.ready = false
	return nil
}

// Health checks enclave health.
func (e *Enclave) Health(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return fmt.Errorf("enclave not ready")
	}
	return nil
}

// =============================================================================
// Enclave Operations
// =============================================================================

// ProcessWithSecret processes data using a secret.
// The secret is accessed inside the enclave and never exposed.
func (e *Enclave) ProcessWithSecret(ctx context.Context, data []byte, secretName string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return nil, fmt.Errorf("enclave not ready")
	}

	if !e.os.HasCapability(os.CapSecrets) {
		return nil, fmt.Errorf("secrets capability not available")
	}

	var result []byte

	// Use the secret inside the callback - secret never leaves enclave
	err := e.os.Secrets().Use(ctx, secretName, func(secret []byte) error {
		// Process data with secret inside enclave
		// This is where you would do encryption, signing, etc.
		result = e.processInternal(data, secret)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("process with secret: %w", err)
	}

	return result, nil
}

// DeriveKey derives a key for the service.
func (e *Enclave) DeriveKey(ctx context.Context, path string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return nil, fmt.Errorf("enclave not ready")
	}

	if !e.os.HasCapability(os.CapKeys) {
		return nil, fmt.Errorf("keys capability not available")
	}

	handle, err := e.os.Keys().DeriveKey(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	return e.os.Keys().GetPublicKey(ctx, handle)
}

// Sign signs data with a derived key.
func (e *Enclave) Sign(ctx context.Context, keyPath string, data []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return nil, fmt.Errorf("enclave not ready")
	}

	if !e.os.HasCapability(os.CapKeysSign) {
		return nil, fmt.Errorf("keys.sign capability not available")
	}

	handle, err := e.os.Keys().DeriveKey(ctx, keyPath)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	return e.os.Keys().Sign(ctx, handle, data)
}

// =============================================================================
// Neo N3 Operations (if available)
// =============================================================================

// CreateNeoWallet creates a Neo N3 wallet inside the enclave.
func (e *Enclave) CreateNeoWallet(ctx context.Context, path string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return "", fmt.Errorf("enclave not ready")
	}

	if !e.os.HasCapability(os.CapNeo) {
		return "", fmt.Errorf("neo capability not available")
	}

	handle, err := e.os.Neo().CreateWallet(ctx, path)
	if err != nil {
		return "", fmt.Errorf("create wallet: %w", err)
	}

	return e.os.Neo().GetAddress(ctx, handle)
}

// SignNeoTransaction signs a Neo transaction inside the enclave.
func (e *Enclave) SignNeoTransaction(ctx context.Context, walletPath string, txHash []byte) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return nil, fmt.Errorf("enclave not ready")
	}

	if !e.os.HasCapability(os.CapNeoSign) {
		return nil, fmt.Errorf("neo.sign capability not available")
	}

	handle, err := e.os.Neo().DeriveWallet(ctx, walletPath)
	if err != nil {
		return nil, fmt.Errorf("derive wallet: %w", err)
	}

	return e.os.Neo().SignTransaction(ctx, handle, txHash)
}

// =============================================================================
// Internal Methods
// =============================================================================

func (e *Enclave) loadSealedConfig(ctx context.Context) error {
	// Load sealed configuration from storage
	// This data is encrypted and can only be decrypted inside the enclave
	return e.os.Storage().Use(ctx, "template/config", func(data []byte) error {
		// Process sealed config
		e.os.Logger().Debug("loaded sealed config", "size", len(data))
		return nil
	})
}

func (e *Enclave) processInternal(data, secret []byte) []byte {
	// Internal processing with secret
	// This is a placeholder - implement actual logic here
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ secret[i%len(secret)]
	}
	return result
}
