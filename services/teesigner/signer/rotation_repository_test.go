package signer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

type fakeKeyVersionRepo struct {
	mu sync.Mutex

	nextID   int64
	versions map[string]database.KeyVersion

	listErr error
	listFn  func(statuses []string) ([]database.KeyVersion, error)

	createErr           error
	createErrStillWrite bool

	updateErr error
}

func newFakeKeyVersionRepo() *fakeKeyVersionRepo {
	return &fakeKeyVersionRepo{
		nextID:   1,
		versions: make(map[string]database.KeyVersion),
	}
}

func (r *fakeKeyVersionRepo) GetActiveKeyVersion(_ context.Context) (*database.KeyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var active *database.KeyVersion
	for _, v := range r.versions {
		if v.Status != database.KeyVersionStatusActive {
			continue
		}
		v := v
		if active == nil || v.ValidFrom.After(active.ValidFrom) {
			active = &v
		}
	}
	if active == nil {
		return nil, database.NewNotFoundError("key_version", "active")
	}
	out := *active
	return &out, nil
}

func (r *fakeKeyVersionRepo) GetKeyVersion(_ context.Context, keyVersion string) (*database.KeyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.versions[keyVersion]
	if !ok {
		return nil, database.NewNotFoundError("key_version", keyVersion)
	}
	out := v
	return &out, nil
}

func (r *fakeKeyVersionRepo) ListKeyVersionsByStatus(_ context.Context, statuses []string) ([]database.KeyVersion, error) {
	if r.listFn != nil {
		return r.listFn(statuses)
	}
	if r.listErr != nil {
		return nil, r.listErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	allowed := make(map[string]struct{}, len(statuses))
	for _, s := range statuses {
		allowed[s] = struct{}{}
	}

	out := make([]database.KeyVersion, 0, len(r.versions))
	for _, v := range r.versions {
		if _, ok := allowed[v.Status]; ok {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ValidFrom.After(out[j].ValidFrom)
	})
	return out, nil
}

func (r *fakeKeyVersionRepo) CreateKeyVersion(_ context.Context, create database.KeyVersionCreate) (*database.KeyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	write := func() (*database.KeyVersion, error) {
		if _, ok := r.versions[create.KeyVersion]; ok {
			return nil, fmt.Errorf("%w: key_version already exists", database.ErrAlreadyExists)
		}
		if create.Status == database.KeyVersionStatusActive {
			for _, v := range r.versions {
				if v.Status == database.KeyVersionStatusActive {
					return nil, fmt.Errorf("active key already exists")
				}
			}
		}

		id := r.nextID
		r.nextID++
		rec := database.KeyVersion{
			ID:         id,
			KeyVersion: create.KeyVersion,
			Status:     create.Status,
			ValidFrom:  create.ValidFrom.UTC(),
			ValidUntil: create.ValidUntil,
			CreatedAt:  create.ValidFrom.UTC(),
		}
		r.versions[rec.KeyVersion] = rec
		out := rec
		return &out, nil
	}

	if r.createErr != nil {
		if r.createErrStillWrite {
			_, _ = write()
		}
		return nil, r.createErr
	}
	return write()
}

func (r *fakeKeyVersionRepo) UpdateKeyVersion(_ context.Context, keyVersion string, update database.KeyVersionUpdate) (*database.KeyVersion, error) {
	if r.updateErr != nil {
		return nil, r.updateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.versions[keyVersion]
	if !ok {
		return nil, database.NewNotFoundError("key_version", keyVersion)
	}
	if update.Status != nil {
		rec.Status = *update.Status
	}
	if update.ValidUntil != nil {
		rec.ValidUntil = update.ValidUntil
	}
	r.versions[keyVersion] = rec
	out := rec
	return &out, nil
}

func TestKeyManager_Repository_BootstrapsInitialVersion(t *testing.T) {
	current := time.Unix(100, 0).UTC()
	nowFn := func() time.Time { return current }

	repo := newFakeKeyVersionRepo()
	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Repository:        repo,
		Now:               nowFn,
		RequireRepository: false,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v100" {
		t.Fatalf("expected active=v100, got %q", got)
	}
	if _, _, err := mgr.SigningKeyAt(context.Background(), "", current); err != nil {
		t.Fatalf("SigningKeyAt(active): %v", err)
	}
}

func TestKeyManager_Repository_RotateAndOverlap(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	repo := newFakeKeyVersionRepo()
	repo.versions["v1"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v1",
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  current,
		CreatedAt:  current,
	}
	repo.nextID = 2

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v1" {
		t.Fatalf("expected active=v1, got %q", got)
	}

	current = time.Unix(2, 0).UTC()
	if _, err := mgr.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v2" {
		t.Fatalf("expected active=v2, got %q", got)
	}

	withinOverlap := time.Unix(2, 0).UTC().Add(6 * 24 * time.Hour)
	if _, used, err := mgr.SigningKeyAt(context.Background(), "v1", withinOverlap); err != nil {
		t.Fatalf("SigningKeyAt(v1 within overlap): %v", err)
	} else if used != "v1" {
		t.Fatalf("expected used=v1, got %q", used)
	}

	afterExpiry := time.Unix(2, 0).UTC().Add(8 * 24 * time.Hour)
	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", afterExpiry); err == nil {
		t.Fatal("expected expired key error")
	} else if !errors.Is(err, ErrKeyVersionExpired) {
		t.Fatalf("expected ErrKeyVersionExpired, got %v", err)
	}
}

