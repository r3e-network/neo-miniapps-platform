package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

// NewMemoryStore creates a new in-memory store for testing.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]Job),
	}
}

// CreateAutomationJob creates a new job.
func (s *MemoryStore) CreateAutomationJob(ctx context.Context, job Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	s.jobs[job.ID] = job
	return job, nil
}

// UpdateAutomationJob updates an existing job.
func (s *MemoryStore) UpdateAutomationJob(ctx context.Context, job Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.jobs[job.ID]
	if !ok {
		return Job{}, fmt.Errorf("job not found: %s", job.ID)
	}

	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now().UTC()
	s.jobs[job.ID] = job
	return job, nil
}

// GetAutomationJob retrieves a job by ID.
func (s *MemoryStore) GetAutomationJob(ctx context.Context, id string) (Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("job not found: %s", id)
	}
	return job, nil
}

// ListAutomationJobs returns all jobs for an account.
func (s *MemoryStore) ListAutomationJobs(ctx context.Context, accountID string) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Job
	for _, job := range s.jobs {
		if job.AccountID == accountID {
			result = append(result, job)
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
