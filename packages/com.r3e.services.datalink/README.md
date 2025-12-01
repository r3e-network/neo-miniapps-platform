# DataLink Service

## Overview

The DataLink service provides cross-chain data linking capabilities through configurable channels and delivery mechanisms. It manages data provider configurations (channels) and handles delivery requests with automatic dispatch, retry logic, and event publishing.

**Package ID**: `com.r3e.services.datalink`
**Service Name**: `datalink`
**Domain**: `datalink`

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     DataLink Service                         │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │   Channel    │      │   Delivery   │                     │
│  │  Management  │      │  Management  │                     │
│  └──────┬───────┘      └──────┬───────┘                     │
│         │                     │                              │
│         │                     │                              │
│         ├─────────────────────┤                              │
│         │                     │                              │
│         ▼                     ▼                              │
│  ┌─────────────────────────────────┐                        │
│  │      Store Interface            │                        │
│  │  (Postgres/Memory)              │                        │
│  └─────────────────────────────────┘                        │
│                                                               │
│  ┌─────────────────────────────────┐                        │
│  │      Dispatcher                 │                        │
│  │  (Pluggable delivery logic)     │                        │
│  └─────────────────────────────────┘                        │
│                                                               │
│  ┌─────────────────────────────────┐                        │
│  │   Event Publisher               │                        │
│  │  (datalink.delivery.created)    │                        │
│  └─────────────────────────────────┘                        │
│                                                               │
└─────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
   ┌──────────┐        ┌──────────┐        ┌──────────┐
   │ Accounts │        │ Wallets  │        │  Store   │
   │ Service  │        │ Service  │        │ (Postgres)│
   └──────────┘        └──────────┘        └──────────┘
