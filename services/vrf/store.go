// Package vrf provides Verifiable Random Function service.
package vrf

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/services/base"
)

// Store manages VRF service data using Supabase PostgreSQL.
type Store struct {
	mu       sync.RWMutex
	requests *base.SupabaseStore[*VRFRequest]
	ready    bool
}

// NewStore creates a new VRF store using Supabase PostgreSQL.
func NewStore() *Store {
	config := base.DefaultSupabaseConfig()
	return &Store{
		requests: base.NewSupabaseStore[*VRFRequest](config, "vrf_requests"),
	}
}

// NewStoreWithConfig creates a store with explicit Supabase configuration.
func NewStoreWithConfig(config base.SupabaseConfig) *Store {
	return &Store{
		requests: base.NewSupabaseStore[*VRFRequest](config, "vrf_requests"),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requests.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize vrf_requests store: %w", err)
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
		return fmt.Errorf("close vrf_requests store: %w", err)
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

// CreateRequest creates a new VRF request.
func (s *Store) CreateRequest(ctx context.Context, req *VRFRequest) error {
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

// GetRequest retrieves a VRF request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.Get(ctx, id)
}

// GetRequestByRequestID retrieves a VRF request by request_id.
func (s *Store) GetRequestByRequestID(ctx context.Context, requestID string) (*VRFRequest, error) {
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

// UpdateRequest updates a VRF request.
func (s *Store) UpdateRequest(ctx context.Context, req *VRFRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	req.SetTimestamps()
	return s.requests.Update(ctx, req)
}

// ListRequests lists all VRF requests.
func (s *Store) ListRequests(ctx context.Context) ([]*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.List(ctx)
}

// ListRequestsByUser lists VRF requests for a specific user.
func (s *Store) ListRequestsByUser(ctx context.Context, userID string) ([]*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "user_id=eq."+userID)
}

// ListRequestsByStatus lists VRF requests by status.
func (s *Store) ListRequestsByStatus(ctx context.Context, status RequestStatus) ([]*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "status=eq."+string(status))
}

// ListRequestsByAccount lists VRF requests by account.
func (s *Store) ListRequestsByAccount(ctx context.Context, accountID string) ([]*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	return s.requests.ListWithFilter(ctx, "account_id=eq."+accountID)
}

// GetPendingRequests returns all pending VRF requests.
func (s *Store) GetPendingRequests(ctx context.Context) ([]*VRFRequest, error) {
	return s.ListRequestsByStatus(ctx, RequestStatusPending)
}

// GetStats returns service statistics.
func (s *Store) GetStats(ctx context.Context) (*VRFStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	requests, err := s.requests.List(ctx)
	if err != nil {
		return nil, err
	}

	var fulfilled, pending, failed int64
	for _, req := range requests {
		switch req.Status {
		case RequestStatusFulfilled:
			fulfilled++
		case RequestStatusPending:
			pending++
		case RequestStatusFailed:
			failed++
		}
	}

	return &VRFStats{
		TotalRequests:     int64(len(requests)),
		FulfilledRequests: fulfilled,
		PendingRequests:   pending,
		FailedRequests:    failed,
	}, nil
}

// CountRequests returns the number of VRF requests.
func (s *Store) CountRequests(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}
	return s.requests.Count(ctx)
}
