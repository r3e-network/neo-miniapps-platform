package cre

import "time"

// StepType represents the type of work executed by a CRE step.
type StepType string

const (
	StepTypeFunctionCall StepType = "function_call"
	StepTypeAutomation   StepType = "automation_job"
	StepTypeHTTPRequest  StepType = "http_request"
)

// RunStatus represents the lifecycle of a playbook run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

// Step describes a single unit within a playbook.
type Step struct {
	Name           string            `json:"name"`
	Type           StepType          `json:"type"`
	Config         map[string]any    `json:"config,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	RetryLimit     int               `json:"retry_limit,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
}

// Playbook represents a declarative orchestration for CRE.
type Playbook struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Steps       []Step            `json:"steps"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Run describes a single execution of a playbook.
type Run struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	PlaybookID  string            `json:"playbook_id"`
	ExecutorID  string            `json:"executor_id,omitempty"`
	Status      RunStatus         `json:"status"`
	Parameters  map[string]any    `json:"parameters,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Results     []StepResult      `json:"results,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StepResult captures the result of a run step.
type StepResult struct {
	RunID       string            `json:"run_id"`
	StepIndex   int               `json:"step_index"`
	Name        string            `json:"name"`
	Type        StepType          `json:"type"`
	Status      RunStatus         `json:"status"`
	Logs        []string          `json:"logs,omitempty"`
	Error       string            `json:"error,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Executor represents an available runner for CRE playbooks.
type Executor struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Endpoint  string            `json:"endpoint"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
