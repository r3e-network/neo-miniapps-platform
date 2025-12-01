# Data Streams Service

High-frequency data streaming service for real-time data ingestion, storage, and distribution with SLA monitoring and latency tracking.

## Overview

The Data Streams Service (`com.r3e.services.datastreams`) provides infrastructure for managing high-frequency data channels (streams) and their associated data samples (frames). It supports real-time data ingestion with latency tracking, SLA monitoring, and automatic data bus publishing for downstream consumers.

**Package ID**: `com.r3e.services.datastreams`
**Service Name**: `datastreams`
**Capabilities**: `stream.publish`, `stream.subscribe`, `datastreams`

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Data Streams Service                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │   Service    │──────│ AccountChecker│                     │
│  │   Engine     │      └──────────────┘                     │
│  └──────┬───────┘                                            │
│         │                                                     │
│         │         ┌──────────────────────────────┐          │
│         ├─────────│  Stream Management           │          │
│         │         │  - Create/Update/Get/List    │          │
│         │         │  - Status: inactive/active/  │          │
│         │         │    paused                    │          │
│         │         │  - SLA tracking              │          │
│         │         └──────────────────────────────┘          │
│         │                                                     │
│         │         ┌──────────────────────────────┐          │
│         ├─────────│  Frame Management            │          │
│         │         │  - Ingest frames             │          │
│         │         │  - Sequence tracking         │          │
│         │         │  - Latency monitoring        │          │
│         │         │  - Status: ok/late/error     │          │
│         │         └──────────────────────────────┘          │
│         │                                                     │
│         │         ┌──────────────────────────────┐          │
│         └─────────│  Data Bus Integration        │          │
│                   │  - Push frames to topics     │          │
│                   │  - Topic: datastreams/{id}   │          │
│                   └──────────────────────────────┘          │
│                                                               │
│  ┌──────────────────────────────────────────────┐           │
│  │              Store Interface                  │           │
│  │  - PostgresStore (production)                 │           │
│  │  - MemoryStore (testing)                      │           │
│  └──────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
   ┌──────────┐                        ┌──────────┐
   │ Database │                        │ Data Bus │
   │ (Postgres)│                       │ (Events) │
   └──────────┘                        └──────────┘
