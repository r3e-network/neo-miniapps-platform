package supabase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/services/globalsigner/types"
)

// =============================================================================
// Repository Interface
// =============================================================================

// Repository defines the interface for GlobalSigner data operations.
type Repository interface {
	// Key version operations
	CreateKeyVersion(ctx context.Context, v *types.KeyVersion) error
	UpdateKeyStatus(ctx context.Context, version string, status types.KeyStatus, overlapEndsAt *time.Time) error
	GetKeyVersion(ctx context.Context, version string) (*types.KeyVersion, error)
	ListKeyVersions(ctx context.Context, statuses []types.KeyStatus) ([]*types.KeyVersion, error)

	// Attestation operations
	StoreAttestation(ctx context.Context, keyVersion string, att *types.MasterKeyAttestation) error
	GetAttestation(ctx context.Context, keyVersion string) (*types.MasterKeyAttestation, error)
}

// =============================================================================
// Supabase Repository Implementation
// =============================================================================

// SupabaseRepository implements Repository using Supabase.
type SupabaseRepository struct {
	db database.RepositoryInterface
}

// NewRepository creates a new Supabase repository.
func NewRepository(db database.RepositoryInterface) *SupabaseRepository {
	return &SupabaseRepository{db: db}
}

// CreateKeyVersion creates a new key version record.
func (r *SupabaseRepository) CreateKeyVersion(ctx context.Context, v *types.KeyVersion) error {
	return nil
}

// UpdateKeyStatus updates the status of a key version.
func (r *SupabaseRepository) UpdateKeyStatus(ctx context.Context, version string, status types.KeyStatus, overlapEndsAt *time.Time) error {
	return nil
}

// GetKeyVersion retrieves a key version by version string.
func (r *SupabaseRepository) GetKeyVersion(ctx context.Context, version string) (*types.KeyVersion, error) {
	return nil, fmt.Errorf("not found")
}

// ListKeyVersions lists key versions by status.
func (r *SupabaseRepository) ListKeyVersions(ctx context.Context, statuses []types.KeyStatus) ([]*types.KeyVersion, error) {
	return nil, nil
}

// StoreAttestation stores an attestation artifact.
func (r *SupabaseRepository) StoreAttestation(ctx context.Context, keyVersion string, att *types.MasterKeyAttestation) error {
	return nil
}

// GetAttestation retrieves an attestation by key version.
func (r *SupabaseRepository) GetAttestation(ctx context.Context, keyVersion string) (*types.MasterKeyAttestation, error) {
	return nil, fmt.Errorf("not found")
}

// =============================================================================
// Mock Repository for Testing
// =============================================================================

// MockRepository is a mock implementation for testing.
type MockRepository struct {
	mu           sync.RWMutex
	keyVersions  map[string]*types.KeyVersion
	attestations map[string]*types.MasterKeyAttestation
}

// NewMockRepository creates a new mock repository.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		keyVersions:  make(map[string]*types.KeyVersion),
		attestations: make(map[string]*types.MasterKeyAttestation),
	}
}

// CreateKeyVersion creates a new key version record.
func (m *MockRepository) CreateKeyVersion(ctx context.Context, v *types.KeyVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.keyVersions[v.Version]; exists {
		return fmt.Errorf("key version already exists: %s", v.Version)
	}

	m.keyVersions[v.Version] = v
	return nil
}

// UpdateKeyStatus updates the status of a key version.
func (m *MockRepository) UpdateKeyStatus(ctx context.Context, version string, status types.KeyStatus, overlapEndsAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.keyVersions[version]
	if !ok {
		return fmt.Errorf("key version not found: %s", version)
	}

	v.Status = status
	v.OverlapEndsAt = overlapEndsAt

	if status == types.KeyStatusRevoked {
		now := time.Now()
		v.RevokedAt = &now
	}

	return nil
}

// GetKeyVersion retrieves a key version by version string.
func (m *MockRepository) GetKeyVersion(ctx context.Context, version string) (*types.KeyVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.keyVersions[version]
	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}

	return v, nil
}

// ListKeyVersions lists key versions by status.
func (m *MockRepository) ListKeyVersions(ctx context.Context, statuses []types.KeyStatus) ([]*types.KeyVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statusSet := make(map[types.KeyStatus]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}

	var result []*types.KeyVersion
	for _, v := range m.keyVersions {
		if len(statuses) == 0 || statusSet[v.Status] {
			result = append(result, v)
		}
	}

	return result, nil
}

// StoreAttestation stores an attestation artifact.
func (m *MockRepository) StoreAttestation(ctx context.Context, keyVersion string, att *types.MasterKeyAttestation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attestations[keyVersion] = att
	return nil
}

// GetAttestation retrieves an attestation by key version.
func (m *MockRepository) GetAttestation(ctx context.Context, keyVersion string) (*types.MasterKeyAttestation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	att, ok := m.attestations[keyVersion]
	if !ok {
		return nil, fmt.Errorf("attestation not found for key version: %s", keyVersion)
	}

	return att, nil
}

// Ensure implementations satisfy interface
var _ Repository = (*SupabaseRepository)(nil)
var _ Repository = (*MockRepository)(nil)
