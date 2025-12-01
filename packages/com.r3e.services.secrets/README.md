# Secrets Service

Enterprise-grade secret management service with encryption, access control, and service-to-service resolution capabilities. Aligned with the SecretsVault.cs smart contract for on-chain/off-chain secret coordination.

## Overview

The Secrets Service provides secure storage and retrieval of sensitive configuration data (API keys, credentials, tokens) with fine-grained access control. Secrets are encrypted at rest using AES-GCM and can be selectively exposed to other services (Oracle, Automation, Functions, JAM) via ACL flags.

**Package ID:** `com.r3e.services.secrets`
**Version:** 1.0.0
**License:** MIT

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP API Layer                            │
│  GET/POST /secrets  │  GET/PUT/DELETE /secrets/{name}           │
└────────────────┬────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│                      Service Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Service    │  │   Resolver   │  │ ServiceEngine│          │
│  │  (CRUD ops)  │  │ (ACL checks) │  │ (validation) │          │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┘          │
│         │                  │                                      │
│         └──────────┬───────┘                                     │
└────────────────────┼─────────────────────────────────────────────┘
                     │
         ┌───────────┴───────────┐
         │                       │
┌────────▼────────┐    ┌────────▼────────┐
│  Cipher Layer   │    │  Store Layer    │
│  ┌───────────┐  │    │  ┌───────────┐  │
│  │ AES-GCM   │  │    │  │ Postgres  │  │
│  │ Noop      │  │    │  │   Store   │  │
│  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘
         │                       │
         └───────────┬───────────┘
                     │
            ┌────────▼────────┐
            │   PostgreSQL    │
            │  (encrypted     │
            │   base64 blobs) │
            └─────────────────┘
```

## Key Components

### 1. Service (`service.go`)

Core business logic implementing CRUD operations and secret resolution.

**Responsibilities:**
- Account validation via `ServiceEngine`
- Secret lifecycle management (Create, Update, Get, List, Delete)
- Encryption/decryption orchestration
- ACL enforcement for service-to-service access
- Observability (metrics, logging)

**Key Methods:**
- `Create(ctx, accountID, name, value)` - Store new secret
- `CreateWithOptions(ctx, accountID, name, value, opts)` - Store with ACL
- `Update(ctx, accountID, name, value)` - Replace secret value
- `UpdateWithOptions(ctx, accountID, name, opts)` - Update value and/or ACL
- `Get(ctx, accountID, name)` - Retrieve decrypted secret
- `List(ctx, accountID)` - List metadata (no values)
- `Delete(ctx, accountID, name)` - Remove secret
- `ResolveSecrets(ctx, accountID, names)` - Batch resolution (no ACL)
- `ResolveSecretsWithACL(ctx, accountID, names, caller)` - ACL-enforced resolution

### 2. Resolver Interface

Exposes secret lookup for other services with optional ACL enforcement.

```go
type Resolver interface {
    ResolveSecrets(ctx, accountID, names) (map[string]string, error)
    ResolveSecretsWithACL(ctx, accountID, names, caller) (map[string]string, error)
}
```

**Caller Services:**
- `CallerOracle` - Oracle service (ACL flag: 0x01)
- `CallerAutomation` - Automation service (ACL flag: 0x02)
- `CallerFunctions` - Functions service (ACL flag: 0x04)
- `CallerJAM` - JAM service (ACL flag: 0x08)

### 3. Store Interface (`store.go`)

Persistence abstraction for secret storage.

```go
type Store interface {
    CreateSecret(ctx, sec) (Secret, error)
    UpdateSecret(ctx, sec) (Secret, error)
    GetSecret(ctx, accountID, name) (Secret, error)
    ListSecrets(ctx, accountID) ([]Secret, error)
    DeleteSecret(ctx, accountID, name) error
}
```

**Implementation:** `PostgresStore` (`store_postgres.go`)

### 4. Cipher Interface (`cipher.go`)

Encryption abstraction supporting pluggable algorithms.

```go
type Cipher interface {
    Encrypt(plaintext []byte) ([]byte, error)
    Decrypt(ciphertext []byte) ([]byte, error)
}
```

**Implementations:**
- `aesCipher` - AES-256-GCM with random nonces (production)
- `noopCipher` - Pass-through (testing/development only)

**Encryption Flow:**
1. Generate random nonce (12 bytes for GCM)
2. Seal plaintext with AES-GCM
3. Prepend nonce to ciphertext
4. Base64-encode for storage

## Domain Types

### Secret

Complete secret record including encrypted value.

```go
type Secret struct {
    ID        string    // UUID
    AccountID string    // Owner account
    Name      string    // Unique name within account
    Value     string    // Encrypted payload (base64)
    Version   int       // Optimistic locking version
    ACL       ACL       // Access control flags
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Metadata

Public information about a secret (excludes value).

```go
type Metadata struct {
    ID        string    `json:"id"`
    AccountID string    `json:"account_id"`
    Name      string    `json:"name"`
    Version   int       `json:"version"`
    ACL       ACL       `json:"acl"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### ACL (Access Control List)

Bitfield flags controlling service access (aligned with SecretsVault.cs contract).

```go
type ACL byte

const (
    ACLNone             ACL = 0x00  // No service access
    ACLOracleAccess     ACL = 0x01  // Oracle can access
    ACLAutomationAccess ACL = 0x02  // Automation can access
    ACLFunctionAccess   ACL = 0x04  // Functions can access
    ACLJAMAccess        ACL = 0x08  // JAM can access
)
```

**Example:** `ACL = 0x05` grants access to Oracle (0x01) and Functions (0x04).

## API Endpoints

All endpoints require authentication and are scoped to the authenticated account.

### List Secrets

```
GET /secrets
```

**Response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "account_id": "acc_123",
    "name": "api_key",
    "version": 1,
    "acl": 5,
    "created_at": "2025-12-01T10:00:00Z",
    "updated_at": "2025-12-01T10:00:00Z"
  }
]
```

### Create Secret

```
POST /secrets
Content-Type: application/json

