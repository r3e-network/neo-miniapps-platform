# Common Service Framework

Shared service infrastructure for all Neo Service Layer marble services.

## Overview

The `infrastructure/service` package provides a consistent foundation for all marble services with:
- Standardized lifecycle management (Start/Stop)
- Background worker registration and management
- Hydration hooks for state loading
- Standard HTTP endpoints (/health, /ready, /info)
- Statistics provider interface

## File Structure

| File | Purpose |
|------|---------|
| `base.go` | BaseService implementation |
| `interfaces.go` | Service interfaces and contracts |
| `routes.go` | Standard HTTP handlers and routes |

## Core Components

### BaseService

The foundation for all marble services.

```go
type BaseService struct {
    *marble.Service

    // Lifecycle management
    stopCh   chan struct{}
    stopOnce sync.Once

    // Extensibility hooks
    hydrate func(context.Context) error
    statsFn func() map[string]any

    // Worker management
    workers []func(context.Context)
}
```

### BaseConfig

Configuration structure for creating a BaseService.

```go
type BaseConfig struct {
    ID      string
    Name    string
    Version string
    Marble  *marble.Marble
    DB      database.RepositoryInterface
}
```

## Service Interfaces

### MarbleService (Required)

All marble services must implement this interface:

```go
type MarbleService interface {
    // Identity
    ID() string
    Name() string
    Version() string

    // Lifecycle
    Start(ctx context.Context) error
    Stop() error

    // HTTP
    Router() *mux.Router
}
```

### StatisticsProvider (Optional)

Services can provide runtime statistics:

```go
type StatisticsProvider interface {
    Statistics() map[string]any
}
```

### Hydratable (Optional)

Services can reload state from persistence:

```go
type Hydratable interface {
    Hydrate(ctx context.Context) error
}
```

### ChainIntegrated (Optional)

Services that interact with blockchain:

```go
type ChainIntegrated interface {
    ChainClient() *chain.Client
    TEEFulfiller() *chain.TEEFulfiller
}
```

### HealthChecker (Optional)

Services with custom health status:

```go
type HealthChecker interface {
    HealthStatus() string              // "healthy", "degraded", "unhealthy"
    HealthDetails() map[string]any
}
```

## Usage

### Creating a Service

```go
package myservice

import (
    commonservice "github.com/R3E-Network/service_layer/infrastructure/service"
)

type Service struct {
    *commonservice.BaseService
    // ... service-specific fields
}

func New(cfg Config) (*Service, error) {
    base := commonservice.NewBase(&commonservice.BaseConfig{
        ID:      "myservice",
        Name:    "My Service",
        Version: "1.0.0",
        Marble:  cfg.Marble,
        DB:      cfg.DB,
    })

    s := &Service{
        BaseService: base,
    }

    // Register hydration hook
    base.WithHydrate(s.loadState)

    // Register statistics provider
    base.WithStats(s.getStatistics)

    // Register background workers
    base.AddWorker(s.runBackgroundTask)
    base.AddTickerWorker(time.Minute, s.runPeriodicTask)

    // Register standard routes
    base.RegisterStandardRoutes()

    // Register service-specific routes
    s.registerRoutes()

    return s, nil
}
```

### Adding Background Workers

```go
// Simple worker (runs once, manages its own loop)
base.AddWorker(func(ctx context.Context) {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-base.StopChan():
            return
        case <-ticker.C:
            // Do work
        }
    }
})

// Ticker worker (convenience for periodic tasks)
base.AddTickerWorker(time.Minute, func(ctx context.Context) error {
    // Called every minute
    return nil
})
```

### Providing Statistics

```go
base.WithStats(func() map[string]any {
    return map[string]any{
        "active_jobs":   s.countActiveJobs(),
        "total_requests": s.totalRequests,
        "uptime":        time.Since(s.startTime).String(),
    }
})
```

## Standard HTTP Endpoints

### GET /health

Returns service health status.

**Response:**
```json
{
    "status": "healthy",
    "service": "My Service",
    "version": "1.0.0",
    "enclave": true,
    "timestamp": "2025-12-10T00:00:00Z"
}
```

### GET /ready

Readiness probe suitable for Kubernetes.

Notes:
- Returns `200` when healthy.
- Returns `503` when degraded/unhealthy.

### GET /info

Returns service status with statistics.

**Response:**
```json
{
    "status": "active",
    "service": "My Service",
    "version": "1.0.0",
    "enclave": true,
    "timestamp": "2025-12-10T00:00:00Z",
    "statistics": {
        "active_jobs": 5,
        "total_requests": 1000
    }
}
```

## net/http ServeMux Integration

Some services are composed into an existing `net/http` server rather than being served directly
from the embedded Gorilla router. In those cases you can register the standard endpoints on a
`*http.ServeMux`:

```go
base.RegisterStandardRoutesOnServeMux(mux)
// or:
base.RegisterStandardRoutesOnServeMuxWithOptions(mux, commonservice.RouteOptions{SkipInfo: true})
```

## Lifecycle Management

### Start Sequence

1. Call `Start(ctx)` on BaseService
2. Underlying marble.Service starts
3. Hydrate function called (if registered)
4. Background workers launched

### Stop Sequence

1. Call `Stop()` on BaseService
2. Stop channel closed (signals workers)
3. Workers receive stop signal via `StopChan()`
4. Underlying marble.Service stops

### Safe Stop Handling

```go
// Stop channel is protected by sync.Once - safe to call multiple times
func (b *BaseService) Stop() error {
    b.stopOnce.Do(func() {
        close(b.stopCh)
    })
    return b.Service.Stop()
}
```

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `infrastructure/marble` | MarbleRun/EGo integration |
| `infrastructure/database` | Repository interface |
| `infrastructure/chain` | Blockchain interaction |
| `infrastructure/httputil` | HTTP response helpers |

### External Packages

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP router |

## Services Using This Framework

All Neo Service Layer services extend BaseService:

- **Datafeeds (NeoFeeds)**: Price feed aggregation
- **Randomness**: Provided via NeoCompute scripts (no dedicated VRF service)
- **Automation (NeoFlow)**: Task automation
- **Confidential Oracle (NeoOracle)**: External fetch with controls
- **Confidential Compute (NeoCompute)**: Restricted JS execution
- **AccountPool (NeoAccounts)**: Account pool management (infrastructure)
- **GlobalSigner**: TEE key management + signing (infrastructure)

## Related Documentation

- [Marble Package](../marble/README.md)
- [Database Package](../database/README.md)