```

## Key Components

### Service (`Service`)

The main service struct that orchestrates channel and delivery management.

**Responsibilities**:
- Channel lifecycle management (create, update, get, list)
- Delivery creation and tracking
- Account and wallet ownership validation
- Event publishing for delivery lifecycle
- Dispatcher orchestration with retry and observability
- HTTP API endpoint exposure

### Store Interface (`Store`)

Persistence abstraction for channels and deliveries.

**Implementations**:
- `PostgresStore`: Production database backend
- `MemoryStore`: In-memory implementation for testing

**Operations**:
- Channel CRUD operations
- Delivery creation and retrieval
- Account-scoped listing with pagination

### Dispatcher Interface (`Dispatcher`)

Pluggable delivery mechanism for sending data to external endpoints.

**Default Behavior**: No-op (returns nil)
**Customization**: Use `WithDispatcher()` to inject custom delivery logic

**Signature**:
```go
type Dispatcher interface {
    Dispatch(ctx context.Context, delivery Delivery, channel Channel) error
}
```

### Event Publisher

Implements `core.EventPublisher` interface to receive delivery events from the core engine.

**Supported Events**:
- `delivery`: Creates a new delivery from event payload
- Publishes `datalink.delivery.created` events to the event bus

## Domain Types

### Channel

Represents a data provider configuration with authentication and signing requirements.

```go
type Channel struct {
    ID        string            // Unique channel identifier
    AccountID string            // Owner account ID
    Name      string            // Human-readable channel name
    Endpoint  string            // Target endpoint URL
    AuthToken string            // Authentication token
    SignerSet []string          // Required wallet signers
    Status    ChannelStatus     // Channel state
    Metadata  map[string]string // Additional key-value data
    CreatedAt time.Time         // Creation timestamp
    UpdatedAt time.Time         // Last update timestamp
}
```

**Channel Status Values**:
- `inactive`: Channel is disabled
- `active`: Channel is operational
- `suspended`: Channel is temporarily disabled

**Validation Rules**:
- `name` is required and trimmed
- `endpoint` is required and trimmed
- `signer_set` must contain at least one wallet address
- All signers must be owned by the channel's account (if wallet checker is configured)
- Status defaults to `inactive` if not specified

### Delivery

Represents a delivery request through a channel.

```go
type Delivery struct {
    ID        string            // Unique delivery identifier
    AccountID string            // Owner account ID
    ChannelID string            // Target channel ID
    Payload   map[string]any    // Arbitrary delivery payload
    Attempts  int               // Number of dispatch attempts
    Status    DeliveryStatus    // Delivery state
    Error     string            // Error message if failed
    Metadata  map[string]string // Additional key-value data
    CreatedAt time.Time         // Creation timestamp
    UpdatedAt time.Time         // Last update timestamp
}
```

**Delivery Status Values**:
- `pending`: Awaiting dispatch
- `dispatched`: Currently being delivered
- `succeeded`: Successfully delivered
- `failed`: Delivery failed

## API Endpoints

All endpoints are automatically registered using the declarative HTTP method naming convention.

### Channels

#### List Channels
```
GET /datalink/channels
```
Returns all channels for the authenticated account.

**Response**: `[]Channel`

#### Create Channel
```
POST /datalink/channels
```
Creates a new channel.

**Request Body**:
```json
{
  "name": "My Data Channel",
  "endpoint": "https://api.example.com/webhook",
  "auth_token": "secret-token",
  "signer_set": ["0x1234...", "0x5678..."],
  "status": "active",
  "metadata": {
    "key": "value"
  }
}
```

**Response**: `Channel`

#### Get Channel
```
GET /datalink/channels/{id}
```
Retrieves a specific channel by ID.

**Response**: `Channel`

#### Update Channel
```
PATCH /datalink/channels/{id}
```
Updates an existing channel. Only provided fields are updated.

**Request Body**: Partial `Channel` object

**Response**: `Channel`

### Deliveries

#### List Deliveries
```
GET /datalink/deliveries?limit=50
```
Returns deliveries for the authenticated account.

**Query Parameters**:
- `limit` (optional): Maximum number of results (default: 100, max: 1000)

**Response**: `[]Delivery`

#### Create Delivery
```
POST /datalink/deliveries
```
Creates and dispatches a new delivery.

**Request Body**:
```json
{
  "channel_id": "channel-uuid",
  "payload": {
    "data": "arbitrary payload"
  },
  "metadata": {
    "key": "value"
  }
}
```

**Response**: `Delivery`

**Side Effects**:
- Publishes `datalink.delivery.created` event
- Invokes dispatcher with retry logic
- Increments `datalink_deliveries_created_total` metric

#### Get Delivery
```
GET /datalink/deliveries/{id}
```
Retrieves a specific delivery by ID.

**Response**: `Delivery`

## Configuration Options

### Service Construction

```go
func New(
    accounts AccountChecker,
    store Store,
    log *logger.Logger,
) *Service
```

**Parameters**:
- `accounts`: Account validation interface
- `store`: Persistence implementation
- `log`: Logger instance

### Configuration Methods

#### WithDispatcher
```go
func (s *Service) WithDispatcher(d Dispatcher)
```
Injects custom delivery dispatch logic.

#### WithDispatcherRetry
```go
func (s *Service) WithDispatcherRetry(policy core.RetryPolicy)
```
Configures retry behavior for dispatcher calls.

#### WithDispatcherHooks
```go
func (s *Service) WithDispatcherHooks(h core.DispatchHooks)
```
Adds observability hooks for dispatcher operations.

#### WithTracer
```go
func (s *Service) WithTracer(t core.Tracer)
```
Configures distributed tracing for dispatcher operations.

#### WithWallets / WithWalletChecker
```go
func (s *Service) WithWallets(wallets WalletChecker)
func (s *Service) WithWalletChecker(w WalletChecker)
```
Enables wallet ownership validation for channel signers.

#### WithObservationHooks
```go
func (s *Service) WithObservationHooks(h core.ObservationHooks)
```
Adds observability hooks for service operations.

## Dependencies

### Required Services
- `store`: Database/persistence layer
- `svc-accounts`: Account validation service

### Required API Surfaces
- `APISurfaceStore`: Database access
- `APISurfaceData`: Data operations
- `APISurfaceEvent`: Event bus access

### Optional Dependencies
- Wallet service (for signer validation)
- Tracer (for distributed tracing)
- Event bus (for delivery events)

## Metrics

The service emits the following Prometheus-compatible metrics:

- `datalink_channels_created_total{account_id}`: Total channels created
- `datalink_channels_updated_total{account_id}`: Total channels updated
- `datalink_deliveries_created_total{channel_id}`: Total deliveries created

## Events

### Published Events

#### datalink.delivery.created
Emitted when a new delivery is created.

**Payload**:
```json
{
  "delivery_id": "uuid",
  "account_id": "account-id",
  "channel_id": "channel-id"
}
```

### Subscribed Events

#### delivery
Accepts delivery creation requests via the event bus.

**Payload**:
```json
{
  "account_id": "account-id",
  "channel_id": "channel-id",
  "payload": {},
  "metadata": {}
}
```

## Testing

### Unit Tests

Run the service tests:
```bash
go test -v ./packages/com.r3e.services.datalink/...
```

### Test Utilities

The package provides testing utilities:

```go
// In-memory store for testing
store := datalink.NewMemoryStore()

