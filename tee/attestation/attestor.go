// Package attestation provides remote attestation capabilities for the TEE enclave.
package attestation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/types"
)

// Config holds attestor configuration.
type Config struct {
	Runtime   enclave.Runtime
	EnclaveID string
}

// Attestor implements types.Attestor.
type Attestor struct {
	mu        sync.RWMutex
	runtime   enclave.Runtime
	enclaveID string
}

// New creates a new attestor.
func New(cfg Config) (*Attestor, error) {
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	if cfg.EnclaveID == "" {
		return nil, fmt.Errorf("enclave_id is required")
	}

	return &Attestor{
		runtime:   cfg.Runtime,
		enclaveID: cfg.EnclaveID,
	}, nil
}

// GenerateQuote generates a quote for remote attestation.
func (a *Attestor) GenerateQuote(ctx context.Context, userData []byte) (*types.Quote, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Get enclave measurements
	mrEnclave, err := a.runtime.GetMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get measurement: %w", err)
	}

	mrSigner, err := a.runtime.GetSignerMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get signer measurement: %w", err)
	}

	// Generate quote
	// In hardware mode, this would call SGX EREPORT/EGETKEY
	// In simulation mode, we create a simulated quote
	quote := a.generateSimulatedQuote(userData, mrEnclave, mrSigner)

	return quote, nil
}

// generateSimulatedQuote creates a simulated quote for testing.
func (a *Attestor) generateSimulatedQuote(userData, mrEnclave, mrSigner []byte) *types.Quote {
	timestamp := time.Now()

	// Create quote structure
	h := sha256.New()
	h.Write([]byte("SGX_QUOTE_V3"))
	h.Write(mrEnclave)
	h.Write(mrSigner)
	h.Write(userData)
	h.Write([]byte(timestamp.Format(time.RFC3339)))

	rawQuote := h.Sum(nil)

	return &types.Quote{
		RawQuote:  rawQuote,
		UserData:  userData,
		MREnclave: hex.EncodeToString(mrEnclave),
		MRSigner:  hex.EncodeToString(mrSigner),
		Timestamp: timestamp,
	}
}

// VerifyQuote verifies a quote.
func (a *Attestor) VerifyQuote(ctx context.Context, quote *types.Quote) (*types.QuoteVerification, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if quote == nil {
		return nil, fmt.Errorf("quote is nil")
	}

	// Get expected measurements
	expectedMREnclave, err := a.runtime.GetMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get measurement: %w", err)
	}

	expectedMRSigner, err := a.runtime.GetSignerMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get signer measurement: %w", err)
	}

	// Verify measurements match
	valid := quote.MREnclave == hex.EncodeToString(expectedMREnclave) &&
		quote.MRSigner == hex.EncodeToString(expectedMRSigner)

	// In hardware mode, this would verify the quote signature with Intel IAS/DCAP
	// For simulation, we just check measurements

	return &types.QuoteVerification{
		Valid:      valid,
		MREnclave:  quote.MREnclave,
		MRSigner:   quote.MRSigner,
		VerifiedAt: time.Now(),
	}, nil
}

// GetReport returns the current attestation report.
func (a *Attestor) GetReport(ctx context.Context) (*types.AttestationReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	mrEnclave, err := a.runtime.GetMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get measurement: %w", err)
	}

	mrSigner, err := a.runtime.GetSignerMeasurement()
	if err != nil {
		return nil, fmt.Errorf("get signer measurement: %w", err)
	}

	mode := "simulation"
	if a.runtime.Mode() == enclave.ModeHardware {
		mode = "hardware"
	}

	return &types.AttestationReport{
		EnclaveID: a.enclaveID,
		Mode:      mode,
		MREnclave: hex.EncodeToString(mrEnclave),
		MRSigner:  hex.EncodeToString(mrSigner),
		Timestamp: time.Now(),
	}, nil
}

// VerifyRemoteQuote verifies a quote from a remote enclave.
// This would typically call Intel Attestation Service (IAS) or DCAP.
func (a *Attestor) VerifyRemoteQuote(ctx context.Context, quote *types.Quote, expectedMREnclave string) (*types.QuoteVerification, error) {
	if quote == nil {
		return nil, fmt.Errorf("quote is nil")
	}

	// Check if MREnclave matches expected value
	valid := quote.MREnclave == expectedMREnclave

	// In production, this would:
	// 1. Send quote to Intel IAS/DCAP for verification
	// 2. Verify the IAS signature
	// 3. Check quote freshness
	// 4. Verify MREnclave/MRSigner against allowlist

	return &types.QuoteVerification{
		Valid:      valid,
		MREnclave:  quote.MREnclave,
		MRSigner:   quote.MRSigner,
		VerifiedAt: time.Now(),
	}, nil
}
