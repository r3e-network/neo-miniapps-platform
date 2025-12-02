// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// Architecture Overview:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│                         TEE Engine                                   │
//	│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
//	│  │  Secret Vault   │  │  Script Engine  │  │  Attestation Mgr    │  │
//	│  │  (Isolated)     │  │  (Occlum+V8)    │  │  (SGX Quotes)       │  │
//	│  └────────┬────────┘  └────────┬────────┘  └──────────┬──────────┘  │
//	│           │                    │                      │             │
//	│  ┌────────┴────────────────────┴──────────────────────┴──────────┐  │
//	│  │                    Enclave Runtime (Rust SGX SDK)              │  │
//	│  │                    Simulation Mode / Hardware Mode             │  │
//	│  └────────────────────────────────────────────────────────────────┘  │
//	└─────────────────────────────────────────────────────────────────────┘
//
// Security Model:
// - Each service has isolated secret namespace (service_id:secret_name)
// - Scripts run in sandboxed V8 isolates within TEE
// - Cross-service secret access requires explicit grants
// - All operations are attested and auditable
package tee

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/system/os/tee/types"
)

// Type aliases for backward compatibility - use types from shared package
type (
	AttestationReport = types.AttestationReport
	EnclaveMode       = types.EnclaveMode
	ExecutionStatus   = types.ExecutionStatus
)

// Re-export constants from types package
const (
	EnclaveModeSimulation = types.EnclaveModeSimulation
	EnclaveModeHardware   = types.EnclaveModeHardware

	ExecutionStatusPending   = types.ExecutionStatusPending
	ExecutionStatusRunning   = types.ExecutionStatusRunning
	ExecutionStatusSucceeded = types.ExecutionStatusSucceeded
	ExecutionStatusFailed    = types.ExecutionStatusFailed
	ExecutionStatusTimeout   = types.ExecutionStatusTimeout
)

// Engine is the main TEE execution engine interface.
// It provides confidential computing capabilities for all services.
type Engine interface {
	// Execute runs a JavaScript function within the TEE enclave.
	// The execution context includes isolated secrets for the caller service.
	Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)

	// GetSecretResolver returns a secret resolver scoped to a specific service.
	// Secrets are isolated by service namespace.
	GetSecretResolver(serviceID string) SecretResolver

	// RegisterService registers a service for TEE access with its permissions.
	RegisterService(ctx context.Context, reg ServiceRegistration) error

	// GetAttestation returns the current TEE attestation report.
	GetAttestation(ctx context.Context) (*AttestationReport, error)

	// Health checks if the TEE enclave is operational.
	Health(ctx context.Context) error

	// Start initializes the TEE enclave.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the TEE enclave.
	Stop(ctx context.Context) error
}

// ExecutionRequest represents a request to execute code in the TEE.
type ExecutionRequest struct {
	// ServiceID identifies the calling service (for secret isolation)
	ServiceID string `json:"service_id"`

	// AccountID identifies the account context
	AccountID string `json:"account_id"`

	// Script is the JavaScript code to execute
	Script string `json:"script"`

	// EntryPoint is the function name to call (default: "main")
	EntryPoint string `json:"entry_point"`

	// Input is the JSON-serializable input to the function
	Input map[string]any `json:"input"`

	// Secrets lists the secret names this execution needs access to
	// These must be within the service's allowed secrets
	Secrets []string `json:"secrets"`

	// Timeout for execution (default: 30s, max: 5m)
	Timeout time.Duration `json:"timeout"`

	// MemoryLimit in bytes (default: 128MB, max: 512MB)
	MemoryLimit int64 `json:"memory_limit"`

	// Metadata for tracing and auditing
	Metadata map[string]string `json:"metadata"`
}

// ExecutionResult contains the result of a TEE execution.
type ExecutionResult struct {
	// Output is the JSON-serializable return value
	Output map[string]any `json:"output"`

	// Logs captured during execution
	Logs []string `json:"logs"`

	// Error message if execution failed
	Error string `json:"error,omitempty"`

	// Status of the execution
	Status ExecutionStatus `json:"status"`

	// Metrics about the execution
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	MemoryUsed  int64         `json:"memory_used"`

	// Attestation proof for this execution
	AttestationHash string `json:"attestation_hash,omitempty"`
}

// SecretResolver provides access to secrets within the TEE.
// Each service gets its own resolver with isolated namespace.
type SecretResolver interface {
	// Resolve retrieves secret values by name within the service's namespace.
	// Returns a map of secret_name -> decrypted_value.
	Resolve(ctx context.Context, accountID string, names []string) (map[string]string, error)

	// ServiceID returns the service this resolver is scoped to.
	ServiceID() string
}

