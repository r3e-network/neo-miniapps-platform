# CCIP Service

Cross-Chain Interoperability Protocol (CCIP) service for managing cross-chain message routing and token transfers.

## Overview

The CCIP service provides a framework for defining cross-chain communication lanes and sending messages with optional token transfers between blockchain networks. It handles message lifecycle management, ownership validation, and dispatcher integration for downstream delivery systems.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CCIP Service                            │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │   Lane       │      │   Message    │                     │
│  │  Management  │      │  Management  │                     │
│  └──────┬───────┘      └──────┬───────┘                     │
│         │                     │                              │
│         │                     │                              │
│  ┌──────▼──────────────────────▼───────┐                    │
│  │      Service Orchestration          │                    │
│  │  - Ownership Validation              │                    │
│  │  - Normalization & Validation        │                    │
│  │  - Event Publishing                  │                    │
│  │  - Metrics & Observability           │                    │
│  └──────┬──────────────────┬────────────┘                   │
│         │                  │                                 │
└─────────┼──────────────────┼─────────────────────────────────┘
          │                  │
          ▼                  ▼
    ┌──────────┐      ┌──────────────┐
    │  Store   │      │  Dispatcher  │
    │  (CRUD)  │      │  (Delivery)  │
    └──────────┘      └──────────────┘
          │
          ▼
    ┌──────────────┐
    │  PostgreSQL  │
    └──────────────┘
