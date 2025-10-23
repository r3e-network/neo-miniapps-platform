package functions

import (
	"context"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/function"
)

// SimpleExecutor is a placeholder executor that echoes the payload.
type SimpleExecutor struct{}

func NewSimpleExecutor() *SimpleExecutor { return &SimpleExecutor{} }

func (e *SimpleExecutor) Execute(ctx context.Context, def function.Definition, payload map[string]any) (function.ExecutionResult, error) {
	started := time.Now().UTC()
	// Copy payload to avoid mutation
	out := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		out[k] = v
	}
	out["function_name"] = def.Name
	out["message"] = "execution completed"
	completed := time.Now().UTC()
	return function.ExecutionResult{
		FunctionID:  def.ID,
		Output:      out,
		Status:      function.ExecutionStatusSucceeded,
		StartedAt:   started,
		CompletedAt: completed,
		Duration:    completed.Sub(started),
	}, nil
}
