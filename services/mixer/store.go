// Package mixer provides privacy-preserving transaction mixing service.
package mixer

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages Mixer service data using Supabase PostgreSQL.
type Store struct {
	mu           sync.RWMutex
	requests     *base.SupabaseStore[*MixRequest]
	pools        *base.SupabaseStore[*PoolAccount]
	transactions *base.SupabaseStore[*MixTransaction]
	ready        bool
}

// NewStore creates a new Mixer store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		requests:     base.NewSupabaseStore[*MixRequest](config, "mix_requests"),
		pools:        base.NewSupabaseStore[*PoolAccount](config, "mixer_pools"),
		transactions: base.NewSupabaseStore[*MixTransaction](config, "mix_transactions"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		requests:     base.NewSupabaseStore[*MixRequest](config, "mix_requests"),
		pools:        base.NewSupabaseStore[*PoolAccount](config, "mixer_pools"),
		transactions: base.NewSupabaseStore[*MixTransaction](config, "mix_transactions"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requests.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize requests store: %w", err)
	}
	if err := s.pools.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize pools store: %w", err)
	}
	if err := s.transactions.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize transactions store: %w", err)
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

	s.requests.Close(ctx)
	s.pools.Close(ctx)
	s.transactions.Close(ctx)

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

// CreateRequest creates a new mix request.
func (s *Store) CreateRequest(ctx context.Context, req *MixRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.GenerateID()
	req.RequestID = req.GetID()
	req.SetTimestamps()
	if req.Status == "" {
		req.Status = RequestStatusPending
	}
	return s.requests.Create(ctx, req)
}

// GetRequest retrieves a mix request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.Get(ctx, id)
}

// UpdateRequest updates a mix request.
func (s *Store) UpdateRequest(ctx context.Context, req *MixRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.SetTimestamps()
	return s.requests.Update(ctx, req)
}

// ListRequests lists all mix requests.
func (s *Store) ListRequests(ctx context.Context) ([]*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.List(ctx)
}

// ListRequestsByUser lists requests for a user.
func (s *Store) ListRequestsByUser(ctx context.Context, userID string) ([]*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "user_id=eq."+userID)
}

// ListRequestsByStatus lists requests by status.
func (s *Store) ListRequestsByStatus(ctx context.Context, status RequestStatus) ([]*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "status=eq."+string(status))
}

// =============================================================================
// Pool Operations
// =============================================================================

// CreatePool creates a new pool account.
func (s *Store) CreatePool(ctx context.Context, pool *PoolAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	pool.GenerateID()
	pool.SetTimestamps()
	if pool.Status == "" {
		pool.Status = PoolAccountStatusActive
	}
	return s.pools.Create(ctx, pool)
}

// GetPool retrieves a pool account by ID.
func (s *Store) GetPool(ctx context.Context, id string) (*PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.pools.Get(ctx, id)
}

// UpdatePool updates a pool account.
func (s *Store) UpdatePool(ctx context.Context, pool *PoolAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	pool.SetTimestamps()
	return s.pools.Update(ctx, pool)
}

// ListPools lists all pool accounts.
func (s *Store) ListPools(ctx context.Context) ([]*PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.pools.List(ctx)
}

// ListActivePools lists active pool accounts.
func (s *Store) ListActivePools(ctx context.Context) ([]*PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.pools.ListWithFilter(ctx, "status=eq."+string(PoolAccountStatusActive))
}

// =============================================================================
// Transaction Operations
// =============================================================================

// CreateTransaction creates a new mix transaction.
func (s *Store) CreateTransaction(ctx context.Context, tx *MixTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	tx.GenerateID()
	tx.SetTimestamps()
	return s.transactions.Create(ctx, tx)
}

// GetTransaction retrieves a transaction by ID.
func (s *Store) GetTransaction(ctx context.Context, id string) (*MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.transactions.Get(ctx, id)
}

// UpdateTransaction updates a transaction.
func (s *Store) UpdateTransaction(ctx context.Context, tx *MixTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	tx.SetTimestamps()
	return s.transactions.Update(ctx, tx)
}

// ListTransactionsByRequest lists transactions for a request.
func (s *Store) ListTransactionsByRequest(ctx context.Context, requestID string) ([]*MixTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.transactions.ListWithFilter(ctx, "request_id=eq."+requestID)
}

// =============================================================================
// Statistics
// =============================================================================

// GetStats returns service statistics.
func (s *Store) GetStats(ctx context.Context) (*MixStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	requests, err := s.requests.List(ctx)
	if err != nil {
		return nil, err
	}

	pools, err := s.pools.List(ctx)
	if err != nil {
		return nil, err
	}

	var active, completed int64
	for _, req := range requests {
		switch req.Status {
		case RequestStatusPending, RequestStatusDeposited, RequestStatusMixing:
			active++
		case RequestStatusCompleted:
			completed++
		}
	}

	var activePools int64
	for _, pool := range pools {
		if pool.Status == PoolAccountStatusActive {
			activePools++
		}
	}

	return &MixStats{
		TotalRequests:      int64(len(requests)),
		ActiveRequests:     active,
		CompletedRequests:  completed,
		ActivePoolAccounts: activePools,
	}, nil
}
