package tee

import (
	"context"
)

// EnclaveRuntime abstracts the TEE runtime environment.
// In simulation mode, this is a no-op wrapper.
// In hardware mode, this interfaces with Rust SGX SDK via CGO.
type EnclaveRuntime interface {
	// Initialize sets up the enclave runtime.
	Initialize(ctx context.Context) error

	// Shutdown tears down the enclave runtime.
	Shutdown(ctx context.Context) error

	// Health checks if the runtime is operational.
	Health(ctx context.Context) error

	// Mode returns the current enclave mode.
	Mode() EnclaveMode

	// SealData encrypts data using enclave sealing key.
	SealData(ctx context.Context, data []byte) ([]byte, error)

	// UnsealData decrypts data using enclave sealing key.
	UnsealData(ctx context.Context, sealed []byte) ([]byte, error)
}

// SecretVault manages encrypted secrets within the TEE.
// Secrets are isolated by service namespace and account.
type SecretVault interface {
	// Initialize sets up the vault.
	Initialize(ctx context.Context) error

	// Shutdown tears down the vault.
	Shutdown(ctx context.Context) error

	// StoreSecret stores an encrypted secret.
	// Key format: {service_id}:{account_id}:{secret_name}
	StoreSecret(ctx context.Context, serviceID, accountID, name string, value []byte) error

	// GetSecret retrieves and decrypts a single secret.
	GetSecret(ctx context.Context, serviceID, accountID, name string) ([]byte, error)

	// GetSecrets retrieves multiple secrets at once.
	GetSecrets(ctx context.Context, serviceID, accountID string, names []string) (map[string]string, error)

	// DeleteSecret removes a secret.
	DeleteSecret(ctx context.Context, serviceID, accountID, name string) error

	// ListSecrets lists secret names (not values) for a service/account.
	ListSecrets(ctx context.Context, serviceID, accountID string) ([]string, error)

	// GrantAccess allows another service to access a secret.
	GrantAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error

	// RevokeAccess removes access grant.
	RevokeAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error
}

// Attestor generates and verifies TEE attestation reports.
type Attestor interface {
	// GenerateReport creates an attestation report for the current enclave state.
	GenerateReport(ctx context.Context) (*AttestationReport, error)

	// VerifyReport verifies an attestation report.
	VerifyReport(ctx context.Context, report *AttestationReport) (bool, error)

	// HashExecution creates a hash of an execution for audit trail.
	HashExecution(ctx context.Context, req ExecutionRequest, result *ExecutionResult) (string, error)
}

// ScriptEngine executes JavaScript code in a sandboxed environment.
// Uses V8 isolates for memory isolation and security.
type ScriptEngine interface {
	// Initialize sets up the script engine.
	Initialize(ctx context.Context) error

	// Shutdown tears down the script engine.
	Shutdown(ctx context.Context) error

	// Execute runs a script and returns the result.
	Execute(ctx context.Context, req ScriptExecutionRequest) (*ScriptExecutionResult, error)

	// ValidateScript checks if a script is syntactically valid.
	ValidateScript(ctx context.Context, script string) error
}

// ScriptExecutionRequest contains parameters for script execution.
type ScriptExecutionRequest struct {
	// Script is the JavaScript source code
	Script string

	// EntryPoint is the function to call
	EntryPoint string

	// Input is passed to the function as the first argument
	Input map[string]any

	// Secrets are available via the `secrets` global object
	Secrets map[string]string

	// MemoryLimit for the V8 isolate
	MemoryLimit int64
}

// ScriptExecutionResult contains the output of script execution.
type ScriptExecutionResult struct {
	// Output is the return value of the function
	Output map[string]any

	// Logs captured from console.log calls
	Logs []string

	// MemoryUsed by the V8 isolate
	MemoryUsed int64
}

// SecretAccessGrant represents a cross-service secret access permission.
type SecretAccessGrant struct {
	OwnerServiceID  string `json:"owner_service_id"`
	TargetServiceID string `json:"target_service_id"`
	AccountID       string `json:"account_id"`
	SecretName      string `json:"secret_name"`
	GrantedAt       int64  `json:"granted_at"`
	ExpiresAt       int64  `json:"expires_at,omitempty"` // 0 = never expires
}