func TestKeyManager_Repository_LoadVersionFromRepository(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	validUntil := now.Add(7 * 24 * time.Hour)
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.versions["v1"] = database.KeyVersion{ID: 2, KeyVersion: "v1", Status: database.KeyVersionStatusDeprecated, ValidFrom: time.Unix(1, 0).UTC(), ValidUntil: &validUntil, CreatedAt: time.Unix(1, 0).UTC()}
	repo.nextID = 3

	// Simulate a partial list response (active only) so v1 must be fetched on-demand.
	repo.listFn = func(statuses []string) ([]database.KeyVersion, error) {
		return []database.KeyVersion{repo.versions["v2"]}, nil
	}

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	if _, used, err := mgr.SigningKeyAt(context.Background(), "v1", now.Add(24*time.Hour)); err != nil {
		t.Fatalf("SigningKeyAt(v1): %v", err)
	} else if used != "v1" {
		t.Fatalf("expected used=v1, got %q", used)
	}
}

func TestKeyManager_Repository_LoadExpiredRejected(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.versions["v1"] = database.KeyVersion{ID: 2, KeyVersion: "v1", Status: database.KeyVersionStatusExpired, ValidFrom: time.Unix(1, 0).UTC(), CreatedAt: time.Unix(1, 0).UTC()}
	repo.nextID = 3

	repo.listFn = func(statuses []string) ([]database.KeyVersion, error) {
		return []database.KeyVersion{repo.versions["v2"]}, nil
	}

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", now); err == nil {
		t.Fatal("expected error for expired key")
	} else if !errors.Is(err, ErrKeyVersionExpired) {
		t.Fatalf("expected ErrKeyVersionExpired, got %v", err)
	}
}

func TestKeyManager_Repository_RotateCreateErrorUsesExistingActive(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	repo := newFakeKeyVersionRepo()
	repo.versions["v1"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v1",
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  current,
		CreatedAt:  current,
	}
	repo.nextID = 2
	repo.createErr = errors.New("duplicate key_version")
	repo.createErrStillWrite = true

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	current = time.Unix(2, 0).UTC()
	if _, err := mgr.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v2" {
		t.Fatalf("expected active=v2, got %q", got)
	}
}

func TestKeyManager_Repository_ListErrorFallsBackToActiveOnly(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.nextID = 2
	repo.listErr = errors.New("boom")

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v2" {
		t.Fatalf("expected active=v2, got %q", got)
	}
	if _, _, err := mgr.SigningKeyAt(context.Background(), "", now); err != nil {
		t.Fatalf("SigningKeyAt(active): %v", err)
	}
}

func TestNewKeyManager_RequireRepository_ErrorsWhenMissingRepo(t *testing.T) {
	_, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		RequireRepository: true,
		Repository:        nil,
		InitialKeyVersion: "v1",
		OverlapPeriod:     DefaultOverlapPeriod,
	})
	if err == nil {
		t.Fatal("expected error when repository is required but missing")
	}
}

