# NeoFlow Supabase Repository

Database layer for the NeoFlow automation service.

## Overview

This package provides NeoFlow-specific data access for triggers and execution records.

The canonical schema lives in `migrations/022_neoflow_schema.sql`:

- `public.neoflow_triggers`
- `public.neoflow_executions`

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Data models |

## Data Models

### Trigger

```go
type Trigger struct {
    ID            string          `json:"id"`
    UserID        string          `json:"user_id"`
    Name          string          `json:"name"`
    TriggerType   string          `json:"trigger_type"` // e.g. "cron", "event", "price_threshold"
    Schedule      string          `json:"schedule,omitempty"`
    Condition     json.RawMessage `json:"condition,omitempty"` // JSON (optional)
    Action        json.RawMessage `json:"action"`              // JSON (required)
    Enabled       bool            `json:"enabled"`
    LastExecution time.Time       `json:"last_execution,omitempty"`
    NextExecution time.Time       `json:"next_execution,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
}
```

### Execution

```go
type Execution struct {
    ID            string          `json:"id"`
    TriggerID     string          `json:"trigger_id"`
    ExecutedAt    time.Time       `json:"executed_at"`
    Success       bool            `json:"success"`
    Error         string          `json:"error,omitempty"`
    ActionType    string          `json:"action_type,omitempty"`
    ActionPayload json.RawMessage `json:"action_payload,omitempty"`
}
```

## Repository Interface

```go
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
```

## Usage

```go
import neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"

repo := neoflowsupabase.NewRepository(baseRepo)

// Create trigger
err := repo.CreateTrigger(ctx, &neoflowsupabase.Trigger{
    UserID:      userID,
    Name:        "Daily Report",
    TriggerType: "cron",
    Schedule:    "*/5 * * * *",
    Action:      json.RawMessage(`{"type":"webhook","url":"https://hooks.miniapps.com/hook"}`),
    Enabled:     true,
})

// Get pending triggers
pending, err := repo.GetPendingTriggers(ctx)
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