// ServiceRegistration defines a service's TEE access permissions.
type ServiceRegistration struct {
	// ServiceID is the unique identifier for the service
	ServiceID string `json:"service_id"`

	// AllowedSecretPatterns defines which secrets this service can access
	// Patterns support wildcards: "db_*", "api_key", "*" (all)
	AllowedSecretPatterns []string `json:"allowed_secret_patterns"`

	// MaxConcurrentExecutions limits parallel executions
	MaxConcurrentExecutions int `json:"max_concurrent_executions"`

	// DefaultTimeout for executions from this service
	DefaultTimeout time.Duration `json:"default_timeout"`

	// DefaultMemoryLimit for executions from this service
	DefaultMemoryLimit int64 `json:"default_memory_limit"`

	// Metadata for the service
	Metadata map[string]string `json:"metadata"`
}

// Errors
var (
	ErrEnclaveNotReady     = errors.New("tee: enclave not ready")
	ErrServiceNotRegistered = errors.New("tee: service not registered")
	ErrSecretAccessDenied  = errors.New("tee: secret access denied")
	ErrExecutionTimeout    = errors.New("tee: execution timeout")
	ErrMemoryLimitExceeded = errors.New("tee: memory limit exceeded")
	ErrInvalidScript       = errors.New("tee: invalid script")
	ErrAttestationFailed   = errors.New("tee: attestation failed")
)

// Default limits
const (
	DefaultExecutionTimeout = 30 * time.Second
	MaxExecutionTimeout     = 5 * time.Minute
	DefaultMemoryLimit      = 128 * 1024 * 1024 // 128MB
	MaxMemoryLimit          = 512 * 1024 * 1024 // 512MB
	DefaultMaxConcurrent    = 10
)

// engineImpl is the default implementation of the TEE Engine.
type engineImpl struct {
	mu sync.RWMutex

	// Configuration
	mode   EnclaveMode
	config EngineConfig

	// State
	ready    bool
	services map[string]*ServiceRegistration

	// Components
	runtime      EnclaveRuntime
	secretVault  SecretVault
	attestor     Attestor
	scriptEngine ScriptEngine
}

// EngineConfig configures the TEE engine.
type EngineConfig struct {
	// Mode: simulation or hardware
	Mode EnclaveMode `json:"mode"`

	// EnclaveID for this instance
	EnclaveID string `json:"enclave_id"`

	// SecretEncryptionKey for the vault (sealed by TEE)
	SecretEncryptionKey []byte `json:"-"`

	// MaxConcurrentExecutions globally
	MaxConcurrentExecutions int `json:"max_concurrent_executions"`

	// ScriptEngine configuration
	V8HeapSize int64 `json:"v8_heap_size"`
}

// NewEngine creates a new TEE engine with the given configuration.
func NewEngine(config EngineConfig) (Engine, error) {
	if config.Mode == "" {
		config.Mode = EnclaveModeSimulation
	}
	if config.MaxConcurrentExecutions <= 0 {
		config.MaxConcurrentExecutions = DefaultMaxConcurrent
	}
	if config.V8HeapSize <= 0 {
		config.V8HeapSize = DefaultMemoryLimit
	}

	e := &engineImpl{
		mode:     config.Mode,
		config:   config,
		services: make(map[string]*ServiceRegistration),
	}

	// Initialize components based on mode
	if config.Mode == EnclaveModeSimulation {
		e.runtime = newSimulationRuntime()
		e.secretVault = newSimulationVault(config.SecretEncryptionKey)
		e.attestor = newSimulationAttestor(config.EnclaveID)
		e.scriptEngine = newV8ScriptEngine(config.V8HeapSize)
	} else {
		// Hardware mode - would use actual SGX SDK
		return nil, fmt.Errorf("hardware mode not yet implemented")
	}

	return e, nil
}

func (e *engineImpl) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ready {
		return nil
	}

	// Initialize enclave runtime
	if err := e.runtime.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize runtime: %w", err)
	}

	// Initialize secret vault
	if err := e.secretVault.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize vault: %w", err)
	}

	// Initialize script engine
	if err := e.scriptEngine.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize script engine: %w", err)
	}

	e.ready = true
	return nil
}

func (e *engineImpl) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.ready {
		return nil
	}

	// Shutdown in reverse order
	if err := e.scriptEngine.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown script engine: %w", err)
	}

	if err := e.secretVault.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown vault: %w", err)
	}

	if err := e.runtime.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown runtime: %w", err)
	}

	e.ready = false
	return nil
}

func (e *engineImpl) Health(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return ErrEnclaveNotReady
	}

	return e.runtime.Health(ctx)
}

func (e *engineImpl) RegisterService(ctx context.Context, reg ServiceRegistration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if reg.ServiceID == "" {
		return fmt.Errorf("service_id required")
	}

	// Apply defaults
	if reg.MaxConcurrentExecutions <= 0 {
		reg.MaxConcurrentExecutions = DefaultMaxConcurrent
	}
	if reg.DefaultTimeout <= 0 {
		reg.DefaultTimeout = DefaultExecutionTimeout
	}
	if reg.DefaultMemoryLimit <= 0 {
		reg.DefaultMemoryLimit = DefaultMemoryLimit
	}

	e.services[reg.ServiceID] = &reg
	return nil
}

