# VRF Service

**Package ID:** `com.r3e.services.vrf`
**Version:** 1.0.0
**License:** MIT

## Overview

The VRF (Verifiable Random Function) Service provides cryptographically secure random number generation for blockchain applications. It manages VRF keys and randomness requests, ensuring verifiable and tamper-proof random outputs aligned with the RandomnessHub smart contract.

This service enables accounts to:
- Register and manage VRF signing keys with wallet-based ownership verification
- Submit randomness requests with consumer-specific seeds
- Track request fulfillment status and retrieve random outputs
- Dispatch requests to downstream VRF executors for cryptographic processing

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      VRF Service                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐      ┌──────────────┐                   │
│  │   Key Mgmt   │      │  Request Mgmt│                   │
│  │              │      │              │                   │
│  │ - Create     │      │ - Create     │                   │
│  │ - Update     │      │ - Get        │                   │
│  │ - Get/List   │      │ - List       │                   │
│  └──────┬───────┘      └──────┬───────┘                   │
│         │                     │                            │
│         └──────────┬──────────┘                            │
│                    │                                       │
│         ┌──────────▼──────────┐                           │
│         │   Store Interface   │                           │
│         │  (PostgreSQL impl)  │                           │
│         └──────────┬──────────┘                           │
│                    │                                       │
└────────────────────┼───────────────────────────────────────┘
                     │
         ┌───────────▼───────────┐
         │   Event Bus (async)   │
         │ vrf.request.created   │
         └───────────┬───────────┘
                     │
         ┌───────────▼───────────┐
         │   Dispatcher          │
         │ (VRF Executor)        │
         └───────────────────────┘
```

### Data Flow

1. **Key Registration**: Account registers VRF public key with wallet ownership verification
2. **Request Creation**: Consumer submits randomness request with key ID and seed
3. **Event Publishing**: Service publishes `vrf.request.created` event to event bus
4. **Dispatch**: Request dispatched to VRF executor for cryptographic processing
5. **Fulfillment**: Executor generates verifiable random output and updates request status

## Key Components

### Service (`service.go`)

The core service implementation providing:

- **Key Management**: CRUD operations for VRF keys with ownership validation
- **Request Management**: Creation and tracking of randomness requests
- **Dispatcher Integration**: Pluggable dispatcher for VRF execution
- **Observability**: Metrics, tracing, and structured logging
- **HTTP API**: Auto-discovered REST endpoints via naming convention

**Configuration Options:**

```go
// Dispatcher customization
svc.WithDispatcher(customDispatcher)

// Retry policy for dispatcher calls
svc.WithDispatcherRetry(core.RetryPolicy{
    MaxAttempts: 3,
    BackoffMs:   100,
})

// Observability hooks
svc.WithDispatcherHooks(core.DispatchHooks{
    OnStart:   func(ctx context.Context) {},
    OnSuccess: func(ctx context.Context) {},
    OnError:   func(ctx context.Context, err error) {},
})

