// Package enclave provides TEE-protected CCIP operations.
// Cross-chain message validation and signing run inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveCCIP handles CCIP operations within the TEE.
type EnclaveCCIP struct {
	*sdk.BaseEnclave
}

// CCIPConfig holds configuration for the CCIP enclave.
type CCIPConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveCCIP creates a new enclave CCIP handler.
func NewEnclaveCCIP() (*EnclaveCCIP, error) {
	base, err := sdk.NewBaseEnclave("ccip")
	if err != nil {
		return nil, err
	}
	return &EnclaveCCIP{BaseEnclave: base}, nil
}

// NewEnclaveCCIPWithSDK creates a CCIP handler with full SDK integration.
func NewEnclaveCCIPWithSDK(cfg *CCIPConfig) (*EnclaveCCIP, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "ccip",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveCCIP{BaseEnclave: base}, nil
}

// InitializeWithSDK initializes the CCIP handler with an existing SDK instance.
func (e *EnclaveCCIP) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// ValidateAndSign validates a cross-chain message and signs it.
func (e *EnclaveCCIP) ValidateAndSign(messageID string, payload []byte, sourceChain, destChain uint64) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	hash := sha256.New()
	hash.Write([]byte(messageID))
	hash.Write(payload)
	hash.Write(big.NewInt(int64(sourceChain)).Bytes())
	hash.Write(big.NewInt(int64(destChain)).Bytes())

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash.Sum(nil))
	if err != nil {
		return nil, err
	}
	return append(r.Bytes(), s.Bytes()...), nil
}