{
  "name": "api_key",
  "value": "sk_live_abc123",
  "acl": 5
}
```

**Response:** Metadata object (excludes value)

### Get Secret Metadata

```
GET /secrets/{name}
```

**Response:** Metadata object (value not exposed via HTTP)

### Update Secret

```
PUT /secrets/{name}
Content-Type: application/json

{
  "value": "sk_live_xyz789",
  "acl": 7
}
```

**Fields:**
- `value` (optional) - New encrypted value
- `acl` (optional) - New ACL flags

**Response:** Updated metadata

### Delete Secret

```
DELETE /secrets/{name}
```

**Response:**
```json
{
  "status": "deleted",
  "name": "api_key"
}
```

## Configuration

### Service Initialization

```go
import (
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
)

// Basic setup (noop cipher)
svc := secrets.New(accountChecker, store, logger)

// Production setup with AES-GCM
key := []byte("32-byte-key-for-aes-256-gcm-here")
cipher, err := secrets.NewAESCipher(key)
if err != nil {
    log.Fatal(err)
}
svc := secrets.New(accountChecker, store, logger, secrets.WithCipher(cipher))

// Runtime cipher replacement
svc.SetCipher(newCipher)
```

### Resource Limits (manifest.yaml)

```yaml
resources:
  max_storage_bytes: 52428800        # 50 MB
  max_concurrent_requests: 1000
  max_requests_per_second: 5000
  max_events_per_second: 1000
```

### Required Permissions

- `system.api.storage` - Database access (required)
- `system.api.bus` - Event publishing (optional)

## Dependencies

### External Packages

- `github.com/google/uuid` - UUID generation
- `github.com/R3E-Network/service_layer/pkg/logger` - Structured logging
- `github.com/R3E-Network/service_layer/system/framework` - ServiceEngine
- `github.com/R3E-Network/service_layer/system/framework/core` - API types

### Database

PostgreSQL with the following schema (managed by `store_postgres.go`):

```sql
CREATE TABLE secrets (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    value TEXT NOT NULL,        -- Base64-encoded ciphertext
    version INTEGER NOT NULL,
    acl SMALLINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE(account_id, name)
);

CREATE INDEX idx_secrets_account ON secrets(account_id);
```

## Security Considerations

### Encryption

- **Algorithm:** AES-256-GCM (authenticated encryption)
- **Nonce:** Random 12-byte nonce per encryption operation
- **Key Management:** Caller responsible for secure key storage (consider HSM/KMS)
- **Storage Format:** Base64-encoded `[nonce || ciphertext]`

### Access Control

1. **Account Isolation:** Secrets are strictly scoped to account_id
2. **ACL Enforcement:** `ResolveSecretsWithACL` validates caller permissions
3. **HTTP API:** Never exposes decrypted values via GET endpoints
4. **Name Validation:** Rejects names containing `|` (reserved delimiter)

### Best Practices

- Rotate encryption keys periodically
- Use `ResolveSecretsWithACL` for service-to-service access
- Set minimal ACL flags (principle of least privilege)
- Monitor `secrets_resolved_total` metric for anomalies
- Never log decrypted secret values

## Testing

### Run Unit Tests

```bash
cd /home/neo/git/service_layer/packages/com.r3e.services.secrets
go test -v
```

### Test Coverage

```bash
go test -cover
```

### Example Test Cases (from `service_test.go`)

- Secret CRUD operations
- ACL enforcement scenarios
- Encryption/decryption round-trips
- Concurrent access patterns
- Error handling (invalid names, missing accounts)

### Integration Testing

```go
// Setup test service
store := NewPostgresStore(testDB, mockAccounts)
cipher, _ := NewAESCipher(testKey)
svc := New(mockAccounts, store, testLogger, WithCipher(cipher))

