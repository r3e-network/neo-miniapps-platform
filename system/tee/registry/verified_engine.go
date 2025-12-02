// Package registry provides verified execution engine with script authenticity verification.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                    Verified Execution Engine                             │
//	├─────────────────────────────────────────────────────────────────────────┤
//	│                                                                          │
//	│  1. Script Loading                                                       │
//	│     - Service requests script execution                                  │
//	│     - Engine computes SHA256 hash of script content                      │
//	│     - Hash compared against registered service enclave hash              │
//	│                                                                          │
//	│  2. Verification Gate                                                    │
//	│     - If hash matches: execution proceeds                                │
//	│     - If hash mismatch: execution rejected with error                    │
//	│     - If service not registered: execution rejected                      │
//	│                                                                          │
//	│  3. Execution                                                            │
//	│     - Verified script passed to underlying TEE engine                    │
//	│     - Result includes verification attestation                           │
//	│                                                                          │
//	└─────────────────────────────────────────────────────────────────────────┘
package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrScriptVerificationFailed = errors.New("script verification failed")
	ErrServiceNotVerified       = errors.New("service not verified in registry")
	ErrEngineNotInitialized     = errors.New("verified engine not initialized")
)

// VerifiedExecutionRequest represents a request to execute verified code.
type VerifiedExecutionRequest struct {
	// ServiceID identifies the calling service
	ServiceID string `json:"service_id"`

	// AccountID identifies the account context
	AccountID string `json:"account_id"`

	// Script is the code to execute (will be verified before execution)
	Script []byte `json:"script"`

	// EntryPoint is the function name to call
	EntryPoint string `json:"entry_point"`

	// Input is the JSON-serializable input
	Input map[string]any `json:"input"`

	// Secrets lists the secret names this execution needs
	Secrets []string `json:"secrets"`

	// Timeout for execution
	Timeout time.Duration `json:"timeout"`

	// Metadata for tracing
	Metadata map[string]string `json:"metadata"`
}

