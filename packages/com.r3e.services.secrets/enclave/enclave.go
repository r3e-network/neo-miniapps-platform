// Package enclave provides TEE-protected secret management operations.
// All cryptographic operations and secret handling run inside the enclave
// to ensure confidentiality and integrity protection.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveSecrets handles all secret operations within the TEE enclave.
// Critical operations:
// - Secret encryption/decryption
// - Key derivation
// - Secure secret storage
type EnclaveSecrets struct {
	*sdk.BaseEnclave
	masterKey  []byte
	sealedData map[string][]byte
}

// SecretsConfig holds configuration for the secrets enclave.
type SecretsConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	MasterKey []byte
}

// NewEnclaveSecrets creates a new enclave secrets handler.
// The master key should be derived from TEE sealing key.
func NewEnclaveSecrets(masterKey []byte) (*EnclaveSecrets, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	base, err := sdk.NewBaseEnclave("secrets")
	if err != nil {
		return nil, err
	}
	return &EnclaveSecrets{
		BaseEnclave: base,
		masterKey:   masterKey,
		sealedData:  make(map[string][]byte),
	}, nil
}

// NewEnclaveSecretsWithSDK creates a secrets handler with full SDK integration.
func NewEnclaveSecretsWithSDK(cfg *SecretsConfig) (*EnclaveSecrets, error) {
	if len(cfg.MasterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}

	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "secrets",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveSecrets{
		BaseEnclave: base,
		masterKey:   cfg.MasterKey,
		sealedData:  make(map[string][]byte),
	}, nil
}

// InitializeWithSDK initializes the secrets handler with an existing SDK instance.
func (e *EnclaveSecrets) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) {
	e.BaseEnclave.InitializeWithSDKSimple(enclaveSDK)
}

// SealSecret encrypts and seals a secret within the enclave.
// This operation never exposes the plaintext outside the TEE.
func (e *EnclaveSecrets) SealSecret(secretID string, plaintext []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	// Use SDK if available
	if e.SDK() != nil && e.IsInitialized() {
		ctx := context.Background()
		resp, err := e.SDK().Secrets().Add(ctx, &sdk.AddSecretRequest{
			Name:  secretID,
			Value: plaintext,
			Type:  sdk.SecretTypeGeneric,
		})
		if err == nil {
			return []byte(resp.SecretID), nil
		}
		// Fall through to local implementation on error
	}

	// Derive a unique key for this secret
	key := e.deriveKey(secretID)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt and seal
	ciphertext := gcm.Seal(nonce, nonce, plaintext, []byte(secretID))

	// Store sealed data
	e.sealedData[secretID] = ciphertext

	return ciphertext, nil
}

// UnsealSecret decrypts a sealed secret within the enclave.
// The plaintext is only available inside the TEE.
func (e *EnclaveSecrets) UnsealSecret(secretID string, ciphertext []byte) ([]byte, error) {
	e.RLock()
	defer e.RUnlock()

	// Use SDK if available
	if e.SDK() != nil && e.IsInitialized() {
		ctx := context.Background()
		secret, err := e.SDK().Secrets().Get(ctx, secretID)
		if err == nil {
			return secret.Value, nil
		}
		// Fall through to local implementation on error
	}

	// Derive the key for this secret
	key := e.deriveKey(secretID)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
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

	// Extract nonce and decrypt
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertextData := ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertextData, []byte(secretID))
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// DeleteSecret removes a secret from the enclave.
func (e *EnclaveSecrets) DeleteSecret(secretID string) error {
	e.Lock()
	defer e.Unlock()

	// Use SDK if available
	if e.SDK() != nil && e.IsInitialized() {
		ctx := context.Background()
		err := e.SDK().Secrets().Delete(ctx, &sdk.DeleteSecretRequest{
			SecretID: secretID,
		})
		if err == nil {
			delete(e.sealedData, secretID)
			return nil
		}
	}

	delete(e.sealedData, secretID)
	return nil
}

// ListSecrets returns a list of secret IDs.
func (e *EnclaveSecrets) ListSecrets() ([]string, error) {
	e.RLock()
	defer e.RUnlock()

	// Use SDK if available
	if e.SDK() != nil && e.IsInitialized() {
		ctx := context.Background()
		resp, err := e.SDK().Secrets().List(ctx, &sdk.ListSecretsRequest{
			Limit: 1000,
		})
		if err == nil {
			ids := make([]string, len(resp.Secrets))
			for i, s := range resp.Secrets {
				ids[i] = s.ID
			}
			return ids, nil
		}
	}

	ids := make([]string, 0, len(e.sealedData))
	for id := range e.sealedData {
		ids = append(ids, id)
	}
	return ids, nil
}

// deriveKey derives a unique encryption key for a secret ID.
func (e *EnclaveSecrets) deriveKey(secretID string) []byte {
	h := sha256.New()
	h.Write(e.masterKey)
	h.Write([]byte(secretID))
	return h.Sum(nil)
}

// GetSealedSecretHash returns the hash of a sealed secret for verification.
func (e *EnclaveSecrets) GetSealedSecretHash(secretID string) (string, error) {
	e.RLock()
	defer e.RUnlock()

	data, exists := e.sealedData[secretID]
	if !exists {
		return "", errors.New("secret not found")
	}

	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
