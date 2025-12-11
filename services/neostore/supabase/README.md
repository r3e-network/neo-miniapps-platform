# NeoStore Supabase Repository

Database layer for the NeoStore secrets management service.

## Overview

This package provides NeoStore-specific data access for secrets, access policies, and audit logs.

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Data models |

## Data Models

### Secret

Represents an encrypted secret stored in the database.

```go
type Secret struct {
    ID             string    `json:"id"`
    UserID         string    `json:"user_id"`
    Name           string    `json:"name"`
    EncryptedValue []byte    `json:"encrypted_value"`
    Version        int       `json:"version"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

### Policy

Defines which services are allowed to access a specific secret.

```go
type Policy struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    SecretName string    `json:"secret_name"`
    ServiceID  string    `json:"service_id"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### AuditLog

Records all operations performed on secrets for security compliance.

```go
type AuditLog struct {
    ID           string    `json:"id"`
    UserID       string    `json:"user_id"`
    SecretName   string    `json:"secret_name"`
    Action       string    `json:"action"`
    ServiceID    string    `json:"service_id,omitempty"`
    IPAddress    string    `json:"ip_address,omitempty"`
    UserAgent    string    `json:"user_agent,omitempty"`
    Success      bool      `json:"success"`
    ErrorMessage string    `json:"error_message,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
}
```

## Repository Interface

```go
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
```

## Database Tables

| Table | Purpose |
|-------|---------|
| `secrets` | Encrypted secret storage |
| `secret_policies` | Service access permissions |
| `secret_audit_logs` | Operation audit trail |

## Usage

```go
import neostoresupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"

repo := neostoresupabase.NewRepository(baseRepo)

// Create a secret
err := repo.CreateSecret(ctx, &neostoresupabase.Secret{
    ID:             uuid.New().String(),
    UserID:         userID,
    Name:           "api_key",
    EncryptedValue: encryptedData,
    Version:        1,
})

// Get secret by name
secret, err := repo.GetSecretByName(ctx, userID, "api_key")

// Set allowed services
err := repo.SetAllowedServices(ctx, userID, "api_key", []string{"neoflow", "neocompute"})

// Get audit logs
logs, err := repo.GetAuditLogs(ctx, userID, 100)
```

## Audit Log Actions

| Action | Description |
|--------|-------------|
| `create` | Secret created |
| `read` | Secret value retrieved |
| `update` | Secret value updated |
| `delete` | Secret deleted |
| `grant` | Service access granted |
| `revoke` | Service access revoked |

## Query Builder Usage

The repository uses the internal query builder for complex queries:

```go
query := database.NewQuery().
    Eq("user_id", userID).
    Eq("name", name).
    Limit(1).
    Build()

rows, err := database.GenericListWithQuery[Secret](r.base, ctx, secretsTable, query)
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
