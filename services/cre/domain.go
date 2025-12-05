// Package cre provides Chainlink Runtime Environment service.
package cre

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// Workflow represents a CRE workflow.
type Workflow struct {
	base.BaseEntity
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Steps       []WorkflowStep    `json:"steps"`
	Status      string            `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// WorkflowStep represents a step in a workflow.
type WorkflowStep struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Config     map[string]any `json:"config,omitempty"`
	DependsOn  []string       `json:"depends_on,omitempty"`
}

// WorkflowResult represents workflow execution result.
type WorkflowResult struct {
	WorkflowID  string         `json:"workflow_id"`
	Success     bool           `json:"success"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	ExecutedAt  time.Time      `json:"executed_at"`
	CompletedAt time.Time      `json:"completed_at"`
}
