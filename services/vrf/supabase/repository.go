// Package supabase provides VRF-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

// RepositoryInterface defines VRF-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	Create(ctx context.Context, req *RequestRecord) error
	Update(ctx context.Context, req *RequestRecord) error
	GetByRequestID(ctx context.Context, requestID string) (*RequestRecord, error)
	ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides VRF-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new VRF repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// Create inserts a VRF request.
func (r *Repository) Create(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("vrf request cannot be nil")
	}
	if req.RequestID == "" {
		return fmt.Errorf("request_id cannot be empty")
	}

	data, err := r.base.Request(ctx, "POST", "vrf_requests", req, "")
	if err != nil {
		return fmt.Errorf("create vrf request: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*req = rows[0]
	}
	return nil
}

// Update updates an existing VRF request.
func (r *Repository) Update(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("vrf request cannot be nil")
	}
	if req.RequestID == "" {
		return fmt.Errorf("request_id cannot be empty")
	}

	query := fmt.Sprintf("request_id=eq.%s", req.RequestID)
	_, err := r.base.Request(ctx, "PATCH", "vrf_requests", req, query)
	if err != nil {
		return fmt.Errorf("update vrf request: %w", err)
	}
	return nil
}

// GetByRequestID fetches a VRF request by request_id.
func (r *Repository) GetByRequestID(ctx context.Context, requestID string) (*RequestRecord, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request_id cannot be empty")
	}

	data, err := r.base.Request(ctx, "GET", "vrf_requests", nil, "request_id=eq."+requestID+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("get vrf request: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal vrf requests: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("vrf_request not found: %s", requestID)
	}
	return &rows[0], nil
}

// ListByStatus lists VRF requests by status.
func (r *Repository) ListByStatus(ctx context.Context, status string) ([]RequestRecord, error) {
	validStatuses := map[string]bool{
		"pending":    true,
		"processing": true,
		"fulfilled":  true,
		"failed":     true,
	}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	data, err := r.base.Request(ctx, "GET", "vrf_requests", nil, "status=eq."+status)
	if err != nil {
		return nil, fmt.Errorf("list vrf requests by status: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal vrf requests: %w", err)
	}
	return rows, nil
}
