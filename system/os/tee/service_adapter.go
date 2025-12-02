package tee

import (
	"context"
	"time"
)

// ServiceProviderAdapter adapts the TEE Engine to the service.TEEProvider interface.
// This allows services to use TEE functionality through the standard ServiceDependencies.
type ServiceProviderAdapter struct {
	engine        Engine
	secretManager *SecretManager
}

// NewServiceProviderAdapter creates a new adapter for the TEE engine.
func NewServiceProviderAdapter(engine Engine, secretManager *SecretManager) *ServiceProviderAdapter {
	return &ServiceProviderAdapter{
		engine:        engine,
		secretManager: secretManager,
	}
}

// ServiceExecutionRequest mirrors service.TEEExecutionRequest for type conversion.
type ServiceExecutionRequest struct {
	ServiceID  string
	AccountID  string
	Script     string
	EntryPoint string
	Input      map[string]any
	Secrets    []string
	Metadata   map[string]string
}

// ServiceExecutionResult mirrors service.TEEExecutionResult for type conversion.
type ServiceExecutionResult struct {
	Output map[string]any
	Logs   []string
	Error  string
	Status string
}

// Execute runs a JavaScript function within the TEE enclave.
func (a *ServiceProviderAdapter) Execute(ctx context.Context, req ServiceExecutionRequest) (*ServiceExecutionResult, error) {
	result, err := a.engine.Execute(ctx, ExecutionRequest{
		ServiceID:  req.ServiceID,
		AccountID:  req.AccountID,
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Input:      req.Input,
		Secrets:    req.Secrets,
		Metadata:   req.Metadata,
	})
	if err != nil {
		return nil, err
	}

	return &ServiceExecutionResult{
		Output: result.Output,
		Logs:   result.Logs,
		Error:  result.Error,
		Status: string(result.Status),
	}, nil
}

// StoreSecret stores a secret for a service/account in the TEE vault.
func (a *ServiceProviderAdapter) StoreSecret(ctx context.Context, serviceID, accountID, name string, value []byte) error {
	return a.secretManager.StoreSecret(ctx, serviceID, accountID, name, value, nil)
}

// GetSecret retrieves a secret from the TEE vault.
func (a *ServiceProviderAdapter) GetSecret(ctx context.Context, serviceID, accountID, name string) ([]byte, error) {
	return a.secretManager.GetSecret(ctx, serviceID, accountID, name)
}

// DeleteSecret removes a secret from the TEE vault.
func (a *ServiceProviderAdapter) DeleteSecret(ctx context.Context, serviceID, accountID, name string) error {
	return a.secretManager.DeleteSecret(ctx, serviceID, accountID, name)
}

// ListSecrets lists secret names for a service/account.
func (a *ServiceProviderAdapter) ListSecrets(ctx context.Context, serviceID, accountID string) ([]string, error) {
	return a.secretManager.ListSecrets(ctx, serviceID, accountID)
}

// GrantAccess grants another service access to a secret.
func (a *ServiceProviderAdapter) GrantAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretPattern string) error {
	return a.secretManager.GrantAccess(ctx, SecretGrant{
		OwnerServiceID:  ownerServiceID,
		TargetServiceID: targetServiceID,
		AccountID:       accountID,
		SecretPattern:   secretPattern,
		GrantedAt:       time.Now(),
	})
}

// RevokeAccess revokes another service's access to a secret.
func (a *ServiceProviderAdapter) RevokeAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretPattern string) error {
	return a.secretManager.RevokeAccess(ctx, ownerServiceID, targetServiceID, accountID, secretPattern)
}

// Health checks if the TEE enclave is operational.
func (a *ServiceProviderAdapter) Health(ctx context.Context) error {
	return a.engine.Health(ctx)
}

// RegisterService registers a service with the TEE engine.
// This should be called during service initialization.
func (a *ServiceProviderAdapter) RegisterService(ctx context.Context, serviceID string, allowedSecretPatterns []string) error {
	// Register with engine
	if err := a.engine.RegisterService(ctx, ServiceRegistration{
		ServiceID:               serviceID,
		AllowedSecretPatterns:   allowedSecretPatterns,
		MaxConcurrentExecutions: DefaultMaxConcurrent,
		DefaultTimeout:          DefaultExecutionTimeout,
		DefaultMemoryLimit:      DefaultMemoryLimit,
	}); err != nil {
		return err
	}

	// Register policy with secret manager
	return a.secretManager.RegisterPolicy(SecretPolicy{
		ServiceID:       serviceID,
		AllowedPatterns: allowedSecretPatterns,
		MaxSecrets:      100,
		CanGrantAccess:  false,
	})
}

// Start initializes the TEE engine.
func (a *ServiceProviderAdapter) Start(ctx context.Context) error {
	return a.engine.Start(ctx)
}

// Stop gracefully shuts down the TEE engine.
func (a *ServiceProviderAdapter) Stop(ctx context.Context) error {
	return a.engine.Stop(ctx)
}
