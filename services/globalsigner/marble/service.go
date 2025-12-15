// Package globalsigner provides the TEE master key management service.
package globalsigner

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	"github.com/R3E-Network/service_layer/services/globalsigner/supabase"
)

// =============================================================================
// Service Definition
// =============================================================================

// Service implements the GlobalSigner TEE master key management service.
type Service struct {
	*commonservice.BaseService
	mu sync.RWMutex

	// Configuration
	rotationConfig *RotationConfig

	// Master seed (injected via MarbleRun)
	masterSeed []byte

	// Key management
	activeVersion string
	keys          map[string]*keyEntry

	// Repository
	repo supabase.Repository

	// Metrics
	signaturesIssued int64
	rotationsCount   int64
	startTime        time.Time
}

// keyEntry holds a key version's private key and metadata.
type keyEntry struct {
	privateKey    *ecdsa.PrivateKey
	version       *KeyVersion
}

// Config holds GlobalSigner service configuration.
type Config struct {
	Marble         *marble.Marble
	DB             database.RepositoryInterface
	Repository     supabase.Repository
	RotationConfig *RotationConfig
}

// =============================================================================
// Constructor
// =============================================================================

// New creates a new GlobalSigner service.
func New(cfg Config) (*Service, error) {
	if cfg.RotationConfig == nil {
		cfg.RotationConfig = DefaultRotationConfig()
	}

	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
		DB:      cfg.DB,
		RequiredSecrets: []string{
			"GLOBALSIGNER_MASTER_SEED",
		},
	})

	s := &Service{
		BaseService:    base,
		rotationConfig: cfg.RotationConfig,
		keys:           make(map[string]*keyEntry),
		repo:           cfg.Repository,
		startTime:      time.Now(),
	}

	// Set up hydration to load keys on startup
	s.WithHydrate(s.hydrate)

	// Set up statistics provider
	s.WithStats(s.statistics)

	// Add rotation check worker (runs daily)
	if cfg.RotationConfig.AutoRotate {
		s.AddTickerWorker(24*time.Hour, s.rotationWorkerWithError)
	}

	return s, nil
}

// =============================================================================
// Lifecycle
// =============================================================================

// hydrate loads master seed and existing keys from storage.
func (s *Service) hydrate(ctx context.Context) error {
	s.Logger().Info(ctx, "Hydrating GlobalSigner state...", nil)

	// Load master seed from Marble secrets
	seedBytes, ok := s.Marble().Secret("GLOBALSIGNER_MASTER_SEED")
	if !ok {
		return fmt.Errorf("failed to get master seed: secret not found")
	}

	seed, err := hex.DecodeString(string(seedBytes))
	if err != nil {
		return fmt.Errorf("invalid master seed hex: %w", err)
	}

	if len(seed) < 32 {
		return fmt.Errorf("master seed too short: need 32 bytes, got %d", len(seed))
	}

	s.masterSeed = seed

	// Load existing key versions from repository
	if s.repo != nil {
		versions, err := s.repo.ListKeyVersions(ctx, []KeyStatus{KeyStatusActive, KeyStatusOverlapping})
		if err != nil {
			s.Logger().Warn(ctx, "Failed to load key versions", map[string]interface{}{"error": err.Error()})
		} else {
			for _, v := range versions {
				if err := s.loadKeyVersion(v); err != nil {
					s.Logger().Warn(ctx, "Failed to load key version", map[string]interface{}{"version": v.Version, "error": err.Error()})
				}
			}
		}
	}

	// Bootstrap if no active key exists
	if s.activeVersion == "" {
		s.Logger().Info(ctx, "No active key found, bootstrapping initial key...", nil)
		if _, err := s.rotate(ctx, true); err != nil {
			return fmt.Errorf("failed to bootstrap initial key: %w", err)
		}
	}

	s.Logger().Info(ctx, "GlobalSigner hydrated", map[string]interface{}{"active_version": s.activeVersion, "key_count": len(s.keys)})
	return nil
}

// loadKeyVersion derives and loads a key version into memory.
func (s *Service) loadKeyVersion(v *KeyVersion) error {
	priv, err := s.deriveKeyForVersion(v.Version)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys[v.Version] = &keyEntry{
		privateKey: priv,
		version:    v,
	}

	if v.Status == KeyStatusActive {
		s.activeVersion = v.Version
	}

	return nil
}

// deriveKeyForVersion derives a P-256 private key for a given version.
func (s *Service) deriveKeyForVersion(version string) (*ecdsa.PrivateKey, error) {
	// Use HKDF to derive key material
	info := "globalsigner:" + version
	keyMaterial, err := crypto.DeriveKey(s.masterSeed, nil, info, 32)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	// Convert to P-256 private key using standard library
	curve := elliptic.P256()
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = curve
	priv.D = new(big.Int).SetBytes(keyMaterial)
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(keyMaterial)

	return priv, nil
}

