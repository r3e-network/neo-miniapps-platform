// Package sdk provides the Enclave SDK base implementation.
// This file contains the base enclave struct that all service enclaves can embed
// to get common functionality without code duplication.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
	"sync"
)

// BaseEnclave provides common functionality for all service enclaves.
// Service-specific enclaves should embed this struct to inherit common methods.
type BaseEnclave struct {
	mu sync.RWMutex

	// Signing key for enclave operations
	signingKey *ecdsa.PrivateKey

	// SDK integration
	sdk         EnclaveSDK
	keyID       string
	initialized bool

	// Service identification
	serviceName string
}

// BaseConfig holds common configuration for all service enclaves.
type BaseConfig struct {
	ServiceID   string
	ServiceName string
	RequestID   string
	CallerID    string
	AccountID   string
	SealKey     []byte
}

// NewBaseEnclave creates a new base enclave with common initialization.
func NewBaseEnclave(serviceName string) (*BaseEnclave, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return &BaseEnclave{
		signingKey:  privateKey,
		serviceName: serviceName,
	}, nil
}

// NewBaseEnclaveWithSDK creates a base enclave with full SDK integration.
func NewBaseEnclaveWithSDK(cfg *BaseConfig) (*BaseEnclave, error) {
	// Create SDK configuration
	sdkCfg := &Config{
		ServiceID: cfg.ServiceID,
		RequestID: cfg.RequestID,
		CallerID:  cfg.CallerID,
		Metadata: map[string]string{
			"account_id": cfg.AccountID,
			"service":    cfg.ServiceName,
		},
	}

	// Create SDK instance
	enclaveSDK := New(sdkCfg)

	// Generate signing key using SDK
	ctx := context.Background()
	keyResp, err := enclaveSDK.Keys().GenerateKey(ctx, &GenerateKeyRequest{
		Type:  KeyTypeECDSA,
		Curve: KeyCurveP256,
	})
	if err != nil {
		return nil, err
	}

	// Also generate local key for backward compatibility
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return &BaseEnclave{
		signingKey:  privateKey,
		sdk:         enclaveSDK,
		keyID:       keyResp.KeyID,
		initialized: true,
		serviceName: cfg.ServiceName,
	}, nil
}

// InitializeWithSDK initializes the enclave with an existing SDK instance.
func (b *BaseEnclave) InitializeWithSDK(enclaveSDK EnclaveSDK) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.sdk = enclaveSDK

	// Generate signing key using SDK
	ctx := context.Background()
	keyResp, err := enclaveSDK.Keys().GenerateKey(ctx, &GenerateKeyRequest{
		Type:  KeyTypeECDSA,
		Curve: KeyCurveP256,
	})
	if err != nil {
		return err
	}

	b.keyID = keyResp.KeyID
	b.initialized = true
	return nil
}

// InitializeWithSDKSimple initializes without generating a new key.
func (b *BaseEnclave) InitializeWithSDKSimple(enclaveSDK EnclaveSDK) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sdk = enclaveSDK
	b.initialized = true
}

// GetPublicKey returns the enclave's public key.
func (b *BaseEnclave) GetPublicKey() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.signingKey == nil {
		return nil
	}
	return elliptic.Marshal(b.signingKey.PublicKey.Curve,
		b.signingKey.PublicKey.X, b.signingKey.PublicKey.Y)
}

// GenerateAttestationReport generates a TEE attestation report.
func (b *BaseEnclave) GenerateAttestationReport(userData []byte) ([]byte, error) {
	// Use SDK attestation if available
	if b.sdk != nil && b.initialized {
		ctx := context.Background()
		report, err := b.sdk.Attestation().GenerateReport(ctx, userData)
		if err == nil {
			return report.ReportData, nil
		}
	}

	// Fallback to local attestation
	h := sha256.New()
	h.Write([]byte(b.serviceName + "_ENCLAVE_ATTESTATION"))
	h.Write(b.GetPublicKey())
	h.Write(userData)
	return h.Sum(nil), nil
}

// SDK returns the underlying Enclave SDK instance.
func (b *BaseEnclave) SDK() EnclaveSDK {
	return b.sdk
}

// IsInitialized returns whether the SDK is initialized.
func (b *BaseEnclave) IsInitialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// KeyID returns the SDK key ID.
func (b *BaseEnclave) KeyID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.keyID
}

// ServiceName returns the service name.
func (b *BaseEnclave) ServiceName() string {
	return b.serviceName
}

// SignData signs data using the enclave's signing key.
func (b *BaseEnclave) SignData(data []byte) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.signingKey == nil {
		return nil, errors.New("signing key not initialized")
	}

	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, b.signingKey, hash[:])
	if err != nil {
		return nil, err
	}

	return append(r.Bytes(), s.Bytes()...), nil
}

// SignDataWithSDK signs data using the SDK signer.
func (b *BaseEnclave) SignDataWithSDK(ctx context.Context, data []byte) ([]byte, []byte, error) {
	if b.sdk == nil || !b.initialized {
		return nil, nil, errors.New("SDK not initialized")
	}

	resp, err := b.sdk.Signer().Sign(ctx, &SignRequest{
		KeyID:   b.keyID,
		Data:    data,
		HashAlg: "sha256",
	})
	if err != nil {
		return nil, nil, err
	}

	return resp.Signature, resp.PublicKey, nil
}

// VerifySignature verifies a signature against a public key.
func VerifySignature(pubKeyBytes []byte, data []byte, signature []byte) (bool, error) {
	if len(signature) < 64 || len(pubKeyBytes) == 0 {
		return false, errors.New("invalid signature or key")
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
	if x == nil {
		return false, errors.New("invalid public key")
	}

	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])

	return ecdsa.Verify(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, hash[:], r, s), nil
}

// Lock acquires the write lock.
func (b *BaseEnclave) Lock() {
	b.mu.Lock()
}

// Unlock releases the write lock.
func (b *BaseEnclave) Unlock() {
	b.mu.Unlock()
}

// RLock acquires the read lock.
func (b *BaseEnclave) RLock() {
	b.mu.RLock()
}

// RUnlock releases the read lock.
func (b *BaseEnclave) RUnlock() {
	b.mu.RUnlock()
}

// GetSigningKey returns the signing key (for internal use by derived enclaves).
func (b *BaseEnclave) GetSigningKey() *ecdsa.PrivateKey {
	return b.signingKey
}

// SetSigningKey sets the signing key (for internal use by derived enclaves).
func (b *BaseEnclave) SetSigningKey(key *ecdsa.PrivateKey) {
	b.signingKey = key
}
