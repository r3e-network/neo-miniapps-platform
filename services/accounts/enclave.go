// Package accounts provides account management service.
package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected account operations.
type Enclave struct {
	*base.BaseEnclave
}

// NewEnclave creates a new accounts enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// HashAPIKey hashes an API key securely.
func (e *Enclave) HashAPIKey(ctx context.Context, key string) (string, error) {
	if !e.IsReady() {
		return "", errors.New("enclave not ready")
	}
	h := sha256.New()
	h.Write([]byte("API_KEY_HASH"))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyAPIKey verifies an API key against its hash.
func (e *Enclave) VerifyAPIKey(ctx context.Context, key, hash string) (bool, error) {
	if !e.IsReady() {
		return false, errors.New("enclave not ready")
	}
	computed, err := e.HashAPIKey(ctx, key)
	if err != nil {
		return false, err
	}
	return computed == hash, nil
}
