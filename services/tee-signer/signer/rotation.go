package signer

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

const (
	DefaultOverlapPeriod = 7 * 24 * time.Hour
)

var (
	ErrKeyVersionNotFound = errors.New("key version not found")
	ErrKeyVersionExpired  = errors.New("key version expired")
)

type KeyVersionRepository interface {
	GetActiveKeyVersion(ctx context.Context) (*database.KeyVersion, error)
	GetKeyVersion(ctx context.Context, keyVersion string) (*database.KeyVersion, error)
	ListKeyVersionsByStatus(ctx context.Context, statuses []string) ([]database.KeyVersion, error)
	CreateKeyVersion(ctx context.Context, create database.KeyVersionCreate) (*database.KeyVersion, error)
	UpdateKeyVersion(ctx context.Context, keyVersion string, update database.KeyVersionUpdate) (*database.KeyVersion, error)
}

type KeyManagerConfig struct {
	MasterKeySeed []byte
	Repository    KeyVersionRepository
	Now           func() time.Time

	OverlapPeriod     time.Duration
	InitialKeyVersion string

	RequireRepository bool
}

type keyEntry struct {
	privateKey *ecdsa.PrivateKey
	status     string
	validFrom  time.Time
	validUntil *time.Time
}

type KeyManager struct {
	mu sync.RWMutex

	masterKeySeed []byte
	repo          KeyVersionRepository
	now           func() time.Time
	overlap       time.Duration
	requireRepo   bool

	activeVersion string
	keys          map[string]keyEntry
}

func NewKeyManager(cfg KeyManagerConfig) (*KeyManager, error) {
	if len(cfg.MasterKeySeed) == 0 {
		return nil, fmt.Errorf("MASTER_KEY_SEED is required")
	}

	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	overlap := cfg.OverlapPeriod
	if overlap <= 0 {
		overlap = DefaultOverlapPeriod
	}

	initial := strings.TrimSpace(cfg.InitialKeyVersion)
	if initial == "" {
		initial = KeyVersionV1
	}

	mgr := &KeyManager{
		masterKeySeed: cfg.MasterKeySeed,
		repo:          cfg.Repository,
		now:           nowFn,
		overlap:       overlap,
		requireRepo:   cfg.RequireRepository,
		keys:          make(map[string]keyEntry),
	}

	if mgr.repo == nil {
		if mgr.requireRepo {
			return nil, fmt.Errorf("key version repository is required")
		}
		priv, err := DeriveP256PrivateKey(cfg.MasterKeySeed, initial)
		if err != nil {
			return nil, err
		}
		mgr.activeVersion = initial
		mgr.keys[initial] = keyEntry{
			privateKey: priv,
			status:     database.KeyVersionStatusActive,
			validFrom:  nowFn().UTC(),
		}
		return mgr, nil
	}

	if err := mgr.refreshFromRepository(context.Background()); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *KeyManager) ActiveVersion() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeVersion
}

// SigningKeyAt returns the private key and version to use at a given time.
// When requestedVersion is empty, the active version is used.
func (m *KeyManager) SigningKeyAt(ctx context.Context, requestedVersion string, now time.Time) (*ecdsa.PrivateKey, string, error) {
	if m == nil {
		return nil, "", fmt.Errorf("key manager is nil")
	}
	requestedVersion = strings.TrimSpace(requestedVersion)

	if err := m.cleanupExpired(ctx, now); err != nil {
		return nil, "", err
	}

	version := requestedVersion
	if version == "" {
		version = m.ActiveVersion()
	}
	if version == "" {
		return nil, "", fmt.Errorf("%w: no active key version", ErrKeyVersionNotFound)
	}

	m.mu.RLock()
	entry, ok := m.keys[version]
	m.mu.RUnlock()
	if !ok && m.repo != nil && requestedVersion != "" {
		if err := m.loadVersionFromRepository(ctx, version); err != nil {
			return nil, "", err
		}
		m.mu.RLock()
		entry, ok = m.keys[version]
		m.mu.RUnlock()
	}
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", ErrKeyVersionNotFound, version)
	}

	switch entry.status {
	case database.KeyVersionStatusActive:
		return entry.privateKey, version, nil
	case database.KeyVersionStatusDeprecated:
		if entry.validUntil != nil && now.Before(entry.validUntil.UTC()) {
			return entry.privateKey, version, nil
		}
		return nil, "", fmt.Errorf("%w: %s", ErrKeyVersionExpired, version)
	case database.KeyVersionStatusExpired:
		return nil, "", fmt.Errorf("%w: %s", ErrKeyVersionExpired, version)
	default:
		return nil, "", fmt.Errorf("unknown key status %q for %s", entry.status, version)
	}
}

type RotationResult struct {
	OldVersion      string     `json:"old_version,omitempty"`
	NewVersion      string     `json:"new_version"`
	DeprecatedUntil *time.Time `json:"deprecated_until,omitempty"`
	RotatedAt       time.Time  `json:"rotated_at"`
	Rotated         bool       `json:"rotated"`
}

