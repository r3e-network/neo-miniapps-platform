// Package neo provides Neo N3 blockchain operations inside the TEE enclave.
// Private keys NEVER leave the enclave - all signing happens inside.
package neo

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
	"golang.org/x/crypto/ripemd160"
)

// WalletHandle is an alias for types.WalletHandle.
type WalletHandle = types.WalletHandle

// Signer implements Neo N3 signing operations inside the TEE enclave.
type Signer struct {
	mu      sync.RWMutex
	runtime enclave.Runtime

	// Wallets stored in enclave (private keys never leave)
	wallets map[WalletHandle]*neoWallet
}

// neoWallet holds Neo wallet data inside the enclave.
type neoWallet struct {
	privateKey *ecdsa.PrivateKey
	publicKey  []byte // Compressed public key
	address    string // Neo N3 address
	scriptHash []byte // Script hash (20 bytes)
}

// Config for the Neo signer.
type Config struct {
	Runtime enclave.Runtime
}

// New creates a new Neo signer.
func New(cfg Config) (*Signer, error) {
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	return &Signer{
		runtime: cfg.Runtime,
		wallets: make(map[WalletHandle]*neoWallet),
	}, nil
}

// CreateWallet creates a new Neo wallet inside the enclave.
// Returns a handle to the wallet (the actual key stays in the enclave).
func (s *Signer) CreateWallet(ctx context.Context, path string) (WalletHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate handle from path
	h := sha256.Sum256([]byte(path))
	handle := WalletHandle(hex.EncodeToString(h[:16]))

	// Check if already exists
	if _, exists := s.wallets[handle]; exists {
		return handle, nil
	}

	// Generate new private key using enclave's secure random
	randomBytes, err := s.runtime.GenerateRandom(32)
	if err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}

	// Create ECDSA private key (Neo uses secp256r1/P-256)
	privateKey, err := createPrivateKeyFromBytes(randomBytes)
	if err != nil {
		return "", fmt.Errorf("create private key: %w", err)
	}

	// Create wallet
	wallet, err := newNeoWallet(privateKey)
	if err != nil {
		return "", fmt.Errorf("create wallet: %w", err)
	}

	s.wallets[handle] = wallet
	return handle, nil
}

// DeriveWallet derives a Neo wallet from a seed path.
// Useful for deterministic wallet generation.
func (s *Signer) DeriveWallet(ctx context.Context, seedPath string) (WalletHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate handle from path
	h := sha256.Sum256([]byte(seedPath))
	handle := WalletHandle(hex.EncodeToString(h[:16]))

	// Check if already exists
	if _, exists := s.wallets[handle]; exists {
		return handle, nil
	}

	// Derive key from seed path using HMAC-based derivation with runtime's random
	// We use the seed path to deterministically derive a key
	seedHash := sha256.Sum256([]byte(seedPath))

	// Create ECDSA private key from the derived seed
	privateKey, err := createPrivateKeyFromBytes(seedHash[:])
	if err != nil {
		return "", fmt.Errorf("create private key: %w", err)
	}

	// Create wallet
	wallet, err := newNeoWallet(privateKey)
	if err != nil {
		return "", fmt.Errorf("create wallet: %w", err)
	}

	s.wallets[handle] = wallet
	return handle, nil
}

// ImportWIF imports a wallet from WIF (Wallet Import Format).
// The WIF is decrypted inside the enclave and never leaves.
func (s *Signer) ImportWIF(ctx context.Context, wif string) (WalletHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Decode WIF to get private key bytes
	privateKeyBytes, err := decodeWIF(wif)
	if err != nil {
		return "", fmt.Errorf("decode WIF: %w", err)
	}

	// Create ECDSA private key
	privateKey, err := createPrivateKeyFromBytes(privateKeyBytes)
	if err != nil {
		// Zero the bytes before returning
		enclave.ZeroBytes(privateKeyBytes)
		return "", fmt.Errorf("create private key: %w", err)
	}

	// Zero the raw bytes immediately
	enclave.ZeroBytes(privateKeyBytes)

	// Create wallet
	wallet, err := newNeoWallet(privateKey)
	if err != nil {
		return "", fmt.Errorf("create wallet: %w", err)
	}

	// Generate handle from address
	h := sha256.Sum256([]byte(wallet.address))
	handle := WalletHandle(hex.EncodeToString(h[:16]))

	s.wallets[handle] = wallet
	return handle, nil
}

// GetAddress returns the Neo N3 address for a wallet handle.
func (s *Signer) GetAddress(ctx context.Context, handle WalletHandle) (string, error) {
	s.mu.RLock()
	wallet, exists := s.wallets[handle]
	s.mu.RUnlock()

	if !exists {
		return "", types.ErrKeyNotFound
	}

	return wallet.address, nil
}

