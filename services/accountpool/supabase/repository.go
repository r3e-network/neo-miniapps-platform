// Package supabase provides AccountPool-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

// RepositoryInterface defines AccountPool-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	Create(ctx context.Context, acc *Account) error
	Update(ctx context.Context, acc *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	List(ctx context.Context) ([]Account, error)
	ListAvailable(ctx context.Context, limit int) ([]Account, error)
	ListByLocker(ctx context.Context, lockerID string) ([]Account, error)
	Delete(ctx context.Context, id string) error
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides AccountPool-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new AccountPool repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// Create inserts a new pool account.
func (r *Repository) Create(ctx context.Context, acc *Account) error {
	if acc == nil {
		return fmt.Errorf("pool account cannot be nil")
	}

	data, err := r.base.Request(ctx, "POST", "pool_accounts", acc, "")
	if err != nil {
		return fmt.Errorf("create pool account: %w", err)
	}
	var rows []Account
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*acc = rows[0]
	}
	return nil
}

// Update updates a pool account by ID.
func (r *Repository) Update(ctx context.Context, acc *Account) error {
	if acc == nil {
		return fmt.Errorf("pool account cannot be nil")
	}
	if acc.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}

	query := fmt.Sprintf("id=eq.%s", acc.ID)
	_, err := r.base.Request(ctx, "PATCH", "pool_accounts", acc, query)
	if err != nil {
		return fmt.Errorf("update pool account: %w", err)
	}
	return nil
}

// GetByID fetches a pool account by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Account, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	data, err := r.base.Request(ctx, "GET", "pool_accounts", nil, "id=eq."+id+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("get pool account: %w", err)
	}
	var rows []Account
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal pool accounts: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("pool_account not found: %s", id)
	}
	return &rows[0], nil
}

// List returns all pool accounts.
func (r *Repository) List(ctx context.Context) ([]Account, error) {
	data, err := r.base.Request(ctx, "GET", "pool_accounts", nil, "")
	if err != nil {
		return nil, fmt.Errorf("list pool accounts: %w", err)
	}
	var rows []Account
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal pool accounts: %w", err)
	}
	return rows, nil
}

// ListAvailable returns unlocked, non-retiring accounts up to limit.
func (r *Repository) ListAvailable(ctx context.Context, limit int) ([]Account, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := fmt.Sprintf("is_retiring=eq.false&locked_by=is.null&order=last_used_at.asc&limit=%d", limit)
	data, err := r.base.Request(ctx, "GET", "pool_accounts", nil, query)
	if err != nil {
		return nil, fmt.Errorf("list available pool accounts: %w", err)
	}
	var rows []Account
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal pool accounts: %w", err)
	}
	return rows, nil
}

// ListByLocker returns accounts locked by a specific service.
func (r *Repository) ListByLocker(ctx context.Context, lockerID string) ([]Account, error) {
	if lockerID == "" {
		return nil, fmt.Errorf("locker_id cannot be empty")
	}

	query := fmt.Sprintf("locked_by=eq.%s", lockerID)
	data, err := r.base.Request(ctx, "GET", "pool_accounts", nil, query)
	if err != nil {
		return nil, fmt.Errorf("list pool accounts by locker: %w", err)
	}
	var rows []Account
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal pool accounts: %w", err)
	}
	return rows, nil
}

// Delete deletes a pool account by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	_, err := r.base.Request(ctx, "DELETE", "pool_accounts", nil, "id=eq."+id)
	if err != nil {
		return fmt.Errorf("delete pool account: %w", err)
	}
	return nil
}
