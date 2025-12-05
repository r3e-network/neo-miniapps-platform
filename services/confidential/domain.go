// Package confidential provides confidential computing service.
package confidential

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// ComputeRequest represents a confidential compute request.
type ComputeRequest struct {
	base.BaseEntity
	UserID      string         `json:"user_id,omitempty"`
	AccountID   string         `json:"account_id,omitempty"`
	Script      string         `json:"script,omitempty"`
	EntryPoint  string         `json:"entry_point,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	SecretNames []string       `json:"secret_names,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
	Status      RequestStatus  `json:"status,omitempty"`
}

// RequestStatus represents compute request status.
type RequestStatus string

const (
	RequestStatusPending   RequestStatus = "pending"
	RequestStatusRunning   RequestStatus = "running"
	RequestStatusCompleted RequestStatus = "completed"
	RequestStatusFailed    RequestStatus = "failed"
)

// ComputeResult represents computation result.
type ComputeResult struct {
	RequestID string         `json:"request_id"`
	Success   bool           `json:"success"`
	Output    map[string]any `json:"output,omitempty"`
	Logs      []string       `json:"logs,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  time.Duration  `json:"duration"`
	Status    string         `json:"status,omitempty"`
}
