// Package automation provides task automation service.
package automation

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// TaskStatus represents task status.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// TriggerType represents trigger type.
type TriggerType string

const (
	TriggerTypeCron      TriggerType = "cron"
	TriggerTypeInterval  TriggerType = "interval"
	TriggerTypeEvent     TriggerType = "event"
	TriggerTypeCondition TriggerType = "condition"
)

// Task represents an automation task.
type Task struct {
	base.BaseEntity
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	TriggerType TriggerType       `json:"trigger_type"`
	TriggerSpec string            `json:"trigger_spec"`
	Script      string            `json:"script"`
	Status      TaskStatus        `json:"status"`
	Enabled     bool              `json:"enabled"`
	LastRunAt   time.Time         `json:"last_run_at,omitempty"`
	NextRunAt   time.Time         `json:"next_run_at,omitempty"`
	RunCount    int64             `json:"run_count"`
	ErrorCount  int64             `json:"error_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskResult represents task execution result.
type TaskResult struct {
	TaskID      string         `json:"task_id"`
	Success     bool           `json:"success"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	Logs        []string       `json:"logs,omitempty"`
	ExecutedAt  time.Time      `json:"executed_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Duration    time.Duration  `json:"duration"`
}
