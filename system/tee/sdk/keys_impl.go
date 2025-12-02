// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// keyManagerImpl implements KeyManager interface.
type keyManagerImpl struct {
	mu       sync.RWMutex
	keys     map[string]*storedKey
	sealKey  []byte
	callerID string
}

// storedKey represents a key stored in the enclave.
type storedKey struct {
	ID         string
	Type       KeyType
	Curve      KeyCurve
	PrivateKey interface{} // *ecdsa.PrivateKey, ed25519.PrivateKey, etc.
	PublicKey  []byte
	CreatedAt  time.Time
	ParentID   string
	Path       string
	Label      string
}

// NewKeyManager creates a new key manager instance.
func NewKeyManager(sealKey []byte, callerID string) KeyManager {
	return &keyManagerImpl{
		keys:     make(map[string]*storedKey),
		sealKey:  sealKey,
		callerID: callerID,
	}
}

func (m *keyManagerImpl) GenerateKey(ctx context.Context, req *GenerateKeyRequest) (*GenerateKeyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var privateKey interface{}
	var publicKeyBytes []byte
	var err error

	switch req.Type {
	case KeyTypeECDSA:
		privateKey, publicKeyBytes, err = m.generateECDSAKey(req.Curve)
	default:
		return nil, errors.New("unsupported key type")
	}

	if err != nil {
		return nil, err
	}

	keyID := generateKeyID(req.Label)
	now := time.Now()

	stored := &storedKey{
		ID:         keyID,
		Type:       req.Type,
		Curve:      req.Curve,
		PrivateKey: privateKey,
		PublicKey:  publicKeyBytes,
		CreatedAt:  now,
		Label:      req.Label,
	}

	m.keys[keyID] = stored

	return &GenerateKeyResponse{
		KeyID:     keyID,
		PublicKey: publicKeyBytes,
		CreatedAt: now,
	}, nil
}

func (m *keyManagerImpl) ImportKey(ctx context.Context, req *ImportKeyRequest) (*ImportKeyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var privateKey interface{}
	var publicKeyBytes []byte
	var err error

	switch req.Type {
	case KeyTypeECDSA:
		privateKey, publicKeyBytes, err = m.importECDSAKey(req.Curve, req.PrivateKey)
	default:
		return nil, errors.New("unsupported key type")
	}

	if err != nil {
		return nil, err
	}

	keyID := generateKeyID(req.Label)
	now := time.Now()

	stored := &storedKey{
		ID:         keyID,
		Type:       req.Type,
		Curve:      req.Curve,
		PrivateKey: privateKey,
		PublicKey:  publicKeyBytes,
		CreatedAt:  now,
		Label:      req.Label,
	}

	m.keys[keyID] = stored

	return &ImportKeyResponse{
		KeyID:     keyID,
		PublicKey: publicKeyBytes,
		CreatedAt: now,
	}, nil
}

func (m *keyManagerImpl) ExportPublicKey(ctx context.Context, keyID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stored, exists := m.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return stored.PublicKey, nil
}

func (m *keyManagerImpl) DeleteKey(ctx context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.keys[keyID]; !exists {
		return ErrKeyNotFound
	}

	delete(m.keys, keyID)
	return nil
}

func (m *keyManagerImpl) ListKeys(ctx context.Context) ([]*KeyInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []*KeyInfo
	for _, stored := range m.keys {
		keys = append(keys, &KeyInfo{
			ID:        stored.ID,
			Type:      stored.Type,
			Curve:     stored.Curve,
			PublicKey: stored.PublicKey,
			CreatedAt: stored.CreatedAt,
			ParentID:  stored.ParentID,
			Path:      stored.Path,
		})
	}

	return keys, nil
}

