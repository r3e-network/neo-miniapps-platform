# Data Feeds Service

## Overview

The Data Feeds Service provides Chainlink-style aggregated price feed management with multi-signer support, configurable aggregation strategies, and threshold-based update mechanisms. This service enables decentralized oracle networks to submit price observations that are aggregated according to configurable quorum and aggregation rules.

**Package ID**: `com.r3e.services.datafeeds`
**Service Name**: `datafeeds`
**Domain**: `datafeeds`

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Data Feeds Service                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐  │
│  │   HTTP API   │      │   Service    │      │    Store     │  │
│  │   Handlers   │─────▶│    Logic     │─────▶│  Interface   │  │
│  └──────────────┘      └──────────────┘      └──────────────┘  │
│         │                      │                      │          │
│         │                      │                      ▼          │
│         │                      │              ┌──────────────┐  │
│         │                      │              │  PostgreSQL  │  │
│         │                      │              │   Storage    │  │
│         │                      │              └──────────────┘  │
│         │                      │                                │
│         │                      ▼                                │
│         │              ┌──────────────┐                         │
│         │              │ Aggregation  │                         │
│         │              │   Engine     │                         │
│         │              │              │                         │
│         │              │ • Median     │                         │
│         │              │ • Mean       │                         │
│         │              │ • Min/Max    │                         │
│         │              └──────────────┘                         │
│         │                      │                                │
│         │                      ▼                                │
│         │              ┌──────────────┐                         │
│         │              │  Validation  │                         │
│         │              │              │                         │
│         │              │ • Heartbeat  │                         │
│         │              │ • Deviation  │                         │
│         │              │ • Quorum     │                         │
│         │              └──────────────┘                         │
│         │                      │                                │
│         │                      ▼                                │
│         └──────────────▶ Event Bus (Data Push)                  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Key Components

### Service Core (`service.go`)

The main service implementation that orchestrates feed management and update processing.

**Responsibilities**:
- Feed lifecycle management (create, update, retrieve, list)
- Price update submission and validation
- Signer authorization and signature verification
- Aggregation strategy execution
- Heartbeat and deviation threshold enforcement
- Event publishing to data bus
- Observability (metrics, logging)

**Key Methods**:
- `CreateFeed()` - Create a new data feed with configuration
- `UpdateFeed()` - Modify feed parameters
- `GetFeed()` / `ListFeeds()` - Retrieve feed information
- `SubmitUpdate()` - Submit a price observation for aggregation
- `ListUpdates()` - Retrieve update history
- `LatestUpdate()` - Get the most recent accepted update
- `Publish()` - Event-driven update submission (EventPublisher interface)

### Domain Types (`domain.go`)

#### Feed

Represents a data feed configuration with aggregation rules.

