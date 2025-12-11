# NeoFlow Supabase Repository

Database layer for the NeoFlow automation service.

## Overview

This package provides NeoFlow-specific data access for triggers and execution records.

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Data models |

## Data Models

### Trigger

```go
type Trigger struct {
    ID            string    `json:"id"`
    UserID        string    `json:"user_id"`
    Name          string    `json:"name"`
    TriggerType   string    `json:"trigger_type"`
    Condition     string    `json:"condition"` // JSON
    Action        string    `json:"action"`    // JSON
    Enabled       bool      `json:"enabled"`
    NextExecution time.Time `json:"next_execution"`
    CreatedAt     time.Time `json:"created_at"`
}
```

### Execution

```go
type Execution struct {
    ID         string    `json:"id"`
    TriggerID  string    `json:"trigger_id"`
    Status     string    `json:"status"`
    Result     string    `json:"result,omitempty"`
    ExecutedAt time.Time `json:"executed_at"`
    Error      string    `json:"error,omitempty"`
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
import neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"

repo := neoflowsupabase.NewRepository(baseRepo)

// Create trigger
err := repo.CreateTrigger(ctx, &neoflowsupabase.Trigger{
    UserID:      userID,
    Name:        "Daily Report",
    TriggerType: "cron",
    Enabled:     true,
})

// Get pending triggers
pending, err := repo.GetPendingTriggers(ctx)
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
