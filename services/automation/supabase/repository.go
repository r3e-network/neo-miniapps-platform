// Package supabase provides Automation-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

// RepositoryInterface defines Automation-specific data access methods.
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

// Repository provides Automation-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new Automation repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// =============================================================================
// Trigger Operations
// =============================================================================

// GetTriggers retrieves automation triggers for a user.
func (r *Repository) GetTriggers(ctx context.Context, userID string) ([]Trigger, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id cannot be empty")
	}

	query := "user_id=eq." + userID
	data, err := r.base.Request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, fmt.Errorf("get automation triggers: %w", err)
	}

	var triggers []Trigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, fmt.Errorf("unmarshal automation triggers: %w", err)
	}
	return triggers, nil
}

// GetTrigger returns a trigger by id scoped to a user.
func (r *Repository) GetTrigger(ctx context.Context, id, userID string) (*Trigger, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("id and user_id cannot be empty")
	}

	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s&limit=1", id, userID)
	data, err := r.base.Request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, fmt.Errorf("get automation trigger: %w", err)
	}

	var triggers []Trigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, fmt.Errorf("unmarshal automation triggers: %w", err)
	}
	if len(triggers) == 0 {
		return nil, fmt.Errorf("automation_trigger not found: %s", id)
	}
	return &triggers[0], nil
}

// CreateTrigger inserts a new automation trigger.
func (r *Repository) CreateTrigger(ctx context.Context, trigger *Trigger) error {
	if trigger == nil {
		return fmt.Errorf("trigger cannot be nil")
	}
	if trigger.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	data, err := r.base.Request(ctx, "POST", "automation_triggers", trigger, "")
	if err != nil {
		return fmt.Errorf("create automation trigger: %w", err)
	}
	var rows []Trigger
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*trigger = rows[0]
	}
	return nil
}

// UpdateTrigger updates a trigger by id.
func (r *Repository) UpdateTrigger(ctx context.Context, trigger *Trigger) error {
	if trigger == nil {
		return fmt.Errorf("trigger cannot be nil")
	}
	if trigger.ID == "" || trigger.UserID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", trigger.ID, trigger.UserID)
	_, err := r.base.Request(ctx, "PATCH", "automation_triggers", trigger, query)
	if err != nil {
		return fmt.Errorf("update automation trigger: %w", err)
	}
	return nil
}

// DeleteTrigger removes a trigger.
func (r *Repository) DeleteTrigger(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", id, userID)
	_, err := r.base.Request(ctx, "DELETE", "automation_triggers", nil, query)
	if err != nil {
		return fmt.Errorf("delete automation trigger: %w", err)
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
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", id, userID)
	_, err := r.base.Request(ctx, "PATCH", "automation_triggers", update, query)
	if err != nil {
		return fmt.Errorf("set automation trigger enabled: %w", err)
	}
	return nil
}

// GetPendingTriggers retrieves triggers that need execution.
func (r *Repository) GetPendingTriggers(ctx context.Context) ([]Trigger, error) {
	now := time.Now().Format(time.RFC3339)
	query := fmt.Sprintf("enabled=eq.true&next_execution=lte.%s", now)
	data, err := r.base.Request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, fmt.Errorf("get pending triggers: %w", err)
	}

	var triggers []Trigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, fmt.Errorf("unmarshal automation triggers: %w", err)
	}
	return triggers, nil
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

	data, err := r.base.Request(ctx, "POST", "automation_executions", exec, "")
	if err != nil {
		return fmt.Errorf("create automation execution: %w", err)
	}
	var rows []Execution
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*exec = rows[0]
	}
	return nil
}

// GetExecutions lists executions for a trigger.
func (r *Repository) GetExecutions(ctx context.Context, triggerID string, limit int) ([]Execution, error) {
	if triggerID == "" {
		return nil, fmt.Errorf("trigger_id cannot be empty")
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	query := fmt.Sprintf("trigger_id=eq.%s&order=executed_at.desc&limit=%d", triggerID, limit)
	data, err := r.base.Request(ctx, "GET", "automation_executions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("get automation executions: %w", err)
	}
	var execs []Execution
	if err := json.Unmarshal(data, &execs); err != nil {
		return nil, fmt.Errorf("unmarshal automation executions: %w", err)
	}
	return execs, nil
}
