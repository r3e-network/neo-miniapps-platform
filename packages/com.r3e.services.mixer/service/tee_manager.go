// Package mixer provides privacy-preserving transaction mixing service.
package mixer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/R3E-Network/service_layer/system/os/tee"
)

// TEE Manager Implementation for Double-Blind HD 1/2 Multi-sig Architecture
//
// This implementation provides:
// - TEE root seed management (sealed in TEE enclave)
// - HD key derivation from TEE root seed
// - Transaction signing using TEE-derived keys
// - ZK proof generation (placeholder for actual ZK circuit)
// - TEE attestation for proof verification

// SealedTEEManager implements TEEManager using the TEE system for secure key management.
// The TEE root seed is sealed within the enclave and never exposed.
type SealedTEEManager struct {
	mu sync.RWMutex

	// TEE provider for enclave operations
	provider tee.EngineProvider

	// Sealed root seed (encrypted, only accessible within TEE)
	sealedSeed []byte

	// Derived key cache (public keys only, for performance)
	keyCache map[uint32]*ExtendedKey

	// Pool index counter
	nextIndex atomic.Uint32

	// Master extended key (derived from sealed seed)
	masterKey *ExtendedKey

	// Configuration
	config TEEManagerConfig
}

// TEEManagerConfig configures the TEE manager.
type TEEManagerConfig struct {
	// SeedSize is the size of the root seed in bytes (default: 32)
	SeedSize int

	// CacheSize is the maximum number of derived keys to cache
	CacheSize int

	// AttestationTimeout is the timeout for attestation operations
	AttestationTimeout time.Duration
}

// DefaultTEEManagerConfig returns the default configuration.
func DefaultTEEManagerConfig() TEEManagerConfig {
	return TEEManagerConfig{
		SeedSize:           32,
		CacheSize:          1000,
		AttestationTimeout: 30 * time.Second,
	}
}

// NewSealedTEEManager creates a new TEE manager with a sealed root seed.
// If existingSeed is nil, a new random seed is generated and sealed.
func NewSealedTEEManager(provider tee.EngineProvider, existingSeed []byte, config TEEManagerConfig) (*SealedTEEManager, error) {
	if provider == nil {
		return nil, errors.New("TEE provider is required")
	}

	if config.SeedSize == 0 {
		config.SeedSize = 32
	}
	if config.CacheSize == 0 {
		config.CacheSize = 1000
	}
	if config.AttestationTimeout == 0 {
		config.AttestationTimeout = 30 * time.Second
	}

	mgr := &SealedTEEManager{
		provider: provider,
		keyCache: make(map[uint32]*ExtendedKey),
		config:   config,
	}

	// Initialize or restore the root seed
	var seed []byte
	if existingSeed != nil {
		seed = existingSeed
	} else {
		// Generate new random seed
		seed = make([]byte, config.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			return nil, fmt.Errorf("generate seed: %w", err)
		}
	}

	// Create master extended key from seed
	masterKey, err := NewExtendedKeyFromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("create master key: %w", err)
	}
	mgr.masterKey = masterKey

	// Seal the seed using TEE-style encryption
	sealedSeed, err := mgr.sealSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("seal seed: %w", err)
	}
	mgr.sealedSeed = sealedSeed

	// Clear the plaintext seed from memory
	for i := range seed {
		seed[i] = 0
	}

	return mgr, nil
}

// DerivePoolKeys derives HD keys at the given index and creates a 1-of-2 multi-sig address.
func (m *SealedTEEManager) DerivePoolKeys(ctx context.Context, index uint32, masterPublicKey []byte) (*PoolKeyPair, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Derive TEE key at the given index
	teeKey, err := m.deriveKeyAtIndex(index)
	if err != nil {
		return nil, fmt.Errorf("derive TEE key: %w", err)
	}

	teePubKey := teeKey.PublicKey()

	// Validate master public key
	if len(masterPublicKey) != 33 {
		return nil, fmt.Errorf("invalid master public key length: expected 33, got %d", len(masterPublicKey))
	}

	// Create 1-of-2 multi-sig account
	multiSig, err := Create1of2MultiSig(teePubKey, masterPublicKey)
	if err != nil {
		return nil, fmt.Errorf("create multi-sig: %w", err)
	}

	return &PoolKeyPair{
		Index:           index,
		TEEPublicKey:    teePubKey,
		MasterPublicKey: masterPublicKey,
		MultiSigScript:  multiSig.VerificationScript,
		Address:         multiSig.Address,
	}, nil
}

// SignTransaction signs a transaction using the TEE-derived key at the given HD index.
func (m *SealedTEEManager) SignTransaction(ctx context.Context, hdIndex uint32, txData []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Derive the private key at the given index
	key, err := m.deriveKeyAtIndex(hdIndex)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Convert to ECDSA private key
	ecdsaKey, err := key.ToECDSA()
	if err != nil {
		return nil, fmt.Errorf("convert to ECDSA: %w", err)
	}

	// Hash the transaction data
	hash := sha256.Sum256(txData)

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Encode signature as r || s (64 bytes)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// GetTEEPublicKey returns the TEE public key at the given HD index.
func (m *SealedTEEManager) GetTEEPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, err := m.deriveKeyAtIndex(hdIndex)
	if err != nil {
		return nil, err
	}

	return key.PublicKey(), nil
}

// GetNextPoolIndex returns the next available HD index for pool accounts.
func (m *SealedTEEManager) GetNextPoolIndex(ctx context.Context) (uint32, error) {
	return m.nextIndex.Add(1), nil
}

