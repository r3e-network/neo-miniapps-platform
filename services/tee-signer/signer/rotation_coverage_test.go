package signer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

type getActiveErrorRepo struct {
	err error
}

func (r getActiveErrorRepo) GetActiveKeyVersion(_ context.Context) (*database.KeyVersion, error) {
	return nil, r.err
}

func (getActiveErrorRepo) GetKeyVersion(_ context.Context, _ string) (*database.KeyVersion, error) {
	return nil, errors.New("unexpected GetKeyVersion call")
}

func (getActiveErrorRepo) ListKeyVersionsByStatus(_ context.Context, _ []string) ([]database.KeyVersion, error) {
	return nil, errors.New("unexpected ListKeyVersionsByStatus call")
}

func (getActiveErrorRepo) CreateKeyVersion(_ context.Context, _ database.KeyVersionCreate) (*database.KeyVersion, error) {
	return nil, errors.New("unexpected CreateKeyVersion call")
}

func (getActiveErrorRepo) UpdateKeyVersion(_ context.Context, _ string, _ database.KeyVersionUpdate) (*database.KeyVersion, error) {
	return nil, errors.New("unexpected UpdateKeyVersion call")
}

func TestKeyManager_refreshFromRepository_NilReceiverErrors(t *testing.T) {
	var mgr *KeyManager
	if err := mgr.refreshFromRepository(context.Background()); err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestKeyManager_refreshFromRepository_NoRepoNoop(t *testing.T) {
	mgr := &KeyManager{}
	if err := mgr.refreshFromRepository(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestKeyManager_refreshFromRepository_NilContextIsHandled(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	repo := newFakeKeyVersionRepo()
	repo.versions["v2"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v2",
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  now,
		CreatedAt:  now,
	}

	mgr := &KeyManager{
		masterKeySeed: []byte("seed"),
		repo:          repo,
		now:           func() time.Time { return now },
		overlap:       DefaultOverlapPeriod,
		keys:          make(map[string]keyEntry),
	}

	if err := mgr.refreshFromRepository(nil); err != nil {
		t.Fatalf("refreshFromRepository(nil): %v", err)
	}
}

func TestKeyManager_New_WithRepositoryGetActiveError_FallsBackWhenNotRequired(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    getActiveErrorRepo{err: errors.New("db down")},
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v1" {
		t.Fatalf("expected fallback active=v1, got %q", got)
	}
}

func TestKeyManager_New_WithRepositoryGetActiveError_FailsClosedWhenRequired(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	_, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Repository:        getActiveErrorRepo{err: errors.New("db down")},
		Now:               func() time.Time { return now },
		RequireRepository: true,
	})
	if err == nil {
		t.Fatal("expected error in requireRepo mode")
	}
}

func TestKeyManager_Rotate_NilReceiverErrors(t *testing.T) {
	var mgr *KeyManager
	if _, err := mgr.Rotate(context.Background()); err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestKeyManager_Rotate_RequireRepoPropagatesErrors(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.versions["v1"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v1",
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  now,
		CreatedAt:  now,
	}
	repo.updateErr = errors.New("db down")

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Repository:        repo,
		Now:               nowFn,
		RequireRepository: true,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	now = time.Unix(2, 0).UTC()
	if _, err := mgr.Rotate(context.Background()); err == nil {
		t.Fatal("expected rotation to fail closed when repository is required")
	}
}