// Custom tracer
svc.WithTracer(customTracer)
```

### Store Interface (`store.go`)

Persistence abstraction for VRF data:

```go
type Store interface {
    // Key operations
    CreateKey(ctx context.Context, key Key) (Key, error)
    UpdateKey(ctx context.Context, key Key) (Key, error)
    GetKey(ctx context.Context, id string) (Key, error)
    ListKeys(ctx context.Context, accountID string) ([]Key, error)

    // Request operations
    CreateRequest(ctx context.Context, req Request) (Request, error)
    GetRequest(ctx context.Context, id string) (Request, error)
    ListRequests(ctx context.Context, accountID string, limit int) ([]Request, error)
}
```

**Implementation:** PostgreSQL-backed store (`store_postgres.go`)

### Dispatcher Interface

Pluggable interface for VRF execution:

```go
type Dispatcher interface {
    Dispatch(ctx context.Context, req Request, key Key) error
}
```

Default implementation is a no-op. Production deployments should provide a custom dispatcher that:
- Validates request parameters
- Generates cryptographic proof using the VRF key
- Submits proof to blockchain contract
- Updates request status upon fulfillment

## Domain Types

### Key (`model.go`)

Represents a VRF signing key owned by an account.

```go
type Key struct {
    ID            string            // Unique key identifier
    AccountID     string            // Owning account
    PublicKey     string            // VRF public key (required)
    Label         string            // Human-readable label
    Status        KeyStatus         // Lifecycle state
    WalletAddress string            // Associated wallet (required, verified)
    Attestation   string            // Optional attestation data
    Metadata      map[string]string // Custom key-value pairs
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**Key Status Lifecycle:**

- `inactive`: Key created but not yet active
- `pending_approval`: Awaiting approval (future use)
- `active`: Key is operational and can sign requests
- `revoked`: Key has been permanently disabled

### Request (`model.go`)

Captures a randomness request from a consumer contract.

```go
type Request struct {
    ID          string            // Unique request identifier
    AccountID   string            // Owning account
    KeyID       string            // VRF key to use (maps to contract ServiceId)
    Consumer    string            // Consumer contract address (required)
    Seed        string            // Request seed (required, maps to SeedHash)
    Status      RequestStatus     // Fulfillment state
    Result      string            // Random output (maps to contract Output)
    Error       string            // Error message if failed
    Metadata    map[string]string // Custom key-value pairs
    CreatedAt   time.Time         // Request timestamp (maps to RequestedAt)
    UpdatedAt   time.Time
    FulfilledAt time.Time         // Fulfillment timestamp
}
```

**Request Status:**

- `pending`: Request created, awaiting fulfillment
- `fulfilled`: Random output generated and available
- `failed`: Request processing failed

**Contract Alignment:** Request fields map directly to the RandomnessHub.cs contract structure for seamless blockchain integration.

## API Endpoints

All endpoints require authentication and are scoped to the authenticated account.

### Keys

#### `GET /vrf/keys`

List all VRF keys for the authenticated account.

**Response:**
```json
[
  {
    "id": "key_abc123",
    "account_id": "acc_xyz",
    "public_key": "0x04a1b2c3...",
    "label": "Production VRF Key",
    "status": "active",
    "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb5",
    "attestation": "",
    "metadata": {},
    "created_at": "2025-12-01T10:00:00Z",
    "updated_at": "2025-12-01T10:00:00Z"
  }
]
```

#### `POST /vrf/keys`

Create a new VRF key.

**Request Body:**
```json
{
  "public_key": "0x04a1b2c3d4e5f6...",
  "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb5",
  "label": "Production VRF Key",
  "attestation": "optional_attestation_data",
  "metadata": {
    "environment": "production"
  }
}
```

**Validation:**
- `public_key`: Required, non-empty
- `wallet_address`: Required, must be owned by account (verified via accounts service)
- `label`: Optional
- `status`: Defaults to `inactive` if not specified

**Response:** Created `Key` object (201 Created)

#### `GET /vrf/keys/{id}`

Retrieve a specific VRF key.

**Path Parameters:**
- `id`: Key identifier

**Response:** `Key` object or 404 if not found/not owned

#### `PATCH /vrf/keys/{id}`

Update mutable fields on a VRF key.

**Path Parameters:**
- `id`: Key identifier

**Request Body (partial update):**
```json
{
  "label": "Updated Label",
  "status": "active",
  "metadata": {
    "environment": "staging"
  }
}
```

**Immutable Fields:** `id`, `account_id`, `public_key`, `wallet_address`, `created_at`

**Response:** Updated `Key` object

### Requests

#### `GET /vrf/requests`

List randomness requests for the authenticated account.

**Query Parameters:**
- `limit`: Maximum results (default: 100, max: 1000)

**Response:**
```json
[
  {
    "id": "req_def456",
    "account_id": "acc_xyz",
    "key_id": "key_abc123",
    "consumer": "0x1234567890abcdef",
    "seed": "0xfedcba0987654321",
    "status": "fulfilled",
    "result": "0x9a8b7c6d5e4f3a2b1c0d",
    "error": "",
    "metadata": {},
    "created_at": "2025-12-01T11:00:00Z",
    "updated_at": "2025-12-01T11:00:05Z",
    "fulfilled_at": "2025-12-01T11:00:05Z"
  }
]
```

#### `POST /vrf/requests`

Create a new randomness request.

**Request Body:**
```json
{
  "key_id": "key_abc123",
  "consumer": "0x1234567890abcdef",
  "seed": "0xfedcba0987654321",
  "metadata": {
    "request_type": "lottery_draw"
  }
}
```

**Validation:**
- `key_id`: Required, must exist and be owned by account
- `consumer`: Required, non-empty consumer contract address
- `seed`: Required, non-empty seed value

**Side Effects:**
1. Request created with `pending` status
2. Event `vrf.request.created` published to event bus
3. Request dispatched to VRF executor (async, with retry)

**Response:** Created `Request` object (201 Created)

**Note:** Dispatcher errors are logged but do not fail the request creation. The request remains in `pending` state for retry.

#### `GET /vrf/requests/{id}`

Retrieve a specific randomness request.

**Path Parameters:**
- `id`: Request identifier

**Response:** `Request` object or 404 if not found/not owned

## Configuration

### Service Configuration

Defined in `manifest.yaml`:

```yaml
resources:
  max_storage_bytes: 104857600      # 100 MB
  max_concurrent_requests: 1000
  max_requests_per_second: 5000
  max_events_per_second: 1000
```

### Dependencies

**Required:**
- `store`: Database access for persistence
- `svc-accounts`: Account and wallet validation

**Optional:**
- Event bus: For publishing `vrf.request.created` events (degrades gracefully if unavailable)

### Permissions

- `system.api.storage`: Required for data persistence
- `system.api.bus`: Optional for event publishing

## Dependencies

### Internal

- `github.com/R3E-Network/service_layer/pkg/logger`: Structured logging
- `github.com/R3E-Network/service_layer/system/core`: Core engine types
- `github.com/R3E-Network/service_layer/system/framework`: Service framework
- `github.com/R3E-Network/service_layer/system/framework/core`: Core utilities

### External

- PostgreSQL database (via store implementation)
- Accounts service (for wallet ownership verification)

## Observability

### Metrics

The service emits the following counters:

- `vrf_keys_created_total{account_id}`: Total keys created per account
- `vrf_keys_updated_total{account_id}`: Total key updates per account
- `vrf_requests_created_total{key_id}`: Total requests created per key

### Tracing

All operations are traced with the following attributes:

- `account_id`: Account identifier
- `key_id`: Key identifier (where applicable)
- `request_id`: Request identifier (where applicable)
- `resource`: Operation type (e.g., `vrf_key`, `vrf_request`)

### Logging

Structured logs include:

- Key creation/update events with key ID and account ID
- Dispatcher errors with request ID
- Event bus unavailability warnings

## Testing

### Unit Tests

Run service-level tests:

```bash
cd /home/neo/git/service_layer/packages/com.r3e.services.vrf
go test -v ./...
```

### Integration Tests

The service includes environment-aware tests (`service_env_internal_test.go`) that verify:

- Service initialization with framework environment
- Account validation integration
- Tracer configuration and propagation

Run integration tests:

```bash
go test -v -tags=integration ./...
```

### Test Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Usage Example

### Initializing the Service

```go
import (
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

// Create service
accounts := NewAccountChecker()
store := vrf.NewPostgresStore(db, accounts)
log := logger.New()
svc := vrf.New(accounts, store, log)

// Configure dispatcher
svc.WithDispatcher(vrf.DispatcherFunc(func(ctx context.Context, req vrf.Request, key vrf.Key) error {
    // Custom VRF execution logic
    return executeVRF(ctx, req, key)
}))

// Configure retry policy
svc.WithDispatcherRetry(core.RetryPolicy{
    MaxAttempts: 3,
    BackoffMs:   100,
})

// Start service
if err := svc.Start(ctx); err != nil {
    log.Fatal(err)
}
```

### Creating a Key

```go
key := vrf.Key{
    AccountID:     "acc_xyz",
    PublicKey:     "0x04a1b2c3d4e5f6...",
    WalletAddress: "0x742d35cc6634c0532925a3b844bc9e7595f0beb5",
    Label:         "Production Key",
    Status:        vrf.KeyStatusActive,
}

created, err := svc.CreateKey(ctx, key)
if err != nil {
    log.Error(err)
}
```

### Submitting a Request

```go
req, err := svc.CreateRequest(
    ctx,
    "acc_xyz",           // account ID
    "key_abc123",        // key ID
    "0x1234567890abcdef", // consumer address
    "0xfedcba0987654321", // seed
    map[string]string{   // metadata
        "request_type": "lottery",
    },
)
if err != nil {
    log.Error(err)
}
```

## Security Considerations

1. **Wallet Ownership**: All keys must be associated with a wallet owned by the account (verified via accounts service)
2. **Account Isolation**: All operations enforce account ownership; users cannot access other accounts' keys or requests
3. **Immutable Fields**: Key public key and wallet address cannot be changed after creation
4. **Attestation**: Optional attestation field supports future TEE/SGX integration for enhanced security

## Future Enhancements

- Key rotation support with versioning
- Request cancellation API
- Batch request submission
- Webhook notifications for request fulfillment
- TEE/SGX attestation verification
- Key approval workflow (utilizing `pending_approval` status)

## License

MIT License - Copyright (c) 2025 R3E Network

## Support

For issues and questions, please refer to the main service layer documentation or contact the R3E Network team.
