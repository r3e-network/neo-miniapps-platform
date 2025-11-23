package auth

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
)

// VerifyNeoSignature verifies an ed25519 signature over message using the provided
// public key. The wallet address must match the derived address of the public key.
func VerifyNeoSignature(wallet, signatureHex, message, pubKeyHex string) error {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return errors.New("wallet required")
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(signatureHex))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return errors.New("invalid signature length")
	}
	pubKeyBytes, err := hex.DecodeString(strings.TrimSpace(pubKeyHex))
	if err != nil {
		return fmt.Errorf("decode pubkey: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return errors.New("invalid pubkey length")
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	// Derive address from pubkey and compare.
	u160 := hash.Hash160(pubKey)
	derived := address.Uint160ToString(u160)
	if !strings.EqualFold(derived, wallet) {
		return errors.New("address does not match public key")
	}

	if !ed25519.Verify(pubKey, []byte(message), sigBytes) {
		return errors.New("invalid signature")
	}
	return nil
}
