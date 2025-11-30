package functions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu         sync.RWMutex
	functions  map[string]Definition
	executions map[string]Execution
	accounts   map[string]mockAccount
	accountSeq int
}

// mockAccount represents a user account for testing.
type mockAccount struct {
	ID    string
	Owner string
}

// NewMemoryStore creates a new in-memory store for testing.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		functions:  make(map[string]Definition),
		executions: make(map[string]Execution),
		accounts:   make(map[string]mockAccount),
	}
}

// CreateAccount creates a mock user account (simulates accounts service).
func (s *MemoryStore) CreateAccount(_ context.Context, owner string) (mockAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountSeq++
	acct := mockAccount{
		ID:    fmt.Sprintf("acct-%d", s.accountSeq),
		Owner: owner,
	}
	s.accounts[acct.ID] = acct
	return acct, nil
}

// AccountExists implements AccountChecker interface.
func (s *MemoryStore) AccountExists(_ context.Context, accountID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.accounts[accountID]; !ok {
		return fmt.Errorf("account not found: %s", accountID)
	}
	return nil
}

// AccountTenant implements AccountChecker interface.
func (s *MemoryStore) AccountTenant(_ context.Context, _ string) string {
	return ""
}

// CreateFunction creates a new function definition.
func (s *MemoryStore) CreateFunction(_ context.Context, def Definition) (Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if def.ID == "" {
		def.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	def.CreatedAt = now
	def.UpdatedAt = now

	s.functions[def.ID] = def
	return def, nil
}

// UpdateFunction updates an existing function definition.
func (s *MemoryStore) UpdateFunction(_ context.Context, def Definition) (Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.functions[def.ID]
	if !ok {
		return Definition{}, fmt.Errorf("function not found: %s", def.ID)
	}

	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC()
	s.functions[def.ID] = def
	return def, nil
}

// GetFunction retrieves a function definition by ID.
func (s *MemoryStore) GetFunction(_ context.Context, id string) (Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, ok := s.functions[id]
	if !ok {
		return Definition{}, fmt.Errorf("function not found: %s", id)
	}
	return def, nil
}

// ListFunctions returns all functions for an account.
func (s *MemoryStore) ListFunctions(_ context.Context, accountID string) ([]Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Definition
	for _, def := range s.functions {
		if def.AccountID == accountID {
			result = append(result, def)
		}
	}
	return result, nil
}

// CreateExecution creates a new execution record.
func (s *MemoryStore) CreateExecution(_ context.Context, exec Execution) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exec.ID == "" {
		exec.ID = uuid.NewString()
	}

	s.executions[exec.ID] = exec
	return exec, nil
}

// GetExecution retrieves an execution by ID.
func (s *MemoryStore) GetExecution(_ context.Context, id string) (Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exec, ok := s.executions[id]
	if !ok {
		return Execution{}, fmt.Errorf("execution not found: %s", id)
	}
	return exec, nil
}

// ListFunctionExecutions returns executions for a function.
func (s *MemoryStore) ListFunctionExecutions(_ context.Context, functionID string, limit int) ([]Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Execution
	for _, exec := range s.executions {
		if exec.FunctionID == functionID {
			result = append(result, exec)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// staticExecutor is a test executor that returns a predefined result.
type staticExecutor struct {
	result ExecutionResult
	err    error
}

func (e *staticExecutor) Execute(_ context.Context, _ Definition, _ map[string]any) (ExecutionResult, error) {
	return e.result, e.err
}

// Compile-time checks
var _ Store = (*MemoryStore)(nil)
var _ AccountChecker = (*MemoryStore)(nil)
var _ FunctionExecutor = (*staticExecutor)(nil)
