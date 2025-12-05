// Package gasbank provides gas fee management service.
package gasbank

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected gas operations.
type Enclave struct {
	*base.BaseEnclave
	mu sync.RWMutex

	// Signing key handle (actual key stays in TEE)
	signingKeyHandle os.KeyHandle
}

// NewEnclave creates a new GasBank enclave.
func NewEnclave(serviceOS os.ServiceOS) (*Enclave, error) {
	if !serviceOS.HasCapability(os.CapKeys) {
		return nil, errors.New("gasbank service requires keys capability")
	}

	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}, nil
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}

	// Derive signing key
	handle, err := e.OS().Keys().DeriveKey(ctx, "gasbank/signing")
	if err != nil {
		return fmt.Errorf("derive signing key: %w", err)
	}
	e.signingKeyHandle = handle

	e.Logger().Info("gasbank enclave initialized with signing key")
	return nil
}

// SignSponsorship signs a gas sponsorship transaction.
func (e *Enclave) SignSponsorship(ctx context.Context, accountID string, txData []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Create sponsorship hash
	h := sha256.New()
	h.Write([]byte("GAS_SPONSORSHIP"))
	h.Write([]byte(accountID))
	h.Write(txData)
	hash := h.Sum(nil)

	// Sign inside TEE
	return e.OS().Keys().Sign(ctx, e.signingKeyHandle, hash)
}

// VerifySponsorship verifies a sponsorship signature.
func (e *Enclave) VerifySponsorship(ctx context.Context, accountID string, txData, signature []byte) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.IsReady() {
		return false, errors.New("enclave not ready")
	}

	// Recreate hash
	h := sha256.New()
	h.Write([]byte("GAS_SPONSORSHIP"))
	h.Write([]byte(accountID))
	h.Write(txData)
	hash := h.Sum(nil)

	// Get public key
	pubKey, err := e.OS().Keys().GetPublicKey(ctx, e.signingKeyHandle)
	if err != nil {
		return false, fmt.Errorf("get public key: %w", err)
	}

	return e.OS().Keys().Verify(ctx, pubKey, hash, signature)
}

// GetPublicKey returns the signing public key.
func (e *Enclave) GetPublicKey(ctx context.Context) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	return e.OS().Keys().GetPublicKey(ctx, e.signingKeyHandle)
}

// GetAddress returns the blockchain address.
func (e *Enclave) GetAddress(ctx context.Context, chain os.ChainType) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.IsReady() {
		return "", errors.New("enclave not ready")
	}

	return e.OS().Keys().GetAddress(ctx, e.signingKeyHandle, chain)
}
