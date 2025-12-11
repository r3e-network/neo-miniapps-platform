// Package supabase provides NeoVault-specific database operations.
package supabase

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

// Table names
const (
	tableRequests      = "neovault_requests"
	tableRegistrations = "neovault_registrations"
	tableAuditLog      = "neovault_audit_log"
)

// =============================================================================
// Repository Interface
// =============================================================================

// RepositoryInterface defines NeoVault-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	// Request operations
	Create(ctx context.Context, req *RequestRecord) error
	Update(ctx context.Context, req *RequestRecord) error
	GetByID(ctx context.Context, id string) (*RequestRecord, error)
	GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error)
	ListByUser(ctx context.Context, userID string) ([]RequestRecord, error)
	ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)

	// Registration operations
	CreateRegistration(ctx context.Context, reg *Registration) error
	UpdateRegistration(ctx context.Context, reg *Registration) error
	GetRegistrationByUserID(ctx context.Context, userID string) (*Registration, error)
	GetRegistrationByID(ctx context.Context, id string) (*Registration, error)
	ListRegistrationsByStatus(ctx context.Context, status RegistrationStatus) ([]Registration, error)
	ListPendingRegistrations(ctx context.Context) ([]Registration, error)

	// Audit log operations
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	ListAuditLogsByUser(ctx context.Context, userID string) ([]AuditLog, error)
	ListAuditLogsByEntity(ctx context.Context, entityType, entityID string) ([]AuditLog, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// =============================================================================
// Repository Implementation
// =============================================================================

// Repository provides NeoVault-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new NeoVault repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// =============================================================================
// Request Operations
// =============================================================================

// Create creates a new neovault request.
func (r *Repository) Create(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("neovault request cannot be nil")
	}
	if req.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, tableRequests, req, func(rows []RequestRecord) {
		if len(rows) > 0 {
			*req = rows[0]
		}
	})
}

// Update updates a neovault request by ID.
func (r *Repository) Update(ctx context.Context, req *RequestRecord) error {
	if req == nil {
		return fmt.Errorf("neovault request cannot be nil")
	}
	if req.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	return database.GenericUpdate(r.base, ctx, tableRequests, "id", req.ID, req)
}

// GetByID fetches a neovault request by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*RequestRecord, error) {
	return database.GenericGetByField[RequestRecord](r.base, ctx, tableRequests, "id", id)
}

// GetByDepositAddress fetches a neovault request by deposit address.
func (r *Repository) GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error) {
	return database.GenericGetByField[RequestRecord](r.base, ctx, tableRequests, "deposit_address", addr)
}

// ListByUser lists requests for a user.
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]RequestRecord, error) {
	return database.GenericListByField[RequestRecord](r.base, ctx, tableRequests, "user_id", userID)
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
	return database.GenericListByField[RequestRecord](r.base, ctx, tableRequests, "status", status)
}

// =============================================================================
// Registration Operations
// =============================================================================

// CreateRegistration creates a new registration application.
func (r *Repository) CreateRegistration(ctx context.Context, reg *Registration) error {
	if reg == nil {
		return fmt.Errorf("registration cannot be nil")
	}
	if reg.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if reg.TermsVersion == "" {
		return fmt.Errorf("terms_version cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, tableRegistrations, reg, func(rows []Registration) {
		if len(rows) > 0 {
			*reg = rows[0]
		}
	})
}

// UpdateRegistration updates a registration by ID.
func (r *Repository) UpdateRegistration(ctx context.Context, reg *Registration) error {
	if reg == nil {
		return fmt.Errorf("registration cannot be nil")
	}
	if reg.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	return database.GenericUpdate(r.base, ctx, tableRegistrations, "id", reg.ID, reg)
}

// GetRegistrationByUserID fetches a user's registration.
func (r *Repository) GetRegistrationByUserID(ctx context.Context, userID string) (*Registration, error) {
	return database.GenericGetByField[Registration](r.base, ctx, tableRegistrations, "user_id", userID)
}

// GetRegistrationByID fetches a registration by ID.
func (r *Repository) GetRegistrationByID(ctx context.Context, id string) (*Registration, error) {
	return database.GenericGetByField[Registration](r.base, ctx, tableRegistrations, "id", id)
}

// ListRegistrationsByStatus lists registrations with a specific status.
func (r *Repository) ListRegistrationsByStatus(ctx context.Context, status RegistrationStatus) ([]Registration, error) {
	return database.GenericListByField[Registration](r.base, ctx, tableRegistrations, "status", string(status))
}

// ListPendingRegistrations lists all pending registration applications.
func (r *Repository) ListPendingRegistrations(ctx context.Context) ([]Registration, error) {
	query := database.NewQuery().
		Eq("status", string(RegStatusPending)).
		OrderAsc("created_at").
		Build()
	return database.GenericListWithQuery[Registration](r.base, ctx, tableRegistrations, query)
}

// =============================================================================
// Audit Log Operations
// =============================================================================

// CreateAuditLog creates an immutable audit log entry.
func (r *Repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	if log == nil {
		return fmt.Errorf("audit log cannot be nil")
	}
	if log.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}
	if log.EntityType == "" {
		return fmt.Errorf("entity_type cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, tableAuditLog, log, func(rows []AuditLog) {
		if len(rows) > 0 {
			*log = rows[0]
		}
	})
}

// ListAuditLogsByUser lists audit logs for a specific user.
func (r *Repository) ListAuditLogsByUser(ctx context.Context, userID string) ([]AuditLog, error) {
	query := database.NewQuery().
		Eq("user_id", userID).
		OrderDesc("created_at").
		Limit(100).
		Build()
	return database.GenericListWithQuery[AuditLog](r.base, ctx, tableAuditLog, query)
}

// ListAuditLogsByEntity lists audit logs for a specific entity.
func (r *Repository) ListAuditLogsByEntity(ctx context.Context, entityType, entityID string) ([]AuditLog, error) {
	if entityType == "" || entityID == "" {
		return nil, fmt.Errorf("entity_type and entity_id cannot be empty")
	}
	query := database.NewQuery().
		Eq("entity_type", entityType).
		Eq("entity_id", entityID).
		OrderDesc("created_at").
		Build()
	return database.GenericListWithQuery[AuditLog](r.base, ctx, tableAuditLog, query)
}
