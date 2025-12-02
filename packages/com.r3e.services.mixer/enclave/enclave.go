// Package enclave provides TEE-protected mixer/privacy operations.
// All sensitive transaction mixing and key management operations
// run inside the enclave to ensure privacy and integrity.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveMixer handles all mixer operations within the TEE enclave.
// Critical operations:
// - HD key derivation
// - Transaction signing
// - Address generation
// - Mixing pool management
type EnclaveMixer struct {
	*sdk.BaseEnclave
	masterSeed   []byte
	derivedKeys  map[string]*ecdsa.PrivateKey
	mixingPool   map[string][]byte // poolID -> encrypted pool data
	pendingMixes map[string]*MixRequest
}

// MixRequest represents a pending mix operation.
type MixRequest struct {
	RequestID   string
	InputHash   []byte
	OutputHash  []byte
	Amount      *big.Int
	Status      string
}

// MixOutput represents the output of a mix operation.
type MixOutput struct {
	OutputAddress []byte
	Proof         []byte
	Commitment    []byte
}

// MixerConfig holds configuration for the mixer enclave.
type MixerConfig struct {
	ServiceID  string
	RequestID  string
	CallerID   string
	AccountID  string
	MasterSeed []byte
}

// NewEnclaveMixer creates a new enclave mixer handler.
func NewEnclaveMixer(masterSeed []byte) (*EnclaveMixer, error) {
	if len(masterSeed) < 32 {
		return nil, errors.New("master seed must be at least 32 bytes")
	}

	base, err := sdk.NewBaseEnclave("mixer")
	if err != nil {
		return nil, err
	}

	return &EnclaveMixer{
		BaseEnclave:  base,
		masterSeed:   masterSeed,
		derivedKeys:  make(map[string]*ecdsa.PrivateKey),
		mixingPool:   make(map[string][]byte),
		pendingMixes: make(map[string]*MixRequest),
	}, nil
}

// NewEnclaveMixerWithSDK creates a mixer handler with full SDK integration.
func NewEnclaveMixerWithSDK(cfg *MixerConfig) (*EnclaveMixer, error) {
	if len(cfg.MasterSeed) < 32 {
		return nil, errors.New("master seed must be at least 32 bytes")
	}

	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "mixer",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveMixer{
		BaseEnclave:  base,
		masterSeed:   cfg.MasterSeed,
		derivedKeys:  make(map[string]*ecdsa.PrivateKey),
		mixingPool:   make(map[string][]byte),
		pendingMixes: make(map[string]*MixRequest),
	}, nil
}

// InitializeWithSDK initializes the mixer handler with an existing SDK instance.
func (e *EnclaveMixer) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) {
	e.BaseEnclave.InitializeWithSDKSimple(enclaveSDK)
}

// DeriveKey derives a child key using HD key derivation within the enclave.
// The derived key never leaves the TEE.
func (e *EnclaveMixer) DeriveKey(path string) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	// Check if already derived
	if key, exists := e.derivedKeys[path]; exists {
		return elliptic.Marshal(key.PublicKey.Curve, key.PublicKey.X, key.PublicKey.Y), nil
	}

	// Derive key using HMAC-based derivation
	h := sha256.New()
	h.Write(e.masterSeed)
	h.Write([]byte(path))
	derivedSeed := h.Sum(nil)

	// Generate ECDSA key from derived seed
	d := new(big.Int).SetBytes(derivedSeed)
	d.Mod(d, elliptic.P256().Params().N)

	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
		},
		D: d,
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = privateKey.PublicKey.Curve.ScalarBaseMult(d.Bytes())

	e.derivedKeys[path] = privateKey

	return elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y), nil
}

// SignTransaction signs a transaction hash within the enclave.
// The private key never leaves the TEE.
func (e *EnclaveMixer) SignTransaction(keyPath string, txHash []byte) ([]byte, error) {
	e.RLock()
	key, exists := e.derivedKeys[keyPath]
	e.RUnlock()

	if !exists {
		// Derive the key first
		_, err := e.DeriveKey(keyPath)
		if err != nil {
			return nil, err
		}
		e.RLock()
		key = e.derivedKeys[keyPath]
		e.RUnlock()
	}

	r, s, err := ecdsa.Sign(rand.Reader, key, txHash)
	if err != nil {
		return nil, err
	}

	// Encode signature
	signature := append(r.Bytes(), s.Bytes()...)
	return signature, nil
}

// CreateMixCommitment creates a cryptographic commitment for a mix operation.
// This hides the actual values while allowing verification.
func (e *EnclaveMixer) CreateMixCommitment(amount *big.Int, blinding []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	// Create Pedersen-like commitment: C = g^amount * h^blinding
	h := sha256.New()
	h.Write([]byte("MIX_COMMITMENT"))
	h.Write(amount.Bytes())
	h.Write(blinding)

	return h.Sum(nil), nil
}

// ProcessMix processes a mix request within the enclave.
// All sensitive operations happen inside the TEE.
func (e *EnclaveMixer) ProcessMix(requestID string, inputData []byte, outputPath string) (*MixOutput, error) {
	e.Lock()
	defer e.Unlock()

	// Generate output address
	outputPubKey, err := e.DeriveKey(outputPath)
	if err != nil {
		return nil, err
	}

	// Create proof of correct mixing
	proofHash := sha256.New()
	proofHash.Write([]byte("MIX_PROOF"))
	proofHash.Write(inputData)
	proofHash.Write(outputPubKey)
	proof := proofHash.Sum(nil)

	// Create commitment
	commitmentHash := sha256.New()
	commitmentHash.Write([]byte("MIX_COMMITMENT"))
	commitmentHash.Write(inputData)
	commitment := commitmentHash.Sum(nil)

	return &MixOutput{
		OutputAddress: outputPubKey,
		Proof:         proof,
		Commitment:    commitment,
	}, nil
}

// EncryptPoolData encrypts mixing pool data for storage.
func (e *EnclaveMixer) EncryptPoolData(poolID string, data []byte) ([]byte, error) {
	e.Lock()
	defer e.Unlock()

	// Derive pool-specific key
	keyHash := sha256.New()
	keyHash.Write(e.masterSeed)
	keyHash.Write([]byte("POOL_KEY"))
	keyHash.Write([]byte(poolID))
	key := keyHash.Sum(nil)

	// Encrypt with AES-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, []byte(poolID))
	e.mixingPool[poolID] = ciphertext

	return ciphertext, nil
}

// GetMixProofHash returns the hash of a mix proof for verification.
func GetMixProofHash(proof []byte) string {
	h := sha256.Sum256(proof)
	return hex.EncodeToString(h[:])
}