```

## Key Components

### Service (`Service`)

The main service implementation that orchestrates stream and frame management.

**Responsibilities**:
- Stream lifecycle management (create, update, retrieve, list)
- Frame ingestion and storage
- Account ownership validation
- SLA and latency monitoring
- Data bus publishing
- Observability (metrics, logging, tracing)

**Key Methods**:
- `CreateStream(ctx, stream)` - Register a new data stream
- `UpdateStream(ctx, stream)` - Modify stream configuration
- `GetStream(ctx, accountID, streamID)` - Retrieve stream with ownership check
- `ListStreams(ctx, accountID)` - List all streams for an account
- `CreateFrame(ctx, accountID, streamID, seq, payload, latencyMS, status, metadata)` - Ingest a data frame
- `ListFrames(ctx, accountID, streamID, limit)` - Retrieve recent frames
- `LatestFrame(ctx, accountID, streamID)` - Get the most recent frame
- `Push(ctx, topic, payload)` - DataEngine interface for external frame ingestion

### Store Interface (`Store`)

Persistence abstraction for streams and frames.

**Implementations**:
- `PostgresStore` - Production persistence using PostgreSQL
- `MemoryStore` - In-memory storage for testing

**Methods**:
```go
type Store interface {
    CreateStream(ctx, stream) (Stream, error)
    UpdateStream(ctx, stream) (Stream, error)
    GetStream(ctx, id) (Stream, error)
    ListStreams(ctx, accountID) ([]Stream, error)

    CreateFrame(ctx, frame) (Frame, error)
    ListFrames(ctx, streamID, limit) ([]Frame, error)
    GetLatestFrame(ctx, streamID) (Frame, error)
}
```

### Account Checker (`AccountChecker`)

Validates account existence and ownership for multi-tenant isolation.

## Domain Types

### Stream

Represents a high-frequency data channel configuration.

```go
type Stream struct {
    ID          string            // Unique stream identifier
    AccountID   string            // Owning account
    Name        string            // Human-readable name (required)
    Symbol      string            // Trading symbol or identifier (required, uppercase)
    Description string            // Optional description
    Frequency   string            // Expected update frequency (e.g., "1s", "100ms")
    SLAms       int               // Service level agreement in milliseconds
    Status      StreamStatus      // inactive, active, paused
    Metadata    map[string]string // Custom key-value pairs
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

**Stream Status Values**:
- `inactive` - Stream is registered but not actively receiving data
- `active` - Stream is operational and receiving data
- `paused` - Stream is temporarily suspended

**Validation Rules**:
- `Name` is required and trimmed
- `Symbol` is required, trimmed, and converted to uppercase
- `Status` defaults to `inactive` if not specified
- `SLAms` cannot be negative (corrected to 0)

### Frame

Captures a single data sample from a stream.

```go
type Frame struct {
    ID        string            // Unique frame identifier
    AccountID string            // Owning account
    StreamID  string            // Parent stream identifier
    Sequence  int64             // Monotonic sequence number (required, positive)
    Payload   map[string]any    // Arbitrary JSON data
    LatencyMS int               // Ingestion latency in milliseconds
    Status    FrameStatus       // ok, late, error
    Metadata  map[string]string // Custom key-value pairs
    CreatedAt time.Time         // Ingestion timestamp
}
```

**Frame Status Values**:
- `ok` - Frame received within SLA
- `late` - Frame received outside SLA window
- `error` - Frame processing encountered an error

**Validation Rules**:
- `Sequence` must be positive (> 0)
- `LatencyMS` cannot be negative (corrected to 0)
- `Status` defaults to `ok` if not specified
- `Payload` must be a map structure

## API Endpoints

All endpoints are automatically registered using the declarative HTTP method naming convention.

### Stream Management

#### List Streams
```
GET /streams
```
Returns all streams for the authenticated account.

**Response**: `[]Stream`

#### Create Stream
```
POST /streams
```
Register a new data stream.

**Request Body**:
```json
{
  "name": "Market Data Feed",
  "symbol": "ETH-USD",
  "description": "Ethereum price feed",
  "frequency": "1s",
  "sla_ms": 1000,
  "status": "active",
  "metadata": {
    "source": "exchange-api",
    "env": "production"
  }
}
```

**Response**: `Stream`

#### Get Stream
```
GET /streams/{id}
```
Retrieve a specific stream with ownership validation.

**Response**: `Stream`

#### Update Stream
```
PATCH /streams/{id}
```
Modify stream configuration. Only provided fields are updated.

**Request Body** (partial update):
```json
{
  "status": "paused",
  "sla_ms": 2000
}
```

**Response**: `Stream`

### Frame Management

#### List Frames
```
GET /streams/{id}/frames?limit=100
```
Retrieve recent frames for a stream.

**Query Parameters**:
- `limit` (optional) - Maximum number of frames to return (default: service limit)

**Response**: `[]Frame`

#### Create Frame
```
POST /streams/{id}/frames
```
Ingest a new data frame.

**Request Body**:
```json
{
  "sequence": 1234567890,
  "payload": {
    "price": 1850.50,
    "volume": 1234.56,
    "timestamp": "2025-12-01T10:30:00Z"
  },
  "latency_ms": 45,
  "status": "ok",
  "metadata": {
    "source": "websocket"
  }
}
```

**Response**: `Frame`

**Notes**:
- If `sequence` is omitted, uses `time.Now().UnixNano()`
- Frame is automatically published to data bus topic `datastreams/{stream_id}`

#### Get Latest Frame
```
GET /streams/{id}/latest
```
Retrieve the most recent frame for a stream.

**Response**: `Frame`

## Configuration

The service is configured through the `ServiceEngine` framework:

```go
framework.ServiceConfig{
    Name:         "datastreams",
    Description:  "Data stream definitions and frames",
    DependsOn:    []string{"store", "svc-accounts"},
    RequiresAPIs: []engine.APISurface{
        engine.APISurfaceStore,  // Database access
        engine.APISurfaceData,   // Data bus publishing
    },
    Capabilities: []string{"datastreams"},
}
```

### Dependencies

- **store** - Database connection for persistence
- **svc-accounts** - Account validation service
- **Data Bus** - Event publishing for frame distribution

### Environment Variables

Configuration is managed through the runtime package. Standard database and logging configuration applies.

## Observability

### Metrics

The service emits the following metrics:

**Counters**:
- `datastreams_streams_created_total{account_id}` - Total streams created
- `datastreams_streams_updated_total{account_id}` - Total stream updates
- `datastreams_frames_created_total{stream_id}` - Total frames ingested

**Histograms**:
- `datastreams_frame_latency_seconds{stream_id}` - Frame ingestion latency distribution

**Observations**:
- Stream operations (create, update, get, list)
- Frame operations (create, list, latest)

### Logging

Structured logging with contextual fields:
- Stream creation: `stream_id`, `account_id`
- Frame ingestion: `stream_id`, `sequence`
- Errors: Full error context with stack traces

### Data Bus Events

Frames are published to the data bus on successful ingestion:

**Topic**: `datastreams/{stream_id}`

**Payload**:
```json
{
  "stream_id": "stream-uuid",
  "sequence": 1234567890,
  "status": "ok",
  "latency_ms": 45,
  "payload": {
    "price": 1850.50,
    "volume": 1234.56
  }
}
```

**Error Handling**:
- If data bus is unavailable (`ErrBusUnavailable`), logs warning and continues
- Other bus errors fail the frame creation

## Testing

### Running Tests

```bash
# Run all tests
go test ./packages/com.r3e.services.datastreams/

# Run with coverage
go test -cover ./packages/com.r3e.services.datastreams/

# Run specific test
go test -run TestService_CreateStreamAndList ./packages/com.r3e.services.datastreams/

# Verbose output
go test -v ./packages/com.r3e.services.datastreams/
```

### Test Coverage

The service includes comprehensive test coverage:

**Stream Management**:
- Create and list streams
- Update stream configuration
- Get stream with ownership validation
- Stream validation (required fields, status values)
- Account ownership enforcement

**Frame Management**:
- Frame lifecycle (create, list, latest)
- Frame validation (sequence, latency)
- Ownership validation

**Service Lifecycle**:
- Start, ready, stop operations
- Manifest and descriptor generation
- Observation hooks integration

**Data Engine Interface**:
- Push method validation
- Service readiness checks
- Payload type validation

### Test Utilities

**Mock Account Checker**:
```go
accounts := NewMockAccountChecker("acct-1", "acct-2")
```

**Memory Store**:
```go
store := NewMemoryStore()
```

**Example Test**:
```go
func TestService_CreateStreamAndList(t *testing.T) {
    store := NewMemoryStore()
    accounts := NewMockAccountChecker("acct-1")
    svc := New(accounts, store, nil)

    stream, err := svc.CreateStream(context.Background(), Stream{
        AccountID: "acct-1",
        Name:      "Market",
        Symbol:    "ETH-USD",
    })
    if err != nil {
        t.Fatalf("create stream: %v", err)
    }

    streams, err := svc.ListStreams(context.Background(), "acct-1")
    if err != nil {
        t.Fatalf("list streams: %v", err)
    }
    if len(streams) != 1 {
        t.Fatalf("expected one stream")
    }
}
```

## Security

### Multi-Tenant Isolation

- All operations validate account ownership
- Streams are scoped to accounts
- Cross-account access is prevented through `ValidateOwnership` checks

### Input Validation

- Required fields enforced (name, symbol)
- Status values validated against enum
- Numeric fields sanitized (negative values corrected)
- Metadata normalized and sanitized

### Data Integrity

- Sequence numbers must be positive
- Stream IDs validated before frame creation
- Ownership verified on all read/write operations

## Performance Considerations

### Frame Ingestion

- Frames are written to database synchronously
- Data bus publishing is asynchronous with error handling
- Latency metrics track end-to-end ingestion time

### Query Optimization

- List operations support limit clamping
- Latest frame queries optimized for single record retrieval
- Account-scoped queries use indexed lookups

### Scalability

- Stateless service design supports horizontal scaling
- Database connection pooling managed by runtime
- Data bus publishing decouples consumers from ingestion path

## Error Handling

### Common Errors

- `ErrNotFound` - Stream or frame does not exist
- `ErrUnauthorized` - Account ownership validation failed
- `ErrRequired` - Required field missing (name, symbol)
- `ErrInvalidStatus` - Invalid stream or frame status value
- `ErrBusUnavailable` - Data bus temporarily unavailable (non-fatal)

### Error Responses

All errors are returned with descriptive messages suitable for API responses.

## Integration Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
)

