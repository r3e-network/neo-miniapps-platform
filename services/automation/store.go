// Package automation provides task automation service.
package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages Automation service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	tasks    *base.SupabaseStore[*Task]
	triggers *base.SupabaseStore[*ContractTrigger]
	ready    bool
}

// NewStore creates a new store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		tasks:    base.NewSupabaseStore[*Task](config, "automation_triggers"),
		triggers: base.NewSupabaseStore[*ContractTrigger](config, "automation_executions"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		tasks:    base.NewSupabaseStore[*Task](config, "automation_triggers"),
		triggers: base.NewSupabaseStore[*ContractTrigger](config, "automation_executions"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.tasks.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize tasks store: %w", err)
	}
	if err := s.triggers.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize triggers store: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks.Close(ctx)
	s.triggers.Close(ctx)
	s.ready = false
	return nil
}

// Shutdown shuts down the store (alias for Close).
func (s *Store) Shutdown(ctx context.Context) error {
	return s.Close(ctx)
}

// Health checks store health.
func (s *Store) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.tasks.Health(ctx)
}

// CreateTask creates a new task.
func (s *Store) CreateTask(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	task.GenerateID()
	task.SetTimestamps()
	return s.tasks.Create(ctx, task)
}

// GetTask gets a task by ID.
func (s *Store) GetTask(ctx context.Context, id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.tasks.Get(ctx, id)
}

// UpdateTask updates a task.
func (s *Store) UpdateTask(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	task.SetTimestamps()
	return s.tasks.Update(ctx, task)
}

// ListTasks lists all tasks.
func (s *Store) ListTasks(ctx context.Context) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.tasks.List(ctx)
}

// ListTasksByAccount lists tasks for an account.
func (s *Store) ListTasksByAccount(ctx context.Context, accountID string) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.tasks.ListWithFilter(ctx, "account_id=eq."+accountID)
}

// =============================================================================
// Contract Trigger Storage
// =============================================================================

// CreateContractTrigger creates a new contract trigger.
func (s *Store) CreateContractTrigger(ctx context.Context, trigger *ContractTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	trigger.GenerateID()
	trigger.SetTimestamps()
	return s.triggers.Create(ctx, trigger)
}

// GetContractTrigger gets a contract trigger by ID.
func (s *Store) GetContractTrigger(ctx context.Context, id string) (*ContractTrigger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.triggers.Get(ctx, id)
}

// UpdateContractTrigger updates a contract trigger.
func (s *Store) UpdateContractTrigger(ctx context.Context, trigger *ContractTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	trigger.SetTimestamps()
	return s.triggers.Update(ctx, trigger)
}

// GetDueContractTriggers returns all active triggers that are due for execution.
func (s *Store) GetDueContractTriggers(ctx context.Context, now time.Time) ([]*ContractTrigger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	// Get all triggers and filter in memory for now
	// TODO: Use Supabase filter when time comparison is supported
	all, err := s.triggers.List(ctx)
	if err != nil {
		return nil, err
	}

	var due []*ContractTrigger
	for _, trigger := range all {
		if trigger.Active && now.After(trigger.NextExecution) {
			due = append(due, trigger)
		}
	}
	return due, nil
}
