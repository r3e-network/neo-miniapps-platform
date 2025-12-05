// Package datafeeds provides data feed aggregation service.
package datafeeds

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages DataFeeds service data using Supabase PostgreSQL.
type Store struct {
	mu     sync.RWMutex
	prices *base.SupabaseStore[*PriceData]
	feeds  *base.SupabaseStore[*Feed]
	ready  bool
}

// NewStore creates a new store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		prices: base.NewSupabaseStore[*PriceData](config, "feed_updates"),
		feeds:  base.NewSupabaseStore[*Feed](config, "datafeeds"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		prices: base.NewSupabaseStore[*PriceData](config, "feed_updates"),
		feeds:  base.NewSupabaseStore[*Feed](config, "datafeeds"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.prices.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize feed_updates store: %w", err)
	}
	if err := s.feeds.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize datafeeds store: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices.Close(ctx)
	s.feeds.Close(ctx)
	s.ready = false
	return nil
}

// Shutdown shuts down the store (alias for Close for Component interface).
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
	return s.prices.Health(ctx)
}

// SavePrice saves price data.
func (s *Store) SavePrice(ctx context.Context, data *PriceData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	// Use symbol as ID for latest price
	data.ID = data.Symbol
	if data.Source == "" {
		data.Source = "unknown"
	}
	data.SetTimestamps()

	existing, _ := s.prices.Get(ctx, data.ID)
	if existing != nil {
		return s.prices.Update(ctx, data)
	}
	return s.prices.Create(ctx, data)
}

// GetLatestPrice gets the latest price for a symbol.
func (s *Store) GetLatestPrice(ctx context.Context, symbol string) (*PriceData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.prices.Get(ctx, symbol)
}

// ListPrices lists all prices.
func (s *Store) ListPrices(ctx context.Context) ([]*PriceData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.prices.List(ctx)
}

// CreateFeed creates a new feed configuration.
func (s *Store) CreateFeed(ctx context.Context, feed *Feed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	feed.GenerateID()
	if feed.Status == "" {
		feed.Status = FeedStatusActive
	}
	feed.SetTimestamps()
	return s.feeds.Create(ctx, feed)
}

// GetFeed gets a feed by ID.
func (s *Store) GetFeed(ctx context.Context, id string) (*Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.feeds.Get(ctx, id)
}

// ListFeeds lists all feeds.
func (s *Store) ListFeeds(ctx context.Context) ([]*Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.feeds.List(ctx)
}