```

## Key Components

### Service (`Service`)
The main orchestrator that coordinates lane and message operations. Responsibilities:
- Account and wallet ownership validation
- Lane creation, update, retrieval, and listing
- Message creation, retrieval, and listing
- Integration with dispatcher for message delivery
- Event publishing and metrics collection
- HTTP API endpoint handling

### Domain Types

#### Lane
Represents an allowed CCIP route between two chains for a specific account.

**Fields:**
- `id` (string): Unique lane identifier
- `account_id` (string): Owner account ID
- `name` (string): Human-readable lane name (required)
- `source_chain` (string): Source blockchain identifier (required, normalized to lowercase)
- `dest_chain` (string): Destination blockchain identifier (required, normalized to lowercase)
- `signer_set` ([]string): Wallet addresses authorized to sign for this lane
- `allowed_tokens` ([]string): Token addresses permitted for transfer
- `delivery_policy` (map[string]any): Custom delivery configuration
- `metadata` (map[string]string): Key-value metadata
- `tags` ([]string): Categorization tags
- `created_at` (time.Time): Creation timestamp
- `updated_at` (time.Time): Last update timestamp

#### Message
Represents a cross-chain message queued through CCIP.

**Fields:**
- `id` (string): Unique message identifier
- `account_id` (string): Owner account ID
- `lane_id` (string): Associated lane ID
- `status` (MessageStatus): Current message state
- `payload` (map[string]any): Arbitrary message data
- `token_transfers` ([]TokenTransfer): Associated token movements
- `trace` ([]string): Delivery trace log
- `error` (string): Error message if failed
- `metadata` (map[string]string): Key-value metadata
- `tags` ([]string): Categorization tags
- `created_at` (time.Time): Creation timestamp
- `updated_at` (time.Time): Last update timestamp
- `delivered_at` (*time.Time): Delivery completion timestamp

**Message Status Values:**
- `pending`: Message created, awaiting dispatch
- `dispatching`: Message being delivered
- `delivered`: Message successfully delivered
- `failed`: Message delivery failed

#### TokenTransfer
Captures a token movement associated with a message.

**Fields:**
- `token` (string): Token contract address (normalized to lowercase)
- `amount` (string): Transfer amount as string
- `recipient` (string): Recipient address

### Interfaces

#### Store
Persistence layer interface for lanes and messages.

**Methods:**
- `CreateLane(ctx, lane) (Lane, error)`
- `UpdateLane(ctx, lane) (Lane, error)`
- `GetLane(ctx, id) (Lane, error)`
- `ListLanes(ctx, accountID) ([]Lane, error)`
- `CreateMessage(ctx, msg) (Message, error)`
- `UpdateMessage(ctx, msg) (Message, error)`
- `GetMessage(ctx, id) (Message, error)`
- `ListMessages(ctx, accountID, limit) ([]Message, error)`

**Implementations:**
- `MemoryStore`: In-memory implementation for testing
- `PostgresStore`: Production PostgreSQL implementation

#### Dispatcher
Notifies downstream systems when a CCIP message is ready for delivery.

**Methods:**
- `Dispatch(ctx, msg Message, lane Lane) error`

**Default Behavior:** No-op dispatcher (returns nil)

#### AccountChecker
Validates account existence (alias for `framework.AccountChecker`).

#### WalletChecker
Validates wallet ownership by accounts (alias for `framework.WalletChecker`).

## API Endpoints

All endpoints are automatically registered via the HTTP method naming convention (`HTTP{Method}{Path}`).

### Lane Management

#### List Lanes
```
GET /lanes
```
Returns all lanes for the authenticated account.

**Response:** Array of Lane objects

#### Create Lane
```
POST /lanes
```
Creates a new lane for the authenticated account.

**Request Body:**
```json
{
  "name": "Ethereum to Neo",
  "source_chain": "ethereum",
  "dest_chain": "neo",
  "signer_set": ["0x1234..."],
  "allowed_tokens": ["0xabcd..."],
  "delivery_policy": {},
  "metadata": {"env": "prod"},
  "tags": ["priority"]
}
```

**Response:** Created Lane object

#### Get Lane
```
GET /lanes/{id}
```
Retrieves a specific lane by ID (ownership validated).

**Response:** Lane object

#### Update Lane
```
PATCH /lanes/{id}
```
Updates an existing lane (ownership validated).

**Request Body:** Partial Lane object with fields to update

**Response:** Updated Lane object

### Message Management

#### List Messages
```
GET /messages?limit=50
```
Returns messages for the authenticated account.

**Query Parameters:**
- `limit` (optional): Maximum results (default: 100, max: 1000)

**Response:** Array of Message objects

#### Send Message
```
POST /messages
```
Creates and dispatches a new cross-chain message.

**Request Body:**
```json
{
  "lane_id": "lane-123",
  "payload": {
    "action": "transfer",
    "data": {}
  },
  "token_transfers": [
    {
      "token": "0xabcd...",
      "amount": "1000000000000000000",
      "recipient": "0x5678..."
    }
  ],
  "metadata": {"priority": "high"},
  "tags": ["urgent"]
}
```

**Response:** Created Message object

**Events Published:**
- `ccip.message.created` with payload: `{message_id, account_id, lane_id}`

#### Get Message
```
GET /messages/{id}
```
Retrieves a specific message by ID (ownership validated).

**Response:** Message object

## Configuration

### Service Initialization

```go
import (
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

// Create service
accounts := // AccountChecker implementation
store := ccip.NewPostgresStore(db, accounts)
log := logger.New()
svc := ccip.New(accounts, store, log)

// Optional: Configure dispatcher
svc.WithDispatcher(myDispatcher)

// Optional: Configure wallet validation
svc.WithWalletChecker(walletChecker)

// Optional: Configure retry policy
svc.WithDispatcherRetry(core.RetryPolicy{
    Attempts: 3,
    Delay: time.Second,
})

// Optional: Configure observability
svc.WithDispatcherHooks(core.DispatchHooks{
    OnStart: func(ctx context.Context) {},
    OnComplete: func(ctx context.Context, err error) {},
})

// Optional: Configure tracing
svc.WithTracer(myTracer)
```

### Resource Limits (from manifest.yaml)

- **Max Storage:** 200 MB
- **Max Concurrent Requests:** 1000
- **Max Requests/Second:** 5000
- **Max Events/Second:** 1000

## Dependencies

### Required
- `store`: Data persistence layer (PostgreSQL)
- `svc-accounts`: Account management service

### Optional
- Event bus for publishing `ccip.message.created` events

### API Surfaces
- `APISurfaceStore`: Storage operations
- `APISurfaceEvent`: Event publishing

## Testing

### Run Unit Tests

```bash
cd /home/neo/git/service_layer/packages/com.r3e.services.ccip
go test -v
```

### Test Coverage

The test suite covers:
- Lane creation, update, retrieval, and listing
- Message sending, retrieval, and listing
- Ownership validation for lanes and messages
- Input normalization (chain names, tokens, signers)
- Token transfer validation and filtering
- Dispatcher integration and retry behavior
- Service lifecycle (Start, Ready, Stop)
- Manifest and descriptor generation

### Example Test Usage

```go
func TestExample(t *testing.T) {
    // Setup
    store := ccip.NewMemoryStore()
    accounts := ccip.NewMockAccountChecker()
    accounts.AddAccountWithTenant("acct-1", "")

    svc := ccip.New(accounts, store, nil)

    // Create lane
    lane, err := svc.CreateLane(context.Background(), ccip.Lane{
        AccountID:   "acct-1",
        Name:        "Test Lane",
        SourceChain: "ethereum",
        DestChain:   "neo",
    })

    // Send message
    msg, err := svc.SendMessage(
        context.Background(),
        "acct-1",
        lane.ID,
        map[string]any{"hello": "world"},
        nil, nil, nil,
    )
}
```

## Observability

### Metrics

The service emits the following metrics:

- `ccip_lanes_created_total{account_id}`: Total lanes created
- `ccip_lanes_updated_total{account_id}`: Total lane updates
- `ccip_messages_created_total{lane_id}`: Total messages created

### Logging

Structured logging with fields:
- `lane_id`: Lane identifier
- `message_id`: Message identifier
- `account_id`: Account identifier

Log levels:
- **Info**: Lane/message creation and updates
- **Warn**: Dispatcher errors, bus unavailability

### Tracing

Dispatcher operations are traced with span name `ccip.dispatch` and attributes:
- `message_id`: Message being dispatched
- `lane_id`: Associated lane

## Security

### Ownership Validation

All operations validate that the requesting account owns the resource:
- Lane operations check `lane.account_id == request.account_id`
- Message operations check `message.account_id == request.account_id`
- Signer validation ensures wallets belong to the account (if WalletChecker configured)

### Input Normalization

- Chain names: Trimmed and lowercased
- Signer addresses: Deduplicated and trimmed
- Token addresses: Lowercased and trimmed
- Metadata: Filtered to string values only
- Tags: Deduplicated and trimmed

### Validation Rules

**Lane Creation:**
- `name` is required and non-empty
- `source_chain` is required and non-empty
- `dest_chain` is required and non-empty
- All signers must be owned by the account (if WalletChecker configured)

**Message Creation:**
- `lane_id` must exist and be owned by the account
- Token transfers with missing fields are filtered out

## Package Information

- **Package ID:** `com.r3e.services.ccip`
- **Version:** 1.0.0
- **Author:** R3E Network
- **License:** MIT
- **Service Name:** `ccip`
- **Domain:** `ccip`

## Capabilities

- `ccip.send`: Send cross-chain messages
- `ccip.receive`: Receive cross-chain messages
- `ccip.execute`: Execute CCIP operations
- `ccip.query`: Query CCIP data

## Files

- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/service.go` - Main service implementation
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/domain.go` - Domain type definitions
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/store.go` - Store interface definition
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/store_postgres.go` - PostgreSQL store implementation
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/package.go` - Package registration and initialization
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/testing.go` - Test utilities and mocks
- `/home/neo/git/service_layer/packages/com.r3e.services.ccip/manifest.yaml` - Package manifest
