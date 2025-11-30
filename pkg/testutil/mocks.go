// Package testutil provides common testing utilities and mock implementations.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockAccountChecker is a test implementation of AccountChecker interface.
type MockAccountChecker struct {
	mu       sync.RWMutex
	accounts map[string]string // accountID -> tenantID
}

// NewMockAccountChecker creates a new mock account checker with the given account IDs.
func NewMockAccountChecker(accountIDs ...string) *MockAccountChecker {
	m := &MockAccountChecker{accounts: make(map[string]string)}
	for _, id := range accountIDs {
		m.accounts[id] = ""
	}
	return m
}

// AddAccount adds an account to the mock checker.
func (m *MockAccountChecker) AddAccount(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[accountID] = ""
}

// AddAccountWithTenant adds an account with a tenant ID.
func (m *MockAccountChecker) AddAccountWithTenant(accountID, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[accountID] = tenantID
}

// AccountExists checks if an account exists.
func (m *MockAccountChecker) AccountExists(_ context.Context, accountID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.accounts[accountID]; !ok {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

// AccountTenant returns the tenant ID for an account.
func (m *MockAccountChecker) AccountTenant(_ context.Context, accountID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accounts[accountID]
}

// MockWalletChecker is a test implementation of WalletChecker interface.
type MockWalletChecker struct {
	mu      sync.RWMutex
	wallets map[string]map[string]bool // accountID -> wallet -> owned
}

// NewMockWalletChecker creates a new mock wallet checker.
func NewMockWalletChecker() *MockWalletChecker {
	return &MockWalletChecker{wallets: make(map[string]map[string]bool)}
}

// AddWallet adds a wallet ownership record.
func (m *MockWalletChecker) AddWallet(accountID, wallet string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.wallets[accountID] == nil {
		m.wallets[accountID] = make(map[string]bool)
	}
	m.wallets[accountID][wallet] = true
}

// WalletOwnedBy checks if a wallet is owned by the account.
func (m *MockWalletChecker) WalletOwnedBy(_ context.Context, accountID, wallet string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if wallets, ok := m.wallets[accountID]; ok {
		if wallets[wallet] {
			return nil
		}
	}
	return fmt.Errorf("wallet %s not owned by account %s", wallet, accountID)
}

// MemoryStore is a generic in-memory store for testing.
type MemoryStore[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore[K comparable, V any]() *MemoryStore[K, V] {
	return &MemoryStore[K, V]{items: make(map[K]V)}
}

// Set stores an item.
func (s *MemoryStore[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
}

// Get retrieves an item.
func (s *MemoryStore[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[key]
	return v, ok
}

// Delete removes an item.
func (s *MemoryStore[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// All returns all items.
func (s *MemoryStore[K, V]) All() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[K]V, len(s.items))
	for k, v := range s.items {
		result[k] = v
	}
	return result
}

// Count returns the number of items.
func (s *MemoryStore[K, V]) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// GenerateID generates a new UUID string.
func GenerateID() string {
	return uuid.NewString()
}

// Now returns the current UTC time.
func Now() time.Time {
	return time.Now().UTC()
}
