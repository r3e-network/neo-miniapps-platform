// Package supabase provides Secrets-specific database operations.
package supabase

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/internal/database"
)

const (
	secretsTable  = "secrets"
	policiesTable = "secret_policies"
	auditTable    = "secret_audit_logs"
)

// RepositoryInterface defines Secrets-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	// Secret Operations
	GetSecrets(ctx context.Context, userID string) ([]Secret, error)
	GetSecretByName(ctx context.Context, userID, name string) (*Secret, error)
	CreateSecret(ctx context.Context, secret *Secret) error
	UpdateSecret(ctx context.Context, secret *Secret) error
	DeleteSecret(ctx context.Context, userID, name string) error
	// Policy Operations
	GetPolicies(ctx context.Context, userID string) ([]Policy, error)
	CreatePolicy(ctx context.Context, policy *Policy) error
	DeletePolicy(ctx context.Context, id, userID string) error
	GetPoliciesForSecret(ctx context.Context, userID, secretName string) ([]Policy, error)
	GetAllowedServices(ctx context.Context, userID, secretName string) ([]string, error)
	SetAllowedServices(ctx context.Context, userID, secretName string, services []string) error
	// Audit Log Operations
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	GetAuditLogs(ctx context.Context, userID string, limit int) ([]AuditLog, error)
	GetAuditLogsForSecret(ctx context.Context, userID, secretName string, limit int) ([]AuditLog, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides Secrets-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new Secrets repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// =============================================================================
// Secret Operations
// =============================================================================

// GetSecrets retrieves all secrets for a user.
func (r *Repository) GetSecrets(ctx context.Context, userID string) ([]Secret, error) {
	return database.GenericListByField[Secret](r.base, ctx, secretsTable, "user_id", userID)
}

// CreateSecret creates a new secret.
func (r *Repository) CreateSecret(ctx context.Context, secret *Secret) error {
	if secret == nil {
		return fmt.Errorf("secret cannot be nil")
	}
	if secret.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if secret.Name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, secretsTable, secret, nil)
}

// GetSecretByName retrieves a secret by user ID and name.
func (r *Repository) GetSecretByName(ctx context.Context, userID, name string) (*Secret, error) {
	if userID == "" || name == "" {
		return nil, fmt.Errorf("user_id and name cannot be empty")
	}

	query := database.NewQuery().
		Eq("user_id", userID).
		Eq("name", name).
		Limit(1).
		Build()

	rows, err := database.GenericListWithQuery[Secret](r.base, ctx, secretsTable, query)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil // Not found
	}
	return &rows[0], nil
}

// UpdateSecret updates an existing secret.
func (r *Repository) UpdateSecret(ctx context.Context, secret *Secret) error {
	if secret == nil {
		return fmt.Errorf("secret cannot be nil")
	}
	if secret.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if secret.Name == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	query := database.NewQuery().
		Eq("user_id", secret.UserID).
		Eq("name", secret.Name).
		Build()

	_, err := r.base.Request(ctx, "PATCH", secretsTable, secret, query)
	if err != nil {
		return fmt.Errorf("update secret: %w", err)
	}
	return nil
}

