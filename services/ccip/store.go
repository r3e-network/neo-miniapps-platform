// Package ccip provides Cross-Chain Interoperability Protocol service.
package ccip

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages CCIP service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	messages *base.SupabaseStore[*CrossChainMessage]
	ready    bool
}

// NewStore creates a new CCIP store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		messages: base.NewSupabaseStore[*CrossChainMessage](config, "ccip_requests"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		messages: base.NewSupabaseStore[*CrossChainMessage](config, "ccip_requests"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.messages.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize ccip_requests store: %w", err)
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

	if err := s.messages.Close(ctx); err != nil {
		return fmt.Errorf("close ccip_requests store: %w", err)
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
	return s.messages.Health(ctx)
}

// CreateMessage creates a new message.
func (s *Store) CreateMessage(ctx context.Context, msg *CrossChainMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	msg.GenerateID()
	msg.SetTimestamps()
	if msg.Status == "" {
		msg.Status = MessageStatusPending
	}
	return s.messages.Create(ctx, msg)
}

// GetMessage gets a message by ID.
func (s *Store) GetMessage(ctx context.Context, id string) (*CrossChainMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.messages.Get(ctx, id)
}

// UpdateMessage updates a message.
func (s *Store) UpdateMessage(ctx context.Context, msg *CrossChainMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	msg.SetTimestamps()
	return s.messages.Update(ctx, msg)
}

// ListMessages lists all messages.
func (s *Store) ListMessages(ctx context.Context) ([]*CrossChainMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.messages.List(ctx)
}