func (e *engineImpl) GetSecretResolver(serviceID string) SecretResolver {
	return &scopedSecretResolver{
		serviceID: serviceID,
		vault:     e.secretVault,
		engine:    e,
	}
}

func (e *engineImpl) GetAttestation(ctx context.Context) (*AttestationReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return nil, ErrEnclaveNotReady
	}

	return e.attestor.GenerateReport(ctx)
}

func (e *engineImpl) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	e.mu.RLock()
	if !e.ready {
		e.mu.RUnlock()
		return nil, ErrEnclaveNotReady
	}

	// Validate service registration
	svc, ok := e.services[req.ServiceID]
	if !ok {
		e.mu.RUnlock()
		return nil, ErrServiceNotRegistered
	}
	e.mu.RUnlock()

	// Apply defaults from service registration
	if req.Timeout <= 0 {
		req.Timeout = svc.DefaultTimeout
	}
	if req.Timeout > MaxExecutionTimeout {
		req.Timeout = MaxExecutionTimeout
	}
	if req.MemoryLimit <= 0 {
		req.MemoryLimit = svc.DefaultMemoryLimit
	}
	if req.MemoryLimit > MaxMemoryLimit {
		req.MemoryLimit = MaxMemoryLimit
	}
	if req.EntryPoint == "" {
		req.EntryPoint = "main"
	}

	// Validate secret access
	for _, secretName := range req.Secrets {
		if !e.isSecretAllowed(svc, secretName) {
			return nil, fmt.Errorf("%w: %s cannot access secret %s", ErrSecretAccessDenied, req.ServiceID, secretName)
		}
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	result := &ExecutionResult{
		Status:    ExecutionStatusRunning,
		StartedAt: time.Now(),
	}

	// Resolve secrets for this execution
	var secrets map[string]string
	if len(req.Secrets) > 0 {
		resolver := e.GetSecretResolver(req.ServiceID)
		var err error
		secrets, err = resolver.Resolve(execCtx, req.AccountID, req.Secrets)
		if err != nil {
			result.Status = ExecutionStatusFailed
			result.Error = fmt.Sprintf("resolve secrets: %v", err)
			result.CompletedAt = time.Now()
			result.Duration = result.CompletedAt.Sub(result.StartedAt)
			return result, nil
		}
	}

	// Execute script in sandboxed environment
	execResult, err := e.scriptEngine.Execute(execCtx, ScriptExecutionRequest{
		Script:      req.Script,
		EntryPoint:  req.EntryPoint,
		Input:       req.Input,
		Secrets:     secrets,
		MemoryLimit: req.MemoryLimit,
	})

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.Status = ExecutionStatusTimeout
			result.Error = ErrExecutionTimeout.Error()
		} else {
			result.Status = ExecutionStatusFailed
			result.Error = err.Error()
		}
		return result, nil
	}

	result.Status = ExecutionStatusSucceeded
	result.Output = execResult.Output
	result.Logs = execResult.Logs
	result.MemoryUsed = execResult.MemoryUsed

	// Generate attestation hash for this execution
	if attestHash, err := e.attestor.HashExecution(ctx, req, result); err == nil {
		result.AttestationHash = attestHash
	}

	return result, nil
}

func (e *engineImpl) isSecretAllowed(svc *ServiceRegistration, secretName string) bool {
	for _, pattern := range svc.AllowedSecretPatterns {
		if pattern == "*" {
			return true
		}
		if matchPattern(pattern, secretName) {
			return true
		}
	}
	return false
}

// matchPattern matches a secret name against a pattern with wildcards.
func matchPattern(pattern, name string) bool {
	if pattern == name {
		return true
	}
	// Simple wildcard matching: "prefix_*" matches "prefix_anything"
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	return false
}

// scopedSecretResolver implements SecretResolver for a specific service.
type scopedSecretResolver struct {
	serviceID string
	vault     SecretVault
	engine    *engineImpl
}

func (r *scopedSecretResolver) ServiceID() string {
	return r.serviceID
}

func (r *scopedSecretResolver) Resolve(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	// Validate service is registered
	r.engine.mu.RLock()
	svc, ok := r.engine.services[r.serviceID]
	r.engine.mu.RUnlock()

	if !ok {
		return nil, ErrServiceNotRegistered
	}

	// Validate each secret is allowed
	for _, name := range names {
		if !r.engine.isSecretAllowed(svc, name) {
			return nil, fmt.Errorf("%w: %s", ErrSecretAccessDenied, name)
		}
	}

	// Resolve from vault with service namespace
	return r.vault.GetSecrets(ctx, r.serviceID, accountID, names)
}