func (m *KeyManager) Rotate(ctx context.Context) (*RotationResult, error) {
	if m == nil {
		return nil, fmt.Errorf("key manager is nil")
	}

	now := m.now().UTC()
	newVersion := KeyVersionFromTime(now)
	oldVersion := m.ActiveVersion()

	// Idempotency: if the computed version matches the current active one, no-op.
	if oldVersion != "" && oldVersion == newVersion {
		if err := m.cleanupExpired(ctx, now); err != nil {
			return nil, err
		}
		return &RotationResult{
			OldVersion: oldVersion,
			NewVersion: newVersion,
			RotatedAt:  now,
			Rotated:    false,
		}, nil
	}

	deprecatedUntil := now.Add(m.overlap).UTC()

	if m.repo == nil {
		m.rotateInMemory(oldVersion, newVersion, now, deprecatedUntil)
		return &RotationResult{
			OldVersion:      oldVersion,
			NewVersion:      newVersion,
			DeprecatedUntil: optionalTimePtr(oldVersion != "", deprecatedUntil),
			RotatedAt:       now,
			Rotated:         true,
		}, nil
	}

	// If persistence is required (production), fail closed instead of silently
	// inventing a new version on restart.
	if m.requireRepo {
		if err := m.rotateInRepository(ctx, oldVersion, newVersion, now, deprecatedUntil); err != nil {
			return nil, err
		}
	} else {
		if err := m.rotateInRepository(ctx, oldVersion, newVersion, now, deprecatedUntil); err != nil {
			// Best-effort fallback for dev/test.
			m.rotateInMemory(oldVersion, newVersion, now, deprecatedUntil)
		}
	}

	return &RotationResult{
		OldVersion:      oldVersion,
		NewVersion:      newVersion,
		DeprecatedUntil: optionalTimePtr(oldVersion != "", deprecatedUntil),
		RotatedAt:       now,
		Rotated:         true,
	}, nil
}

func optionalTimePtr(ok bool, t time.Time) *time.Time {
	if !ok {
		return nil
	}
	tt := t
	return &tt
}

func (m *KeyManager) rotateInMemory(oldVersion, newVersion string, now, deprecatedUntil time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if oldVersion != "" {
		if entry, ok := m.keys[oldVersion]; ok {
			entry.status = database.KeyVersionStatusDeprecated
			entry.validUntil = optionalTimePtr(true, deprecatedUntil)
			m.keys[oldVersion] = entry
		}
	}

	priv, err := DeriveP256PrivateKey(m.masterKeySeed, newVersion)
	if err == nil {
		m.keys[newVersion] = keyEntry{
			privateKey: priv,
			status:     database.KeyVersionStatusActive,
			validFrom:  now,
		}
		m.activeVersion = newVersion
	}
}

func (m *KeyManager) rotateInRepository(ctx context.Context, oldVersion, newVersion string, now, deprecatedUntil time.Time) error {
	if m.repo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Step 1: deprecate the current active key.
	if oldVersion != "" {
		deprecateStatus := database.KeyVersionStatusDeprecated
		_, err := m.repo.UpdateKeyVersion(ctx, oldVersion, database.KeyVersionUpdate{
			Status:     &deprecateStatus,
			ValidUntil: optionalTimePtr(true, deprecatedUntil),
		})
		if err != nil && !database.IsNotFound(err) {
			return err
		}
	}

	// Step 2: create the new active key version.
	_, err := m.repo.CreateKeyVersion(ctx, database.KeyVersionCreate{
		KeyVersion: newVersion,
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  now,
	})
	if err != nil {
		// If it already exists, re-check and accept if it's active.
		existing, getErr := m.repo.GetKeyVersion(ctx, newVersion)
		if getErr != nil {
			return err
		}
		if existing.Status != database.KeyVersionStatusActive {
			return fmt.Errorf("existing key_version %s has status %s", newVersion, existing.Status)
		}
	}

	// Step 3: refresh local state from the repository for consistency.
	if err := m.refreshFromRepository(ctx); err != nil {
		return err
	}
	return nil
}