// Mock account checker
accounts := datalink.NewMockAccountChecker()

// Mock wallet checker
wallets := datalink.NewMockWalletChecker()

// Create test service
svc := datalink.New(accounts, store, logger)
```

### Example Test

```go
func TestCreateChannel(t *testing.T) {
    store := datalink.NewMemoryStore()
    accounts := datalink.NewMockAccountChecker()
    svc := datalink.New(accounts, store, logger)

    ch := datalink.Channel{
        AccountID: "test-account",
        Name:      "Test Channel",
        Endpoint:  "https://example.com",
        SignerSet: []string{"0x1234"},
        Status:    datalink.ChannelStatusActive,
    }

    created, err := svc.CreateChannel(context.Background(), ch)
    if err != nil {
        t.Fatalf("CreateChannel failed: %v", err)
    }

    if created.ID == "" {
        t.Error("Expected channel ID to be set")
    }
}
```

## Error Handling

The service returns structured errors for common failure scenarios:

- **Account Not Found**: Account validation fails
- **Channel Not Found**: Channel ID does not exist
- **Delivery Not Found**: Delivery ID does not exist
- **Ownership Violation**: Resource does not belong to requesting account
- **Validation Error**: Required fields missing or invalid
- **Signer Not Owned**: Wallet signer not owned by account (when wallet checker enabled)

## Security Considerations

1. **Account Isolation**: All operations enforce account ownership
2. **Wallet Validation**: Optional signer ownership verification
3. **Token Storage**: Auth tokens stored in plaintext (consider encryption)
4. **Endpoint Validation**: No URL validation performed (implement in dispatcher)
5. **Payload Size**: No size limits enforced (implement at HTTP layer)

## Performance Characteristics

- **Channel Operations**: O(1) for get/create/update, O(n) for list
- **Delivery Operations**: O(1) for get/create, O(n) for list
- **Concurrent Access**: Thread-safe through store implementations
- **Pagination**: Supported for delivery listing with configurable limits

## Integration Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    // Initialize dependencies
    store := datalink.NewPostgresStore(db, accounts)
    log := logger.New()

    // Create service
    svc := datalink.New(accounts, store, log)

    // Configure custom dispatcher
    svc.WithDispatcher(datalink.DispatcherFunc(
        func(ctx context.Context, del datalink.Delivery, ch datalink.Channel) error {
            // Custom delivery logic
            return sendHTTPRequest(ch.Endpoint, ch.AuthToken, del.Payload)
        },
    ))

    // Configure retry policy
    svc.WithDispatcherRetry(core.RetryPolicy{
        MaxAttempts: 3,
        BackoffMs:   1000,
    })

    // Start service
    if err := svc.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## File Structure

```
/home/neo/git/service_layer/packages/com.r3e.services.datalink/
├── domain.go              # Domain types (Channel, Delivery)
├── service.go             # Core service implementation
├── store.go               # Store interface definition
├── store_postgres.go      # PostgreSQL store implementation
├── testing.go             # Test utilities (MemoryStore, mocks)
├── package.go             # Package registration and initialization
├── service_test.go        # Service unit tests
└── README.md              # This file
```

## Related Documentation

- [Service Framework Documentation](../../system/framework/README.md)
- [API Router Documentation](../../system/framework/core/README.md)
- [Account Service](../com.r3e.services.accounts/README.md)
