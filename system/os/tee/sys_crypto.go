// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// sys.crypto - Cryptographic Operations
//
// This file implements the sys.crypto API for cryptographic operations within the TEE.
// All operations are designed to be secure and suitable for use in trusted computing.
package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"math/big"
	"sync"

	"golang.org/x/crypto/sha3"
)

// sysCryptoImpl implements SysCrypto with real cryptographic operations.
type sysCryptoImpl struct {
	mu   sync.RWMutex
	keys map[string]*storedKey
}

// storedKey represents a key stored in the crypto module.
type storedKey struct {
	KeyID      string
	KeyType    string
	PrivateKey any
	PublicKey  []byte
}

// NewSysCrypto creates a new SysCrypto implementation.
func NewSysCrypto() SysCrypto {
	return &sysCryptoImpl{
		keys: make(map[string]*storedKey),
	}
}

// Hash computes a cryptographic hash.
func (c *sysCryptoImpl) Hash(algorithm string, data []byte) ([]byte, error) {
	var h hash.Hash

	switch algorithm {
	case "sha256", "SHA256":
		h = sha256.New()
	case "sha512", "SHA512":
		h = sha512.New()
	case "sha3-256", "SHA3-256":
		h = sha3.New256()
	case "sha3-512", "SHA3-512":
		h = sha3.New512()
	case "keccak256", "KECCAK256":
		h = sha3.NewLegacyKeccak256()
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	h.Write(data)
	return h.Sum(nil), nil
}

// Sign signs data using the enclave's signing key.
func (c *sysCryptoImpl) Sign(data []byte) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Use default signing key if available
	key, ok := c.keys["default"]
	if !ok {
		// Generate a default key if none exists
		c.mu.RUnlock()
		kp, err := c.GenerateKey("ecdsa-p256")
		c.mu.RLock()
		if err != nil {
			return nil, fmt.Errorf("no signing key available: %w", err)
		}
		key = c.keys[kp.KeyID]
	}

	if key == nil {
		return nil, fmt.Errorf("no signing key available")
	}

	switch k := key.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		hash := sha256.Sum256(data)
		r, s, err := ecdsa.Sign(rand.Reader, k, hash[:])
		if err != nil {
			return nil, fmt.Errorf("sign failed: %w", err)
		}
		// Encode r and s as 32-byte big-endian integers
		sig := make([]byte, 64)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[32-len(rBytes):32], rBytes)
		copy(sig[64-len(sBytes):64], sBytes)
		return sig, nil
	default:
		return nil, fmt.Errorf("unsupported key type for signing")
	}
}

// Verify verifies a signature.
func (c *sysCryptoImpl) Verify(data []byte, signature []byte, publicKey []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("invalid signature length: expected 64, got %d", len(signature))
	}

	// Parse public key (assume P-256 uncompressed format)
	if len(publicKey) != 65 || publicKey[0] != 0x04 {
		return false, fmt.Errorf("invalid public key format")
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), publicKey)
	if x == nil {
		return false, fmt.Errorf("failed to parse public key")
	}

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	hash := sha256.Sum256(data)
	return ecdsa.Verify(pubKey, hash[:], r, s), nil
}

// Encrypt encrypts data using the specified key.
func (c *sysCryptoImpl) Encrypt(keyID string, plaintext []byte) ([]byte, error) {
	c.mu.RLock()
	key, ok := c.keys[keyID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	switch k := key.PrivateKey.(type) {
	case []byte:
		// AES key
		return c.aesEncrypt(k, plaintext)
	default:
		return nil, fmt.Errorf("key type does not support encryption")
	}
}

// Decrypt decrypts data using the specified key.
func (c *sysCryptoImpl) Decrypt(keyID string, ciphertext []byte) ([]byte, error) {
	c.mu.RLock()
	key, ok := c.keys[keyID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	switch k := key.PrivateKey.(type) {
	case []byte:
		// AES key
		return c.aesDecrypt(k, ciphertext)
	default:
		return nil, fmt.Errorf("key type does not support decryption")
	}
}

// GenerateKey generates a new key pair.
func (c *sysCryptoImpl) GenerateKey(keyType string) (*KeyPair, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyID := generateKeyID()

	switch keyType {
	case "ecdsa-p256", "ECDSA-P256":
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ECDSA key: %w", err)
		}
		publicKey := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)

		c.keys[keyID] = &storedKey{
			KeyID:      keyID,
			KeyType:    keyType,
			PrivateKey: privateKey,
			PublicKey:  publicKey,
		}

		// Set as default if no default exists
		if _, ok := c.keys["default"]; !ok {
			c.keys["default"] = c.keys[keyID]
		}

		return &KeyPair{
			KeyID:     keyID,
			KeyType:   keyType,
			PublicKey: publicKey,
		}, nil

	case "aes-128", "AES-128":
		key := make([]byte, 16)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate AES-128 key: %w", err)
		}

		c.keys[keyID] = &storedKey{
			KeyID:      keyID,
			KeyType:    keyType,
			PrivateKey: key,
			PublicKey:  nil, // Symmetric key has no public key
		}

		return &KeyPair{
			KeyID:     keyID,
			KeyType:   keyType,
			PublicKey: nil,
		}, nil

	case "aes-256", "AES-256":
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate AES-256 key: %w", err)
		}

		c.keys[keyID] = &storedKey{
			KeyID:      keyID,
			KeyType:    keyType,
			PrivateKey: key,
			PublicKey:  nil,
		}

		return &KeyPair{
			KeyID:     keyID,
			KeyType:   keyType,
			PublicKey: nil,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// RandomBytes generates cryptographically secure random bytes.
func (c *sysCryptoImpl) RandomBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}
	if length > 1024*1024 { // 1MB limit
		return nil, fmt.Errorf("length exceeds maximum (1MB)")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return bytes, nil
}

// HMAC computes an HMAC.
func (c *sysCryptoImpl) HMAC(algorithm string, key []byte, data []byte) ([]byte, error) {
	var h func() hash.Hash

	switch algorithm {
	case "sha256", "SHA256":
		h = sha256.New
	case "sha512", "SHA512":
		h = sha512.New
	default:
		return nil, fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
	}

	mac := hmac.New(h, key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

// aesEncrypt encrypts data using AES-GCM.
func (c *sysCryptoImpl) aesEncrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// aesDecrypt decrypts data using AES-GCM.
func (c *sysCryptoImpl) aesDecrypt(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// generateKeyID generates a unique key ID.
func generateKeyID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "key_" + hex.EncodeToString(bytes)
}

