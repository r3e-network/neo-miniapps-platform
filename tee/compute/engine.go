// Package compute provides confidential computation inside the TEE enclave.
package compute

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/types"
)

// VaultProvider provides access to secrets.
type VaultProvider interface {
	UseMultiple(ctx context.Context, refs []types.SecretRef, fn types.MultiSecretConsumer) error
}

// ScriptExecutor executes scripts inside the enclave.
type ScriptExecutor interface {
	Execute(ctx context.Context, script, entryPoint string, input map[string]any, secrets map[string][]byte) (map[string]any, []string, error)
}

// Config holds compute engine configuration.
type Config struct {
	Runtime        enclave.Runtime
	Vault          VaultProvider
	Executor       ScriptExecutor
	DefaultTimeout time.Duration
	MaxMemory      int64
}

// Engine implements types.ConfidentialCompute.
type Engine struct {
	mu             sync.RWMutex
	runtime        enclave.Runtime
	vault          VaultProvider
	executor       ScriptExecutor
	defaultTimeout time.Duration
	maxMemory      int64
}

// New creates a new compute engine.
func New(cfg Config) (*Engine, error) {
	if cfg.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}

	timeout := cfg.DefaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxMem := cfg.MaxMemory
	if maxMem == 0 {
		maxMem = 128 * 1024 * 1024 // 128MB default
	}

	return &Engine{
		runtime:        cfg.Runtime,
		vault:          cfg.Vault,
		executor:       cfg.Executor,
		defaultTimeout: timeout,
		maxMemory:      maxMem,
	}, nil
}

// Execute runs code inside the enclave.
func (e *Engine) Execute(ctx context.Context, req types.ComputeRequest) (*types.ComputeResult, error) {
	return e.executeInternal(ctx, req, nil)
}

// ExecuteWithSecrets runs code with access to secrets.
func (e *Engine) ExecuteWithSecrets(ctx context.Context, req types.ComputeRequest, secretRefs []types.SecretRef) (*types.ComputeResult, error) {
	if e.vault == nil {
		return nil, fmt.Errorf("vault not configured")
	}

	if len(secretRefs) == 0 {
		return e.Execute(ctx, req)
	}

	var result *types.ComputeResult
	var execErr error

	err := e.vault.UseMultiple(ctx, secretRefs, func(secrets map[string][]byte) error {
		result, execErr = e.executeInternal(ctx, req, secrets)
		return execErr
	})

	if err != nil {
		return nil, err
	}

	return result, execErr
}

// executeInternal performs the actual execution.
func (e *Engine) executeInternal(ctx context.Context, req types.ComputeRequest, secrets map[string][]byte) (*types.ComputeResult, error) {
	startTime := time.Now()

	// Set timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = e.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := &types.ComputeResult{
		Status:    types.ComputeStatusRunning,
		StartedAt: startTime,
	}

	// Execute script
	if e.executor == nil {
		// No executor configured - return error
		result.Status = types.ComputeStatusFailed
		result.Error = "no script executor configured"
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(startTime)
		return result, nil
	}

	output, logs, err := e.executor.Execute(ctx, req.Script, req.EntryPoint, req.Input, secrets)

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startTime)
	result.Logs = logs

	if ctx.Err() == context.DeadlineExceeded {
		result.Status = types.ComputeStatusTimeout
		result.Error = "execution timeout"
		return result, types.ErrComputeTimeout
	}

	if err != nil {
		result.Status = types.ComputeStatusFailed
		result.Error = err.Error()
		return result, nil
	}

	result.Status = types.ComputeStatusSucceeded
	result.Output = output

	return result, nil
}

// DefaultExecutor is a simple script executor for testing.
type DefaultExecutor struct{}

// Execute executes a script (placeholder implementation).
func (e *DefaultExecutor) Execute(ctx context.Context, script, entryPoint string, input map[string]any, secrets map[string][]byte) (map[string]any, []string, error) {
	// This is a placeholder - real implementation would use a JS/Lua/WASM runtime
	logs := []string{
		fmt.Sprintf("Executing script with entry point: %s", entryPoint),
		fmt.Sprintf("Input keys: %v", mapKeys(input)),
		fmt.Sprintf("Secret keys: %v", mapKeys(secretsToAny(secrets))),
	}

	// Return input as output for testing
	output := map[string]any{
		"success":    true,
		"entryPoint": entryPoint,
		"inputKeys":  mapKeys(input),
	}

	return output, logs, nil
}

// mapKeys returns the keys of a map.
func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// secretsToAny converts secrets map to any map for key extraction.
func secretsToAny(secrets map[string][]byte) map[string]any {
	result := make(map[string]any, len(secrets))
	for k := range secrets {
		result[k] = nil // Only need keys
	}
	return result
}
