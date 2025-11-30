package functions

import (
	"context"
	"fmt"
	"time"
)

// MockTEEExecutor fakes TEE-backed execution by echoing inputs after validating secrets.
type MockTEEExecutor struct {
	resolver SecretResolver
}

// NewMockTEEExecutor constructs a mock executor implementation.
func NewMockTEEExecutor() *MockTEEExecutor {
	return &MockTEEExecutor{}
}

// SetSecretResolver implements SecretAwareExecutor.
func (m *MockTEEExecutor) SetSecretResolver(resolver SecretResolver) {
	m.resolver = resolver
}

// Execute returns the provided payload with metadata, ensuring secrets exist when specified.
func (m *MockTEEExecutor) Execute(ctx context.Context, def Definition, payload map[string]any) (ExecutionResult, error) {
	if len(def.Secrets) > 0 {
		if m.resolver == nil {
			return ExecutionResult{}, fmt.Errorf("secret resolver not configured")
		}
		if _, err := m.resolver.ResolveSecrets(ctx, def.AccountID, def.Secrets); err != nil {
			return ExecutionResult{}, err
		}
	}

	started := time.Now().UTC()
	output := clonePayload(payload)
	if output == nil {
		output = map[string]any{}
	}
	output["function_name"] = def.Name
	output["message"] = "mock execution (TEE disabled)"

	completed := time.Now().UTC()
	return ExecutionResult{
		FunctionID:  def.ID,
		Output:      output,
		Status:      ExecutionStatusSucceeded,
		StartedAt:   started,
		CompletedAt: completed,
		Duration:    completed.Sub(started),
	}, nil
}
