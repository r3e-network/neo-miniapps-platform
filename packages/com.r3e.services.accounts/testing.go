package accounts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu       sync.RWMutex
	accounts map[string]Account
	wallets  map[string]WorkspaceWallet
}

// NewMemoryStore creates a new in-memory store for testing.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		accounts: make(map[string]Account),
		wallets:  make(map[string]WorkspaceWallet),
	}
}

// CreateAccount creates a new account.
func (s *MemoryStore) CreateAccount(ctx context.Context, acct Account) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acct.ID == "" {
		acct.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	acct.CreatedAt = now
	acct.UpdatedAt = now

	s.accounts[acct.ID] = acct
	return acct, nil
}

// UpdateAccount updates an existing account.
func (s *MemoryStore) UpdateAccount(ctx context.Context, acct Account) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.accounts[acct.ID]
	if !ok {
		return Account{}, fmt.Errorf("account not found: %s", acct.ID)
	}

	acct.CreatedAt = existing.CreatedAt
	acct.UpdatedAt = time.Now().UTC()
	s.accounts[acct.ID] = acct
	return acct, nil
}

// GetAccount retrieves an account by ID.
func (s *MemoryStore) GetAccount(ctx context.Context, id string) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acct, ok := s.accounts[id]
	if !ok {
		return Account{}, fmt.Errorf("account not found: %s", id)
	}
	return acct, nil
}

// ListAccounts returns all accounts.
func (s *MemoryStore) ListAccounts(ctx context.Context) ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Account, 0, len(s.accounts))
	for _, acct := range s.accounts {
		result = append(result, acct)
	}
	return result, nil
}

// DeleteAccount removes an account by ID.
func (s *MemoryStore) DeleteAccount(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.accounts[id]; !ok {
		return fmt.Errorf("account not found: %s", id)
	}
	delete(s.accounts, id)
	return nil
}

// CreateWorkspaceWallet creates a new workspace wallet.
func (s *MemoryStore) CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if wallet.ID == "" {
		wallet.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	wallet.CreatedAt = now
	wallet.UpdatedAt = now

	s.wallets[wallet.ID] = wallet
	return wallet, nil
}

// GetWorkspaceWallet retrieves a workspace wallet by ID.
func (s *MemoryStore) GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallet, ok := s.wallets[id]
	if !ok {
		return WorkspaceWallet{}, fmt.Errorf("wallet not found: %s", id)
	}
	return wallet, nil
}

// ListWorkspaceWallets lists all wallets for a workspace.
func (s *MemoryStore) ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []WorkspaceWallet
	for _, wallet := range s.wallets {
		if wallet.WorkspaceID == workspaceID {
			result = append(result, wallet)
		}
	}
	return result, nil
}

// FindWorkspaceWalletByAddress finds a wallet by address within a workspace.
func (s *MemoryStore) FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, walletAddr string) (WorkspaceWallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, wallet := range s.wallets {
		if wallet.WorkspaceID == workspaceID && wallet.WalletAddress == walletAddr {
			return wallet, nil
		}
	}
	return WorkspaceWallet{}, fmt.Errorf("wallet not found: %s", walletAddr)
}

// Compile-time check that MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)
