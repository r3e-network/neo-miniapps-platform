// Package enclave provides TEE-protected DTA (Data Trust Authority) operations.
// Data attestation and trust verification run inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveDTA handles DTA operations within the TEE.
type EnclaveDTA struct {
	*sdk.BaseEnclave
}

// DTAConfig holds configuration for the DTA enclave.
type DTAConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveDTA creates a new enclave DTA handler.
func NewEnclaveDTA() (*EnclaveDTA, error) {
	base, err := sdk.NewBaseEnclave("dta")
	if err != nil {
		return nil, err
	}
	return &EnclaveDTA{BaseEnclave: base}, nil
}

// NewEnclaveDTAWithSDK creates a DTA handler with full SDK integration.
func NewEnclaveDTAWithSDK(cfg *DTAConfig) (*EnclaveDTA, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "dta",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveDTA{BaseEnclave: base}, nil
}

// InitializeWithSDK initializes the DTA handler with an existing SDK instance.
func (e *EnclaveDTA) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// AttestData attests data integrity and signs it.
func (e *EnclaveDTA) AttestData(dataID string, data []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	hash := sha256.New()
	hash.Write([]byte(dataID))
	hash.Write(data)

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash.Sum(nil))
	if err != nil {
		return nil, err
	}
	return append(r.Bytes(), s.Bytes()...), nil
}
