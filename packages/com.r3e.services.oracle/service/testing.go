package oracle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore provides an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu       sync.RWMutex
	sources  map[string]DataSource
	requests map[string]Request
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sources:  make(map[string]DataSource),
		requests: make(map[string]Request),
	}
}

// DataSource operations

func (s *MemoryStore) CreateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	src.ID = uuid.New().String()
	now := time.Now().UTC()
	src.CreatedAt = now
	src.UpdatedAt = now
	s.sources[src.ID] = src
	return src, nil
}

func (s *MemoryStore) UpdateDataSource(ctx context.Context, src DataSource) (DataSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sources[src.ID]; !ok {
		return DataSource{}, fmt.Errorf("data source not found: %s", src.ID)
	}
	src.UpdatedAt = time.Now().UTC()
	s.sources[src.ID] = src
	return src, nil
}

func (s *MemoryStore) GetDataSource(ctx context.Context, id string) (DataSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src, ok := s.sources[id]
	if !ok {
		return DataSource{}, fmt.Errorf("data source not found: %s", id)
	}
	return src, nil
}

func (s *MemoryStore) ListDataSources(ctx context.Context, accountID string) ([]DataSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []DataSource
	for _, src := range s.sources {
		if accountID == "" || src.AccountID == accountID {
			result = append(result, src)
		}
	}
	return result, nil
}

// Request operations

func (s *MemoryStore) CreateRequest(ctx context.Context, req Request) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req.ID = uuid.New().String()
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	s.requests[req.ID] = req
	return req, nil
}

func (s *MemoryStore) UpdateRequest(ctx context.Context, req Request) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.requests[req.ID]; !ok {
		return Request{}, fmt.Errorf("request not found: %s", req.ID)
	}
	req.UpdatedAt = time.Now().UTC()
	s.requests[req.ID] = req
	return req, nil
}

func (s *MemoryStore) GetRequest(ctx context.Context, id string) (Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return Request{}, fmt.Errorf("request not found: %s", id)
	}
	return req, nil
}

func (s *MemoryStore) ListRequests(ctx context.Context, accountID string, limit int, status string) ([]Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Request
	for _, req := range s.requests {
		if accountID != "" && req.AccountID != accountID {
			continue
		}
		if status != "" && string(req.Status) != status {
			continue
		}
		result = append(result, req)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *MemoryStore) ListPendingRequests(ctx context.Context) ([]Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Request
	for _, req := range s.requests {
		if req.Status == StatusPending {
			result = append(result, req)
		}
	}
	return result, nil
}

// MockAccountChecker provides a mock account checker for testing.
type MockAccountChecker struct {
	accounts map[string]string // accountID -> tenant
}

// NewMockAccountChecker creates a new mock account checker.
func NewMockAccountChecker() *MockAccountChecker {
	return &MockAccountChecker{
		accounts: make(map[string]string),
	}
}

// AddAccount adds an account to the mock.
func (m *MockAccountChecker) AddAccount(accountID string) {
	m.accounts[accountID] = ""
}

// AddAccountWithTenant adds an account with a tenant to the mock.
func (m *MockAccountChecker) AddAccountWithTenant(accountID, tenant string) {
	m.accounts[accountID] = tenant
}

// AccountExists checks if an account exists.
func (m *MockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	if _, ok := m.accounts[accountID]; !ok {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

// AccountTenant returns the tenant for an account.
func (m *MockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	return m.accounts[accountID]
}

// MockFeeCollector provides a mock fee collector for testing.
type MockFeeCollector struct {
	mu       sync.Mutex
	charges  []FeeCharge
	balances map[string]int64
}

// FeeCharge records a fee charge.
type FeeCharge struct {
	AccountID string
	Amount    int64
	Reason    string
}

// NewMockFeeCollector creates a new mock fee collector.
func NewMockFeeCollector() *MockFeeCollector {
	return &MockFeeCollector{
		charges:  make([]FeeCharge, 0),
		balances: make(map[string]int64),
	}
}

// SetBalance sets the balance for an account.
func (m *MockFeeCollector) SetBalance(accountID string, balance int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[accountID] = balance
}

// ChargeFee charges a fee to an account.
func (m *MockFeeCollector) ChargeFee(ctx context.Context, accountID string, amount int64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	balance := m.balances[accountID]
	if balance < amount {
		return fmt.Errorf("insufficient balance: have %d, need %d", balance, amount)
	}
	m.balances[accountID] = balance - amount
	m.charges = append(m.charges, FeeCharge{
		AccountID: accountID,
		Amount:    amount,
		Reason:    reason,
	})
	return nil
}

// GetCharges returns all recorded charges.
func (m *MockFeeCollector) GetCharges() []FeeCharge {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]FeeCharge{}, m.charges...)
}
