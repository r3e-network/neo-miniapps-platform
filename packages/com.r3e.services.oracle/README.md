# Oracle Service

**Package:** `com.r3e.services.oracle`
**Version:** 1.0.0
**License:** MIT

Decentralized oracle service for fetching external data through configurable HTTP data sources. Provides request lifecycle management, automatic retry logic, multi-source aggregation, and fee collection capabilities.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Key Components](#key-components)
- [Domain Types](#domain-types)
- [API Endpoints](#api-endpoints)
- [Configuration](#configuration)
- [Dependencies](#dependencies)
- [Usage Examples](#usage-examples)
- [Testing](#testing)
- [Monitoring](#monitoring)

## Overview

The Oracle service enables smart contracts and applications to fetch external data through a managed request/response lifecycle. It supports:

- **Configurable Data Sources**: Define HTTP endpoints with custom headers, methods, and payloads
- **Request Management**: Asynchronous request processing with status tracking
- **Multi-Source Aggregation**: Query multiple data sources and compute median values for numeric results
- **Automatic Retry**: Configurable retry policies with exponential backoff
- **Fee Collection**: Optional per-request fee charging aligned with OracleHub.cs contract model
- **Observability**: Built-in metrics, logging, and distributed tracing support

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Oracle Service                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐  │
│  │   Service    │◄─────┤  Dispatcher  │─────►│  Resolver    │  │
│  │   (Core)     │      │  (Runner)    │      │  (HTTP)      │  │
│  └──────┬───────┘      └──────────────┘      └──────┬───────┘  │
│         │                                             │          │
│         │ manages                          executes  │          │
│         ▼                                             ▼          │
│  ┌──────────────┐                          ┌──────────────┐    │
│  │ DataSource   │                          │ HTTP Client  │    │
│  │   Store      │                          │  (External)  │    │
│  └──────────────┘                          └──────────────┘    │
│         │                                                        │
│         │ persists                                              │
│         ▼                                                        │
│  ┌──────────────┐                                               │
│  │   Request    │                                               │
│  │   Store      │                                               │
│  └──────────────┘                                               │
│                                                                   │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │  Storage Layer        │
                    │  (PostgreSQL/Memory)  │
                    └───────────────────────┘

Request Lifecycle:
┌─────────┐    ┌─────────┐    ┌───────────┐    ┌──────────┐
│ Pending │───►│ Running │───►│ Succeeded │    │  Failed  │
└─────────┘    └─────────┘    └───────────┘    └──────────┘
     │              │                                  ▲
     │              └──────────────────────────────────┘
     └────────────────────────────────────────────────┘
                    (retry on failure)
```

## Key Components

### Service (service.go)

Core service managing data sources and oracle requests. Implements the `EventPublisher` interface for event-driven request creation.

**Responsibilities:**
- Data source CRUD operations with ownership validation
- Request lifecycle management (create, mark running, complete, fail, retry)
- Account validation and authorization
- Fee collection integration
- HTTP API endpoint handlers
- Observability (metrics, logging, tracing)

**Key Methods:**
- `CreateSource()` - Register new HTTP data source
- `UpdateSource()` - Modify data source configuration
- `SetSourceEnabled()` - Enable/disable data source
- `CreateRequest()` - Enqueue oracle request with optional fee
- `CompleteRequest()` - Mark request as succeeded with result
- `FailRequest()` - Mark request as failed with optional fee refund
- `RetryRequest()` - Reset failed request to pending state

### Dispatcher (dispatcher.go)

Background worker that polls pending requests and forwards them to the configured resolver. Implements lifecycle management (Start/Stop/Ready).

**Responsibilities:**
- Periodic polling of pending requests (default: 10s interval)
- Request state transitions (pending → running → succeeded/failed)
- Retry scheduling with backoff
- TTL enforcement and dead-letter handling
- Attempt counting and max-attempt enforcement
- Metrics recording for staleness and attempt outcomes

**Configuration:**
- `WithResolver()` - Set request resolver implementation
- `WithRetryPolicy()` - Configure max attempts, backoff interval, TTL
- `EnableDeadLetter()` - Toggle automatic failure of exhausted requests
- `WithTracer()` - Enable distributed tracing

### HTTPResolver (resolver_http.go)

Executes oracle requests by making HTTP calls to configured data sources. Supports multi-source aggregation with median calculation.

**Responsibilities:**
- HTTP request execution with configurable timeout (default: 10s)
- Response body reading with size limit (default: 1 MiB)
- Transient error detection and retry signaling (429, 5xx status codes)
- Multi-source query execution with alternate sources
- Numeric result aggregation using median calculation
- Distributed tracing integration

**Features:**
- Automatic payload encoding (JSON to query params for GET, body for POST)
- Custom header support
- Retry on transient failures (5s default retry interval)
- Alternate source fallback via `alternate_source_ids` in payload

### Store Interface (store.go)

Persistence contract for data sources and requests. Implementations provided by infrastructure layer.

**Required Methods:**
```go
CreateDataSource(ctx, src) (DataSource, error)
UpdateDataSource(ctx, src) (DataSource, error)
GetDataSource(ctx, id) (DataSource, error)
ListDataSources(ctx, accountID) ([]DataSource, error)

CreateRequest(ctx, req) (Request, error)
UpdateRequest(ctx, req) (Request, error)
GetRequest(ctx, id) (Request, error)
ListRequests(ctx, accountID, limit, status) ([]Request, error)
ListPendingRequests(ctx) ([]Request, error)
```

## Domain Types

### DataSource

Represents a configured HTTP endpoint for fetching external data.

```go
type DataSource struct {
    ID          string            // Unique identifier
    AccountID   string            // Owner account
    Name        string            // Human-readable name (unique per account)
    Description string            // Optional description
    URL         string            // HTTP endpoint URL
    Method      string            // HTTP method (GET, POST, etc.)
    Headers     map[string]string // Custom HTTP headers
    Body        string            // Default request body
    Enabled     bool              // Enable/disable flag
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Request

Represents a single oracle data fetch request.

```go
type Request struct {
    ID           string        // Unique identifier
    AccountID    string        // Requesting account
    DataSourceID string        // Target data source
    Status       RequestStatus // pending|running|succeeded|failed
    Attempts     int           // Execution attempt counter
    Fee          int64         // Request fee in smallest unit
    Payload      string        // Request-specific payload (JSON)
    Result       string        // Fetched data (on success)
    Error        string        // Error message (on failure)
    CreatedAt    time.Time     // Request timestamp
    UpdatedAt    time.Time
    CompletedAt  time.Time     // Fulfillment timestamp
}
```

**Status Transitions:**
- `pending` → `running` (dispatcher picks up request)
- `running` → `succeeded` (resolver returns success)
- `running` → `failed` (resolver returns failure or max attempts exceeded)
- `failed` → `pending` (manual retry via API)

### RequestResolver Interface

Strategy interface for resolving oracle requests.

```go
type RequestResolver interface {
    Resolve(ctx, req) (done, success bool, result, errMsg string, retryAfter time.Duration, err error)
}
```

**Return Values:**
- `done` - Request is terminal (succeeded or permanently failed)
- `success` - Request succeeded (only valid if done=true)
- `result` - Fetched data (if success=true)
- `errMsg` - Error description (if success=false)
- `retryAfter` - Suggested retry delay (if done=false)
- `err` - Transient error requiring retry

## API Endpoints

All endpoints require authentication and are scoped to the authenticated account.

### Data Sources

#### List Data Sources
```http
GET /oracle/sources
```

**Response:**
```json
[
  {
    "id": "src_abc123",
    "account_id": "acc_xyz",
    "name": "CoinGecko ETH Price",
    "description": "Ethereum price in USD",
    "url": "https://api.coingecko.com/api/v3/simple/price",
    "method": "GET",
    "headers": {},
    "body": "",
    "enabled": true,
    "created_at": "2025-12-01T10:00:00Z",
    "updated_at": "2025-12-01T10:00:00Z"
  }
]
```

#### Create Data Source
```http
POST /oracle/sources
Content-Type: application/json

{
  "name": "CoinGecko ETH Price",
  "url": "https://api.coingecko.com/api/v3/simple/price",
  "method": "GET",
  "description": "Ethereum price in USD",
  "headers": {
    "Accept": "application/json"
  },
  "body": ""
}
```

**Validation:**
- `name` - Required, unique per account
- `url` - Required, valid HTTP(S) URL
- `method` - Optional, defaults to GET

#### Get Data Source
```http
GET /oracle/sources/{id}
```

**Authorization:** Must own the data source.

#### Update Data Source
```http
PATCH /oracle/sources/{id}
Content-Type: application/json

{
  "description": "Updated description",
  "enabled": false
}
```

**Updatable Fields:**
- `name` - Must remain unique per account
- `url`
- `method`
- `description`
- `headers`
- `body`
- `enabled` - Triggers enable/disable action

**Authorization:** Must own the data source.

### Oracle Requests

#### List Requests
```http
GET /oracle/requests?limit=50&status=pending
```

**Query Parameters:**
- `limit` - Max results (default: 100, max: 1000)
- `status` - Filter by status (pending|running|succeeded|failed)

**Response:**
```json
[
  {
    "id": "req_def456",
    "account_id": "acc_xyz",
    "data_source_id": "src_abc123",
    "status": "succeeded",
    "attempts": 1,
    "fee": 1000,
    "payload": "{\"ids\":\"ethereum\",\"vs_currencies\":\"usd\"}",
    "result": "{\"ethereum\":{\"usd\":2500.00}}",
    "error": "",
    "created_at": "2025-12-01T10:05:00Z",
    "updated_at": "2025-12-01T10:05:15Z",
    "completed_at": "2025-12-01T10:05:15Z"
  }
]
```

#### Create Request
```http
POST /oracle/requests
Content-Type: application/json

{
  "data_source_id": "src_abc123",
  "payload": "{\"ids\":\"ethereum\",\"vs_currencies\":\"usd\"}"
}
```

**Validation:**
- `data_source_id` - Required, must exist and be enabled
- `payload` - Optional JSON string (merged with data source body)

**Fee Collection:**
If fee collector is configured and default fee > 0, fee is charged immediately upon request creation.

#### Get Request
```http
GET /oracle/requests/{id}
```

**Authorization:** Must own the request.

#### Retry Failed Request
```http
PATCH /oracle/requests/{id}
Content-Type: application/json

{
  "status": "retry"
}
```

**Behavior:**
- Resets status to `pending`
- Clears `attempts`, `error`, `result`, `completed_at`
- Request will be picked up by dispatcher on next tick

**Authorization:** Must own the request.

### Event Publishing

The service implements `EventPublisher` for event-driven request creation:

```go
service.Publish(ctx, "request", map[string]any{
    "account_id": "acc_xyz",
    "source_id": "src_abc123",
    "payload": map[string]any{
        "ids": "ethereum",
        "vs_currencies": "usd",
    },
})
```

## Configuration

### Service Options

```go
// Fee collection (aligned with OracleHub.cs contract)
WithFeeCollector(feeCollector engine.FeeCollector)
WithDefaultFee(fee int64) // Fee in smallest unit (e.g., wei)

// Example initialization
svc := oracle.New(
    accountChecker,
    store,
    logger,
    oracle.WithDefaultFee(1000), // 1000 wei per request
    oracle.WithFeeCollector(feeCollector),
)
```

### Dispatcher Configuration

```go
dispatcher := oracle.NewDispatcher(service, logger)

// Configure resolver
httpResolver := oracle.NewHTTPResolver(service, nil, logger)
httpResolver.WithTracer(tracer)
dispatcher.WithResolver(httpResolver)

// Configure retry policy
dispatcher.WithRetryPolicy(
    5,                  // max attempts
    10*time.Second,     // backoff interval
    5*time.Minute,      // TTL
)

// Enable dead-letter queue
dispatcher.EnableDeadLetter(true)

// Enable tracing
dispatcher.WithTracer(tracer)
```

### HTTPResolver Configuration

```go
// Custom HTTP client with timeout
client := &http.Client{
    Timeout: 30 * time.Second,
}

resolver := oracle.NewHTTPResolver(service, client, logger)
resolver.WithTracer(tracer)
```

### Multi-Source Requests

Request payload can specify alternate data sources for redundancy:

```json
{
  "data_source_id": "primary_source",
  "payload": "{\"alternate_source_ids\":[\"backup1\",\"backup2\"],\"symbol\":\"ETH\"}"
}
```

**Behavior:**
- Queries primary source first, then alternates if primary fails
- For numeric results, computes median across all successful responses
- Returns first non-numeric result if no numeric values available

## Dependencies

### Required Services
- **store** - Persistence layer (PostgreSQL recommended)
- **svc-accounts** - Account validation

### Required APIs
- `APISurfaceStore` - Data persistence
- `APISurfaceData` - Data operations
- `APISurfaceEvent` - Event publishing

### Optional Dependencies
- **FeeCollector** - For request fee charging/refunding
- **Tracer** - For distributed tracing (OpenTelemetry compatible)

### External Libraries
```go
import (
    "github.com/R3E-Network/service_layer/pkg/logger"
    "github.com/R3E-Network/service_layer/system/framework"
    "github.com/R3E-Network/service_layer/system/framework/core"
)
```

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "time"

    "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    ctx := context.Background()
    log := logger.NewDefault("oracle-example")

    // Initialize service
    svc := oracle.New(accountChecker, store, log)

    // Create data source
    source, err := svc.CreateSource(
        ctx,
        "acc_123",
        "ETH Price",
        "https://api.coingecko.com/api/v3/simple/price",
        "GET",
        "Ethereum price feed",
        map[string]string{"Accept": "application/json"},
        "",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create request
    req, err := svc.CreateRequest(
        ctx,
        "acc_123",
        source.ID,
        `{"ids":"ethereum","vs_currencies":"usd"}`,
    )
    if err != nil {
        log.Fatal(err)
    }

    // Setup dispatcher
    dispatcher := oracle.NewDispatcher(svc, log)
    resolver := oracle.NewHTTPResolver(svc, nil, log)
    dispatcher.WithResolver(resolver)

    // Start dispatcher
    if err := dispatcher.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer dispatcher.Stop(ctx)

    // Wait for completion
    time.Sleep(15 * time.Second)

    // Check result
    result, err := svc.GetRequest(ctx, req.ID)
    if err != nil {
        log.Fatal(err)
    }

    log.Infof("Status: %s, Result: %s", result.Status, result.Result)
}
```

### Fee-Based Requests

```go
// Initialize with fee collection
svc := oracle.New(
    accountChecker,
    store,
    log,
    oracle.WithDefaultFee(1000),
    oracle.WithFeeCollector(feeCollector),
)

// Create request with custom fee
req, err := svc.CreateRequestWithOptions(
    ctx,
    accountID,
    sourceID,
    payload,
    oracle.CreateRequestOptions{
        Fee: ptrInt64(2000), // Override default fee
    },
)

// Fail request with fee refund
_, err = svc.FailRequestWithOptions(
    ctx,
    req.ID,
    "upstream service unavailable",
    oracle.FailRequestOptions{
        RefundFee: true,
    },
)
```

### Custom Resolver

```go
// Implement custom resolver
type CustomResolver struct{}

func (r *CustomResolver) Resolve(
    ctx context.Context,
    req oracle.Request,
) (done, success bool, result, errMsg string, retryAfter time.Duration, err error) {
    // Custom resolution logic
    return true, true, "custom result", "", 0, nil
}

// Use custom resolver
dispatcher.WithResolver(&CustomResolver{})
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./packages/com.r3e.services.oracle/...

# Run with coverage
go test -cover ./packages/com.r3e.services.oracle/...

# Run specific test
go test -run TestServiceCreateSource ./packages/com.r3e.services.oracle/
```

### Integration Tests

```bash
# Requires PostgreSQL
export DATABASE_URL="postgres://user:pass@localhost/testdb"
go test -tags=integration ./packages/com.r3e.services.oracle/...
```

### Manual Testing

```bash
# Start service
go run cmd/server/main.go

# Create data source
curl -X POST http://localhost:8080/oracle/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Source",
    "url": "https://httpbin.org/json",
    "method": "GET"
  }'

# Create request
curl -X POST http://localhost:8080/oracle/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "data_source_id": "src_xxx",
    "payload": "{}"
  }'

# Check request status
curl http://localhost:8080/oracle/requests/req_xxx \
  -H "Authorization: Bearer $TOKEN"
```

## Monitoring

### Metrics

The service exposes the following metrics:

**Counters:**
- `oracle_sources_created_total{account_id}` - Data sources created
- `oracle_sources_updated_total{account_id}` - Data sources updated
- `oracle_sources_state_total{account_id,status}` - Enable/disable events
- `oracle_requests_created_total{account_id}` - Requests created
- `oracle_requests_running_total{account_id}` - Requests marked running
- `oracle_requests_completed_total{account_id}` - Successful completions
- `oracle_requests_failed_total{account_id}` - Failed requests
- `oracle_request_attempts_total{account_id}` - Total execution attempts

**Gauges:**
- `oracle_source_enabled{source_id,account_id}` - Source enabled state (0/1)

**Custom Metrics (via pkg/metrics):**
- `oracle_staleness` - Age of oldest pending request
- `oracle_attempt{status}` - Attempt outcomes (success/fail/exhausted)

### Logging

Structured logging with contextual fields:

```go
log.WithField("source_id", src.ID).
    WithField("account_id", accountID).
    Info("oracle source created")
```

**Log Levels:**
- `Info` - Lifecycle events (created, updated, completed)
- `Warn` - Retryable errors, configuration issues
- `Error` - Permanent failures, system errors
- `Debug` - HTTP resolver execution details

### Tracing

Distributed tracing support via `core.Tracer` interface:

**Spans:**
- `oracle.dispatch` - Dispatcher processing
- `oracle.http_resolve` - HTTP resolver execution

**Attributes:**
- `request_id` - Oracle request ID
- `account_id` - Account identifier
- `data_source_id` - Data source ID

### Health Checks

```go
// Check service readiness
err := service.Ready(ctx)

// Check dispatcher readiness
err := dispatcher.Ready(ctx)
```

## Resource Limits

Defined in `manifest.yaml`:

- **Storage:** 200 MB max
- **Concurrent Requests:** 1000 max
- **Requests/Second:** 5000 max
- **Events/Second:** 1000 max

## Security Considerations

1. **Ownership Validation:** All operations validate account ownership
2. **Data Source Isolation:** Sources are scoped to accounts
3. **URL Validation:** Data source URLs are validated on creation
4. **Response Size Limits:** HTTP responses limited to 1 MiB
5. **Timeout Protection:** HTTP requests timeout after 10s
6. **Fee Collection:** Fees charged before request execution to prevent abuse

## Troubleshooting

### Request Stuck in Pending

**Symptoms:** Request remains in `pending` status indefinitely.

**Causes:**
- Dispatcher not running
- Resolver not configured
- Data source disabled

**Resolution:**
```bash
# Check dispatcher status
curl http://localhost:8080/health/oracle-dispatcher

# Verify data source enabled
curl http://localhost:8080/oracle/sources/{id}

# Check dispatcher logs
grep "oracle-dispatcher" /var/log/service.log
```

### Request Failing with Timeout

**Symptoms:** Requests fail with "timeout awaiting oracle callback" error.

**Causes:**
- Upstream service slow/unavailable
- HTTP client timeout too short
- Network connectivity issues

**Resolution:**
```go
// Increase HTTP client timeout
client := &http.Client{Timeout: 30 * time.Second}
resolver := oracle.NewHTTPResolver(service, client, logger)

// Increase dispatcher TTL
dispatcher.WithRetryPolicy(5, 10*time.Second, 10*time.Minute)
```

### Fee Collection Failures

**Symptoms:** Requests fail with "fee collection failed" error.

**Causes:**
- Insufficient account balance
- Fee collector not configured
- Fee collector service unavailable

**Resolution:**
- Verify account has sufficient balance
- Check fee collector service health
- Review fee collector logs for errors

## License

MIT License - see LICENSE file for details.

## Support

For issues and questions:
- GitHub Issues: https://github.com/R3E-Network/service_layer/issues
- Documentation: https://docs.r3e.network/services/oracle
- Contact: support@r3e.network