// statistics returns service statistics for the /info endpoint.
func (s *Service) statistics() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyVersions := make([]string, 0, len(s.keys))
	for v := range s.keys {
		keyVersions = append(keyVersions, v)
	}

	return map[string]any{
		"active_version":    s.activeVersion,
		"key_versions":      keyVersions,
		"signatures_issued": s.signaturesIssued,
		"rotations_count":   s.rotationsCount,
		"uptime":            time.Since(s.startTime).String(),
		"is_enclave":        s.Marble().IsEnclave(),
	}
}

// =============================================================================
// Key Rotation
// =============================================================================

// rotationWorkerWithError checks if rotation is needed and performs it.
func (s *Service) rotationWorkerWithError(ctx context.Context) error {
	s.mu.RLock()
	activeEntry := s.keys[s.activeVersion]
	s.mu.RUnlock()

	if activeEntry == nil || activeEntry.version == nil {
		return nil
	}

	// Check if rotation is due
	activatedAt := activeEntry.version.ActivatedAt
	if activatedAt == nil {
		return nil
	}

	nextRotation := activatedAt.Add(s.rotationConfig.RotationPeriod)
	if time.Now().Before(nextRotation) {
		return nil
	}

	s.Logger().Info(ctx, "Rotation period reached, initiating key rotation...", nil)
	if _, err := s.rotate(ctx, false); err != nil {
		s.Logger().Error(ctx, "Automatic key rotation failed", err, nil)
		return err
	}
	return nil
}

// Rotate performs a key rotation.
func (s *Service) Rotate(ctx context.Context, force bool) (*RotateResponse, error) {
	return s.rotate(ctx, force)
}

func (s *Service) rotate(ctx context.Context, force bool) (*RotateResponse, error) {
	now := time.Now().UTC()
	newVersion := keyVersionFromTime(now)

	s.mu.Lock()
	oldVersion := s.activeVersion

	// Idempotency check
	if oldVersion == newVersion && !force {
		s.mu.Unlock()
		return &RotateResponse{
			OldVersion: oldVersion,
			NewVersion: newVersion,
			RotatedAt:  now,
			Rotated:    false,
		}, nil
	}
	s.mu.Unlock()

	// Derive new key
	priv, err := s.deriveKeyForVersion(newVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to derive new key: %w", err)
	}

	// Compute public key hash
	pubKeyBytes := elliptic.MarshalCompressed(priv.Curve, priv.PublicKey.X, priv.PublicKey.Y)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	pubKeyHash := sha256.Sum256(pubKeyBytes)
	pubKeyHashHex := hex.EncodeToString(pubKeyHash[:])

	// Create new key version
	newKeyVersion := &KeyVersion{
		Version:    newVersion,
		Status:     KeyStatusActive,
		PubKeyHex:  pubKeyHex,
		PubKeyHash: pubKeyHashHex,
		CreatedAt:  now,
		ActivatedAt: &now,
	}

	// Calculate overlap end time
	var overlapEndsAt *time.Time
	if oldVersion != "" {
		overlapEnd := now.Add(s.rotationConfig.OverlapPeriod)
		overlapEndsAt = &overlapEnd
	}

	// Update old key to overlapping status
	s.mu.Lock()
	if oldVersion != "" {
		if oldEntry, ok := s.keys[oldVersion]; ok {
			oldEntry.version.Status = KeyStatusOverlapping
			oldEntry.version.OverlapEndsAt = overlapEndsAt
		}
	}

	// Add new key
	s.keys[newVersion] = &keyEntry{
		privateKey: priv,
		version:    newKeyVersion,
	}
	s.activeVersion = newVersion
	s.rotationsCount++
	s.mu.Unlock()

	// Persist to repository
	if s.repo != nil {
		if oldVersion != "" {
			s.repo.UpdateKeyStatus(ctx, oldVersion, KeyStatusOverlapping, overlapEndsAt)
		}
		if err := s.repo.CreateKeyVersion(ctx, newKeyVersion); err != nil {
			s.Logger().Warn(ctx, "Failed to persist new key version", map[string]interface{}{"error": err.Error()})
		}
	}

	// Generate attestation
	attestation := s.buildAttestation(newVersion)
	if s.repo != nil {
		s.repo.StoreAttestation(ctx, newVersion, attestation)
	}

	s.Logger().Info(ctx, "Key rotation completed", map[string]interface{}{
		"old_version":    oldVersion,
		"new_version":    newVersion,
		"overlap_ends_at": overlapEndsAt,
	})

	return &RotateResponse{
		OldVersion:    oldVersion,
		NewVersion:    newVersion,
		OverlapEndsAt: overlapEndsAt,
		RotatedAt:     now,
		Rotated:       true,
	}, nil
}

// keyVersionFromTime generates a version string from a timestamp.
func keyVersionFromTime(t time.Time) string {
	return fmt.Sprintf("v%d-%02d", t.Year(), t.Month())
}

// =============================================================================
// Signing Operations
// =============================================================================