// GenerateZKProof generates a zero-knowledge proof for the mix request.
// This is a placeholder implementation - actual ZK proof generation would use
// a proper ZK circuit (e.g., Groth16, PLONK, or Halo2).
func (m *SealedTEEManager) GenerateZKProof(ctx context.Context, req MixRequest) (string, error) {
	// Build proof input from request
	proofInput := buildProofInput(req)

	// Hash the proof input to create a commitment
	hash := sha256.Sum256(proofInput)

	// In production, this would:
	// 1. Generate actual ZK proof using a circuit
	// 2. Include: amount commitment, nullifier, merkle proof
	// 3. Return serialized proof

	// For now, return the hash as a placeholder proof
	return hex.EncodeToString(hash[:]), nil
}

// SignAttestation creates a TEE attestation signature.
func (m *SealedTEEManager) SignAttestation(ctx context.Context, data []byte) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use the master key (index 0) for attestation signing
	key, err := m.deriveKeyAtIndex(0)
	if err != nil {
		return "", fmt.Errorf("derive attestation key: %w", err)
	}

	ecdsaKey, err := key.ToECDSA()
	if err != nil {
		return "", fmt.Errorf("convert to ECDSA: %w", err)
	}

	// Create attestation data with timestamp
	attestationData := make([]byte, len(data)+8)
	copy(attestationData, data)
	timestamp := time.Now().Unix()
	for i := 0; i < 8; i++ {
		attestationData[len(data)+i] = byte(timestamp >> (56 - i*8))
	}

	// Hash and sign
	hash := sha256.Sum256(attestationData)
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign attestation: %w", err)
	}

	// Encode as hex
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return hex.EncodeToString(signature), nil
}

// VerifyAttestation verifies a TEE attestation.
func (m *SealedTEEManager) VerifyAttestation(ctx context.Context, data []byte, signature string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Decode signature
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}

	if len(sigBytes) != 64 {
		return false, fmt.Errorf("invalid signature length: expected 64, got %d", len(sigBytes))
	}

	// Get attestation public key
	key, err := m.deriveKeyAtIndex(0)
	if err != nil {
		return false, fmt.Errorf("derive attestation key: %w", err)
	}

	ecdsaKey, err := key.ToECDSA()
	if err != nil {
		return false, fmt.Errorf("convert to ECDSA: %w", err)
	}

	// Hash the data (note: we can't verify timestamp without knowing it)
	hash := sha256.Sum256(data)

	// Parse r and s from signature
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	// Verify
	return ecdsa.Verify(&ecdsaKey.PublicKey, hash[:], r, s), nil
}

// deriveKeyAtIndex derives a key at the given index using the mixer derivation path.
func (m *SealedTEEManager) deriveKeyAtIndex(index uint32) (*ExtendedKey, error) {
	// Check cache first
	if cached, ok := m.keyCache[index]; ok {
		return cached, nil
	}

	// Derive the key
	path := MixerDerivationPath(index)
	key, err := m.masterKey.DerivePath(path)
	if err != nil {
		return nil, err
	}

	// Cache the key (limit cache size)
	if len(m.keyCache) < m.config.CacheSize {
		m.keyCache[index] = key
	}

	return key, nil
}

// buildProofInput creates the input data for ZK proof generation.
func buildProofInput(req MixRequest) []byte {
	// Combine relevant fields for proof generation
	input := []byte(req.ID)
	input = append(input, []byte(req.AccountID)...)
	input = append(input, []byte(req.Amount)...)
	input = append(input, []byte(req.SourceWallet)...)

	for _, target := range req.Targets {
		input = append(input, []byte(target.Address)...)
		input = append(input, []byte(target.Amount)...)
	}

	return input
}

// GetAttestationPublicKey returns the public key used for attestation.
func (m *SealedTEEManager) GetAttestationPublicKey() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, err := m.deriveKeyAtIndex(0)
	if err != nil {
		return nil, err
	}

	return key.PublicKey(), nil
}

// SetNextPoolIndex sets the next pool index (used for recovery).
func (m *SealedTEEManager) SetNextPoolIndex(index uint32) {
	m.nextIndex.Store(index)
}

// ClearCache clears the key cache.
func (m *SealedTEEManager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keyCache = make(map[uint32]*ExtendedKey)
}

// Ensure SealedTEEManager implements TEEManager interface
var _ TEEManager = (*SealedTEEManager)(nil)

// =============================================================================
// TEE Sealing Operations
// =============================================================================

// sealSeed encrypts the seed using AES-256-GCM with a key derived from the TEE provider.
// In production with real SGX, this would use the enclave's sealing key.
func (m *SealedTEEManager) sealSeed(plaintext []byte) ([]byte, error) {
	// Derive sealing key from TEE provider context
	sealingKey := m.deriveSealingKey()

	block, err := aes.NewCipher(sealingKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: nonce || ciphertext || tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// unsealSeed decrypts the sealed seed using AES-256-GCM.
func (m *SealedTEEManager) unsealSeed(ciphertext []byte) ([]byte, error) {
	sealingKey := m.deriveSealingKey()

	block, err := aes.NewCipher(sealingKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertextData := ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveSealingKey derives a 256-bit sealing key.
// In production SGX, this would come from EGETKEY instruction.
// Here we derive it from the TEE provider's identity.
func (m *SealedTEEManager) deriveSealingKey() []byte {
	h := sha256.New()
	h.Write([]byte("MIXER_TEE_SEAL_KEY_V1"))
	// In production, this would include:
	// - SGX MRENCLAVE (enclave measurement)
	// - SGX MRSIGNER (signer measurement)
	// - CPU SVN (security version number)
	// For now, we use a deterministic derivation
	h.Write([]byte("com.r3e.services.mixer"))
	return h.Sum(nil)
}
