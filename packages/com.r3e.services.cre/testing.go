package cre

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu        sync.RWMutex
	playbooks map[string]Playbook
	runs      map[string]Run
	executors map[string]Executor
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		playbooks: make(map[string]Playbook),
		runs:      make(map[string]Run),
		executors: make(map[string]Executor),
	}
}

func (s *MemoryStore) CreatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pb.ID == "" {
		pb.ID = uuid.NewString()
	}
	s.playbooks[pb.ID] = pb
	return pb, nil
}

func (s *MemoryStore) UpdatePlaybook(ctx context.Context, pb Playbook) (Playbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playbooks[pb.ID] = pb
	return pb, nil
}

func (s *MemoryStore) GetPlaybook(ctx context.Context, id string) (Playbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if pb, ok := s.playbooks[id]; ok {
		return pb, nil
	}
	return Playbook{}, nil
}

func (s *MemoryStore) ListPlaybooks(ctx context.Context, accountID string) ([]Playbook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Playbook
	for _, pb := range s.playbooks {
		if pb.AccountID == accountID {
			result = append(result, pb)
		}
	}
	return result, nil
}

func (s *MemoryStore) CreateRun(ctx context.Context, r Run) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	s.runs[r.ID] = r
	return r, nil
}

func (s *MemoryStore) UpdateRun(ctx context.Context, r Run) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[r.ID] = r
	return r, nil
}

func (s *MemoryStore) GetRun(ctx context.Context, id string) (Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.runs[id]; ok {
		return r, nil
	}
	return Run{}, nil
}

func (s *MemoryStore) ListRuns(ctx context.Context, accountID string, limit int) ([]Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Run
	for _, r := range s.runs {
		if r.AccountID == accountID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *MemoryStore) CreateExecutor(ctx context.Context, e Executor) (Executor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	s.executors[e.ID] = e
	return e, nil
}

func (s *MemoryStore) UpdateExecutor(ctx context.Context, e Executor) (Executor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executors[e.ID] = e
	return e, nil
}

func (s *MemoryStore) GetExecutor(ctx context.Context, id string) (Executor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.executors[id]; ok {
		return e, nil
	}
	return Executor{}, nil
}

func (s *MemoryStore) ListExecutors(ctx context.Context, accountID string) ([]Executor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Executor
	for _, e := range s.executors {
		if e.AccountID == accountID {
			result = append(result, e)
		}
	}
	return result, nil
}

// MockAccountChecker is a mock implementation of AccountChecker for testing.
type MockAccountChecker struct {
	accounts map[string]string
}

// NewMockAccountChecker creates a new mock account checker.
func NewMockAccountChecker() *MockAccountChecker {
	return &MockAccountChecker{accounts: make(map[string]string)}
}

// AddAccount adds an account to the mock.
func (m *MockAccountChecker) AddAccount(accountID, tenant string) {
	m.accounts[accountID] = tenant
}

// AccountExists checks if an account exists.
func (m *MockAccountChecker) AccountExists(ctx context.Context, accountID string) error {
	return nil
}

// AccountTenant returns the tenant for an account.
func (m *MockAccountChecker) AccountTenant(ctx context.Context, accountID string) string {
	return m.accounts[accountID]
}
