package chain

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// Wallet and Signing
// =============================================================================

// Wallet represents a Neo N3 wallet for signing transactions.
type Wallet struct {
	privateKey *ecdsa.PrivateKey
	publicKey  []byte
	scriptHash []byte
	address    string
}

// NewWallet creates a new wallet from a private key.
func NewWallet(privateKeyHex string) (*Wallet, error) {
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Set the private key D value
	keyPair.PrivateKey.D = new(big.Int).SetBytes(keyBytes)
	keyPair.PrivateKey.PublicKey.X, keyPair.PrivateKey.PublicKey.Y =
		keyPair.PrivateKey.Curve.ScalarBaseMult(keyBytes)

	publicKey := crypto.PublicKeyToBytes(&keyPair.PrivateKey.PublicKey)
	scriptHash := crypto.PublicKeyToScriptHash(publicKey)
	address := crypto.ScriptHashToAddress(scriptHash)

	return &Wallet{
		privateKey: keyPair.PrivateKey,
		publicKey:  publicKey,
		scriptHash: scriptHash,
		address:    address,
	}, nil
}

// Address returns the wallet address.
func (w *Wallet) Address() string {
	return w.address
}

// ScriptHash returns the wallet script hash.
func (w *Wallet) ScriptHash() []byte {
	return w.scriptHash
}

// ScriptHashHex returns the wallet script hash as hex string.
func (w *Wallet) ScriptHashHex() string {
	// Reverse for Neo N3 little-endian format
	reversed := make([]byte, len(w.scriptHash))
	for i, b := range w.scriptHash {
		reversed[len(w.scriptHash)-1-i] = b
	}
	return hex.EncodeToString(reversed)
}

// PublicKey returns the wallet public key.
func (w *Wallet) PublicKey() []byte {
	return w.publicKey
}

// Sign signs data with the wallet's private key.
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	return crypto.Sign(w.privateKey, data)
}
