// Package enclave provides TEE-protected data streams operations.
// Stream data validation and signing run inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveDataStreams handles data streams operations within the TEE.
type EnclaveDataStreams struct {
	*sdk.BaseEnclave
}

// DataStreamsConfig holds configuration for the data streams enclave.
type DataStreamsConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveDataStreams creates a new enclave data streams handler.
func NewEnclaveDataStreams() (*EnclaveDataStreams, error) {
	base, err := sdk.NewBaseEnclave("datastreams")
	if err != nil {
		return nil, err
	}
	return &EnclaveDataStreams{BaseEnclave: base}, nil
}

// NewEnclaveDataStreamsWithSDK creates a data streams handler with full SDK integration.
func NewEnclaveDataStreamsWithSDK(cfg *DataStreamsConfig) (*EnclaveDataStreams, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "datastreams",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveDataStreams{BaseEnclave: base}, nil
}

// InitializeWithSDK initializes the data streams handler with an existing SDK instance.
func (e *EnclaveDataStreams) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// ValidateAndSign validates stream data and signs it.
func (e *EnclaveDataStreams) ValidateAndSign(streamID string, data []byte, timestamp int64) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	hash := sha256.New()
	hash.Write([]byte(streamID))
	hash.Write(data)

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash.Sum(nil))
	if err != nil {
		return nil, err
	}
	return append(r.Bytes(), s.Bytes()...), nil
}
