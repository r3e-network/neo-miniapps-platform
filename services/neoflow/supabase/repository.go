// Package supabase provides NeoFlow-specific database operations.
package supabase

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

const (
	triggersTable   = "neoflow_triggers"
	executionsTable = "neoflow_executions"
)

// RepositoryInterface defines NeoFlow-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	// Trigger Operations
	GetTriggers(ctx context.Context, userID string) ([]Trigger, error)
	GetTrigger(ctx context.Context, id, userID string) (*Trigger, error)
	CreateTrigger(ctx context.Context, trigger *Trigger) error
	UpdateTrigger(ctx context.Context, trigger *Trigger) error
	DeleteTrigger(ctx context.Context, id, userID string) error
	SetTriggerEnabled(ctx context.Context, id, userID string, enabled bool) error
	GetPendingTriggers(ctx context.Context) ([]Trigger, error)
	// Execution Operations
	CreateExecution(ctx context.Context, exec *Execution) error
	GetExecutions(ctx context.Context, triggerID string, limit int) ([]Execution, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides NeoFlow-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new NeoFlow repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// =============================================================================
// Trigger Operations
// =============================================================================

// GetTriggers retrieves neoflow triggers for a user.
func (r *Repository) GetTriggers(ctx context.Context, userID string) ([]Trigger, error) {
	return database.GenericListByField[Trigger](r.base, ctx, triggersTable, "user_id", userID)
}

// GetTrigger returns a trigger by id scoped to a user.
func (r *Repository) GetTrigger(ctx context.Context, id, userID string) (*Trigger, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("id and user_id cannot be empty")
	}

	query := database.NewQuery().
		Eq("id", id).
		Eq("user_id", userID).
		Limit(1).
		Build()

	rows, err := database.GenericListWithQuery[Trigger](r.base, ctx, triggersTable, query)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, database.NewNotFoundError(triggersTable, id)
	}
	return &rows[0], nil
}

// CreateTrigger inserts a new neoflow trigger.
func (r *Repository) CreateTrigger(ctx context.Context, trigger *Trigger) error {
	if trigger == nil {
		return fmt.Errorf("trigger cannot be nil")
	}
	if trigger.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, triggersTable, trigger, func(rows []Trigger) {
		if len(rows) > 0 {
			*trigger = rows[0]
		}
	})
}

// UpdateTrigger updates a trigger by id.
func (r *Repository) UpdateTrigger(ctx context.Context, trigger *Trigger) error {
	if trigger == nil {
		return fmt.Errorf("trigger cannot be nil")
	}
	if trigger.ID == "" || trigger.UserID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	query := database.NewQuery().
		Eq("id", trigger.ID).
		Eq("user_id", trigger.UserID).
		Build()

	_, err := r.base.Request(ctx, "PATCH", triggersTable, trigger, query)
	if err != nil {
		return fmt.Errorf("update neoflow trigger: %w", err)
	}
	return nil
}

// DeleteTrigger removes a trigger.
func (r *Repository) DeleteTrigger(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	query := database.NewQuery().
		Eq("id", id).
		Eq("user_id", userID).
		Build()

	_, err := r.base.Request(ctx, "DELETE", triggersTable, nil, query)
	if err != nil {
		return fmt.Errorf("delete neoflow trigger: %w", err)
	}
	return nil
}

// SetTriggerEnabled sets enabled flag.
func (r *Repository) SetTriggerEnabled(ctx context.Context, id, userID string, enabled bool) error {
	if id == "" || userID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	update := map[string]interface{}{"enabled": enabled}
	if enabled {
		update["next_execution"] = time.Now()
	}

	query := database.NewQuery().
		Eq("id", id).
		Eq("user_id", userID).
		Build()

	_, err := r.base.Request(ctx, "PATCH", triggersTable, update, query)
	if err != nil {
		return fmt.Errorf("set neoflow trigger enabled: %w", err)
	}
	return nil
}

// GetPendingTriggers retrieves triggers that need execution.
func (r *Repository) GetPendingTriggers(ctx context.Context) ([]Trigger, error) {
	now := time.Now().Format(time.RFC3339)

	query := database.NewQuery().
		IsTrue("enabled").
		Lte("next_execution", now).
		Build()

	return database.GenericListWithQuery[Trigger](r.base, ctx, triggersTable, query)
}

// =============================================================================
// Execution Operations
// =============================================================================

// CreateExecution logs a trigger execution.
func (r *Repository) CreateExecution(ctx context.Context, exec *Execution) error {
	if exec == nil {
		return fmt.Errorf("execution cannot be nil")
	}
	if exec.TriggerID == "" {
		return fmt.Errorf("trigger_id cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, executionsTable, exec, func(rows []Execution) {
		if len(rows) > 0 {
			*exec = rows[0]
		}
	})
}

// GetExecutions lists executions for a trigger.
func (r *Repository) GetExecutions(ctx context.Context, triggerID string, limit int) ([]Execution, error) {
	if triggerID == "" {
		return nil, fmt.Errorf("trigger_id cannot be empty")
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	query := database.NewQuery().
		Eq("trigger_id", triggerID).
		OrderDesc("executed_at").
		Limit(limit).
		Build()

	return database.GenericListWithQuery[Execution](r.base, ctx, executionsTable, query)
}
