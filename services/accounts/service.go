// Package accounts provides account management service.
package accounts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "accounts"
	ServiceName = "Accounts Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Account management service",
		RequiredCapabilities: []os.Capability{
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapAuth,          // Authentication
			os.CapCache,         // Account caching
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  64 * 1024 * 1024,
			MaxCPUTime: 10 * time.Second,
		},
	}
}

// Service implements the Accounts service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new Accounts service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	s := &Service{
		BaseService: base.NewBaseService(ServiceID, ServiceName, Version, serviceOS),
		enclave:     NewEnclave(serviceOS),
		store:       NewStore(),
	}

	// Register components for lifecycle management
	s.SetEnclave(s.enclave)
	s.SetStore(s.store)

	// Set lifecycle hooks
	s.SetHooks(base.LifecycleHooks{
		OnAfterStart: s.onAfterStart,
	})

	return s, nil
}

// onAfterStart is called after components are initialized.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Optional sealed config hydrate inside enclave
	if ok, err := s.enclave.StorageExists(ctx, "accounts/config"); err == nil && ok {
		if err := s.enclave.UseStorage(ctx, "accounts/config", func(_ []byte) error { return nil }); err != nil {
			s.Logger().Warn("accounts storage hydrate failed", "err", err)
		}
	} else if err != nil && !base.IsCapabilityDenied(err) {
		s.Logger().Warn("accounts storage hydrate skipped", "err", err)
	}
	return nil
}

// CreateAccount creates a new account.
func (s *Service) CreateAccount(ctx context.Context, account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	account.Status = AccountStatusActive
	account.SetTimestamps()
	return s.store.CreateAccount(ctx, account)
}

// GetAccount gets an account by ID.
func (s *Service) GetAccount(ctx context.Context, id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetAccount(ctx, id)
}

// ListAccounts returns all accounts.
func (s *Service) ListAccounts(ctx context.Context) ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.ListAccounts(ctx)
}

// DeleteAccount deletes an account.
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.DeleteAccount(ctx, id)
}
