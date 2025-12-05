// Package ccip provides Cross-Chain Interoperability Protocol service.
package ccip

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected CCIP operations.
type Enclave struct {
	*base.BaseEnclave
	signingKey os.KeyHandle
}

// NewEnclave creates a new CCIP enclave.
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
	if e.OS().HasCapability(os.CapKeys) {
		handle, err := e.OS().Keys().DeriveKey(ctx, "ccip/signing")
		if err != nil {
			return fmt.Errorf("derive signing key: %w", err)
		}
		e.signingKey = handle
	}
	e.Logger().Info("ccip enclave initialized")
	return nil
}

// SignMessage signs a cross-chain message.
func (e *Enclave) SignMessage(ctx context.Context, msg *CrossChainMessage) ([]byte, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	if e.signingKey == "" {
		return nil, errors.New("signing key not available")
	}

	h := sha256.New()
	h.Write([]byte(msg.SourceChain))
	h.Write([]byte(msg.DestChain))
	h.Write(msg.Payload)
	hash := h.Sum(nil)

	return e.OS().Keys().Sign(ctx, e.signingKey, hash)
}

// VerifyMessage verifies a cross-chain message signature.
func (e *Enclave) VerifyMessage(ctx context.Context, msg *CrossChainMessage) (bool, error) {
	if !e.IsReady() {
		return false, errors.New("enclave not ready")
	}
	if len(msg.Signature) == 0 {
		return false, errors.New("no signature")
	}

	h := sha256.New()
	h.Write([]byte(msg.SourceChain))
	h.Write([]byte(msg.DestChain))
	h.Write(msg.Payload)
	hash := h.Sum(nil)

	pubKey, err := e.OS().Keys().GetPublicKey(ctx, e.signingKey)
	if err != nil {
		return false, err
	}

	return e.OS().Keys().Verify(ctx, pubKey, hash, msg.Signature)
}
