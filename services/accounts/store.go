// Package accounts provides account management service.
package accounts

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// StoreInterface defines the interface for account storage.
type StoreInterface interface {
	base.Store
	CreateAccount(ctx context.Context, account *Account) error
	GetAccount(ctx context.Context, id string) (*Account, error)
	GetAccountByUserID(ctx context.Context, userID string) (*Account, error)
	UpdateAccount(ctx context.Context, account *Account) error
	ListAccounts(ctx context.Context) ([]*Account, error)
	DeleteAccount(ctx context.Context, id string) error
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	ListAPIKeysByAccount(ctx context.Context, accountID string) ([]*APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
}

// Store manages Accounts service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	accounts *base.SupabaseStore[*Account]
	apiKeys  *base.SupabaseStore[*APIKey]
	ready    bool
}

// NewStore creates a new store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		accounts: base.NewSupabaseStore[*Account](config, "accounts"),
		apiKeys:  base.NewSupabaseStore[*APIKey](config, "api_keys"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		accounts: base.NewSupabaseStore[*Account](config, "accounts"),
		apiKeys:  base.NewSupabaseStore[*APIKey](config, "api_keys"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.accounts.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize accounts store: %w", err)
	}
	if err := s.apiKeys.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize api_keys store: %w", err)
	}

	s.ready = true
	return nil
}

// Shutdown shuts down the store (implements Component interface).
func (s *Store) Shutdown(ctx context.Context) error {
	return s.Close(ctx)
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.accounts.Close(ctx)
	s.apiKeys.Close(ctx)
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

// =============================================================================
// Account Operations
// =============================================================================

// CreateAccount creates a new account.
func (s *Store) CreateAccount(ctx context.Context, account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	account.GenerateID()
	account.SetTimestamps()
	return s.accounts.Create(ctx, account)
}

// GetAccount gets an account by ID.
func (s *Store) GetAccount(ctx context.Context, id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.accounts.Get(ctx, id)
}

// GetAccountByUserID gets an account by Supabase user ID.
func (s *Store) GetAccountByUserID(ctx context.Context, userID string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	accounts, err := s.accounts.ListWithFilter(ctx, "user_id=eq."+userID+"&limit=1")
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("account not found for user: %s", userID)
	}
	return accounts[0], nil
}

// GetAccountByAddress gets an account by blockchain address.
func (s *Store) GetAccountByAddress(ctx context.Context, address string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	accounts, err := s.accounts.ListWithFilter(ctx, "address=eq."+address+"&limit=1")
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("account not found for address: %s", address)
	}
	return accounts[0], nil
}

// UpdateAccount updates an account.
func (s *Store) UpdateAccount(ctx context.Context, account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	account.SetTimestamps()
	return s.accounts.Update(ctx, account)
}

// ListAccounts lists all accounts.
func (s *Store) ListAccounts(ctx context.Context) ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.accounts.List(ctx)
}

// DeleteAccount deletes an account.
func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.accounts.Delete(ctx, id)
}

// CountAccounts returns the number of accounts.
func (s *Store) CountAccounts(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}
	return s.accounts.Count(ctx)
}

// =============================================================================
// API Key Operations
// =============================================================================

// CreateAPIKey creates a new API key.
func (s *Store) CreateAPIKey(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	key.GenerateID()
	key.SetTimestamps()
	return s.apiKeys.Create(ctx, key)
}

// GetAPIKey gets an API key by ID.
func (s *Store) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.apiKeys.Get(ctx, id)
}

// GetAPIKeyByHash gets an API key by hash.
func (s *Store) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	keys, err := s.apiKeys.ListWithFilter(ctx, "key_hash=eq."+hash+"&limit=1")
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("api key not found")
	}
	return keys[0], nil
}

// ListAPIKeysByAccount lists all API keys for an account.
func (s *Store) ListAPIKeysByAccount(ctx context.Context, accountID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.apiKeys.ListWithFilter(ctx, "account_id=eq."+accountID)
}

// DeleteAPIKey deletes an API key.
func (s *Store) DeleteAPIKey(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.apiKeys.Delete(ctx, id)
}

// UpdateAPIKey updates an API key.
func (s *Store) UpdateAPIKey(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.apiKeys.Update(ctx, key)
}