func TestKeyManager_Rotate_IdempotentNoOp(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Now:               nowFn,
		InitialKeyVersion: "v2",
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	res, err := mgr.Rotate(context.Background())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if res.Rotated {
		t.Fatal("expected rotated=false for idempotent rotation")
	}
	if res.NewVersion != "v2" || res.OldVersion != "v2" {
		t.Fatalf("unexpected versions: %+v", res)
	}
}

func TestOptionalTimePtr(t *testing.T) {
	if got := optionalTimePtr(false, time.Unix(1, 0)); got != nil {
		t.Fatal("expected nil for optionalTimePtr(false, ...)")
	}
}

func TestKeyManager_Rotate_RepositoryErrorFallsBackToMemory(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	repo := newFakeKeyVersionRepo()
	repo.versions["v1"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v1",
		Status:     database.KeyVersionStatusActive,
		ValidFrom:  current,
		CreatedAt:  current,
	}
	repo.nextID = 2
	repo.updateErr = errors.New("db down")

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	current = time.Unix(2, 0).UTC()
	if _, err := mgr.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v2" {
		t.Fatalf("expected active=v2 after fallback rotation, got %q", got)
	}
}

func TestKeyManager_Repository_BootstrapCreateErrorFallsBackToV1(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.createErr = errors.New("create failed")

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v1" {
		t.Fatalf("expected active=v1 fallback, got %q", got)
	}
}

func TestKeyManager_Repository_ActiveMissingFromListIsAdded(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	validUntil := now.Add(7 * 24 * time.Hour)
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.versions["v1"] = database.KeyVersion{ID: 2, KeyVersion: "v1", Status: database.KeyVersionStatusDeprecated, ValidFrom: time.Unix(1, 0).UTC(), ValidUntil: &validUntil, CreatedAt: time.Unix(1, 0).UTC()}
	repo.nextID = 3

	// Only return the deprecated key in the list call.
	repo.listFn = func(statuses []string) ([]database.KeyVersion, error) {
		return []database.KeyVersion{repo.versions["v1"]}, nil
	}

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if got := mgr.ActiveVersion(); got != "v2" {
		t.Fatalf("expected active=v2, got %q", got)
	}
	if _, used, err := mgr.SigningKeyAt(context.Background(), "", now); err != nil {
		t.Fatalf("SigningKeyAt(active): %v", err)
	} else if used != "v2" {
		t.Fatalf("expected used=v2, got %q", used)
	}
}

func TestKeyManager_Repository_ExpiredDeprecatedIsMarkedExpired(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	expiredUntil := now.Add(-1 * time.Second)
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.versions["v1"] = database.KeyVersion{ID: 2, KeyVersion: "v1", Status: database.KeyVersionStatusDeprecated, ValidFrom: time.Unix(1, 0).UTC(), ValidUntil: &expiredUntil, CreatedAt: time.Unix(1, 0).UTC()}
	repo.nextID = 3

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", now); err == nil {
		t.Fatal("expected error for expired deprecated key")
	} else if !errors.Is(err, ErrKeyVersionExpired) {
		t.Fatalf("expected ErrKeyVersionExpired, got %v", err)
	}
}

func TestKeyManager_Repository_LoadNotFoundReturnsNotFound(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.nextID = 2

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", now); err == nil {
		t.Fatal("expected error for unknown key version")
	} else if !errors.Is(err, ErrKeyVersionNotFound) {
		t.Fatalf("expected ErrKeyVersionNotFound, got %v", err)
	}
}

func TestKeyManager_rotateInRepository_NotFoundUpdateIsIgnored(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	repo := newFakeKeyVersionRepo()

	mgr := &KeyManager{
		masterKeySeed: []byte("seed"),
		repo:          repo,
		now:           func() time.Time { return now },
		overlap:       DefaultOverlapPeriod,
		keys:          make(map[string]keyEntry),
	}

	if err := mgr.rotateInRepository(nil, "v1", "v2", now, now.Add(DefaultOverlapPeriod)); err != nil {
		t.Fatalf("rotateInRepository: %v", err)
	}
	if _, err := repo.GetKeyVersion(context.Background(), "v2"); err != nil {
		t.Fatalf("expected v2 to be created: %v", err)
	}
}

