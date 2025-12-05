package coordinator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
)

// RecoveryManager handles multi-party key recovery.
// It encrypts the coordinator's sealing key with multiple RSA public keys,
// allowing recovery when a threshold of key holders provide their shares.
type RecoveryManager struct {
	mu sync.Mutex

	// Recovery configuration
	threshold int // Minimum keys needed for recovery

	// Recovery keys (user name -> RSA public key)
	recoveryKeys map[string]*rsa.PublicKey

	// Encrypted key shares (user name -> encrypted sealing key)
	keyShares map[string][]byte

	// Current sealing key (only in memory, never persisted in plaintext)
	sealingKey []byte
}

// NewRecoveryManager creates a new RecoveryManager.
func NewRecoveryManager(threshold int) *RecoveryManager {
	if threshold < 1 {
		threshold = 1
	}

	return &RecoveryManager{
		threshold:    threshold,
		recoveryKeys: make(map[string]*rsa.PublicKey),
		keyShares:    make(map[string][]byte),
	}
}

// SetRecoveryKeys configures recovery keys from the manifest.
func (rm *RecoveryManager) SetRecoveryKeys(keys map[string]string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.recoveryKeys = make(map[string]*rsa.PublicKey, len(keys))

	for name, pemKey := range keys {
		pubKey, err := parseRSAPublicKey(pemKey)
		if err != nil {
			return fmt.Errorf("parse recovery key %q: %w", name, err)
		}
		rm.recoveryKeys[name] = pubKey
	}

	// Validate threshold
	if rm.threshold > len(rm.recoveryKeys) {
		return fmt.Errorf("threshold (%d) exceeds number of recovery keys (%d)",
			rm.threshold, len(rm.recoveryKeys))
	}

	return nil
}

// GenerateRecoveryData generates encrypted key shares for each recovery key holder.
// This should be called after the coordinator is initialized with a sealing key.
func (rm *RecoveryManager) GenerateRecoveryData(sealingKey []byte) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(rm.recoveryKeys) == 0 {
		return nil // No recovery keys configured
	}

	rm.sealingKey = make([]byte, len(sealingKey))
	copy(rm.sealingKey, sealingKey)

	rm.keyShares = make(map[string][]byte, len(rm.recoveryKeys))

	for name, pubKey := range rm.recoveryKeys {
		// Encrypt sealing key with user's RSA public key using OAEP
		encrypted, err := rsa.EncryptOAEP(
			sha256.New(),
			rand.Reader,
			pubKey,
			sealingKey,
			[]byte("neo-service-layer-recovery"),
		)
		if err != nil {
			return fmt.Errorf("encrypt for %q: %w", name, err)
		}
		rm.keyShares[name] = encrypted
	}

	return nil
}

// GetRecoveryData returns the encrypted recovery data for a specific user.
func (rm *RecoveryManager) GetRecoveryData(userName string) ([]byte, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	share, ok := rm.keyShares[userName]
	if !ok {
		return nil, fmt.Errorf("no recovery data for user %q", userName)
	}

	// Return a copy
	result := make([]byte, len(share))
	copy(result, share)
	return result, nil
}

// RecoverSealingKey recovers the sealing key from provided decrypted shares.
// Each share should be the decrypted sealing key from a recovery key holder.
func (rm *RecoveryManager) RecoverSealingKey(shares map[string][]byte) ([]byte, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(shares) < rm.threshold {
		return nil, fmt.Errorf("insufficient shares: got %d, need %d", len(shares), rm.threshold)
	}

	// Verify all shares match (they should all be the same sealing key)
	var sealingKey []byte
	for name, share := range shares {
		// Verify this user has a recovery key
		if _, ok := rm.recoveryKeys[name]; !ok {
			return nil, fmt.Errorf("unknown recovery key holder: %q", name)
		}

		if sealingKey == nil {
			sealingKey = share
		} else {
			// All shares should decrypt to the same key
			if !bytesEqual(sealingKey, share) {
				return nil, fmt.Errorf("share mismatch from %q", name)
			}
		}
	}

	if sealingKey == nil {
		return nil, fmt.Errorf("no valid shares provided")
	}

	// Store recovered key
	rm.sealingKey = make([]byte, len(sealingKey))
	copy(rm.sealingKey, sealingKey)

	return sealingKey, nil
}

// GetSealingKey returns the current sealing key (if available).
func (rm *RecoveryManager) GetSealingKey() []byte {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.sealingKey == nil {
		return nil
	}

	result := make([]byte, len(rm.sealingKey))
	copy(result, rm.sealingKey)
	return result
}

// HasRecoveryKeys returns true if recovery keys are configured.
func (rm *RecoveryManager) HasRecoveryKeys() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return len(rm.recoveryKeys) > 0
}

// GetRecoveryKeyNames returns the names of configured recovery key holders.
func (rm *RecoveryManager) GetRecoveryKeyNames() []string {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	names := make([]string, 0, len(rm.recoveryKeys))
	for name := range rm.recoveryKeys {
		names = append(names, name)
	}
	return names
}

// Zero zeros all sensitive data.
func (rm *RecoveryManager) Zero() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.sealingKey != nil {
		zeroBytes(rm.sealingKey)
		rm.sealingKey = nil
	}

	for name, share := range rm.keyShares {
		zeroBytes(share)
		delete(rm.keyShares, name)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func parseRSAPublicKey(pemData string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try parsing as PKIX public key first
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}
		return rsaPub, nil
	}

	// Try parsing as PKCS1 public key
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	return rsaPub, nil
}

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
