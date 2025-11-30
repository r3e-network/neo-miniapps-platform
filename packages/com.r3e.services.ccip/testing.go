package ccip

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
	lanes    map[string]Lane
	messages map[string]Message
}

// NewMemoryStore creates a new in-memory store for testing.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		lanes:    make(map[string]Lane),
		messages: make(map[string]Message),
	}
}

// CreateLane creates a new lane.
func (s *MemoryStore) CreateLane(ctx context.Context, lane Lane) (Lane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lane.ID == "" {
		lane.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	lane.CreatedAt = now
	lane.UpdatedAt = now

	s.lanes[lane.ID] = lane
	return lane, nil
}

// UpdateLane updates an existing lane.
func (s *MemoryStore) UpdateLane(ctx context.Context, lane Lane) (Lane, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.lanes[lane.ID]
	if !ok {
		return Lane{}, fmt.Errorf("lane not found: %s", lane.ID)
	}

	lane.CreatedAt = existing.CreatedAt
	lane.UpdatedAt = time.Now().UTC()
	s.lanes[lane.ID] = lane
	return lane, nil
}

// GetLane retrieves a lane by ID.
func (s *MemoryStore) GetLane(ctx context.Context, id string) (Lane, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lane, ok := s.lanes[id]
	if !ok {
		return Lane{}, fmt.Errorf("lane not found: %s", id)
	}
	return lane, nil
}

// ListLanes returns all lanes for an account.
func (s *MemoryStore) ListLanes(ctx context.Context, accountID string) ([]Lane, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Lane
	for _, lane := range s.lanes {
		if lane.AccountID == accountID {
			result = append(result, lane)
		}
	}
	return result, nil
}

// CreateMessage creates a new message.
func (s *MemoryStore) CreateMessage(ctx context.Context, msg Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	msg.CreatedAt = now
	msg.UpdatedAt = now

	s.messages[msg.ID] = msg
	return msg, nil
}

// UpdateMessage updates an existing message.
func (s *MemoryStore) UpdateMessage(ctx context.Context, msg Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.messages[msg.ID]
	if !ok {
		return Message{}, fmt.Errorf("message not found: %s", msg.ID)
	}

	msg.CreatedAt = existing.CreatedAt
	msg.UpdatedAt = time.Now().UTC()
	s.messages[msg.ID] = msg
	return msg, nil
}

// GetMessage retrieves a message by ID.
func (s *MemoryStore) GetMessage(ctx context.Context, id string) (Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msg, ok := s.messages[id]
	if !ok {
		return Message{}, fmt.Errorf("message not found: %s", id)
	}
	return msg, nil
}

// ListMessages returns messages for an account.
func (s *MemoryStore) ListMessages(ctx context.Context, accountID string, limit int) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Message
	for _, msg := range s.messages {
		if msg.AccountID == accountID {
			result = append(result, msg)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// Compile-time check that MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)

// MockAccountChecker is a mock implementation of AccountChecker for testing.
type MockAccountChecker struct {
	mu       sync.RWMutex
	accounts map[string]string // accountID -> tenant
}

// NewMockAccountChecker creates a new mock account checker.
func NewMockAccountChecker() *MockAccountChecker {
	return &MockAccountChecker{
		accounts: make(map[string]string),
	}
}

// AddAccount adds an account to the mock.
func (m *MockAccountChecker) AddAccount(id, tenant string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[id] = tenant
}

// AccountExists checks if an account exists.
func (m *MockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.accounts[accountID]; !ok {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

// AccountTenant returns the tenant for an account.
func (m *MockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accounts[accountID]
}

// Compile-time check that MockAccountChecker implements AccountChecker.
var _ AccountChecker = (*MockAccountChecker)(nil)

// MockWalletChecker is a mock implementation of WalletChecker for testing.
type MockWalletChecker struct {
	mu      sync.RWMutex
	wallets map[string]map[string]bool // accountID -> walletAddr -> exists
}

// NewMockWalletChecker creates a new mock wallet checker.
func NewMockWalletChecker() *MockWalletChecker {
	return &MockWalletChecker{
		wallets: make(map[string]map[string]bool),
	}
}

// AddWallet adds a wallet to the mock.
func (m *MockWalletChecker) AddWallet(accountID, walletAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.wallets[accountID] == nil {
		m.wallets[accountID] = make(map[string]bool)
	}
	m.wallets[accountID][walletAddr] = true
}

// WalletOwnedBy checks if a wallet is owned by an account.
func (m *MockWalletChecker) WalletOwnedBy(ctx context.Context, accountID, walletAddr string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if wallets, ok := m.wallets[accountID]; ok {
		if wallets[walletAddr] {
			return nil
		}
	}
	return fmt.Errorf("wallet %s not owned by account %s", walletAddr, accountID)
}

// Compile-time check that MockWalletChecker implements WalletChecker.
var _ WalletChecker = (*MockWalletChecker)(nil)