// Test create + resolve
meta, err := svc.Create(ctx, "acc_123", "test_key", "secret_value")
require.NoError(t, err)

resolved, err := svc.ResolveSecrets(ctx, "acc_123", []string{"test_key"})
require.NoError(t, err)
assert.Equal(t, "secret_value", resolved["test_key"])
```

## Observability

### Metrics

The service emits the following metrics via `ServiceEngine`:

- `secrets_created_total{account_id}` - Counter
- `secrets_updated_total{account_id}` - Counter
- `secrets_deleted_total{account_id}` - Counter
- `secrets_resolved_total{account_id, caller}` - Counter

### Logging

Structured logs via `pkg/logger`:

- `LogCreated("secret", id, accountID)` - Secret creation
- `LogUpdated("secret", id, accountID)` - Secret updates
- `LogDeleted("secret", name, accountID)` - Secret deletion

### Tracing

Operations are instrumented via `StartObservation(ctx, attrs)` for distributed tracing.

## Usage Examples

### Basic Secret Management

```go
// Create secret
meta, err := svc.Create(ctx, "acc_123", "stripe_key", "sk_test_abc")

// Update value
meta, err = svc.Update(ctx, "acc_123", "stripe_key", "sk_live_xyz")

// Retrieve secret
secret, err := svc.Get(ctx, "acc_123", "stripe_key")
fmt.Println(secret.Value) // "sk_live_xyz"

// Delete secret
err = svc.Delete(ctx, "acc_123", "stripe_key")
```

### ACL-Based Access

```go
// Create secret with Oracle + Functions access
opts := secrets.CreateOptions{
    ACL: secrets.ACLOracleAccess | secrets.ACLFunctionAccess,
}
meta, err := svc.CreateWithOptions(ctx, "acc_123", "api_key", "secret", opts)

// Oracle service resolves (allowed)
resolved, err := svc.ResolveSecretsWithACL(
    ctx, "acc_123", []string{"api_key"}, secrets.CallerOracle,
)

// Automation service resolves (denied)
_, err = svc.ResolveSecretsWithACL(
    ctx, "acc_123", []string{"api_key"}, secrets.CallerAutomation,
)
// Returns: "secret \"api_key\": access denied for automation service"
```

### Batch Resolution

```go
// Resolve multiple secrets atomically
names := []string{"db_password", "api_key", "jwt_secret"}
resolved, err := svc.ResolveSecrets(ctx, "acc_123", names)

for name, value := range resolved {
    fmt.Printf("%s: %s\n", name, value)
}
```

## Error Handling

### Common Errors

- `RequiredError("name")` - Missing or empty name
- `RequiredError("value")` - Missing or empty value
- `"name cannot contain '|'"` - Invalid name format
- `"secret not found"` - Store returns ErrNotFound
- `"access denied for {service}"` - ACL check failed
- `"decode secret: ..."` - Corrupted base64 data
- `"decrypt: ..."` - Cipher decryption failure

### Error Response Format

```json
{
  "error": "secret \"api_key\": access denied for automation service (ACL: 1, required: 2)"
}
```

## Migration Guide

### From Legacy Secrets System

1. Export existing secrets: `SELECT account_id, name, value FROM old_secrets`
2. Re-encrypt with new cipher: `cipher.Encrypt(oldValue)`
3. Import via `CreateWithOptions` with appropriate ACL flags
4. Update service configurations to use new resolver interface
5. Verify ACL enforcement in staging environment
6. Deprecate old system after validation period

## Troubleshooting

### Secret Decryption Fails

**Symptom:** `"decrypt: cipher: message authentication failed"`

**Causes:**
- Encryption key mismatch
- Corrupted database value
- Wrong cipher algorithm

**Solution:**
1. Verify cipher key matches encryption key
2. Check database value is valid base64
3. Confirm nonce size matches GCM requirements (12 bytes)

### ACL Access Denied

**Symptom:** `"access denied for {service}"`

**Solution:**
1. Check secret ACL: `GET /secrets/{name}`
2. Verify caller service enum matches ACL flag
3. Update ACL: `PUT /secrets/{name}` with `{"acl": <new_flags>}`

### Performance Degradation

**Symptom:** Slow secret resolution

**Solution:**
1. Check database index on `(account_id, name)`
2. Monitor `max_concurrent_requests` limit
3. Consider caching frequently accessed secrets
4. Review encryption key derivation overhead

## License

MIT License - Copyright (c) 2025 R3E Network

## Support

For issues and feature requests, contact the R3E Network service layer team.
