// Package cre provides Chainlink Runtime Environment service.
package cre

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages CRE service data using Supabase PostgreSQL.
type Store struct {
	mu        sync.RWMutex
	workflows *base.SupabaseStore[*Workflow]
	ready     bool
}

// NewStore creates a new CRE store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		workflows: base.NewSupabaseStore[*Workflow](config, "automation_executions"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		workflows: base.NewSupabaseStore[*Workflow](config, "automation_executions"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.workflows.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize automation_executions store: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Shutdown shuts down the store (implements base.Component interface).
func (s *Store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.workflows.Close(ctx); err != nil {
		return fmt.Errorf("close automation_executions store: %w", err)
	}

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
	return s.workflows.Health(ctx)
}

// CreateWorkflow creates a new workflow.
func (s *Store) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	workflow.GenerateID()
	workflow.SetTimestamps()
	if workflow.Status == "" {
		workflow.Status = "pending"
	}
	return s.workflows.Create(ctx, workflow)
}

// GetWorkflow gets a workflow by ID.
func (s *Store) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.workflows.Get(ctx, id)
}
