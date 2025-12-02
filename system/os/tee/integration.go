package tee

import (
	"context"
	"fmt"
	"sync"
)

// EngineProvider provides access to the global TEE engine instance.
// Services use this to execute confidential computations.
type EngineProvider interface {
	// GetEngine returns the TEE engine instance.
	GetEngine() Engine

	// GetSecretManager returns the secret manager.
	GetSecretManager() *SecretManager

	// ExecuteConfidential executes a script in the TEE for a service.
	ExecuteConfidential(ctx context.Context, req ConfidentialRequest) (*ExecutionResult, error)
}

// ConfidentialRequest is a simplified request for services to use TEE.
type ConfidentialRequest struct {
	// ServiceID is automatically set from the calling service
	ServiceID string `json:"-"`

	// AccountID for the execution context
	AccountID string `json:"account_id"`

	// Script to execute (JavaScript)
	Script string `json:"script"`

	// EntryPoint function name (default: "main")
	EntryPoint string `json:"entry_point"`

	// Input data passed to the function
	Input map[string]any `json:"input"`

	// Secrets to make available (must be allowed by service policy)
	Secrets []string `json:"secrets"`

	// Metadata for tracing
	Metadata map[string]string `json:"metadata"`
}

// globalProvider is the singleton TEE engine provider.
var (
	globalProvider     *engineProvider
	globalProviderOnce sync.Once
	globalProviderMu   sync.RWMutex
)

// engineProvider implements EngineProvider.
type engineProvider struct {
	engine        Engine
	secretManager *SecretManager
	config        ProviderConfig
}

// ProviderConfig configures the TEE engine provider.
type ProviderConfig struct {
	// Mode: simulation or hardware
	Mode EnclaveMode `json:"mode"`

	// EnclaveID for this instance (auto-generated if empty)
	EnclaveID string `json:"enclave_id"`

	// SecretEncryptionKey for the vault
	SecretEncryptionKey []byte `json:"-"`

	// MaxConcurrentExecutions globally
	MaxConcurrentExecutions int `json:"max_concurrent_executions"`

	// V8HeapSize for script engine
	V8HeapSize int64 `json:"v8_heap_size"`

	// RegisterDefaultPolicies registers default service policies
	RegisterDefaultPolicies bool `json:"register_default_policies"`
}

// InitializeProvider initializes the global TEE engine provider.
// This should be called once during application startup.
func InitializeProvider(config ProviderConfig) error {
	globalProviderMu.Lock()
	defer globalProviderMu.Unlock()

	if globalProvider != nil {
		return fmt.Errorf("TEE provider already initialized")
	}

	// Create engine
	engine, err := NewEngine(EngineConfig{
		Mode:                    config.Mode,
		EnclaveID:               config.EnclaveID,
		SecretEncryptionKey:     config.SecretEncryptionKey,
		MaxConcurrentExecutions: config.MaxConcurrentExecutions,
		V8HeapSize:              config.V8HeapSize,
	})
	if err != nil {
		return fmt.Errorf("create engine: %w", err)
	}

	// Create secret vault and manager
	var vault SecretVault
	if config.Mode == EnclaveModeSimulation {
		vault = newSimulationVault(config.SecretEncryptionKey)
	} else {
		return fmt.Errorf("hardware mode not yet implemented")
	}

	secretManager := NewSecretManager(vault)

	// Register default policies if requested
	if config.RegisterDefaultPolicies {
		for _, policy := range DefaultServicePolicies() {
			if err := secretManager.RegisterPolicy(policy); err != nil {
				return fmt.Errorf("register policy for %s: %w", policy.ServiceID, err)
			}
		}
	}

	globalProvider = &engineProvider{
		engine:        engine,
		secretManager: secretManager,
		config:        config,
	}

	return nil
}

// GetProvider returns the global TEE engine provider.
// Returns nil if not initialized.
func GetProvider() EngineProvider {
	globalProviderMu.RLock()
	defer globalProviderMu.RUnlock()
	return globalProvider
}

// MustGetProvider returns the global provider or panics if not initialized.
func MustGetProvider() EngineProvider {
	p := GetProvider()
	if p == nil {
		panic("TEE provider not initialized - call InitializeProvider first")
	}
	return p
}

// StartProvider starts the TEE engine.
func StartProvider(ctx context.Context) error {
	p := GetProvider()
	if p == nil {
		return fmt.Errorf("TEE provider not initialized")
	}
	return p.GetEngine().Start(ctx)
}

// StopProvider stops the TEE engine.
func StopProvider(ctx context.Context) error {
	p := GetProvider()
	if p == nil {
		return nil
	}
	return p.GetEngine().Stop(ctx)
}

func (p *engineProvider) GetEngine() Engine {
	return p.engine
}