func TestKeyManager_rotateInRepository_CreateErrorExistingNotActiveFails(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	repo := newFakeKeyVersionRepo()
	repo.createErr = errors.New("create failed")
	repo.versions["v2"] = database.KeyVersion{
		ID:         1,
		KeyVersion: "v2",
		Status:     database.KeyVersionStatusExpired,
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

	err := mgr.rotateInRepository(context.Background(), "", "v2", now, now.Add(DefaultOverlapPeriod))
	if err == nil {
		t.Fatal("expected error when existing version is not active")
	}
}

func TestKeyManager_rotateInRepository_CreateErrorAndGetErrReturnsCreateErr(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	createErr := errors.New("create failed")
	repo := newFakeKeyVersionRepo()
	repo.createErr = createErr

	mgr := &KeyManager{
		masterKeySeed: []byte("seed"),
		repo:          repo,
		now:           func() time.Time { return now },
		overlap:       DefaultOverlapPeriod,
		keys:          make(map[string]keyEntry),
	}

	err := mgr.rotateInRepository(context.Background(), "", "v2", now, now.Add(DefaultOverlapPeriod))
	if err == nil {
		t.Fatal("expected error")
	}
	if err != createErr {
		t.Fatalf("expected createErr to be returned, got %v", err)
	}
}

func TestKeyManager_SigningKeyAt_UnknownStatusErrors(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	mgr.mu.Lock()
	entry := mgr.keys[mgr.activeVersion]
	entry.status = "mystery"
	mgr.keys[mgr.activeVersion] = entry
	mgr.mu.Unlock()

	if _, _, err := mgr.SigningKeyAt(context.Background(), "", now); err == nil {
		t.Fatal("expected error for unknown key status")
	}
}

func TestKeyManager_Repository_RequireRepoCreateErrorFailsClosed(t *testing.T) {
	now := time.Unix(10, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.createErr = errors.New("create failed")

	_, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Repository:        repo,
		Now:               nowFn,
		RequireRepository: true,
	})
	if err == nil {
		t.Fatal("expected error when repository is required and bootstrap create fails")
	}
}

func TestKeyManager_Repository_RequireRepoListErrorFailsClosed(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.listErr = errors.New("list failed")

	_, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed:     []byte("seed"),
		Repository:        repo,
		Now:               nowFn,
		RequireRepository: true,
	})
	if err == nil {
		t.Fatal("expected error when repository is required and list fails")
	}
}

func TestKeyManager_loadVersionFromRepository_DeprecatedExpiredIsRejected(t *testing.T) {
	now := time.Unix(2, 0).UTC()
	nowFn := func() time.Time { return now }

	repo := newFakeKeyVersionRepo()
	expiredUntil := now.Add(-1 * time.Second)
	repo.versions["v2"] = database.KeyVersion{ID: 1, KeyVersion: "v2", Status: database.KeyVersionStatusActive, ValidFrom: now, CreatedAt: now}
	repo.versions["v1"] = database.KeyVersion{ID: 2, KeyVersion: "v1", Status: database.KeyVersionStatusDeprecated, ValidFrom: time.Unix(1, 0).UTC(), ValidUntil: &expiredUntil, CreatedAt: time.Unix(1, 0).UTC()}

	// Keep the deprecated key out of the initial keyring so it must be loaded.
	repo.listFn = func(statuses []string) ([]database.KeyVersion, error) {
		return []database.KeyVersion{repo.versions["v2"]}, nil
	}

	mgr, err := NewKeyManager(KeyManagerConfig{
		MasterKeySeed: []byte("seed"),
		Repository:    repo,
		Now:           nowFn,
	})
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	if _, _, err := mgr.SigningKeyAt(context.Background(), "v1", now); err == nil {
		t.Fatal("expected error for expired deprecated key")
	} else if !errors.Is(err, ErrKeyVersionExpired) {
		t.Fatalf("expected ErrKeyVersionExpired, got %v", err)
	}
}

func TestService_RotateKey_AllowsNilCtxAndNoAudit(t *testing.T) {
	current := time.Unix(1, 0).UTC()
	nowFn := func() time.Time { return current }

	svc, err := NewService(ServiceConfig{
		MasterKeySeed: []byte("seed"),
		Now:           nowFn,
		RateLimiter:   NewClientRateLimiter(1000, 1000, nowFn),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	current = time.Unix(2, 0).UTC()
	res, err := svc.RotateKey(nil)
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if res.NewVersion != "v2" {
		t.Fatalf("expected new_version=v2, got %q", res.NewVersion)
	}
}