// VerifiedExecutionResult contains the result of a verified execution.
type VerifiedExecutionResult struct {
	// Output is the return value
	Output map[string]any `json:"output"`

	// Logs captured during execution
	Logs []string `json:"logs"`

	// Error message if execution failed
	Error string `json:"error,omitempty"`

	// Status of the execution
	Status VerifiedExecutionStatus `json:"status"`

	// Verification info
	ScriptHash       string `json:"script_hash"`
	VerifiedAt       time.Time `json:"verified_at"`
	ServiceVersion   uint64 `json:"service_version"`

	// Execution metrics
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`

	// Attestation
	AttestationHash string `json:"attestation_hash,omitempty"`
}

// VerifiedExecutionStatus represents the status of a verified execution.
type VerifiedExecutionStatus string

const (
	VerifiedStatusPending          VerifiedExecutionStatus = "pending"
	VerifiedStatusVerifying        VerifiedExecutionStatus = "verifying"
	VerifiedStatusVerified         VerifiedExecutionStatus = "verified"
	VerifiedStatusVerificationFailed VerifiedExecutionStatus = "verification_failed"
	VerifiedStatusRunning          VerifiedExecutionStatus = "running"
	VerifiedStatusSucceeded        VerifiedExecutionStatus = "succeeded"
	VerifiedStatusFailed           VerifiedExecutionStatus = "failed"
	VerifiedStatusTimeout          VerifiedExecutionStatus = "timeout"
)

// ScriptExecutor is the interface for the underlying execution engine.
type ScriptExecutor interface {
	// Execute runs the script after verification
	Execute(ctx context.Context, serviceID, accountID string, script []byte, entryPoint string, input map[string]any, secrets []string, timeout time.Duration) (map[string]any, []string, error)
}

// VerifiedEngineConfig holds configuration for the verified execution engine.
type VerifiedEngineConfig struct {
	// Registry for service enclave verification
	Registry *Registry

	// Executor for actual script execution
	Executor ScriptExecutor

	// StrictMode rejects execution if service not registered (default: true)
	StrictMode bool

	// AllowUnverifiedServices allows execution without verification (for development)
	AllowUnverifiedServices bool
}

// VerifiedEngine wraps script execution with authenticity verification.
type VerifiedEngine struct {
	mu sync.RWMutex

	registry    *Registry
	executor    ScriptExecutor
	strictMode  bool
	allowUnverified bool

	// Metrics
	totalExecutions      int64
	verifiedExecutions   int64
	failedVerifications  int64

	initialized bool
}

// NewVerifiedEngine creates a new verified execution engine.
func NewVerifiedEngine(cfg *VerifiedEngineConfig) (*VerifiedEngine, error) {
	if cfg.Registry == nil {
		return nil, errors.New("registry required")
	}

	strictMode := true
	if !cfg.StrictMode && cfg.AllowUnverifiedServices {
		strictMode = false
	}

	return &VerifiedEngine{
		registry:        cfg.Registry,
		executor:        cfg.Executor,
		strictMode:      strictMode,
		allowUnverified: cfg.AllowUnverifiedServices,
	}, nil
}

// Initialize initializes the verified engine.
func (e *VerifiedEngine) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.initialized {
		return nil
	}

	// Ensure registry is initialized
	if !e.registry.IsInitialized() {
		if err := e.registry.Initialize(ctx); err != nil {
			return fmt.Errorf("initialize registry: %w", err)
		}
	}

	e.initialized = true
	return nil
}

// Execute runs a script with verification.
func (e *VerifiedEngine) Execute(ctx context.Context, req VerifiedExecutionRequest) (*VerifiedExecutionResult, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, ErrEngineNotInitialized
	}
	e.mu.RUnlock()

	result := &VerifiedExecutionResult{
		Status:    VerifiedStatusPending,
		StartedAt: time.Now(),
	}

	// Step 1: Compute script hash
	result.Status = VerifiedStatusVerifying
	scriptHash := computeScriptHash(req.Script)
	result.ScriptHash = scriptHash

	// Step 2: Verify against registry
	verified, serviceVersion, err := e.verifyScript(req.ServiceID, req.Script)
	result.VerifiedAt = time.Now()
	result.ServiceVersion = serviceVersion

	if err != nil {
		if errors.Is(err, ErrServiceNotRegistered) && e.allowUnverified {
			// Allow unverified execution in development mode
			verified = true
		} else {
			result.Status = VerifiedStatusVerificationFailed
			result.Error = fmt.Sprintf("verification failed: %v", err)
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			e.incrementFailedVerifications()
			return result, nil
		}
	}

	if !verified {
		result.Status = VerifiedStatusVerificationFailed
		result.Error = "script hash does not match registered hash"
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		e.incrementFailedVerifications()
		return result, nil
	}

	result.Status = VerifiedStatusVerified
	e.incrementVerifiedExecutions()

	// Step 3: Execute verified script
	if e.executor == nil {
		result.Status = VerifiedStatusSucceeded
		result.Output = map[string]any{"verified": true, "script_hash": scriptHash}
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		return result, nil
	}

	result.Status = VerifiedStatusRunning
	output, logs, execErr := e.executor.Execute(
		ctx,
		req.ServiceID,
		req.AccountID,
		req.Script,
		req.EntryPoint,
		req.Input,
		req.Secrets,
		req.Timeout,
	)

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.Output = output
	result.Logs = logs

	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) {
			result.Status = VerifiedStatusTimeout
		} else {
			result.Status = VerifiedStatusFailed
		}
		result.Error = execErr.Error()
		return result, nil
	}

	result.Status = VerifiedStatusSucceeded

	// Generate attestation hash
	result.AttestationHash = e.generateAttestationHash(req, result)

	return result, nil
}

// verifyScript verifies a script against the registry.
func (e *VerifiedEngine) verifyScript(serviceID string, script []byte) (bool, uint64, error) {
	// Get registered service
	service, err := e.registry.GetServiceEnclave(serviceID)
	if err != nil {
		return false, 0, err
	}

	if !service.Active {
		return false, service.Version, errors.New("service is not active")
	}

	// Compute and compare hash
	scriptHash := computeScriptHash(script)
	if scriptHash != service.ScriptHash {
		return false, service.Version, fmt.Errorf("%w: expected %s, got %s",
			ErrScriptHashMismatch, service.ScriptHash, scriptHash)
	}

	return true, service.Version, nil
}

// VerifyScriptOnly verifies a script without executing it.
func (e *VerifiedEngine) VerifyScriptOnly(serviceID string, script []byte) (*ScriptVerificationResult, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, ErrEngineNotInitialized
	}
	e.mu.RUnlock()

	scriptHash := computeScriptHash(script)

	result := &ScriptVerificationResult{
		ServiceID:  serviceID,
		ScriptHash: scriptHash,
		VerifiedAt: time.Now(),
	}

	verified, version, err := e.verifyScript(serviceID, script)
	result.Version = version

	if err != nil {
		result.Verified = false
		result.Error = err.Error()
		return result, nil
	}

	result.Verified = verified
	return result, nil
}

// ScriptVerificationResult contains the result of script verification.
type ScriptVerificationResult struct {
	ServiceID  string    `json:"service_id"`
	ScriptHash string    `json:"script_hash"`
	Verified   bool      `json:"verified"`
	Version    uint64    `json:"version"`
	VerifiedAt time.Time `json:"verified_at"`
	Error      string    `json:"error,omitempty"`
}

// RegisterAndVerifyService registers a service and verifies its script.
func (e *VerifiedEngine) RegisterAndVerifyService(ctx context.Context, serviceID, serviceName string, script []byte) (*ServiceEnclave, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized {
		return nil, ErrEngineNotInitialized
	}

	// Register the service enclave
	return e.registry.RegisterServiceEnclave(ctx, serviceID, serviceName, script)
}

// UpdateServiceScript updates a service's script and re-registers it.
func (e *VerifiedEngine) UpdateServiceScript(ctx context.Context, serviceID string, newScript []byte) (*ServiceEnclave, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized {
		return nil, ErrEngineNotInitialized
	}

	return e.registry.UpdateServiceEnclave(ctx, serviceID, newScript)
}

// GetServiceInfo returns information about a registered service.
func (e *VerifiedEngine) GetServiceInfo(serviceID string) (*ServiceEnclave, error) {
	return e.registry.GetServiceEnclave(serviceID)
}

// ListServices returns all registered services.
func (e *VerifiedEngine) ListServices() []*ServiceEnclave {
	return e.registry.ListServices()
}

// generateAttestationHash generates an attestation hash for the execution.
func (e *VerifiedEngine) generateAttestationHash(req VerifiedExecutionRequest, result *VerifiedExecutionResult) string {
	h := sha256.New()
	h.Write([]byte(req.ServiceID))
	h.Write([]byte(result.ScriptHash))
	h.Write([]byte(fmt.Sprintf("%d", result.ServiceVersion)))
	h.Write([]byte(result.VerifiedAt.Format(time.RFC3339Nano)))
	h.Write([]byte(string(result.Status)))
	return hex.EncodeToString(h.Sum(nil))
}

// Metrics methods
func (e *VerifiedEngine) incrementVerifiedExecutions() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.totalExecutions++
	e.verifiedExecutions++
}

func (e *VerifiedEngine) incrementFailedVerifications() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.totalExecutions++
	e.failedVerifications++
}

// GetMetrics returns execution metrics.
func (e *VerifiedEngine) GetMetrics() VerifiedEngineMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return VerifiedEngineMetrics{
		TotalExecutions:     e.totalExecutions,
		VerifiedExecutions:  e.verifiedExecutions,
		FailedVerifications: e.failedVerifications,
	}
}

// VerifiedEngineMetrics contains engine metrics.
type VerifiedEngineMetrics struct {
	TotalExecutions     int64 `json:"total_executions"`
	VerifiedExecutions  int64 `json:"verified_executions"`
	FailedVerifications int64 `json:"failed_verifications"`
}

// IsInitialized returns whether the engine is initialized.
func (e *VerifiedEngine) IsInitialized() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.initialized
}

// SetExecutor sets the script executor (for late binding).
func (e *VerifiedEngine) SetExecutor(executor ScriptExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executor = executor
}

// GetRegistry returns the underlying registry.
func (e *VerifiedEngine) GetRegistry() *Registry {
	return e.registry
}
