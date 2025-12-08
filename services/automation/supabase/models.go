// Package supabase provides Automation-specific database operations.
package supabase

import (
	"encoding/json"
	"time"
)

// Trigger represents an automation trigger.
type Trigger struct {
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

// Execution represents an execution log entry.
type Execution struct {
	ID            string          `json:"id"`
	TriggerID     string          `json:"trigger_id"`
	ExecutedAt    time.Time       `json:"executed_at"`
	Success       bool            `json:"success"`
	Error         string          `json:"error,omitempty"`
	ActionType    string          `json:"action_type,omitempty"`
	ActionPayload json.RawMessage `json:"action_payload,omitempty"`
}
