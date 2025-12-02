// Package enclave provides TEE-protected account operations.
// Account key management and signing run inside the enclave.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveAccounts handles account operations within the TEE.
type EnclaveAccounts struct {
	*sdk.BaseEnclave
	masterKey []byte
	accounts  map[string]*ecdsa.PrivateKey
}

// AccountsConfig holds configuration for the accounts enclave.
type AccountsConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	MasterKey []byte
}

// NewEnclaveAccounts creates a new enclave accounts handler.
func NewEnclaveAccounts(masterKey []byte) (*EnclaveAccounts, error) {
	if len(masterKey) < 32 {
		return nil, errors.New("master key must be at least 32 bytes")
	}
	base, err := sdk.NewBaseEnclave("accounts")
	if err != nil {
		return nil, err
	}
	return &EnclaveAccounts{
		BaseEnclave: base,
		masterKey:   masterKey,
		accounts:    make(map[string]*ecdsa.PrivateKey),
	}, nil
}

// NewEnclaveAccountsWithSDK creates an accounts handler with full SDK integration.
func NewEnclaveAccountsWithSDK(cfg *AccountsConfig) (*EnclaveAccounts, error) {
	if len(cfg.MasterKey) < 32 {
		return nil, errors.New("master key must be at least 32 bytes")
	}

	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "accounts",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveAccounts{
		BaseEnclave: base,
		masterKey:   cfg.MasterKey,
		accounts:    make(map[string]*ecdsa.PrivateKey),
	}, nil
}

// InitializeWithSDK initializes the accounts handler with an existing SDK instance.
func (e *EnclaveAccounts) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) {
	e.BaseEnclave.InitializeWithSDKSimple(enclaveSDK)
}

// DeriveAccount derives an account key within the enclave.
func (e *EnclaveAccounts) DeriveAccount(accountID string) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	if key, exists := e.accounts[accountID]; exists {
		return elliptic.Marshal(key.PublicKey.Curve, key.PublicKey.X, key.PublicKey.Y), nil
	}

	h := sha256.New()
	h.Write(e.masterKey)
	h.Write([]byte(accountID))
	seed := h.Sum(nil)

	d := new(big.Int).SetBytes(seed)
	d.Mod(d, elliptic.P256().Params().N)

	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()},
		D:         d,
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.PublicKey.Curve.ScalarBaseMult(d.Bytes())

	e.accounts[accountID] = privateKey
	return elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y), nil
}

// SignMessage signs a message for an account within the enclave.
func (e *EnclaveAccounts) SignMessage(accountID string, message []byte) ([]byte, error) {
	e.RLock()
	key, exists := e.accounts[accountID]
	e.RUnlock()

	if !exists {
		if _, err := e.DeriveAccount(accountID); err != nil {
			return nil, err
		}
		e.RLock()
		key = e.accounts[accountID]
		e.RUnlock()
	}

	hash := sha256.Sum256(message)
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return nil, err
	}

	return append(r.Bytes(), s.Bytes()...), nil
}