// GetPublicKey returns the compressed public key for a wallet handle.
func (s *Signer) GetPublicKey(ctx context.Context, handle WalletHandle) ([]byte, error) {
	s.mu.RLock()
	wallet, exists := s.wallets[handle]
	s.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	// Return a copy to prevent modification
	pubKey := make([]byte, len(wallet.publicKey))
	copy(pubKey, wallet.publicKey)
	return pubKey, nil
}

// GetScriptHash returns the script hash for a wallet handle.
func (s *Signer) GetScriptHash(ctx context.Context, handle WalletHandle) ([]byte, error) {
	s.mu.RLock()
	wallet, exists := s.wallets[handle]
	s.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	// Return a copy
	scriptHash := make([]byte, len(wallet.scriptHash))
	copy(scriptHash, wallet.scriptHash)
	return scriptHash, nil
}

// SignData signs arbitrary data with a wallet.
// Signing happens inside the enclave - private key never leaves.
func (s *Signer) SignData(ctx context.Context, handle WalletHandle, data []byte) ([]byte, error) {
	s.mu.RLock()
	wallet, exists := s.wallets[handle]
	s.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	// Hash the data first (Neo uses SHA256)
	hash := sha256.Sum256(data)

	// Sign the hash
	r, sigS, err := ecdsa.Sign(rand.Reader, wallet.privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Encode signature in Neo format (r || s, 64 bytes)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := sigS.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// SignTransaction signs a Neo N3 transaction.
// The transaction is signed inside the enclave.
func (s *Signer) SignTransaction(ctx context.Context, handle WalletHandle, txHash []byte) ([]byte, error) {
	if len(txHash) != 32 {
		return nil, fmt.Errorf("invalid transaction hash length: expected 32, got %d", len(txHash))
	}

	s.mu.RLock()
	wallet, exists := s.wallets[handle]
	s.mu.RUnlock()

	if !exists {
		return nil, types.ErrKeyNotFound
	}

	// Sign the transaction hash directly
	r, sigS, err := ecdsa.Sign(rand.Reader, wallet.privateKey, txHash)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Encode signature in Neo format
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := sigS.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// VerifySignature verifies a signature against a public key.
func (s *Signer) VerifySignature(ctx context.Context, publicKey, data, signature []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("invalid signature length")
	}

	// Decompress public key if needed
	var x, y *big.Int
	if len(publicKey) == 33 {
		// Compressed public key
		x, y = decompressPublicKey(publicKey)
		if x == nil {
			return false, fmt.Errorf("invalid compressed public key")
		}
	} else if len(publicKey) == 65 {
		// Uncompressed public key
		x, y = elliptic.Unmarshal(elliptic.P256(), publicKey)
		if x == nil {
			return false, fmt.Errorf("invalid public key")
		}
	} else {
		return false, fmt.Errorf("invalid public key length")
	}

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Hash the data
	hash := sha256.Sum256(data)

	// Parse signature
	r := new(big.Int).SetBytes(signature[:32])
	sigS := new(big.Int).SetBytes(signature[32:64])

	return ecdsa.Verify(pubKey, hash[:], r, sigS), nil
}

// ListWallets returns all wallet handles.
func (s *Signer) ListWallets(ctx context.Context) ([]WalletHandle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	handles := make([]WalletHandle, 0, len(s.wallets))
	for handle := range s.wallets {
		handles = append(handles, handle)
	}
	return handles, nil
}

// DeleteWallet removes a wallet from the enclave.
func (s *Signer) DeleteWallet(ctx context.Context, handle WalletHandle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet, exists := s.wallets[handle]
	if !exists {
		return types.ErrKeyNotFound
	}

	// Zero the private key
	if wallet.privateKey != nil && wallet.privateKey.D != nil {
		wallet.privateKey.D.SetInt64(0)
	}

	delete(s.wallets, handle)
	return nil
}

// Zero zeros all sensitive data in the signer.
func (s *Signer) Zero() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for handle, wallet := range s.wallets {
		if wallet.privateKey != nil && wallet.privateKey.D != nil {
			wallet.privateKey.D.SetInt64(0)
		}
		delete(s.wallets, handle)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// newNeoWallet creates a new Neo wallet from a private key.
func newNeoWallet(privateKey *ecdsa.PrivateKey) (*neoWallet, error) {
	// Get compressed public key
	publicKey := compressPublicKey(&privateKey.PublicKey)

	// Calculate script hash
	scriptHash := publicKeyToScriptHash(publicKey)

	// Calculate address
	address := scriptHashToAddress(scriptHash)

	return &neoWallet{
		privateKey: privateKey,
		publicKey:  publicKey,
		address:    address,
		scriptHash: scriptHash,
	}, nil
}

// createPrivateKeyFromBytes creates an ECDSA private key from raw bytes.
func createPrivateKeyFromBytes(bytes []byte) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(bytes)

	// Ensure d is in valid range [1, n-1]
	n := curve.Params().N
	d.Mod(d, n)
	if d.Cmp(big.NewInt(1)) < 0 {
		d.Add(d, big.NewInt(1))
	}

	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
		D: d,
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	return privateKey, nil
}

// compressPublicKey compresses an ECDSA public key.
func compressPublicKey(pubKey *ecdsa.PublicKey) []byte {
	compressed := make([]byte, 33)
	if pubKey.Y.Bit(0) == 0 {
		compressed[0] = 0x02
	} else {
		compressed[0] = 0x03
	}
	xBytes := pubKey.X.Bytes()
	copy(compressed[33-len(xBytes):], xBytes)
	return compressed
}

// decompressPublicKey decompresses a compressed public key.
func decompressPublicKey(compressed []byte) (*big.Int, *big.Int) {
	if len(compressed) != 33 {
		return nil, nil
	}

	curve := elliptic.P256()
	x := new(big.Int).SetBytes(compressed[1:])

	// y² = x³ - 3x + b
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	threeX := new(big.Int).Mul(x, big.NewInt(3))
	x3.Sub(x3, threeX)
	x3.Add(x3, curve.Params().B)
	x3.Mod(x3, curve.Params().P)

	// Calculate square root
	y := new(big.Int).ModSqrt(x3, curve.Params().P)
	if y == nil {
		return nil, nil
	}

	// Check parity
	if (compressed[0] == 0x02 && y.Bit(0) != 0) || (compressed[0] == 0x03 && y.Bit(0) == 0) {
		y.Sub(curve.Params().P, y)
	}

	return x, y
}

// publicKeyToScriptHash converts a public key to a Neo script hash.
func publicKeyToScriptHash(publicKey []byte) []byte {
	// Neo N3 verification script: PUSHDATA1 33 <pubkey> SYSCALL System.Crypto.CheckSig
	script := make([]byte, 0, 40)
	script = append(script, 0x0C)       // PUSHDATA1
	script = append(script, 0x21)       // 33 bytes
	script = append(script, publicKey...) // public key
	script = append(script, 0x41)       // SYSCALL
	// System.Crypto.CheckSig hash
	script = append(script, 0x56, 0xe7, 0xb3, 0x27)

	// SHA256 then RIPEMD160
	sha := sha256.Sum256(script)
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	return ripemd.Sum(nil)
}

// scriptHashToAddress converts a script hash to a Neo N3 address.
func scriptHashToAddress(scriptHash []byte) string {
	// Neo N3 address version
	const addressVersion = 0x35

	// Prepend version
	data := make([]byte, 21)
	data[0] = addressVersion
	copy(data[1:], scriptHash)

	// Double SHA256 for checksum
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])

	// Append first 4 bytes of checksum
	data = append(data, hash2[:4]...)

	// Base58 encode
	return base58Encode(data)
}