func (m *KeyManager) refreshFromRepository(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("key manager is nil")
	}
	if m.repo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	active, err := m.repo.GetActiveKeyVersion(ctx)
	if err != nil {
		if database.IsNotFound(err) {
			// Bootstrap the first version deterministically from wall clock.
			bootstrapTime := m.now().UTC()
			version := KeyVersionFromTime(bootstrapTime)
			created, createErr := m.repo.CreateKeyVersion(ctx, database.KeyVersionCreate{
				KeyVersion: version,
				Status:     database.KeyVersionStatusActive,
				ValidFrom:  bootstrapTime,
			})
			if createErr != nil {
				if m.requireRepo {
					return createErr
				}
				// Dev/test fallback: behave like a single static version.
				priv, err := DeriveP256PrivateKey(m.masterKeySeed, KeyVersionV1)
				if err != nil {
					return err
				}
				m.mu.Lock()
				m.activeVersion = KeyVersionV1
				m.keys = map[string]keyEntry{
					KeyVersionV1: {
						privateKey: priv,
						status:     database.KeyVersionStatusActive,
						validFrom:  bootstrapTime,
					},
				}
				m.mu.Unlock()
				return nil
			}
			active = created
		} else {
			if m.requireRepo {
				return err
			}
			// Best-effort fallback for dev/test.
			priv, err := DeriveP256PrivateKey(m.masterKeySeed, KeyVersionV1)
			if err != nil {
				return err
			}
			m.mu.Lock()
			m.activeVersion = KeyVersionV1
			m.keys = map[string]keyEntry{
				KeyVersionV1: {
					privateKey: priv,
					status:     database.KeyVersionStatusActive,
					validFrom:  m.now().UTC(),
				},
			}
			m.mu.Unlock()
			return nil
		}
	}

	records, err := m.repo.ListKeyVersionsByStatus(ctx, []string{
		database.KeyVersionStatusActive,
		database.KeyVersionStatusDeprecated,
	})
	if err != nil {
		if m.requireRepo {
			return err
		}
		version := strings.TrimSpace(active.KeyVersion)
		if version == "" {
			version = KeyVersionV1
		}
		priv, derr := DeriveP256PrivateKey(m.masterKeySeed, version)
		if derr != nil {
			return derr
		}
		m.mu.Lock()
		m.activeVersion = version
		m.keys = map[string]keyEntry{
			version: {
				privateKey: priv,
				status:     database.KeyVersionStatusActive,
				validFrom:  active.ValidFrom.UTC(),
				validUntil: active.ValidUntil,
			},
		}
		m.mu.Unlock()
		return nil
	}

	keys := make(map[string]keyEntry, len(records))
	for _, record := range records {
		record := record
		version := strings.TrimSpace(record.KeyVersion)
		if version == "" {
			continue
		}

		// Expired deprecated keys should not remain in the in-memory keyring.
		if record.Status == database.KeyVersionStatusDeprecated && record.ValidUntil != nil && !m.now().UTC().Before(record.ValidUntil.UTC()) {
			expiredStatus := database.KeyVersionStatusExpired
			_, _ = m.repo.UpdateKeyVersion(ctx, version, database.KeyVersionUpdate{
				Status:     &expiredStatus,
				ValidUntil: record.ValidUntil,
			})
			continue
		}

		priv, err := DeriveP256PrivateKey(m.masterKeySeed, version)
		if err != nil {
			return err
		}
		keys[version] = keyEntry{
			privateKey: priv,
			status:     record.Status,
			validFrom:  record.ValidFrom.UTC(),
			validUntil: record.ValidUntil,
		}
	}

	m.mu.Lock()
	m.activeVersion = strings.TrimSpace(active.KeyVersion)
	m.keys = keys
	// Ensure the active version key is present even if list returned a partial view.
	if m.activeVersion != "" {
		if _, ok := m.keys[m.activeVersion]; !ok {
			priv, err := DeriveP256PrivateKey(m.masterKeySeed, m.activeVersion)
			if err != nil {
				m.mu.Unlock()
				return err
			}
			m.keys[m.activeVersion] = keyEntry{
				privateKey: priv,
				status:     database.KeyVersionStatusActive,
				validFrom:  active.ValidFrom.UTC(),
				validUntil: active.ValidUntil,
			}
		}
	}
	m.mu.Unlock()

	return nil
}

func (m *KeyManager) loadVersionFromRepository(ctx context.Context, version string) error {
	if m == nil || m.repo == nil {
		return fmt.Errorf("%w: %s", ErrKeyVersionNotFound, version)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	record, err := m.repo.GetKeyVersion(ctx, version)
	if err != nil {
		if database.IsNotFound(err) {
			return fmt.Errorf("%w: %s", ErrKeyVersionNotFound, version)
		}
		return err
	}

	if record.Status == database.KeyVersionStatusExpired {
		return fmt.Errorf("%w: %s", ErrKeyVersionExpired, version)
	}
	if record.Status == database.KeyVersionStatusDeprecated && record.ValidUntil != nil && !m.now().UTC().Before(record.ValidUntil.UTC()) {
		return fmt.Errorf("%w: %s", ErrKeyVersionExpired, version)
	}

	priv, err := DeriveP256PrivateKey(m.masterKeySeed, version)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.keys[version] = keyEntry{
		privateKey: priv,
		status:     record.Status,
		validFrom:  record.ValidFrom.UTC(),
		validUntil: record.ValidUntil,
	}
	m.mu.Unlock()
	return nil
}

func (m *KeyManager) cleanupExpired(ctx context.Context, now time.Time) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	expired := make([]string, 0)

	m.mu.Lock()
	for version, entry := range m.keys {
		if entry.status != database.KeyVersionStatusDeprecated || entry.validUntil == nil {
			continue
		}
		if now.Before(entry.validUntil.UTC()) {
			continue
		}
		entry.status = database.KeyVersionStatusExpired
		m.keys[version] = entry
		expired = append(expired, version)
	}
	m.mu.Unlock()

	if m.repo == nil {
		return nil
	}
	for _, version := range expired {
		expiredStatus := database.KeyVersionStatusExpired
		_, _ = m.repo.UpdateKeyVersion(ctx, version, database.KeyVersionUpdate{
			Status: &expiredStatus,
		})
	}
	return nil
}
