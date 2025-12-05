// Package oracle provides the Oracle service.
package oracle

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages Oracle service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	requests *base.SupabaseStore[*OracleRequest]
	feeds    *base.SupabaseStore[*DataFeed]
	ready    bool
}

// OracleRequest represents an oracle request stored in Supabase.
type OracleRequest struct {
	base.BaseEntity
	UserID           string                 `json:"user_id,omitempty"`
	AccountID        string                 `json:"account_id,omitempty"`
	RequestID        string                 `json:"request_id"`
	URL              string                 `json:"url"`
	Method           string                 `json:"method"`
	Headers          map[string]string      `json:"headers,omitempty"`
	Body             string                 `json:"body,omitempty"`
	CallbackContract string                 `json:"callback_contract,omitempty"`
	CallbackMethod   string                 `json:"callback_method,omitempty"`
	Status           string                 `json:"status"`
	Result           map[string]interface{} `json:"result,omitempty"`
	Error            string                 `json:"error,omitempty"`
	GasUsed          float64                `json:"gas_used"`
	ProcessedAt      *string                `json:"processed_at,omitempty"`
}

// NewStore creates a new Oracle store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		requests: base.NewSupabaseStore[*OracleRequest](config, "oracle_requests"),
		feeds:    base.NewSupabaseStore[*DataFeed](config, "datafeeds"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		requests: base.NewSupabaseStore[*OracleRequest](config, "oracle_requests"),
		feeds:    base.NewSupabaseStore[*DataFeed](config, "datafeeds"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requests.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize oracle_requests store: %w", err)
	}
	if err := s.feeds.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize datafeeds store: %w", err)
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
	return s.requests.Health(ctx)
}

// =============================================================================
// Request Operations
// =============================================================================

// CreateRequest creates a new oracle request.
func (s *Store) CreateRequest(ctx context.Context, req *OracleRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.GenerateID()
	req.RequestID = req.GetID()
	req.SetTimestamps()
	if req.Status == "" {
		req.Status = "pending"
	}
	return s.requests.Create(ctx, req)
}

// GetRequest retrieves an oracle request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.Get(ctx, id)
}

// GetRequestByRequestID retrieves an oracle request by request_id.
func (s *Store) GetRequestByRequestID(ctx context.Context, requestID string) (*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	requests, err := s.requests.ListWithFilter(ctx, "request_id=eq."+requestID+"&limit=1")
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, fmt.Errorf("request not found: %s", requestID)
	}
	return requests[0], nil
}

// UpdateRequest updates an oracle request.
func (s *Store) UpdateRequest(ctx context.Context, req *OracleRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.SetTimestamps()
	return s.requests.Update(ctx, req)
}

// DeleteRequest deletes an oracle request.
func (s *Store) DeleteRequest(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.requests.Delete(ctx, id)
}

// ListRequests lists all oracle requests.
func (s *Store) ListRequests(ctx context.Context) ([]*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.List(ctx)
}

// ListRequestsByUser lists oracle requests by user ID.
func (s *Store) ListRequestsByUser(ctx context.Context, userID string) ([]*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "user_id=eq."+userID)
}

// ListRequestsByStatus lists oracle requests by status.
func (s *Store) ListRequestsByStatus(ctx context.Context, status string) ([]*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "status=eq."+status)
}

// ListRequestsByAccount lists oracle requests by account.
func (s *Store) ListRequestsByAccount(ctx context.Context, accountID string) ([]*OracleRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "account_id=eq."+accountID)
}

// CountRequests returns the number of oracle requests.
func (s *Store) CountRequests(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}
	return s.requests.Count(ctx)
}

// GetPendingRequests returns all pending oracle requests.
func (s *Store) GetPendingRequests(ctx context.Context) ([]*OracleRequest, error) {
	return s.ListRequestsByStatus(ctx, "pending")
}

// =============================================================================
// Feed Operations
// =============================================================================

// CreateFeed creates a new data feed.
func (s *Store) CreateFeed(ctx context.Context, feed *DataFeed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	feed.GenerateID()
	feed.SetTimestamps()
	return s.feeds.Create(ctx, feed)
}

// GetFeed retrieves a data feed by ID.
func (s *Store) GetFeed(ctx context.Context, id string) (*DataFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.feeds.Get(ctx, id)
}

// UpdateFeed updates a data feed.
func (s *Store) UpdateFeed(ctx context.Context, feed *DataFeed) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	feed.SetTimestamps()
	return s.feeds.Update(ctx, feed)
}

// DeleteFeed deletes a data feed.
func (s *Store) DeleteFeed(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return s.feeds.Delete(ctx, id)
}

// ListFeeds lists all data feeds.
func (s *Store) ListFeeds(ctx context.Context) ([]*DataFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.feeds.List(ctx)
}

// CountFeeds returns the number of feeds.
func (s *Store) CountFeeds(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}
	return s.feeds.Count(ctx)
}
