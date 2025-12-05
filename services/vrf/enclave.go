// Package vrf provides Verifiable Random Function service.
package vrf

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected VRF operations.
type Enclave struct {
	*base.BaseEnclave
	mu sync.RWMutex

	// VRF key handle (actual key stays in TEE)
	vrfKeyHandle os.KeyHandle
}

// NewEnclave creates a new VRF enclave.
func NewEnclave(serviceOS os.ServiceOS) (*Enclave, error) {
	if !serviceOS.HasCapability(os.CapKeys) {
		return nil, errors.New("VRF service requires keys capability")
	}

	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}, nil
}

// Initialize initializes the enclave and derives VRF key.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}

	// Derive VRF signing key
	handle, err := e.OS().Keys().DeriveKey(ctx, "vrf/signing")
	if err != nil {
		return fmt.Errorf("derive VRF key: %w", err)
	}
	e.vrfKeyHandle = handle

	e.Logger().Info("VRF enclave initialized with signing key")
	return nil
}

// GenerateRandomness generates verifiable randomness inside the TEE.
func (e *Enclave) GenerateRandomness(ctx context.Context, seed []byte, blockHash []byte) (*VRFOutput, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Create VRF input
	h := sha256.New()
	h.Write([]byte("VRF_INPUT"))
	h.Write(seed)
	h.Write(blockHash)
	vrfInput := h.Sum(nil)

	// Sign the input (this is the VRF proof)
	proof, err := e.OS().Keys().Sign(ctx, e.vrfKeyHandle, vrfInput)
	if err != nil {
		return nil, fmt.Errorf("generate VRF proof: %w", err)
	}

	// Derive randomness from proof
	randomHash := sha256.New()
	randomHash.Write([]byte("VRF_OUTPUT"))
	randomHash.Write(proof)
	randomness := randomHash.Sum(nil)

	return &VRFOutput{
		Randomness: randomness,
		Proof:      proof,
		Input:      vrfInput,
	}, nil
}

// VerifyRandomness verifies a VRF output.
func (e *Enclave) VerifyRandomness(ctx context.Context, output *VRFOutput) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.IsReady() {
		return false, errors.New("enclave not ready")
	}

	// Get public key
	pubKey, err := e.OS().Keys().GetPublicKey(ctx, e.vrfKeyHandle)
	if err != nil {
		return false, fmt.Errorf("get public key: %w", err)
	}

	// Verify signature
	valid, err := e.OS().Keys().Verify(ctx, pubKey, output.Input, output.Proof)
	if err != nil {
		return false, fmt.Errorf("verify proof: %w", err)
	}

	if !valid {
		return false, nil
	}

	// Verify randomness derivation
	expectedHash := sha256.New()
	expectedHash.Write([]byte("VRF_OUTPUT"))
	expectedHash.Write(output.Proof)
	expected := expectedHash.Sum(nil)

	if len(output.Randomness) != len(expected) {
		return false, nil
	}
	for i := range expected {
		if output.Randomness[i] != expected[i] {
			return false, nil
		}
	}

	return true, nil
}

// GetPublicKey returns the VRF public key.
func (e *Enclave) GetPublicKey(ctx context.Context) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	return e.OS().Keys().GetPublicKey(ctx, e.vrfKeyHandle)
}