```go
type Feed struct {
    ID           string            // Unique feed identifier
    AccountID    string            // Owner account
    Pair         string            // Trading pair (e.g., "ETH/USD")
    Description  string            // Human-readable description
    Decimals     int               // Price precision (e.g., 8 for 8 decimals)
    Heartbeat    time.Duration     // Maximum time between updates
    ThresholdPPM int               // Deviation threshold in parts per million
    SignerSet    []string          // Authorized signer addresses
    Threshold    int               // Minimum signers required for quorum
    Aggregation  string            // Strategy: "median", "mean", "min", "max"
    Metadata     map[string]string // Additional key-value data
    Tags         []string          // Classification tags
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

#### Update

Represents a single price observation submitted by a signer.

```go
type Update struct {
    ID        string            // Unique update identifier
    AccountID string            // Owner account
    FeedID    string            // Associated feed
    RoundID   int64             // Aggregation round number
    Price     string            // Submitted price (decimal string)
    Signer    string            // Signer address
    Timestamp time.Time         // Observation timestamp
    Signature string            // Cryptographic signature
    Status    UpdateStatus      // "pending", "accepted", "rejected"
    Error     string            // Error message if rejected
    Metadata  map[string]string // Aggregation metadata
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### UpdateStatus

```go
const (
    UpdateStatusPending  UpdateStatus = "pending"  // Awaiting quorum
    UpdateStatusAccepted UpdateStatus = "accepted" // Quorum met, aggregated
    UpdateStatusRejected UpdateStatus = "rejected" // Validation failed
)
```

### Store Interface (`store.go`)

Defines the persistence contract for feeds and updates.

```go
type Store interface {
    // Feed operations
    CreateDataFeed(ctx context.Context, feed Feed) (Feed, error)
    UpdateDataFeed(ctx context.Context, feed Feed) (Feed, error)
    GetDataFeed(ctx context.Context, id string) (Feed, error)
    ListDataFeeds(ctx context.Context, accountID string) ([]Feed, error)

    // Update operations
    CreateDataFeedUpdate(ctx context.Context, upd Update) (Update, error)
    ListDataFeedUpdates(ctx context.Context, feedID string, limit int) ([]Update, error)
    ListDataFeedUpdatesByRound(ctx context.Context, feedID string, roundID int64) ([]Update, error)
    GetLatestDataFeedUpdate(ctx context.Context, feedID string) (Update, error)
}
```

**Implementation**: PostgreSQL-backed storage (`store_postgres.go`)

### Aggregation Engine

The service implements four aggregation strategies for combining multiple price observations:

1. **Median** (default): Middle value of sorted prices
2. **Mean**: Arithmetic average of all prices
3. **Min**: Lowest submitted price
4. **Max**: Highest submitted price

**Algorithm Details**:
- Prices are normalized to big integers with configurable decimal precision
- Aggregation occurs when quorum threshold is met
- Result is stored in update metadata as `aggregated_price`

### Validation Rules

#### Feed Creation
- `pair` is required and normalized to uppercase
- `decimals` must be positive
- `heartbeat` defaults to 1 minute if not specified
- `threshold_ppm` must be non-negative
- `signer_set` must contain at least `minSigners` (if configured)
- All signers must be owned by the account (wallet verification)

#### Update Submission
- `round_id` must be positive and monotonically increasing
- `price` must be positive and respect decimal precision
- `signer` must be in the feed's authorized `signer_set`
- `signature` is required when signer is specified
- Duplicate submissions from the same signer in a round are rejected
- New rounds require heartbeat or deviation threshold to be exceeded

#### Heartbeat and Deviation Logic

An update triggers a new round if either condition is met:

1. **Heartbeat**: `current_time - last_update_time >= heartbeat`
2. **Deviation**: `|new_price - last_price| / last_price >= threshold_ppm / 1,000,000`

Example: With `threshold_ppm = 5000` (0.5%), a price change from $100 to $100.51 would trigger an update.

## API Endpoints

All endpoints are automatically registered via the declarative HTTP routing system using the `HTTP{Method}{Path}` naming convention.

### Feed Management

#### List Feeds
```
GET /feeds
```
Returns all feeds owned by the authenticated account.

**Response**: Array of `Feed` objects

#### Create Feed
```
POST /feeds
```
Creates a new data feed configuration.

**Request Body**:
```json
{
  "pair": "ETH/USD",
  "description": "Ethereum to US Dollar",
  "decimals": 8,
  "heartbeat": "5m",
  "threshold_ppm": 5000,
  "aggregation": "median",
  "signer_set": ["0xSigner1", "0xSigner2", "0xSigner3"],
  "tags": ["crypto", "ethereum"],
  "metadata": {
    "source": "chainlink",
    "network": "mainnet"
  }
}
```

**Response**: Created `Feed` object

#### Get Feed
```
GET /feeds/{id}
```
Retrieves a specific feed by ID.

**Response**: `Feed` object

#### Update Feed
```
PATCH /feeds/{id}
```
Updates mutable feed properties.

**Request Body**: Partial `Feed` object (only fields to update)

**Response**: Updated `Feed` object

### Price Updates

#### Submit Update
```
POST /feeds/{id}/updates
```
Submits a price observation for aggregation.

**Request Body**:
```json
{
  "round_id": 42,
  "price": "1850.25",
  "signer": "0xSignerAddress",
  "signature": "0x...",
  "metadata": {
    "source": "binance",
    "confidence": "high"
  }
}
```

**Response**: Created `Update` object with aggregation metadata

**Metadata Fields** (auto-populated):
- `aggregation`: Strategy used
- `signer_count`: Number of submissions in this round
- `quorum`: Required threshold
- `quorum_met`: "true" or "false"
- `aggregated_price`: Final price (if quorum met)

#### List Updates
```
GET /feeds/{id}/updates?limit=50
```
Retrieves recent updates for a feed.

**Query Parameters**:
- `limit` (optional): Max results (default: 100, max: 1000)

**Response**: Array of `Update` objects

#### Get Latest Update
```
GET /feeds/{id}/latest
```
Retrieves the most recent accepted update.

**Response**: `Update` object

## Configuration Options

### Service Initialization

```go
svc := New(accounts, store, log)
```

**Parameters**:
- `accounts`: Account existence validator
- `store`: Persistence implementation
- `log`: Structured logger

### Optional Configuration

#### Aggregation Defaults
```go
svc.WithAggregationConfig(minSigners int, aggregation string)
```
Sets baseline requirements for all feeds.

**Parameters**:
- `minSigners`: Minimum required signers (default: length of signer_set)
- `aggregation`: Default strategy ("median", "mean", "min", "max")

#### Wallet Verification
```go
svc.WithWalletChecker(walletChecker)
```
Enables signer ownership validation during feed creation/update.

#### Observability Hooks
```go
svc.WithObservationHooks(hooks)
```
Configures custom metrics and tracing callbacks.

## Dependencies

### Required Services
- **store**: Database persistence layer
- **svc-accounts**: Account management service

### Required APIs
- `APISurfaceStore`: Database access
- `APISurfaceData`: Event bus for data publishing

### External Libraries
- `github.com/R3E-Network/service_layer/pkg/logger`: Structured logging
- `github.com/R3E-Network/service_layer/system/framework`: Service framework
- Standard library: `context`, `math/big`, `time`, `sort`, `strings`

## Data Flow

### Feed Creation Flow
```
1. HTTP Request → HTTPPostFeeds()
2. Validate account exists
3. Normalize feed configuration
4. Verify signer ownership (if wallet checker configured)
5. Store feed in database
6. Emit metrics and logs
7. Return created feed
```

### Update Submission Flow
```
1. HTTP Request → HTTPPostFeedsIdUpdates()
2. Validate account and feed ownership
3. Verify signer authorization
4. Normalize and validate price
5. Check heartbeat/deviation thresholds (for new rounds)
6. Prevent duplicate signer submissions
7. Retrieve existing round submissions
8. Calculate aggregation if quorum met
9. Store update with status (pending/accepted)
10. Publish to event bus (topic: "datafeeds/{feed_id}")
11. Record staleness metrics
12. Return created update
```

## Testing Instructions

### Unit Tests

The service includes comprehensive test coverage for:
- Feed lifecycle operations
- Update submission and validation
- Signer verification
- Aggregation strategies (median, mean, min, max)
- Heartbeat and deviation thresholds
- Ownership enforcement

**Run tests**:
```bash
cd /home/neo/git/service_layer/packages/com.r3e.services.datafeeds
go test -v
```

**Note**: Most tests require database integration and are skipped in unit test mode. Run with the full integration test suite:

```bash
go test -v -tags=integration
```

### Integration Testing

The service integrates with:
1. PostgreSQL database (via Store interface)
2. Account service (for account validation)
3. Wallet service (for signer ownership verification)
4. Event bus (for data publishing)

**Test Coverage**:
- `TestService_CreateFeed`: Feed creation with validation
- `TestService_CreateFeedRequiresRegisteredSigners`: Signer ownership checks
- `TestService_SubmitUpdate`: Basic update submission
- `TestService_SubmitUpdateSignerVerificationAndAggregation`: Multi-signer quorum
- `TestService_SubmitUpdateHeartbeatDeviation`: Threshold enforcement
- `TestService_UpdateFeed`: Feed modification
- `TestService_GetFeedOwnership`: Access control validation

## Metrics and Observability

### Counters
- `datafeeds_created_total{account_id}`: Total feeds created
- `datafeeds_updated_total{account_id}`: Total feed updates
- `datafeeds_updates_total{feed_id}`: Total price updates submitted

### Gauges
- `datafeeds_feed_staleness_seconds{feed_id,status}`: Time since last update

### Logs
- Feed creation/update events (INFO level)
- Update submissions (INFO level)
- Validation errors (WARN/ERROR level)
- Aggregation fallbacks (WARN level)

### Tracing
Observations are tracked with attributes:
- `account_id`: Owner account
- `feed_id`: Feed identifier
- `resource`: Resource type ("datafeed", "datafeed_update")

## Event Publishing

The service implements the `EventPublisher` interface and publishes updates to the data bus.

### Published Events

**Topic**: `datafeeds/{feed_id}`

**Payload**:
```json
{
  "feed_id": "feed-123",
  "round_id": 42,
  "price": "1850.25",
  "status": "accepted",
  "timestamp": "2025-12-01T10:30:00Z",
  "metadata": {
    "aggregation": "median",
    "signer_count": "3",
    "quorum": "3",
    "quorum_met": "true",
    "aggregated_price": "1850.30"
  }
}
```

### Event Consumption

External services can subscribe to feed updates via the `Publish()` method:

```go
err := service.Publish(ctx, "update", map[string]any{
    "account_id": "acc-123",
    "feed_id": "feed-456",
    "round_id": 42,
    "price": "1850.25",
})
```

## Security Considerations

1. **Signer Authorization**: Only addresses in `signer_set` can submit updates
2. **Signature Verification**: Signatures are required for all signer submissions
3. **Ownership Validation**: Feeds can only be accessed/modified by the owning account
4. **Wallet Ownership**: Signers must be owned by the account (when wallet checker is configured)
5. **Duplicate Prevention**: Each signer can only submit once per round
6. **Monotonic Rounds**: Round IDs must be monotonically increasing

## Error Handling

### Common Errors

- `account not found`: Account does not exist
- `feed not found`: Feed ID is invalid
- `unauthorized`: Account does not own the resource
- `signer {addr} is not authorized for feed {id}`: Signer not in authorized set
- `signer {addr} already submitted for round {n}`: Duplicate submission
- `round_id must be at least {n}`: Round ID too low
- `heartbeat/deviation thresholds not met for new round {n}`: Update not justified
- `price must be positive`: Invalid price value
- `price exceeds maximum decimals ({n})`: Too many decimal places
- `signature is required for signer submissions`: Missing signature

## Performance Characteristics

- **Feed Creation**: O(1) database insert + O(n) signer validation
- **Update Submission**: O(1) database insert + O(m) round aggregation (m = signers per round)
- **Aggregation**: O(n log n) for median, O(n) for mean/min/max
- **List Operations**: Paginated with configurable limits (default: 100, max: 1000)

## Future Enhancements

Potential improvements for consideration:
- Historical price data retention policies
- Advanced aggregation strategies (weighted median, trimmed mean)
- Real-time WebSocket subscriptions for feed updates
- Feed pause/resume functionality
- Automated signer reputation scoring
- Cross-feed price correlation analysis

## References

- Service Framework: `/home/neo/git/service_layer/system/framework/`
- Core API Types: `/home/neo/git/service_layer/system/framework/core/api.go`
- Package Registration: `/home/neo/git/service_layer/packages/com.r3e.services.datafeeds/package.go`
- Domain Types: `/home/neo/git/service_layer/packages/com.r3e.services.datafeeds/domain.go`
- Service Implementation: `/home/neo/git/service_layer/packages/com.r3e.services.datafeeds/service.go`
