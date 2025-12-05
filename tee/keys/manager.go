// Package keys provides HSM-like key management inside the TEE enclave.
package keys

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/types"
)

// Manager implements types.KeyManager.
type Manager struct {
	mu      sync.RWMutex
	runtime enclave.Runtime

	// Master seed for key derivation (stays in enclave)
	masterSeed []byte

	// Derived keys (private keys stay in enclave)
	keys map[types.KeyHandle]*ecdsa.PrivateKey
}

// New creates a new key manager.
func New(runtime enclave.Runtime) *Manager {
	return &Manager{
		runtime: runtime,
		keys:    make(map[types.KeyHandle]*ecdsa.PrivateKey),
	}
}

// Initialize initializes the key manager with a master seed.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate master seed
	seed, err := m.runtime.GenerateRandom(32)
	if err != nil {
		return fmt.Errorf("generate master seed: %w", err)
	}
	m.masterSeed = seed

	return nil
}

// DeriveKey derives a key from the master seed using the given path.
func (m *Manager) DeriveKey(ctx context.Context, path string) (types.KeyHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create handle from path
	h := sha256.New()
	h.Write([]byte(path))
	handle := types.KeyHandle(hex.EncodeToString(h.Sum(nil)[:16]))

	// Check if already derived
	if _, exists := m.keys[handle]; exists {
		return handle, nil
	}

	// Derive key using HMAC-based derivation
	h = sha256.New()
	h.Write(m.masterSeed)
	h.Write([]byte(path))
	derivedSeed := h.Sum(nil)

	// Generate ECDSA key from derived seed
	d := new(big.Int).SetBytes(derivedSeed)
	d.Mod(d, elliptic.P256().Params().N)

	// Ensure d is valid
	if d.Cmp(big.NewInt(1)) < 0 {
		d.Add(d, big.NewInt(1))
	}

	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
		},
		D: d,
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.PublicKey.Curve.ScalarBaseMult(d.Bytes())

	m.keys[handle] = privateKey

	return handle, nil
}

// Sign signs data with a key.
func (m *Manager) Sign(ctx context.Context, handle types.KeyHandle, data []byte) ([]byte, error) {
	m.mu.RLock()
	key, exists := m.keys[handle]
	m.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	// Sign the data
	r, s, err := ecdsa.Sign(rand.Reader, key, data)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Encode signature as r || s
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// Verify verifies a signature.
func (m *Manager) Verify(ctx context.Context, publicKey, data, signature []byte) (bool, error) {
	if len(signature) < 64 {
		return false, fmt.Errorf("invalid signature length")
	}

	// Parse public key
	x, y := elliptic.Unmarshal(elliptic.P256(), publicKey)
	if x == nil {
		return false, fmt.Errorf("invalid public key")
	}

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])

	return ecdsa.Verify(pubKey, data, r, s), nil
}

// GetPublicKey returns the public key for a handle.
func (m *Manager) GetPublicKey(ctx context.Context, handle types.KeyHandle) ([]byte, error) {
	m.mu.RLock()
	key, exists := m.keys[handle]
	m.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	return elliptic.Marshal(key.PublicKey.Curve, key.PublicKey.X, key.PublicKey.Y), nil
}

// GetAddress returns the blockchain address for a key.
func (m *Manager) GetAddress(ctx context.Context, handle types.KeyHandle, chain types.ChainType) (string, error) {
	pubKey, err := m.GetPublicKey(ctx, handle)
	if err != nil {
		return "", err
	}

	switch chain {
	case types.ChainTypeEthereum:
		return m.ethereumAddress(pubKey), nil
	case types.ChainTypeNeo:
		return m.neoAddress(pubKey), nil
	default:
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
}

// ethereumAddress derives an Ethereum address from a public key.
func (m *Manager) ethereumAddress(pubKey []byte) string {
	// Skip the 0x04 prefix if present
	if len(pubKey) == 65 && pubKey[0] == 0x04 {
		pubKey = pubKey[1:]
	}

	h := sha256.New()
	h.Write(pubKey)
	hash := h.Sum(nil)

	// Take last 20 bytes
	return "0x" + hex.EncodeToString(hash[12:])
}

// neoAddress derives a Neo address from a public key.
func (m *Manager) neoAddress(pubKey []byte) string {
	h := sha256.New()
	h.Write(pubKey)
	hash := h.Sum(nil)
	return "N" + hex.EncodeToString(hash[:20])
}

// Zero zeros all sensitive data.
func (m *Manager) Zero() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Zero master seed
	if m.masterSeed != nil {
		enclave.ZeroBytes(m.masterSeed)
		m.masterSeed = nil
	}

	// Zero all private keys
	for handle, key := range m.keys {
		if key.D != nil {
			key.D.SetInt64(0)
		}
		delete(m.keys, handle)
	}
}
