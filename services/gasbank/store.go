// Package gasbank provides gas fee management service.
package gasbank

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages GasBank service data using Supabase PostgreSQL.
type Store struct {
	mu           sync.RWMutex
	accounts     *base.SupabaseStore[*GasAccount]
	transactions *base.SupabaseStore[*GasTransaction]
	ready        bool
}

// NewStore creates a new GasBank store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		accounts:     base.NewSupabaseStore[*GasAccount](config, "gas_accounts"),
		transactions: base.NewSupabaseStore[*GasTransaction](config, "gas_transactions"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		accounts:     base.NewSupabaseStore[*GasAccount](config, "gas_accounts"),
		transactions: base.NewSupabaseStore[*GasTransaction](config, "gas_transactions"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.accounts.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize accounts store: %w", err)
	}
	if err := s.transactions.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize transactions store: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store (alias for Shutdown for backward compatibility).
func (s *Store) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Shutdown shuts down the store.
func (s *Store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.accounts.Close(ctx)
	s.transactions.Close(ctx)
	s.ready = false
	return nil
}

// Health checks store health.
func (s *Store) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.accounts.Health(ctx)
}

// CreateAccount creates a new gas account.
func (s *Store) CreateAccount(ctx context.Context, account *GasAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	account.GenerateID()
	account.SetTimestamps()
	if account.Status == "" {
		account.Status = AccountStatusActive
	}
	return s.accounts.Create(ctx, account)
}

// GetAccount retrieves a gas account.
func (s *Store) GetAccount(ctx context.Context, id string) (*GasAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.accounts.Get(ctx, id)
}

// UpdateAccount updates a gas account.
func (s *Store) UpdateAccount(ctx context.Context, account *GasAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	account.SetTimestamps()
	return s.accounts.Update(ctx, account)
}

// ListAccounts lists all gas accounts.
func (s *Store) ListAccounts(ctx context.Context) ([]*GasAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.accounts.List(ctx)
}

// CreateTransaction creates a new gas transaction.
func (s *Store) CreateTransaction(ctx context.Context, tx *GasTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	tx.GenerateID()
	tx.SetTimestamps()
	if tx.Status == "" {
		tx.Status = TxStatusPending
	}
	return s.transactions.Create(ctx, tx)
}

// GetTransaction retrieves a gas transaction.
func (s *Store) GetTransaction(ctx context.Context, id string) (*GasTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.transactions.Get(ctx, id)
}

// ListTransactionsByAccount lists transactions for an account.
func (s *Store) ListTransactionsByAccount(ctx context.Context, accountID string) ([]*GasTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.transactions.ListWithFilter(ctx, "account_id=eq."+accountID)
}

// ListTransactions lists all transactions.
func (s *Store) ListTransactions(ctx context.Context) ([]*GasTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.transactions.List(ctx)
}

// GetStats returns service statistics.
func (s *Store) GetStats(ctx context.Context) (*GasStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	accounts, err := s.accounts.List(ctx)
	if err != nil {
		return nil, err
	}

	transactions, err := s.transactions.List(ctx)
	if err != nil {
		return nil, err
	}

	var activeAccounts int64
	for _, acc := range accounts {
		if acc.Status == AccountStatusActive {
			activeAccounts++
		}
	}

	return &GasStats{
		TotalAccounts:     int64(len(accounts)),
		ActiveAccounts:    activeAccounts,
		TotalTransactions: int64(len(transactions)),
	}, nil
}
