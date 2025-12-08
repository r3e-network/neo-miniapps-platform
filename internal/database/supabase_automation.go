package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// Automation Trigger Model
// =============================================================================

// AutomationTrigger represents an automation trigger.
type AutomationTrigger struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	Name          string          `json:"name"`
	TriggerType   string          `json:"trigger_type"`
	Schedule      string          `json:"schedule,omitempty"`
	Condition     json.RawMessage `json:"condition,omitempty"`
	Action        json.RawMessage `json:"action"`
	Enabled       bool            `json:"enabled"`
	LastExecution time.Time       `json:"last_execution,omitempty"`
	NextExecution time.Time       `json:"next_execution,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AutomationExecution represents an execution log entry.
type AutomationExecution struct {
	ID            string          `json:"id"`
	TriggerID     string          `json:"trigger_id"`
	ExecutedAt    time.Time       `json:"executed_at"`
	Success       bool            `json:"success"`
	Error         string          `json:"error,omitempty"`
	ActionType    string          `json:"action_type,omitempty"`
	ActionPayload json.RawMessage `json:"action_payload,omitempty"`
}

// =============================================================================
// Automation Trigger Operations
// =============================================================================

// GetAutomationTriggers retrieves automation triggers for a user.
func (r *Repository) GetAutomationTriggers(ctx context.Context, userID string) ([]AutomationTrigger, error) {
	query := ""
	if userID != "" {
		query = "user_id=eq." + userID
	}
	data, err := r.client.request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, err
	}

	var triggers []AutomationTrigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, err
	}
	return triggers, nil
}

// GetAutomationTrigger returns a trigger by id scoped to a user.
func (r *Repository) GetAutomationTrigger(ctx context.Context, id, userID string) (*AutomationTrigger, error) {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s&limit=1", id, userID)
	data, err := r.client.request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, err
	}

	var triggers []AutomationTrigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, err
	}
	if len(triggers) == 0 {
		return nil, fmt.Errorf("trigger not found")
	}
	return &triggers[0], nil
}

// CreateAutomationTrigger inserts a new automation trigger.
func (r *Repository) CreateAutomationTrigger(ctx context.Context, trigger *AutomationTrigger) error {
	data, err := r.client.request(ctx, "POST", "automation_triggers", trigger, "")
	if err != nil {
		return err
	}
	var rows []AutomationTrigger
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*trigger = rows[0]
	}
	return nil
}

// UpdateAutomationTrigger updates a trigger by id.
func (r *Repository) UpdateAutomationTrigger(ctx context.Context, trigger *AutomationTrigger) error {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", trigger.ID, trigger.UserID)
	_, err := r.client.request(ctx, "PATCH", "automation_triggers", trigger, query)
	return err
}

// DeleteAutomationTrigger removes a trigger.
func (r *Repository) DeleteAutomationTrigger(ctx context.Context, id, userID string) error {
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", id, userID)
	_, err := r.client.request(ctx, "DELETE", "automation_triggers", nil, query)
	return err
}

// SetAutomationTriggerEnabled sets enabled flag.
func (r *Repository) SetAutomationTriggerEnabled(ctx context.Context, id, userID string, enabled bool) error {
	update := map[string]interface{}{"enabled": enabled}
	if enabled {
		update["next_execution"] = time.Now()
	}
	query := fmt.Sprintf("id=eq.%s&user_id=eq.%s", id, userID)
	_, err := r.client.request(ctx, "PATCH", "automation_triggers", update, query)
	return err
}

// GetPendingTriggers retrieves triggers that need execution.
func (r *Repository) GetPendingTriggers(ctx context.Context) ([]AutomationTrigger, error) {
	now := time.Now().Format(time.RFC3339)
	query := fmt.Sprintf("enabled=eq.true&next_execution=lte.%s", now)
	data, err := r.client.request(ctx, "GET", "automation_triggers", nil, query)
	if err != nil {
		return nil, err
	}

	var triggers []AutomationTrigger
	if err := json.Unmarshal(data, &triggers); err != nil {
		return nil, err
	}
	return triggers, nil
}

// =============================================================================
// Automation Execution Operations
// =============================================================================

// CreateAutomationExecution logs a trigger execution.
func (r *Repository) CreateAutomationExecution(ctx context.Context, exec *AutomationExecution) error {
	data, err := r.client.request(ctx, "POST", "automation_executions", exec, "")
	if err != nil {
		return err
	}
	var rows []AutomationExecution
	if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
		*exec = rows[0]
	}
	return nil
}

// GetAutomationExecutions lists executions for a trigger.
func (r *Repository) GetAutomationExecutions(ctx context.Context, triggerID string, limit int) ([]AutomationExecution, error) {
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf("trigger_id=eq.%s&order=executed_at.desc&limit=%d", triggerID, limit)
	data, err := r.client.request(ctx, "GET", "automation_executions", nil, query)
	if err != nil {
		return nil, err
	}
	var execs []AutomationExecution
	if err := json.Unmarshal(data, &execs); err != nil {
		return nil, err
	}
	return execs, nil
}
