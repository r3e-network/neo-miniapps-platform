# NeoStore Marble Service

TEE-secured secrets management service running inside MarbleRun enclave.

## Overview

The NeoStore Marble service implements encrypted secrets management:
1. Users store secrets via API (API keys, credentials, etc.)
2. TEE encrypts secrets with AES-256-GCM using derived keys
3. Secrets are only decrypted inside the TEE when accessed
4. Service-to-service access is controlled via policies

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                    │
│                                                               │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   Handler   │    │  Crypto     │    │   Policy    │        │
│  │  (REST API) │───>│  (AES-GCM)  │    │  Enforcer   │        │
│  └─────────────┘    └──────┬──────┘    └──────┬──────┘        │
│         │                  │                  │               │
│  ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐        │
│  │   Audit     │    │  Master Key │    │  Supabase   │        │
│  │   Logger    │    │  (Sealed)   │    │ Repository  │        │
│  └─────────────┘    └─────────────┘    └─────────────┘        │
└───────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │    Supabase     │
                    │  (Encrypted at  │
                    │     Rest)       │
                    └─────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization and encryption |
| `lifecycle.go` | Service lifecycle (Start/Stop) |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `types.go` | Request/response types |

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    db         Store
    encryptKey []byte
}
```

### Store Interface

```go
type Store interface {
    GetSecrets(ctx context.Context, userID string) ([]Secret, error)
    GetSecretByName(ctx context.Context, userID, name string) (*Secret, error)
    CreateSecret(ctx context.Context, secret *Secret) error
    UpdateSecret(ctx context.Context, secret *Secret) error
    DeleteSecret(ctx context.Context, userID, name string) error
    GetAllowedServices(ctx context.Context, userID, secretName string) ([]string, error)
    SetAllowedServices(ctx context.Context, userID, secretName string, services []string) error
    CreateAuditLog(ctx context.Context, log *AuditLog) error
    GetAuditLogs(ctx context.Context, userID string, limit int) ([]AuditLog, error)
    GetAuditLogsForSecret(ctx context.Context, userID, secretName string, limit int) ([]AuditLog, error)
}
```

## Security Features

### Encryption

- **Algorithm**: AES-256-GCM
- **Key**: 32-byte master key from MarbleRun secrets
- **Nonce**: Random 12 bytes per encryption

### Service-to-Service Access

Allowed internal services:
- `neooracle`
- `neocompute`
- `neoflow`
- `neovault`
- `neorand`

### Policy Enforcement

Users can grant specific services access to individual secrets:

```go
// Set allowed services for a secret
POST /neostore/{name}/permissions
{
    "services": ["neoflow", "neocompute"]
}
```

### Audit Logging

All secret operations are logged:
- Action type (create, read, update, delete)
- Service ID (for service-to-service calls)
- IP address and User-Agent
- Success/failure status

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/neostore` | GET | List secrets (metadata) |
| `/neostore` | POST | Create/update secret |
| `/neostore/{name}` | GET | Get secret value |
| `/neostore/{name}` | DELETE | Delete secret |
| `/neostore/{name}/permissions` | GET | Get allowed services |
| `/neostore/{name}/permissions` | PUT | Set allowed services |
| `/neostore/audit` | GET | Get audit logs |
| `/neostore/{name}/audit` | GET | Get secret audit logs |

## Request/Response Types

### SecretRecord

```go
type SecretRecord struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Version   int       `json:"version"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### CreateSecretInput

```go
type CreateSecretInput struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

### GetSecretResponse

```go
type GetSecretResponse struct {
    Name    string `json:"name"`
    Value   string `json:"value"`
    Version int    `json:"version"`
}
```

## Configuration

```go
type Config struct {
    Marble     *marble.Marble
    DB         Store
    EncryptKey []byte // optional override
}
```

### Required Secrets

| Secret | Description |
|--------|-------------|
| `SECRETS_MASTER_KEY` | 32-byte AES-256 encryption key |

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `ServiceID` | `neostore` | Service identifier |
| `ServiceName` | `NeoStore Service` | Display name |
| `Version` | `1.0.0` | Service version |
| `ServiceIDHeader` | `X-Service-ID` | Service auth header |

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/crypto` | AES-GCM encryption/decryption |
| `internal/marble` | MarbleRun TEE utilities |
| `internal/httputil` | HTTP response helpers |
| `services/common/service` | Base service framework |
| `services/neostore/supabase` | Database repository |

### External Packages

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP router |
| `github.com/google/uuid` | UUID generation |

## Related Documentation

- [NeoStore Service Overview](../README.md)
- [Database Layer](../supabase/README.md)