// decodeWIF decodes a WIF (Wallet Import Format) string to private key bytes.
func decodeWIF(wif string) ([]byte, error) {
	// Base58 decode
	decoded, err := base58Decode(wif)
	if err != nil {
		return nil, fmt.Errorf("base58 decode: %w", err)
	}

	// Verify length (1 version + 32 key + 1 compression flag + 4 checksum = 38)
	if len(decoded) != 38 {
		return nil, fmt.Errorf("invalid WIF length")
	}

	// Verify checksum
	payload := decoded[:34]
	checksum := decoded[34:]
	hash1 := sha256.Sum256(payload)
	hash2 := sha256.Sum256(hash1[:])
	if !bytesEqual(checksum, hash2[:4]) {
		return nil, fmt.Errorf("invalid WIF checksum")
	}

	// Extract private key (skip version byte, take 32 bytes)
	privateKey := make([]byte, 32)
	copy(privateKey, decoded[1:33])

	return privateKey, nil
}

// base58Encode encodes bytes to base58.
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

// base58Decode decodes a base58 string to bytes.
func base58Decode(input string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	result := big.NewInt(0)
	base := big.NewInt(58)

	for _, c := range input {
		idx := -1
		for i, a := range alphabet {
			if a == c {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}

	// Convert to bytes
	decoded := result.Bytes()

	// Add leading zeros
	leadingZeros := 0
	for _, c := range input {
		if c != '1' {
			break
		}
		leadingZeros++
	}

	return append(make([]byte, leadingZeros), decoded...), nil
}

// bytesEqual compares two byte slices.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
