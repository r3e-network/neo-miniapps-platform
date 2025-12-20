// Package neovrf provides verifiable randomness service.
package neovrf

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"

	"github.com/R3E-Network/service_layer/infrastructure/crypto"
	"github.com/R3E-Network/service_layer/infrastructure/database"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
)

const (
	ServiceID   = "neovrf"
	ServiceName = "NeoVRF Service"
	Version     = "1.0.0"
)

// Service implements the VRF service.
type Service struct {
	*commonservice.BaseService
	signingKey      []byte
	privateKey      *ecdsa.PrivateKey
	publicKey       []byte
	attestationHash []byte
}

// Config holds VRF service configuration.
type Config struct {
	Marble *marble.Marble
	DB     database.RepositoryInterface
}

// New creates a new NeoVRF service.
func New(cfg Config) (*Service, error) {
	if cfg.Marble == nil {
		return nil, fmt.Errorf("neovrf: marble is required")
	}

	strict := runtime.StrictIdentityMode() || cfg.Marble.IsEnclave()

	requiredSecrets := []string(nil)
	if strict {
		requiredSecrets = []string{"NEOVRF_SIGNING_KEY"}
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:              ServiceID,
		Name:            ServiceName,
		Version:         Version,
		Marble:          cfg.Marble,
		DB:              cfg.DB,
		RequiredSecrets: requiredSecrets,
	})

	s := &Service{
		BaseService: base,
	}
	s.attestationHash = computeAttestationHash(cfg.Marble)

	if key, ok := cfg.Marble.Secret("NEOVRF_SIGNING_KEY"); ok && len(key) >= 32 {
		s.signingKey = key
	} else if strict {
		return nil, fmt.Errorf("neovrf: NEOVRF_SIGNING_KEY is required and must be at least 32 bytes")
	} else {
		s.Logger().WithFields(nil).Warn("NEOVRF_SIGNING_KEY not configured; generating ephemeral signing key (development/testing only)")
	}

	if err := s.initSigningKey(); err != nil {
		return nil, err
	}

	base.WithStats(s.statistics)
	base.RegisterStandardRoutes()
	s.registerRoutes()

	return s, nil
}

func (s *Service) initSigningKey() error {
	if len(s.signingKey) >= 32 {
		priv, pub, err := deriveSigningKey(s.signingKey)
		if err != nil {
			return fmt.Errorf("neovrf: derive signing key: %w", err)
		}
		s.privateKey = priv
		s.publicKey = pub
		return nil
	}

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("neovrf: generate signing key: %w", err)
	}
	s.privateKey = keyPair.PrivateKey
	s.publicKey = crypto.PublicKeyToBytes(keyPair.PublicKey)
	return nil
}

func deriveSigningKey(master []byte) (*ecdsa.PrivateKey, []byte, error) {
	seed, err := crypto.DeriveKey(master, nil, "vrf-signing", 32)
	if err != nil {
		return nil, nil, err
	}
	defer crypto.ZeroBytes(seed)

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(seed)
	n := new(big.Int).Sub(curve.Params().N, big.NewInt(1))
	d.Mod(d, n)
	d.Add(d, big.NewInt(1))

	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	pub := crypto.PublicKeyToBytes(&priv.PublicKey)
	return priv, pub, nil
}

func (s *Service) statistics() map[string]any {
	stats := map[string]any{
		"attestation_hash": fmt.Sprintf("%x", s.attestationHash),
	}
	if len(s.publicKey) > 0 {
		stats["public_key"] = fmt.Sprintf("%x", s.publicKey)
	}
	return stats
}
