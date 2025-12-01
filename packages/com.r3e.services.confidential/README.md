# Confidential Computing Service

A service for managing Trusted Execution Environment (TEE) enclaves, sealed keys, and attestation proofs within the R3E Network service layer.

## Overview

The Confidential Computing Service provides infrastructure for registering and managing TEE-based secure enclaves, storing sealed cryptographic keys, and verifying attestation proofs. This service enables confidential computing workloads by maintaining the lifecycle of trusted execution environments and their associated cryptographic materials.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP API Layer                           │
│  GET/POST /enclaves, GET/PATCH /enclaves/{id}              │
│  GET/POST /enclaves/{id}/keys                              │
│  GET/POST /enclaves/{id}/attestations                      │
│  GET /attestations                                         │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│              Confidential Service                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Enclave    │  │ Sealed Key   │  │ Attestation  │     │
│  │  Management  │  │  Management  │  │  Management  │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│         │                 │                  │              │
│         └─────────────────┴──────────────────┘              │
│                           │                                 │
└───────────────────────────┼─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                    Store Interface                          │
│  ┌────────────────────────────────────────────────────┐    │
│  │  PostgresStore (Production)                        │    │
│  │  MemoryStore (Testing)                             │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                  TEE Infrastructure                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │ Enclave  │  │ Enclave  │  │ Enclave  │                 │
│  │    A     │  │    B     │  │    C     │                 │
│  └──────────┘  └──────────┘  └──────────┘                 │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### Service (`service.go`)

The main service implementation that orchestrates enclave registration, key management, and attestation verification.

**Responsibilities:**
- Enclave lifecycle management (create, update, list, get)
- Sealed key storage and retrieval
- Attestation proof recording and validation
- Account ownership verification
- Observability hooks for monitoring
- HTTP endpoint handlers with automatic route discovery

### Domain Types (`domain.go`)

Core domain models representing confidential computing primitives:

**Enclave:**
- Represents a registered TEE runner
- Tracks endpoint, attestation data, and lifecycle status
- Supports metadata for extensibility

**SealedKey:**
- Links encrypted key material to specific enclaves
- Stores sealed blobs that can only be unsealed within the TEE
- Associates keys with accounts and enclaves

**Attestation:**
- Captures cryptographic proof of enclave integrity
- Includes attestation reports and validity periods
- Tracks verification status

### Store Interface (`store.go`)

Defines the persistence contract for confidential computing data:
- Enclave CRUD operations
- Sealed key storage and listing
- Attestation recording and querying
- Account-scoped and enclave-scoped queries

**Implementations:**
- `PostgresStore` - Production persistence layer
- `MemoryStore` - In-memory implementation for testing

## Domain Types

### Enclave

```go
type Enclave struct {
    ID          string            // Unique enclave identifier
    AccountID   string            // Owner account ID
    Name        string            // Human-readable name
    Endpoint    string            // TEE endpoint URL (required)
    Attestation string            // Initial attestation data
    Status      EnclaveStatus     // Lifecycle state
    Metadata    map[string]string // Extensible metadata
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

**EnclaveStatus Values:**
- `inactive` - Enclave registered but not yet active (default)
- `active` - Enclave is operational and accepting requests
- `revoked` - Enclave has been decommissioned or compromised

### SealedKey

```go
type SealedKey struct {
    ID        string            // Unique key identifier
    AccountID string            // Owner account ID
    EnclaveID string            // Associated enclave ID
    Name      string            // Key name (required)
    Blob      []byte            // Sealed key material
    Metadata  map[string]string // Extensible metadata
    CreatedAt time.Time         // Creation timestamp
}
```

### Attestation

```go
type Attestation struct {
    ID         string            // Unique attestation identifier
    AccountID  string            // Owner account ID
    EnclaveID  string            // Associated enclave ID
    Report     string            // Attestation report (required)
    ValidUntil *time.Time        // Optional expiration time
    Status     string            // Verification status (default: "pending")
    Metadata   map[string]string // Extensible metadata
    CreatedAt  time.Time         // Creation timestamp
}
```

## API Endpoints

All endpoints are automatically registered via the declarative HTTP method naming convention.

### Enclave Management

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/enclaves` | `HTTPGetEnclaves` | List all enclaves for account |
| POST | `/enclaves` | `HTTPPostEnclaves` | Register new enclave |
| GET | `/enclaves/{id}` | `HTTPGetEnclavesById` | Get specific enclave |
| PATCH | `/enclaves/{id}` | `HTTPPatchEnclavesById` | Update enclave metadata/status |

