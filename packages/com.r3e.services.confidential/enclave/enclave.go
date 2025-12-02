// Package enclave provides TEE-protected confidential computing operations.
// All confidential data processing runs inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveConfidential handles confidential operations within the TEE.
type EnclaveConfidential struct {
	*sdk.BaseEnclave
	masterKey []byte
}

// ConfidentialConfig holds configuration for the confidential enclave.
type ConfidentialConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	MasterKey []byte
}

// NewEnclaveConfidential creates a new enclave confidential handler.
func NewEnclaveConfidential(masterKey []byte) (*EnclaveConfidential, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	base, err := sdk.NewBaseEnclave("confidential")
	if err != nil {
		return nil, err
	}
	return &EnclaveConfidential{
		BaseEnclave: base,
		masterKey:   masterKey,
	}, nil
}

// NewEnclaveConfidentialWithSDK creates a confidential handler with full SDK integration.
func NewEnclaveConfidentialWithSDK(cfg *ConfidentialConfig) (*EnclaveConfidential, error) {
	if len(cfg.MasterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}

	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "confidential",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveConfidential{
		BaseEnclave: base,
		masterKey:   cfg.MasterKey,
	}, nil
}

// InitializeWithSDK initializes the confidential handler with an existing SDK instance.
func (e *EnclaveConfidential) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) {
	e.BaseEnclave.InitializeWithSDKSimple(enclaveSDK)
}

// ProcessConfidential processes data confidentially within the enclave.
func (e *EnclaveConfidential) ProcessConfidential(data []byte, processor func([]byte) ([]byte, error)) ([]byte, error) {
	e.Lock()
	defer e.Unlock()
	return processor(data)
}

// Encrypt encrypts data within the enclave.
func (e *EnclaveConfidential) Encrypt(plaintext []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts data within the enclave.
func (e *EnclaveConfidential) Decrypt(ciphertext []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], nil)
}
