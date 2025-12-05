// Package gasbank provides gas fee management service.
package gasbank

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "gasbank"
	ServiceName = "GasBank Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Gas fee management and sponsorship service",
		RequiredCapabilities: []os.Capability{
			os.CapKeys,
			os.CapKeysSign,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapSecrets,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapNeo,           // Neo N3 blockchain
			os.CapNeoSign,       // Neo transaction signing
			os.CapCache,         // Balance caching
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  64 * 1024 * 1024,
			MaxCPUTime: 30 * time.Second,
		},
	}
}

// Service implements the GasBank service.
type Service struct {
	*base.BaseService

	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new GasBank service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)

	enclave, err := NewEnclave(serviceOS)
	if err != nil {
		return nil, fmt.Errorf("create enclave: %w", err)
	}

	store := NewStore()

	svc := &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}

	// Register components with BaseService for automatic lifecycle management
	baseService.SetEnclave(enclave)
	baseService.SetStore(store)

	// Set lifecycle hooks
	baseService.SetHooks(base.LifecycleHooks{
		OnAfterStart: svc.onAfterStart,
	})

	return svc, nil
}

// Store exposes the GasBank store.
func (s *Service) Store() *Store {
	return s.store
}

// onAfterStart is called after components are initialized.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Optional sealed config hydrate inside enclave
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "gasbank/config", &cfg); err != nil {
		if !base.IsCapabilityDenied(err) {
			s.Logger().Warn("gasbank storage hydrate failed", "err", err)
		}
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("gasbank config loaded from sealed storage")
	}

	// Register metrics if available
	if s.OS().HasCapability(os.CapMetrics) {
		s.RegisterMetrics("gasbank", func(metrics os.MetricsAPI) {
			metrics.RegisterCounter("gasbank_deposits_total", "Total number of deposits")
			metrics.RegisterCounter("gasbank_sponsorships_total", "Total number of gas sponsorships")
			metrics.RegisterCounter("gasbank_sponsorships_failed", "Total number of failed sponsorships")
			metrics.RegisterGauge("gasbank_total_balance", "Total balance across all accounts")
			metrics.RegisterHistogram("gasbank_sponsorship_duration_seconds", "Sponsorship processing duration", []float64{0.01, 0.05, 0.1, 0.5, 1})
		})
	}

	return nil
}

// =============================================================================
// GasBank Operations
// =============================================================================

// CreateAccount creates a new gas account.
func (s *Service) CreateAccount(ctx context.Context, accountID string) (*GasAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	account := &GasAccount{
		AccountID: accountID,
		Balance:   "0",
		Status:    AccountStatusActive,
	}
	account.ID = accountID
	account.SetTimestamps()

	if err := s.store.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	return account, nil
}

// GetAccount retrieves a gas account.
func (s *Service) GetAccount(ctx context.Context, accountID string) (*GasAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetAccount(ctx, accountID)
}

// Store exposes the GasBank store.
func (s *Service) Store() *Store {
	return s.store
}

// Deposit adds funds to an account.
func (s *Service) Deposit(ctx context.Context, accountID, amount, txHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	// Get account
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	// Update balance (simplified - real impl would use big.Int)
	account.Balance = addBalances(account.Balance, amount)
	account.TotalDeposited = addBalances(account.TotalDeposited, amount)
	account.SetTimestamps()

	if err := s.store.UpdateAccount(ctx, account); err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	// Record transaction
	tx := &GasTransaction{
		AccountID: accountID,
		Type:      TxTypeDeposit,
		Amount:    amount,
		TxHash:    txHash,
		Status:    TxStatusConfirmed,
	}
	tx.ID = fmt.Sprintf("tx-%d", time.Now().UnixNano())
	tx.SetTimestamps()

	return s.store.CreateTransaction(ctx, tx)
}

// SponsorGas sponsors gas for a transaction.
func (s *Service) SponsorGas(ctx context.Context, accountID string, gasAmount string, targetTx []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Get account
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	// Check balance (simplified)
	if account.Balance < gasAmount {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Sign the sponsorship in enclave
	signature, err := s.enclave.SignSponsorship(ctx, accountID, targetTx)
	if err != nil {
		return nil, fmt.Errorf("sign sponsorship: %w", err)
	}

	// Deduct balance
	account.Balance = subtractBalances(account.Balance, gasAmount)
	account.TotalSpent = addBalances(account.TotalSpent, gasAmount)
	account.SetTimestamps()

	if err := s.store.UpdateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}

	return signature, nil
}

// GetStats returns service statistics.
func (s *Service) GetStats(ctx context.Context) (*GasStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, err := s.store.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	stats.GeneratedAt = time.Now()
	return stats, nil
}

// Helper functions for balance operations (simplified)
func addBalances(a, b string) string {
	// Real implementation would use big.Int
	return a + "+" + b
}

func subtractBalances(a, b string) string {
	// Real implementation would use big.Int
	return a + "-" + b
}
