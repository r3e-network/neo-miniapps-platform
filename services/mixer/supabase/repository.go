// Package supabase provides Mixer-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

// RepositoryInterface defines Mixer-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	Create(ctx context.Context, req *RequestRecord) error
	Update(ctx context.Context, req *RequestRecord) error
	GetByID(ctx context.Context, id string) (*RequestRecord, error)
	GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error)
	ListByUser(ctx context.Context, userID string) ([]RequestRecord, error)
	ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides Mixer-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new Mixer repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// Create creates a new mixer request.
func (r *Repository) Create(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("mixer request cannot be nil")
	}
	if req.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	data, err := r.base.Request(ctx, "POST", "mixer_requests", req, "")
	if err != nil {
		return fmt.Errorf("create mixer request: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*req = rows[0]
	}
	return nil
}

// Update updates a mixer request by ID.
func (r *Repository) Update(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("mixer request cannot be nil")
	}
	if req.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}

	query := fmt.Sprintf("id=eq.%s", req.ID)
	_, err := r.base.Request(ctx, "PATCH", "mixer_requests", req, query)
	if err != nil {
		return fmt.Errorf("update mixer request: %w", err)
	}
	return nil
}

// GetByID fetches a mixer request by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*RequestRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	data, err := r.base.Request(ctx, "GET", "mixer_requests", nil, "id=eq."+id+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("get mixer request: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal mixer requests: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("mixer_request not found: %s", id)
	}
	return &rows[0], nil
}

// GetByDepositAddress fetches a mixer request by deposit address.
func (r *Repository) GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error) {
	if addr == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	data, err := r.base.Request(ctx, "GET", "mixer_requests", nil, "deposit_address=eq."+addr+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("get mixer request by deposit address: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal mixer requests: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("mixer_request not found: %s", addr)
	}
	return &rows[0], nil
}

// ListByUser lists requests for a user.
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]RequestRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id cannot be empty")
	}

	data, err := r.base.Request(ctx, "GET", "mixer_requests", nil, "user_id=eq."+userID)
	if err != nil {
		return nil, fmt.Errorf("list mixer requests by user: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal mixer requests: %w", err)
	}
	return rows, nil
}

// ListByStatus lists requests with a specific status.
func (r *Repository) ListByStatus(ctx context.Context, status string) ([]RequestRecord, error) {
	validStatuses := map[string]bool{
		"pending":   true,
		"deposited": true,
		"mixing":    true,
		"delivered": true,
		"failed":    true,
		"refunded":  true,
	}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	data, err := r.base.Request(ctx, "GET", "mixer_requests", nil, "status=eq."+status)
	if err != nil {
		return nil, fmt.Errorf("list mixer requests by status: %w", err)
	}
	var rows []RequestRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal mixer requests: %w", err)
	}
	return rows, nil
}
