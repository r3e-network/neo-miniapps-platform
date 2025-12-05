// Package dta provides Data Trust Authority service.
package dta

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected DTA operations.
type Enclave struct {
	*base.BaseEnclave
	signingKey os.KeyHandle
}

// NewEnclave creates a new DTA enclave.
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
		handle, err := e.OS().Keys().DeriveKey(ctx, "dta/signing")
		if err != nil {
			return fmt.Errorf("derive signing key: %w", err)
		}
		e.signingKey = handle
	}
	e.Logger().Info("dta enclave initialized")
	return nil
}

// IssueCertificate issues a data certificate inside the TEE.
func (e *Enclave) IssueCertificate(ctx context.Context, dataHash []byte, metadata map[string]string) (*Certificate, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	if e.signingKey == "" {
		return nil, errors.New("signing key not available")
	}

	now := time.Now()
	cert := &Certificate{
		DataHash:  dataHash,
		Metadata:  metadata,
		IssuedAt:  now,
		ExpiresAt: now.Add(365 * 24 * time.Hour), // 1 year validity
		Status:    CertStatusValid,
	}
	cert.ID = fmt.Sprintf("cert-%d", now.UnixNano())
	cert.SetTimestamps()

	// Create certificate hash
	h := sha256.New()
	h.Write([]byte(cert.ID))
	h.Write(dataHash)
	h.Write([]byte(cert.IssuedAt.Format(time.RFC3339)))
	certHash := h.Sum(nil)

	// Sign certificate
	signature, err := e.OS().Keys().Sign(ctx, e.signingKey, certHash)
	if err != nil {
		return nil, fmt.Errorf("sign certificate: %w", err)
	}
	cert.Signature = signature

	return cert, nil
}

// VerifyCertificate verifies a certificate signature.
func (e *Enclave) VerifyCertificate(ctx context.Context, cert *Certificate) (bool, error) {
	if !e.IsReady() {
		return false, errors.New("enclave not ready")
	}
	if len(cert.Signature) == 0 {
		return false, errors.New("no signature")
	}

	// Recreate certificate hash
	h := sha256.New()
	h.Write([]byte(cert.ID))
	h.Write(cert.DataHash)
	h.Write([]byte(cert.IssuedAt.Format(time.RFC3339)))
	certHash := h.Sum(nil)

	// Get public key
	pubKey, err := e.OS().Keys().GetPublicKey(ctx, e.signingKey)
	if err != nil {
		return false, err
	}

	// Verify signature
	valid, err := e.OS().Keys().Verify(ctx, pubKey, certHash, cert.Signature)
	if err != nil {
		return false, err
	}

	// Check expiration
	if time.Now().After(cert.ExpiresAt) {
		return false, errors.New("certificate expired")
	}

	return valid, nil
}

// GetPublicKey returns the signing public key.
func (e *Enclave) GetPublicKey(ctx context.Context) ([]byte, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	return e.OS().Keys().GetPublicKey(ctx, e.signingKey)
}
