// Package enclave provides TEE-protected data link operations.
// External data fetching and validation run inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveDataLink handles data link operations within the TEE.
type EnclaveDataLink struct {
	*sdk.BaseEnclave
}

// DataLinkConfig holds configuration for the data link enclave.
type DataLinkConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveDataLink creates a new enclave data link handler.
func NewEnclaveDataLink() (*EnclaveDataLink, error) {
	base, err := sdk.NewBaseEnclave("datalink")
	if err != nil {
		return nil, err
	}
	return &EnclaveDataLink{BaseEnclave: base}, nil
}

// NewEnclaveDataLinkWithSDK creates a data link handler with full SDK integration.
func NewEnclaveDataLinkWithSDK(cfg *DataLinkConfig) (*EnclaveDataLink, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "datalink",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveDataLink{BaseEnclave: base}, nil
}

// InitializeWithSDK initializes the data link handler with an existing SDK instance.
func (e *EnclaveDataLink) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// FetchAndSign fetches external data and signs it within the enclave.
func (e *EnclaveDataLink) FetchAndSign(requestID string, data []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	hash := sha256.New()
	hash.Write([]byte(requestID))
	hash.Write(data)

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash.Sum(nil))
	if err != nil {
		return nil, err
	}
	return append(r.Bytes(), s.Bytes()...), nil
}