func (m *keyManagerImpl) DeriveKey(ctx context.Context, req *DeriveKeyRequest) (*DeriveKeyResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parent, exists := m.keys[req.ParentKeyID]
	if !exists {
		return nil, ErrKeyNotFound
	}

	// For now, implement simple key derivation using HMAC
	// In production, use proper BIP32/BIP44 derivation
	var derivedPrivateKey interface{}
	var derivedPublicKey []byte
	var err error

	switch parent.Type {
	case KeyTypeECDSA:
		derivedPrivateKey, derivedPublicKey, err = m.deriveECDSAKey(parent, req.Path)
	default:
		return nil, errors.New("key derivation not supported for this key type")
	}

	if err != nil {
		return nil, err
	}

	keyID := generateKeyID(req.Label)
	now := time.Now()

	stored := &storedKey{
		ID:         keyID,
		Type:       parent.Type,
		Curve:      parent.Curve,
		PrivateKey: derivedPrivateKey,
		PublicKey:  derivedPublicKey,
		CreatedAt:  now,
		ParentID:   req.ParentKeyID,
		Path:       req.Path,
		Label:      req.Label,
	}

	m.keys[keyID] = stored

	return &DeriveKeyResponse{
		KeyID:     keyID,
		PublicKey: derivedPublicKey,
		Path:      req.Path,
		CreatedAt: now,
	}, nil
}

// GetPrivateKey returns the private key for internal use (signing).
func (m *keyManagerImpl) GetPrivateKey(keyID string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stored, exists := m.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return stored.PrivateKey, nil
}

// generateECDSAKey generates a new ECDSA key pair.
func (m *keyManagerImpl) generateECDSAKey(curve KeyCurve) (*ecdsa.PrivateKey, []byte, error) {
	var c elliptic.Curve
	switch curve {
	case KeyCurveP256:
		c = elliptic.P256()
	case KeyCurveP384:
		c = elliptic.P384()
	case KeyCurveSecp256k1:
		// Note: secp256k1 requires external library in production
		c = elliptic.P256() // Fallback for now
	default:
		c = elliptic.P256()
	}

	privateKey, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	publicKeyBytes := elliptic.Marshal(c, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	return privateKey, publicKeyBytes, nil
}

// importECDSAKey imports an existing ECDSA private key.
func (m *keyManagerImpl) importECDSAKey(curve KeyCurve, privateKeyBytes []byte) (*ecdsa.PrivateKey, []byte, error) {
	var c elliptic.Curve
	switch curve {
	case KeyCurveP256:
		c = elliptic.P256()
	case KeyCurveP384:
		c = elliptic.P384()
	default:
		c = elliptic.P256()
	}

	// Parse private key bytes (assuming raw D value)
	if len(privateKeyBytes) != 32 && len(privateKeyBytes) != 48 {
		return nil, nil, ErrInvalidKey
	}

	privateKey := new(ecdsa.PrivateKey)
	privateKey.Curve = c
	privateKey.D.SetBytes(privateKeyBytes)
	privateKey.PublicKey.X, privateKey.PublicKey.Y = c.ScalarBaseMult(privateKeyBytes)

	publicKeyBytes := elliptic.Marshal(c, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	return privateKey, publicKeyBytes, nil
}

// deriveECDSAKey derives a child ECDSA key.
func (m *keyManagerImpl) deriveECDSAKey(parent *storedKey, path string) (*ecdsa.PrivateKey, []byte, error) {
	parentKey, ok := parent.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, errors.New("invalid parent key type")
	}

	// Simple derivation using HMAC-SHA256
	// In production, use proper BIP32 derivation
	h := sha256.New()
	h.Write(parentKey.D.Bytes())
	h.Write([]byte(path))
	derivedD := h.Sum(nil)

	derivedKey := new(ecdsa.PrivateKey)
	derivedKey.Curve = parentKey.Curve
	derivedKey.D.SetBytes(derivedD)
	derivedKey.PublicKey.X, derivedKey.PublicKey.Y = parentKey.Curve.ScalarBaseMult(derivedD)

	publicKeyBytes := elliptic.Marshal(parentKey.Curve, derivedKey.PublicKey.X, derivedKey.PublicKey.Y)
	return derivedKey, publicKeyBytes, nil
}

// generateKeyID generates a unique key ID.
func generateKeyID(label string) string {
	h := sha256.New()
	h.Write([]byte(label))
	h.Write([]byte(time.Now().String()))
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	h.Write(randBytes)
	return "key_" + hex.EncodeToString(h.Sum(nil))[:24]
}
