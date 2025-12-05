// Package confidential provides confidential computing service.
package confidential

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages Confidential service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	requests *base.SupabaseStore[*ComputeRequest]
	ready    bool
}

// NewStore creates a new Confidential store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		requests: base.NewSupabaseStore[*ComputeRequest](config, "secrets"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		requests: base.NewSupabaseStore[*ComputeRequest](config, "secrets"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requests.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize secrets store: %w", err)
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

	if err := s.requests.Close(ctx); err != nil {
		return fmt.Errorf("close secrets store: %w", err)
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
	return s.requests.Health(ctx)
}

// CreateRequest creates a new request.
func (s *Store) CreateRequest(ctx context.Context, req *ComputeRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.GenerateID()
	req.SetTimestamps()
	return s.requests.Create(ctx, req)
}

// GetRequest gets a request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*ComputeRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.Get(ctx, id)
}
