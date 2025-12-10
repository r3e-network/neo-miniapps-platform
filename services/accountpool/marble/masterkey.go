package accountpoolmarble

import (
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/marble"
)

const (
	secretPoolMasterKey           = "POOL_MASTER_KEY"
	secretPoolMasterKeyHash       = "POOL_MASTER_KEY_HASH"
	secretPoolMasterAttestationID = "POOL_MASTER_ATTESTATION_HASH"
	secretCoordinatorMasterSeed   = "COORD_MASTER_SEED"
)

// loadMasterKey pulls the coordinator-provided master key and validates it
// against the expected hash (when provided). The hash is required when running
// inside an enclave to ensure the key is anchored to Coordinator attestation
// (MRSIGNER/ISVSVN policy). In non-enclave/test environments, the hash is
// optional to keep tests simple.
func (s *Service) loadMasterKey(m *marble.Marble) error {
	key, ok := m.Secret(secretPoolMasterKey)
	if !ok || len(key) == 0 {
		// Derive master key from a coordinator-provided seed for stable upgrades.
		seed, okSeed := m.Secret(secretCoordinatorMasterSeed)
		if !okSeed || len(seed) == 0 {
			return fmt.Errorf("missing %s or %s secret", secretPoolMasterKey, secretCoordinatorMasterSeed)
		}
		derived, err := deriveMasterKeyFromSeed(seed)
		if err != nil {
			return err
		}
		key = derived
	}
	if len(key) < 32 {
		return fmt.Errorf("%s must be at least 32 bytes", secretPoolMasterKey)
	}

	pubKeyCompressed, err := deriveMasterPubKey(key)
	if err != nil {
		return err
	}
	computedHash := sha256.Sum256(pubKeyCompressed)

	if expected, ok := m.Secret(secretPoolMasterKeyHash); ok {
		expHash, err := parseHash(expected)
		if err != nil {
			return fmt.Errorf("parse %s: %w", secretPoolMasterKeyHash, err)
		}
		if !equalHash(expHash, computedHash[:]) {
			return fmt.Errorf("%s mismatch: expected %s got %s", secretPoolMasterKeyHash, hex.EncodeToString(expHash), hex.EncodeToString(computedHash[:]))
		}
	} else if m.IsEnclave() {
		return fmt.Errorf("missing %s secret in enclave mode", secretPoolMasterKeyHash)
	}

	attHash, _ := m.Secret(secretPoolMasterAttestationID)

	s.masterKey = key
	s.masterPubKey = pubKeyCompressed
	s.masterKeyHash = computedHash[:]
	s.masterKeyAttestationID = strings.TrimSpace(string(attHash))
	return nil
}

func deriveMasterKeyFromSeed(seed []byte) ([]byte, error) {
	if len(seed) < 16 {
		return nil, fmt.Errorf("%s must be at least 16 bytes", secretCoordinatorMasterSeed)
	}
	return crypto.DeriveKey(seed, []byte("coordinator-master-key"), "accountpool", 32)
}

// deriveMasterPubKey deterministically derives a P-256 pubkey from the master key.
// This is used for anchoring and attestation; private key never leaves the enclave.
func deriveMasterPubKey(masterKey []byte) ([]byte, error) {
	if len(masterKey) < 32 {
		return nil, fmt.Errorf("%s must be at least 32 bytes", secretPoolMasterKey)
	}
	// Derive a 32-byte scalar, reduce mod curve order.
	derived, err := crypto.DeriveKey(masterKey, []byte("coordinator-master-pubkey"), "accountpool", 32)
	if err != nil {
		return nil, err
	}
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(derived)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1))
	x, y := curve.ScalarBaseMult(d.Bytes())
	pubCompressed := elliptic.MarshalCompressed(curve, x, y)
	return pubCompressed, nil
}

func parseHash(v []byte) ([]byte, error) {
	if len(v) == sha256.Size {
		return v, nil
	}
	trimmed := strings.TrimSpace(string(v))
	b, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("expected raw 32 bytes or hex: %w", err)
	}
	if len(b) != sha256.Size {
		return nil, fmt.Errorf("hash length must be %d bytes", sha256.Size)
	}
	return b, nil
}

func equalHash(a, b []byte) bool {
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

// MasterKeySummary exposes non-sensitive metadata for off-chain attestation
// verification and on-chain anchoring without revealing the key material.
type MasterKeySummary struct {
	Hash            string `json:"hash"`
	PubKeyHex       string `json:"pubkey,omitempty"`
	AttestationHash string `json:"attestation_hash,omitempty"`
	Source          string `json:"source"`
	RequiresHash    bool   `json:"requires_hash"`
}

func (s *Service) masterKeySummary() MasterKeySummary {
	return MasterKeySummary{
		Hash:            hex.EncodeToString(s.masterKeyHash),
		PubKeyHex:       hex.EncodeToString(s.masterPubKey),
		AttestationHash: s.masterKeyAttestationID,
		Source:          "coordinator",
		RequiresHash:    s.Marble().IsEnclave(),
	}
}
