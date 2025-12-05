// Package datastreams provides real-time data streaming service.
package datastreams

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages DataStreams service data using Supabase PostgreSQL.
type Store struct {
	mu      sync.RWMutex
	streams *base.SupabaseStore[*DataStream]
	ready   bool
}

// NewStore creates a new DataStreams store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		streams: base.NewSupabaseStore[*DataStream](config, "feed_updates"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		streams: base.NewSupabaseStore[*DataStream](config, "feed_updates"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.streams.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize feed_updates store: %w", err)
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

	if err := s.streams.Close(ctx); err != nil {
		return fmt.Errorf("close feed_updates store: %w", err)
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
	return s.streams.Health(ctx)
}

// CreateStream creates a new stream.
func (s *Store) CreateStream(ctx context.Context, stream *DataStream) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	stream.GenerateID()
	stream.SetTimestamps()
	if stream.Status == "" {
		stream.Status = StreamStatusActive
	}
	return s.streams.Create(ctx, stream)
}

// GetStream gets a stream by ID.
func (s *Store) GetStream(ctx context.Context, id string) (*DataStream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.streams.Get(ctx, id)
}

// ListStreams lists all streams.
func (s *Store) ListStreams(ctx context.Context) ([]*DataStream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.streams.List(ctx)
}