func main() {
    // Initialize service
    accounts := datastreams.NewMockAccountChecker("acct-1")
    store := datastreams.NewMemoryStore()
    svc := datastreams.New(accounts, store, nil)

    ctx := context.Background()
    svc.Start(ctx)
    defer svc.Stop(ctx)

    // Create a stream
    stream, _ := svc.CreateStream(ctx, datastreams.Stream{
        AccountID:   "acct-1",
        Name:        "Price Feed",
        Symbol:      "BTC-USD",
        Frequency:   "1s",
        SLAms:       1000,
        Status:      datastreams.StreamStatusActive,
    })

    // Ingest frames
    frame, _ := svc.CreateFrame(ctx, "acct-1", stream.ID, 1,
        map[string]any{"price": 45000.00, "volume": 1.5},
        50, datastreams.FrameStatusOK, nil)

    // Query latest
    latest, _ := svc.LatestFrame(ctx, "acct-1", stream.ID)
    println("Latest frame sequence:", latest.Sequence)
}
```

## Related Services

- **Accounts Service** (`svc-accounts`) - Account management and validation
- **Data Feeds Service** (`com.r3e.services.datafeeds`) - Oracle data aggregation
- **Functions Service** (`com.r3e.services.functions`) - Serverless data processing

## License

Part of the R3E Network Service Layer.