### Sealed Key Management

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/enclaves/{id}/keys` | `HTTPGetEnclavesIdKeys` | List sealed keys for enclave |
| POST | `/enclaves/{id}/keys` | `HTTPPostEnclavesIdKeys` | Store new sealed key |

**Query Parameters:**
- `limit` - Maximum number of keys to return (default: 25, max: 500)

### Attestation Management

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/enclaves/{id}/attestations` | `HTTPGetEnclavesIdAttestations` | List attestations for enclave |
| POST | `/enclaves/{id}/attestations` | `HTTPPostEnclavesIdAttestations` | Submit attestation proof |
| GET | `/attestations` | `HTTPGetAttestations` | List all attestations for account |

**Query Parameters:**
- `limit` - Maximum number of attestations to return (default: 25, max: 500)

## Configuration Options

### Service Initialization

```go
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service
```

**Parameters:**
- `accounts` - Account validation interface
- `store` - Persistence implementation
- `log` - Structured logger instance

### Observability Hooks

```go
func (s *Service) WithSealedKeyHooks(h core.ObservationHooks)
func (s *Service) WithAttestationHooks(h core.ObservationHooks)
```

Configure custom observability hooks for sealed key and attestation operations. Useful for metrics collection, audit logging, or external system integration.

### Metrics

The service emits the following counters:
- `confidential_enclaves_created_total` - Total enclaves registered
- `confidential_enclaves_updated_total` - Total enclave updates
- `confidential_sealed_keys_created_total` - Total sealed keys stored
- `confidential_attestations_created_total` - Total attestations recorded

All metrics include `account_id` label; key and attestation metrics also include `enclave_id`.

## Dependencies

### Internal Dependencies
- `github.com/R3E-Network/service_layer/pkg/logger` - Structured logging
- `github.com/R3E-Network/service_layer/system/framework` - Service framework
- `github.com/R3E-Network/service_layer/system/framework/core` - Core utilities

### External Dependencies
- `github.com/google/uuid` - UUID generation (testing)

### Interface Dependencies
- `AccountChecker` - Account validation interface from framework
- `Store` - Persistence interface (must be provided during initialization)

## Testing Instructions

### Running Tests

```bash
# Run all tests in the package
go test -v ./packages/com.r3e.services.confidential

# Run specific test
go test -v ./packages/com.r3e.services.confidential -run TestService_CreateEnclave

# Run with coverage
go test -cover ./packages/com.r3e.services.confidential
```

### Test Structure

The service includes comprehensive unit tests covering:

1. **Enclave Operations** (`TestService_CreateEnclave`)
   - Enclave registration
   - Default status assignment
   - Account validation

2. **Sealed Key Operations** (`TestService_CreateSealedKey`)
   - Key storage
   - Enclave association
   - Name validation

3. **Attestation Operations** (`TestService_CreateAttestation`)
   - Attestation recording
   - Report validation
   - Enclave verification

4. **Service Lifecycle** (`TestService_Lifecycle`)
   - Start/Ready/Stop operations
   - Graceful shutdown

5. **Service Metadata** (`TestService_Manifest`, `TestService_Descriptor`)
   - Service identification
   - Metadata exposure

### Testing Utilities

The package provides testing utilities in `testing.go`:

**MemoryStore:**
```go
store := confidential.NewMemoryStore()
```
Thread-safe in-memory implementation of the Store interface for unit testing.

**MockAccountChecker:**
```go
accounts := confidential.NewMockAccountChecker()
accounts.AddAccountWithTenant("acct-1", "")
```
Mock account validation for isolated testing.

### Example Test Setup

```go
func TestExample(t *testing.T) {
    // Setup
    store := confidential.NewMemoryStore()
    accounts := confidential.NewMockAccountChecker()
    accounts.AddAccountWithTenant("test-account", "")
    svc := confidential.New(accounts, store, nil)

    // Test enclave creation
    enclave, err := svc.CreateEnclave(context.Background(), confidential.Enclave{
        AccountID: "test-account",
        Name:      "test-enclave",
        Endpoint:  "https://enclave.example.com",
    })
    if err != nil {
        t.Fatalf("failed to create enclave: %v", err)
    }

    // Verify
    if enclave.Status != confidential.EnclaveStatusInactive {
        t.Errorf("expected inactive status, got %s", enclave.Status)
    }
}
```

## Security Considerations

1. **Attestation Verification**: The service stores attestation reports but does not perform cryptographic verification. Implement verification logic in custom attestation hooks.

2. **Sealed Key Protection**: Key blobs are stored as-is. Ensure keys are properly sealed by the TEE before submission.

3. **Account Isolation**: All operations enforce account ownership. Cross-account access is prevented at the service layer.

4. **Endpoint Validation**: Enclave endpoints are stored but not validated. Implement endpoint verification in production deployments.

5. **Metadata Sanitization**: Metadata fields are normalized but not sanitized. Avoid storing sensitive data in metadata.

## Package Registration

The service is automatically registered as a service package:

```go
Package ID:   com.r3e.services.confidential
Display Name: Confidential Service
Description:  Confidential computing with enclaves
Service Name: confidential
```

The package creates a single service instance with PostgreSQL persistence in production environments.