// DeleteSecret deletes a secret by user ID and name.
func (r *Repository) DeleteSecret(ctx context.Context, userID, name string) error {
	if userID == "" || name == "" {
		return fmt.Errorf("user_id and name cannot be empty")
	}

	query := database.NewQuery().
		Eq("user_id", userID).
		Eq("name", name).
		Build()

	_, err := r.base.Request(ctx, "DELETE", secretsTable, nil, query)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// =============================================================================
// Policy Operations
// =============================================================================

// GetPolicies retrieves all policies for a user.
func (r *Repository) GetPolicies(ctx context.Context, userID string) ([]Policy, error) {
	return database.GenericListByField[Policy](r.base, ctx, policiesTable, "user_id", userID)
}

// CreatePolicy creates a new secret policy.
func (r *Repository) CreatePolicy(ctx context.Context, policy *Policy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}
	if policy.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if policy.SecretName == "" {
		return fmt.Errorf("secret_name cannot be empty")
	}
	if policy.ServiceID == "" {
		return fmt.Errorf("service_id cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, policiesTable, policy, nil)
}

// DeletePolicy deletes a secret policy.
func (r *Repository) DeletePolicy(ctx context.Context, id, userID string) error {
	if id == "" || userID == "" {
		return fmt.Errorf("id and user_id cannot be empty")
	}

	query := database.NewQuery().
		Eq("id", id).
		Eq("user_id", userID).
		Build()

	_, err := r.base.Request(ctx, "DELETE", policiesTable, nil, query)
	if err != nil {
		return fmt.Errorf("delete secret policy: %w", err)
	}
	return nil
}

// GetPoliciesForSecret retrieves policies for a specific secret.
func (r *Repository) GetPoliciesForSecret(ctx context.Context, userID, secretName string) ([]Policy, error) {
	if userID == "" || secretName == "" {
		return nil, fmt.Errorf("user_id and secret_name cannot be empty")
	}

	query := database.NewQuery().
		Eq("user_id", userID).
		Eq("secret_name", secretName).
		Build()

	return database.GenericListWithQuery[Policy](r.base, ctx, policiesTable, query)
}

// GetAllowedServices returns the list of service IDs allowed to access a user's secret.
func (r *Repository) GetAllowedServices(ctx context.Context, userID, secretName string) ([]string, error) {
	policies, err := r.GetPoliciesForSecret(ctx, userID, secretName)
	if err != nil {
		return nil, err
	}

	services := make([]string, 0, len(policies))
	for _, p := range policies {
		services = append(services, p.ServiceID)
	}
	return services, nil
}

// SetAllowedServices replaces the allowed service list for a user's secret.
func (r *Repository) SetAllowedServices(ctx context.Context, userID, secretName string, services []string) error {
	if userID == "" || secretName == "" {
		return fmt.Errorf("user_id and secret_name cannot be empty")
	}

	// Remove existing policies
	query := database.NewQuery().
		Eq("user_id", userID).
		Eq("secret_name", secretName).
		Build()

	_, err := r.base.Request(ctx, "DELETE", policiesTable, nil, query)
	if err != nil {
		return fmt.Errorf("delete secret policies: %w", err)
	}

	// Insert new policies
	if len(services) == 0 {
		return nil
	}
	rows := make([]Policy, 0, len(services))
	for _, svc := range services {
		if svc == "" {
			continue
		}
		rows = append(rows, Policy{UserID: userID, SecretName: secretName, ServiceID: svc})
	}
	if len(rows) == 0 {
		return nil
	}

	_, err = r.base.Request(ctx, "POST", policiesTable, rows, "")
	if err != nil {
		return fmt.Errorf("create secret policies: %w", err)
	}
	return nil
}

// =============================================================================
// Audit Log Operations
// =============================================================================

// CreateAuditLog creates a new audit log entry.
func (r *Repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	if log == nil {
		return fmt.Errorf("audit log cannot be nil")
	}
	if log.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}
	if log.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}
	return database.GenericCreate(r.base, ctx, auditTable, log, nil)
}

// GetAuditLogs retrieves audit logs for a user with optional limit.
func (r *Repository) GetAuditLogs(ctx context.Context, userID string, limit int) ([]AuditLog, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id cannot be empty")
	}

	qb := database.NewQuery().
		Eq("user_id", userID).
		OrderDesc("created_at")
	if limit > 0 {
		qb.Limit(limit)
	}

	return database.GenericListWithQuery[AuditLog](r.base, ctx, auditTable, qb.Build())
}

// GetAuditLogsForSecret retrieves audit logs for a specific secret with optional limit.
func (r *Repository) GetAuditLogsForSecret(ctx context.Context, userID, secretName string, limit int) ([]AuditLog, error) {
	if userID == "" || secretName == "" {
		return nil, fmt.Errorf("user_id and secret_name cannot be empty")
	}

	qb := database.NewQuery().
		Eq("user_id", userID).
		Eq("secret_name", secretName).
		OrderDesc("created_at")
	if limit > 0 {
		qb.Limit(limit)
	}

	return database.GenericListWithQuery[AuditLog](r.base, ctx, auditTable, qb.Build())
}
