// Package datalink provides data linking service.
package datalink

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages DataLink service data using Supabase PostgreSQL.
type Store struct {
	mu    sync.RWMutex
	links *base.SupabaseStore[*DataLink]
	ready bool
}

// NewStore creates a new DataLink store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		links: base.NewSupabaseStore[*DataLink](config, "oracle_requests"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		links: base.NewSupabaseStore[*DataLink](config, "oracle_requests"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.links.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize oracle_requests store: %w", err)
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

	if err := s.links.Close(ctx); err != nil {
		return fmt.Errorf("close oracle_requests store: %w", err)
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
	return s.links.Health(ctx)
}

// CreateLink creates a new link.
func (s *Store) CreateLink(ctx context.Context, link *DataLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	link.GenerateID()
	link.SetTimestamps()
	if link.Status == "" {
		link.Status = LinkStatusActive
	}
	return s.links.Create(ctx, link)
}

// GetLink gets a link by ID.
func (s *Store) GetLink(ctx context.Context, id string) (*DataLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.links.Get(ctx, id)
}

// ListLinks lists all links.
func (s *Store) ListLinks(ctx context.Context) ([]*DataLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.links.List(ctx)
}
