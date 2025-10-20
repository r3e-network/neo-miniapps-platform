package function

import "time"

// ExecutionResult represents the outcome of running a function definition.
type ExecutionResult struct {
	FunctionID  string         `json:"function_id"`
	Output      map[string]any `json:"output"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Duration    time.Duration  `json:"duration"`
}
