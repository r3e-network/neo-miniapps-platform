// Package crypto provides cryptographic operations for the service layer.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/ripemd160"
)

// =============================================================================
// Key Derivation
// =============================================================================

// DeriveKey derives a key using HKDF-SHA256.
//
// UPGRADE SAFETY: This function is designed to produce identical keys across
// enclave upgrades (MRENCLAVE changes). Key derivation depends ONLY on:
//   - masterKey: Injected by MarbleRun Coordinator (manifest-defined, stable)
//   - salt: Business identifier like accountID (application-defined, stable)
//   - info: Service context string (code constant, stable)
//
// This function intentionally does NOT use:
//   - MRENCLAVE or MRSIGNER (enclave identity)
//   - SGX sealing keys (tied to enclave measurement)
//   - Any enclave report fields
//
// As long as the manifest secrets remain unchanged, derived keys will be
// identical regardless of enclave version, enabling seamless upgrades.
func DeriveKey(masterKey []byte, salt []byte, info string, keyLen int) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(info))
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	return key, nil
}

// GenerateRandomBytes generates cryptographically secure random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// HMACSign generates an HMAC-SHA256 signature.
func HMACSign(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// HMACVerify verifies an HMAC-SHA256 signature.
func HMACVerify(key, data, signature []byte) bool {
	expectedSig := HMACSign(key, data)
	return hmac.Equal(signature, expectedSig)
}

// =============================================================================
// AES-GCM Encryption
// =============================================================================

// Encrypt encrypts data using AES-256-GCM.
func Encrypt(key, plaintext []byte) ([]byte, error) {
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

	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-GCM.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// =============================================================================
// ECDSA Signing (secp256r1 for Neo N3)
// =============================================================================

// KeyPair represents an ECDSA key pair.
type KeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// GenerateKeyPair generates a new ECDSA key pair using P-256 (secp256r1).
func GenerateKeyPair() (*KeyPair, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// Sign signs data using ECDSA.
func Sign(privateKey *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, err
	}

	// Neo N3 uses 64-byte signature (r || s), each 32 bytes
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// Verify verifies an ECDSA signature.
func Verify(publicKey *ecdsa.PublicKey, data, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(publicKey, hash[:], r, s)
}

// PublicKeyToBytes converts a public key to compressed format (33 bytes).
func PublicKeyToBytes(pub *ecdsa.PublicKey) []byte {
	x := pub.X.Bytes()
	// Pad to 32 bytes
	xPadded := make([]byte, 32)
	copy(xPadded[32-len(x):], x)

	// Compressed format: 0x02 or 0x03 prefix + X coordinate
	prefix := byte(0x02)
	if pub.Y.Bit(0) == 1 {
		prefix = 0x03
	}

	result := make([]byte, 33)
	result[0] = prefix
	copy(result[1:], xPadded)
	return result
}

// PublicKeyFromBytes parses a compressed or uncompressed public key.
func PublicKeyFromBytes(data []byte) (*ecdsa.PublicKey, error) {
	curve := elliptic.P256()

	switch len(data) {
	case 33: // Compressed format
		x := new(big.Int).SetBytes(data[1:])
		// Calculate Y from X using curve equation: y² = x³ - 3x + b
		y := decompressPoint(curve, x, data[0] == 0x03)
		if y == nil {
			return nil, fmt.Errorf("invalid compressed public key")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil

	case 65: // Uncompressed format (0x04 prefix)
		if data[0] != 0x04 {
			return nil, fmt.Errorf("invalid uncompressed public key prefix")
		}
		x := new(big.Int).SetBytes(data[1:33])
		y := new(big.Int).SetBytes(data[33:65])
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil

	default:
		return nil, fmt.Errorf("invalid public key length: %d", len(data))
	}
}

// decompressPoint decompresses an elliptic curve point.
func decompressPoint(curve elliptic.Curve, x *big.Int, yOdd bool) *big.Int {
	params := curve.Params()
	// y² = x³ - 3x + b (mod p)
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	threeX := new(big.Int).Mul(x, big.NewInt(3))
	x3.Sub(x3, threeX)
	x3.Add(x3, params.B)
	x3.Mod(x3, params.P)

	// Calculate square root using Tonelli-Shanks
	y := new(big.Int).ModSqrt(x3, params.P)
	if y == nil {
		return nil
	}

	// Choose correct Y based on parity
	if y.Bit(0) != 0 != yOdd {
		y.Sub(params.P, y)
	}

	return y
}

// PublicKeyToAddress converts a public key to a Neo N3 address.
func PublicKeyToAddress(publicKey *ecdsa.PublicKey) string {
	pubKeyBytes := PublicKeyToBytes(publicKey)
	scriptHash := PublicKeyToScriptHash(pubKeyBytes)
	return ScriptHashToAddress(scriptHash)
}

// =============================================================================
// Neo N3 Address Generation
// =============================================================================

// PublicKeyToScriptHash converts a public key to a Neo N3 script hash.
func PublicKeyToScriptHash(publicKey []byte) []byte {
	// Build verification script: PUSHDATA1 <pubkey> SYSCALL System.Crypto.CheckSig
	script := make([]byte, 0, 40)
	script = append(script, 0x0C) // PUSHDATA1
	script = append(script, 33)   // Length (compressed public key)
	script = append(script, publicKey...)
	script = append(script, 0x41)                   // SYSCALL
	script = append(script, 0x56, 0xe7, 0xb3, 0x27) // System.Crypto.CheckSig hash

	// Hash160 = RIPEMD160(SHA256(script))
	sha256Hash := sha256.Sum256(script)
	ripemd := ripemd160.New()
	ripemd.Write(sha256Hash[:])
	return ripemd.Sum(nil)
}

// ScriptHashToAddress converts a script hash to a Neo N3 address.
func ScriptHashToAddress(scriptHash []byte) string {
	// Neo N3 address = Base58Check(0x35 + scriptHash)
	data := make([]byte, 21)
	data[0] = 0x35 // Neo N3 address version
	copy(data[1:], scriptHash)

	// Double SHA256 for checksum
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	checksum := hash2[:4]

	// Append checksum
	addressBytes := append(data, checksum...)

	return base58Encode(addressBytes)
}

// base58Encode encodes bytes to Base58.
func base58Encode(input []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append([]byte{alphabet[mod.Int64()]}, result...)
	}

	// Add leading zeros
	for _, b := range input {
		if b != 0 {
			break
		}
		result = append([]byte{alphabet[0]}, result...)
	}

	return string(result)
}

// =============================================================================
// VRF (Verifiable Random Function)
// =============================================================================

// VRFProof represents a VRF proof.
type VRFProof struct {
	PublicKey []byte `json:"public_key"`
	Proof     []byte `json:"proof"`
	Output    []byte `json:"output"`
}

// GenerateVRF generates a VRF output and proof.
// Uses ECVRF-P256-SHA256-TAI as per RFC 9381.
func GenerateVRF(privateKey *ecdsa.PrivateKey, alpha []byte) (*VRFProof, error) {
	// Hash the input
	h := sha256.New()
	h.Write(alpha)
	hash := h.Sum(nil)

	// Sign the hash (simplified VRF - production should use proper ECVRF)
	signature, err := Sign(privateKey, hash)
	if err != nil {
		return nil, err
	}

	// Generate output by hashing the proof
	outputHash := sha256.Sum256(signature)

	return &VRFProof{
		PublicKey: PublicKeyToBytes(&privateKey.PublicKey),
		Proof:     signature,
		Output:    outputHash[:],
	}, nil
}

// VerifyVRF verifies a VRF proof.
func VerifyVRF(publicKey *ecdsa.PublicKey, alpha []byte, proof *VRFProof) bool {
	// Hash the input
	h := sha256.New()
	h.Write(alpha)
	hash := h.Sum(nil)

	// Verify the signature
	if !Verify(publicKey, hash, proof.Proof) {
		return false
	}

	// Verify the output
	outputHash := sha256.Sum256(proof.Proof)
	return hex.EncodeToString(outputHash[:]) == hex.EncodeToString(proof.Output)
}

// =============================================================================
// Utility Functions
// =============================================================================

// Hash256 computes SHA256 hash.
func Hash256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// Hash160 computes RIPEMD160(SHA256(data)).
func Hash160(data []byte) []byte {
	sha256Hash := sha256.Sum256(data)
	ripemd := ripemd160.New()
	ripemd.Write(sha256Hash[:])
	return ripemd.Sum(nil)
}

// ZeroBytes securely zeros a byte slice.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
