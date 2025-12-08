// Package confidential provides types for the confidential compute service.
package confidentialmarble

import "time"

// =============================================================================
// Request/Response Types
// =============================================================================

// ExecuteRequest represents a script execution request.
type ExecuteRequest struct {
	Script     string                 `json:"script"`
	EntryPoint string                 `json:"entry_point,omitempty"`
	Input      map[string]interface{} `json:"input,omitempty"`
	SecretRefs []string               `json:"secret_refs,omitempty"`
	Timeout    int                    `json:"timeout,omitempty"`
}

// ExecuteResponse represents a script execution response.
type ExecuteResponse struct {
	JobID     string                 `json:"job_id"`
	Status    string                 `json:"status"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Logs      []string               `json:"logs,omitempty"`
	Error     string                 `json:"error,omitempty"`
	GasUsed   int64                  `json:"gas_used"`
	StartedAt time.Time              `json:"started_at"`
	Duration  string                 `json:"duration,omitempty"`
}