func (p *engineProvider) GetSecretManager() *SecretManager {
	return p.secretManager
}

func (p *engineProvider) ExecuteConfidential(ctx context.Context, req ConfidentialRequest) (*ExecutionResult, error) {
	if req.ServiceID == "" {
		return nil, fmt.Errorf("service_id required")
	}

	return p.engine.Execute(ctx, ExecutionRequest{
		ServiceID:  req.ServiceID,
		AccountID:  req.AccountID,
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Input:      req.Input,
		Secrets:    req.Secrets,
		Metadata:   req.Metadata,
	})
}

// ServiceTEEAdapter provides a service-specific interface to the TEE engine.
// Each service gets its own adapter with pre-configured service ID.
type ServiceTEEAdapter struct {
	serviceID     string
	provider      EngineProvider
	secretManager *SecretManager
}

// NewServiceTEEAdapter creates a TEE adapter for a specific service.
func NewServiceTEEAdapter(serviceID string) *ServiceTEEAdapter {
	return &ServiceTEEAdapter{
		serviceID: serviceID,
	}
}

// Initialize connects the adapter to the global provider.
// Call this after the provider is initialized.
func (a *ServiceTEEAdapter) Initialize() error {
	p := GetProvider()
	if p == nil {
		return fmt.Errorf("TEE provider not initialized")
	}
	a.provider = p
	a.secretManager = p.GetSecretManager()

	// Register service with engine
	return a.provider.GetEngine().RegisterService(context.Background(), ServiceRegistration{
		ServiceID:               a.serviceID,
		AllowedSecretPatterns:   []string{"*"}, // Will be filtered by SecretManager
		MaxConcurrentExecutions: DefaultMaxConcurrent,
		DefaultTimeout:          DefaultExecutionTimeout,
		DefaultMemoryLimit:      DefaultMemoryLimit,
	})
}

// Execute runs a script in the TEE.
func (a *ServiceTEEAdapter) Execute(ctx context.Context, accountID, script, entryPoint string, input map[string]any, secrets []string) (*ExecutionResult, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}

	return a.provider.ExecuteConfidential(ctx, ConfidentialRequest{
		ServiceID:  a.serviceID,
		AccountID:  accountID,
		Script:     script,
		EntryPoint: entryPoint,
		Input:      input,
		Secrets:    secrets,
	})
}

// StoreSecret stores a secret for this service.
func (a *ServiceTEEAdapter) StoreSecret(ctx context.Context, accountID, name string, value []byte) error {
	if a.secretManager == nil {
		return fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.StoreSecret(ctx, a.serviceID, accountID, name, value, nil)
}

// GetSecret retrieves a secret for this service.
func (a *ServiceTEEAdapter) GetSecret(ctx context.Context, accountID, name string) ([]byte, error) {
	if a.secretManager == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.GetSecret(ctx, a.serviceID, accountID, name)
}

// GetSecrets retrieves multiple secrets for this service.
func (a *ServiceTEEAdapter) GetSecrets(ctx context.Context, accountID string, names []string) (map[string]string, error) {
	if a.secretManager == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.GetSecrets(ctx, a.serviceID, accountID, names)
}

// DeleteSecret removes a secret for this service.
func (a *ServiceTEEAdapter) DeleteSecret(ctx context.Context, accountID, name string) error {
	if a.secretManager == nil {
		return fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.DeleteSecret(ctx, a.serviceID, accountID, name)
}

// ListSecrets lists secret names for this service.
func (a *ServiceTEEAdapter) ListSecrets(ctx context.Context, accountID string) ([]string, error) {
	if a.secretManager == nil {
		return nil, fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.ListSecrets(ctx, a.serviceID, accountID)
}

// GrantAccess grants another service access to a secret.
func (a *ServiceTEEAdapter) GrantAccess(ctx context.Context, targetServiceID, accountID, secretPattern string) error {
	if a.secretManager == nil {
		return fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.GrantAccess(ctx, SecretGrant{
		OwnerServiceID:  a.serviceID,
		TargetServiceID: targetServiceID,
		AccountID:       accountID,
		SecretPattern:   secretPattern,
	})
}

// RevokeAccess revokes another service's access to a secret.
func (a *ServiceTEEAdapter) RevokeAccess(ctx context.Context, targetServiceID, accountID, secretPattern string) error {
	if a.secretManager == nil {
		return fmt.Errorf("adapter not initialized")
	}
	return a.secretManager.RevokeAccess(ctx, a.serviceID, targetServiceID, accountID, secretPattern)
}

// GetSecretResolver returns a SecretResolver for use in TEE executions.
func (a *ServiceTEEAdapter) GetSecretResolver() SecretResolver {
	if a.secretManager == nil {
		return nil
	}
	return NewServiceSecretResolver(a.serviceID, a.secretManager)
}
