// Package supabase provides VRF-specific database operations.
package supabase

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

const tableName = "vrf_requests"

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
	return database.GenericCreate(r.base, ctx, tableName, req, func(rows []RequestRecord) {
		if len(rows) > 0 {
			*req = rows[0]
		}
	})
}

// Update updates an existing VRF request.
func (r *Repository) Update(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("vrf request cannot be nil")
	}
	if req.RequestID == "" {
		return fmt.Errorf("request_id cannot be empty")
	}
	return database.GenericUpdate(r.base, ctx, tableName, "request_id", req.RequestID, req)
}

// GetByRequestID fetches a VRF request by request_id.
func (r *Repository) GetByRequestID(ctx context.Context, requestID string) (*RequestRecord, error) {
	return database.GenericGetByField[RequestRecord](r.base, ctx, tableName, "request_id", requestID)
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
	return database.GenericListByField[RequestRecord](r.base, ctx, tableName, "status", status)
}