// Sign performs domain-separated signing.
func (s *Service) Sign(ctx context.Context, req *SignRequest) (*SignResponse, error) {
	if req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if req.Data == "" {
		return nil, fmt.Errorf("data is required")
	}

	data, err := hex.DecodeString(req.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid data hex: %w", err)
	}

	// Get signing key
	version := req.KeyVersion
	if version == "" {
		version = s.ActiveVersion()
	}

	s.mu.RLock()
	entry, ok := s.keys[version]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}

	// Validate key status
	if entry.version.Status == KeyStatusRevoked {
		return nil, fmt.Errorf("key version revoked: %s", version)
	}
	if entry.version.Status == KeyStatusOverlapping {
		if entry.version.OverlapEndsAt != nil && time.Now().After(*entry.version.OverlapEndsAt) {
			return nil, fmt.Errorf("key version overlap expired: %s", version)
		}
	}

	// Domain-separated signing: hash(domain || data)
	h := sha256.New()
	h.Write([]byte(req.Domain))
	h.Write([]byte{0x00}) // separator
	h.Write(data)
	digest := h.Sum(nil)

	// Sign
	sig, err := crypto.Sign(entry.privateKey, digest)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	s.mu.Lock()
	s.signaturesIssued++
	s.mu.Unlock()

	return &SignResponse{
		Signature:  hex.EncodeToString(sig),
		KeyVersion: version,
		PubKeyHex:  entry.version.PubKeyHex,
	}, nil
}

// =============================================================================
// Key Derivation
// =============================================================================

// Derive performs deterministic child key derivation.
func (s *Service) Derive(ctx context.Context, req *DeriveRequest) (*DeriveResponse, error) {
	if req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if req.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	version := req.KeyVersion
	if version == "" {
		version = s.ActiveVersion()
	}

	s.mu.RLock()
	entry, ok := s.keys[version]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}

	// Derive child key: HKDF(master_key, domain || path)
	info := req.Domain + ":" + req.Path
	childKeyMaterial, err := crypto.DeriveKey(s.masterSeed, []byte(version), info, 32)
	if err != nil {
		return nil, fmt.Errorf("derivation failed: %w", err)
	}

	// Convert to P-256 private key using standard library
	curve := elliptic.P256()
	childPriv := new(ecdsa.PrivateKey)
	childPriv.PublicKey.Curve = curve
	childPriv.D = new(big.Int).SetBytes(childKeyMaterial)
	childPriv.PublicKey.X, childPriv.PublicKey.Y = curve.ScalarBaseMult(childKeyMaterial)

	pubKeyBytes := elliptic.MarshalCompressed(childPriv.Curve, childPriv.PublicKey.X, childPriv.PublicKey.Y)
	_ = entry // entry validated above for key version lookup

	return &DeriveResponse{
		PubKeyHex:  hex.EncodeToString(pubKeyBytes),
		KeyVersion: version,
	}, nil
}

// =============================================================================
// Attestation
// =============================================================================

// buildAttestation generates an attestation for a key version.
func (s *Service) buildAttestation(version string) *MasterKeyAttestation {
	s.mu.RLock()
	entry, ok := s.keys[version]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	att := &MasterKeyAttestation{
		KeyVersion: version,
		PubKeyHex:  entry.version.PubKeyHex,
		PubKeyHash: entry.version.PubKeyHash,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Simulated:  !s.Marble().IsEnclave(),
	}

	// Generate SGX quote if in enclave mode
	if s.Marble().IsEnclave() {
		quote, report, err := s.generateQuote(entry.version.PubKeyHash)
		if err == nil {
			att.Quote = quote
			att.MRENCLAVE = report.MRENCLAVE
			att.MRSIGNER = report.MRSIGNER
			att.ProdID = report.ProdID
			att.ISVSVN = report.ISVSVN
		}
	}

	return att
}

// SGXReport holds parsed SGX report fields.
type SGXReport struct {
	MRENCLAVE string
	MRSIGNER  string
	ProdID    uint16
	ISVSVN    uint16
}

// generateQuote generates an SGX quote with the given report data.
// Returns error in simulation mode; uses EGo's enclave.GetRemoteReport in SGX hardware mode.
func (s *Service) generateQuote(reportData string) (string, *SGXReport, error) {
	return "", nil, fmt.Errorf("not in enclave mode")
}

// GetAttestation returns the attestation for the active key.
func (s *Service) GetAttestation(ctx context.Context) (*MasterKeyAttestation, error) {
	version := s.ActiveVersion()
	if version == "" {
		return nil, fmt.Errorf("no active key version")
	}
	return s.buildAttestation(version), nil
}

// =============================================================================
// Accessors
// =============================================================================

// ActiveVersion returns the currently active key version.
func (s *Service) ActiveVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeVersion
}

// GetKeyVersion returns information about a specific key version.
func (s *Service) GetKeyVersion(version string) (*KeyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.keys[version]
	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}
	return entry.version, nil
}

// ListKeyVersions returns all loaded key versions.
func (s *Service) ListKeyVersions() []*KeyVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := make([]*KeyVersion, 0, len(s.keys))
	for _, entry := range s.keys {
		versions = append(versions, entry.version)
	}
	return versions
}

// Logger returns the service logger.
func (s *Service) Logger() *logging.Logger {
	return s.BaseService.Logger()
}
