# Marble Services Refactoring Plan

## Executive Summary

对 `./services` 目录下 8 个 marble 服务进行代码审查后，发现存在大量重复代码和架构不一致问题。本文档提出标准化方案，预计可消除约 **200+ 行重复代码**，并建立统一的服务接口规范。

## Current State Analysis

### Services Inventory

| Service     | Base Type                   | Workers | Hydration | stopCh       | Standard Routes |
| ----------- | --------------------------- | ------- | --------- | ------------ | --------------- |
| neoaccounts | `commonservice.BaseService` | ✓       | ✓         | ✓ (via Base) | ✓               |
| neocompute  | `marble.Service`            | Manual  | ✗         | Manual       | ✓               |
| neofeeds    | `marble.Service`            | Manual  | ✗         | Manual       | ✓               |
| neoflow     | `marble.Service`            | Manual  | Manual    | Manual       | ✓               |
| neooracle   | `marble.Service`            | ✗       | ✗         | ✗            | ✓               |
| neorand     | `commonservice.BaseService` | ✓       | ✓         | ✓ (via Base) | ✓               |
| neostore    | `marble.Service`            | ✗       | ✗         | ✗            | Custom          |
| neovault    | `marble.Service`            | Manual  | Manual    | Manual       | ✓               |

### Key Issues Identified

#### 1. Inconsistent Base Service Usage

- Only `neoaccounts` and `neorand` use `commonservice.BaseService`
- Other services directly embed `marble.Service`, duplicating lifecycle code

#### 2. Duplicated stopCh Management

```go
// Repeated in: neofeeds, neovault, neoflow, neocompute
stopCh chan struct{}

func (s *Service) Stop() error {
    close(s.stopCh)  // Risk: double-close panic without sync.Once
    return s.Service.Stop()
}
```

#### 3. Duplicated Worker Loop Pattern

```go
// Repeated in: neoflow, neofeeds, neovault, neorand
go s.runXXXLoop(ctx)

func (s *Service) runXXXLoop(ctx context.Context) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-s.stopCh: return
        case <-ticker.C: s.doWork(ctx)
        }
    }
}
```

#### 4. Inconsistent /info Endpoint

- Each service implements `handleInfo` differently
- No standard response structure
- neostore uses custom `marbleHealth` instead of `marble.HealthHandler`

#### 5. Duplicated Hydration Pattern

```go
// Repeated in: neoaccounts, neorand, neovault, neoflow
func (s *Service) hydrate(ctx context.Context) error {
    // Load state from repository
}
```

## Proposed Architecture

### Directory Structure

```
services/common/
├── service/
│   ├── base.go          # Enhanced BaseService
│   ├── interfaces.go    # Standard interfaces
│   ├── workers.go       # Worker management utilities
│   ├── routes.go        # Standard route registration
│   └── stats.go         # Statistics collection interface
```

### Standard Interfaces

```go
// services/common/service/interfaces.go

// MarbleService is the interface all marble services must implement.
type MarbleService interface {
    ID() string
    Name() string
    Version() string
    Start(ctx context.Context) error
    Stop() error
    Router() *mux.Router
}

// StatisticsProvider provides runtime statistics for /info endpoint.
type StatisticsProvider interface {
    Statistics() map[string]any
}

// Hydratable services can reload state from persistence on startup.
type Hydratable interface {
    Hydrate(ctx context.Context) error
}

// ChainIntegrated services interact with blockchain.
type ChainIntegrated interface {
    ChainClient() *chain.Client
    TEEFulfiller() *chain.TEEFulfiller
}
```

### Enhanced BaseService

```go
// services/common/service/base.go

type BaseConfig struct {
    ID      string
    Name    string
    Version string
    Marble  *marble.Marble
    DB      database.RepositoryInterface

    // RequiredSecrets defines secrets that must be present for the service to be healthy.
    RequiredSecrets []string
}

type BaseService struct {
    *marble.Service

    // Lifecycle management
    stopCh   chan struct{}
    stopOnce sync.Once

    // Extensibility hooks
    hydrate func(context.Context) error
    statsFn   func() map[string]any

    // Worker management
    workers []func(context.Context)

    // Health tracking
    requiredSecrets []string
}

// NewBase creates a BaseService with proper initialization.
func NewBase(cfg *BaseConfig) *BaseService

// WithHydrate sets the hydration hook (called during Start).
func (b *BaseService) WithHydrate(fn func(context.Context) error) *BaseService

// WithStats sets the statistics provider for /info endpoint.
func (b *BaseService) WithStats(fn func() map[string]any) *BaseService

// AddWorker adds a background worker goroutine.
func (b *BaseService) AddWorker(fn func(context.Context)) *BaseService

// AddTickerWorker adds a periodic worker with optional metadata.
func (b *BaseService) AddTickerWorker(interval time.Duration, fn func(ctx context.Context) error, opts ...TickerWorkerOption) *BaseService

// StopChan returns the stop channel for custom workers.
func (b *BaseService) StopChan() <-chan struct{}

// RegisterStandardRoutes registers /health and /info endpoints.
func (b *BaseService) RegisterStandardRoutes()

// Start initializes service, runs hydration, starts workers.
func (b *BaseService) Start(ctx context.Context) error

// Stop signals workers and stops service (idempotent via sync.Once).
func (b *BaseService) Stop() error
```

### Standard Routes

```go
// services/common/service/routes.go

type ServiceInfoResponse struct {
    Status     string         `json:"status"`
    Service    string         `json:"service"`
    Version    string         `json:"version"`
    Enclave    bool           `json:"enclave"`
    Timestamp  string         `json:"timestamp"`
    Statistics map[string]any `json:"statistics,omitempty"`
}

// InfoHandler returns a standardized /info handler.
func InfoHandler(s *BaseService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        resp := ServiceInfoResponse{
            Status:    "active",
            Service:   s.Name(),
            Version:   s.Version(),
            Enclave:   s.Marble().IsEnclave(),
            Timestamp: time.Now().Format(time.RFC3339),
        }
        if s.statsFn != nil {
            resp.Statistics = s.statsFn()
        }
        httputil.WriteJSON(w, http.StatusOK, resp)
    }
}
```

## Migration Plan

### Phase 1: Enhance Base Infrastructure (Priority: HIGH)

**Files to modify:**

- `services/common/service/base.go` - Enhance with stopOnce, stats, workers

**New files to create:**

- `services/common/service/interfaces.go`
- `services/common/service/routes.go`

**Estimated effort:** 2-3 hours

### Phase 2: Migrate Simple Services (Priority: HIGH)

Start with services that have minimal custom logic:

#### 2.1 neostore (Simplest)

- Remove custom `marbleHealth` function
- Use `BaseService` with `RegisterStandardRoutes()`
- No workers to migrate

#### 2.2 neooracle

- Migrate to `BaseService`
- Remove direct `marble.Service` embedding

#### 2.3 neocompute

- Migrate to `BaseService`
- Use `AddWorker` for cleanup worker
- Remove manual stopCh management

**Estimated effort:** 1-2 hours each

### Phase 3: Migrate Complex Services (Priority: MEDIUM)

#### 3.1 neofeeds

- Migrate to `BaseService`
- Convert `runChainPushLoop` to `AddTickerWorker`
- Use standard stopCh via `StopChan()`

#### 3.2 neoflow

- Migrate to `BaseService`
- Convert scheduler workers to `AddWorker`
- Simplify `hydrateSchedulerCache` via `WithHydrate`

#### 3.3 neovault

- Migrate to `BaseService`
- Convert mixing/delivery workers to `AddWorker`
- Use `WithHydrate` for `resumeRequests`

**Estimated effort:** 2-3 hours each

### Phase 4: Standardize Existing Services (Priority: LOW)

#### 4.1 neoaccounts

- Already uses `BaseService`
- Add `WithStats` for pool statistics
- Ensure consistent with new interfaces

#### 4.2 neorand

- Already uses `BaseService`
- Add `WithStats` for VRF statistics
- Ensure consistent with new interfaces

**Estimated effort:** 30 minutes each

## Code Reduction Summary

| Pattern                 | Current Lines          | After Refactoring  | Lines Saved    |
| ----------------------- | ---------------------- | ------------------ | -------------- |
| stopCh management       | ~30 lines × 5 services | 0 (in BaseService) | ~150 lines     |
| Worker loop boilerplate | ~15 lines × 8 workers  | ~5 lines × 8       | ~80 lines      |
| handleInfo duplication  | ~20 lines × 8 services | ~5 lines × 8       | ~120 lines     |
| Custom health handlers  | ~10 lines × 1 service  | 0                  | ~10 lines      |
| **Total**               |                        |                    | **~360 lines** |

## Testing Strategy

### Unit Tests

- Test `BaseService` lifecycle (Start/Stop idempotency)
- Test `sync.Once` stop behavior (no panic on double-stop)
- Test worker registration and shutdown

### Integration Tests

- Verify `/health` endpoint consistency across all services
- Verify `/info` endpoint response structure
- Test service startup order with hydration

### Migration Validation

- Before/after comparison of API responses
- Performance benchmarks for startup time
- Memory usage comparison

## Risk Assessment

| Risk                                | Mitigation                               |
| ----------------------------------- | ---------------------------------------- |
| Breaking existing API contracts     | Keep response fields, only add structure |
| Double-stop panics during migration | sync.Once guarantees safety              |
| Worker shutdown order issues        | WorkerGroup handles graceful shutdown    |
| Test coverage gaps                  | Add unit tests before migration          |

## Success Criteria

1. All 8 services use `BaseService` or implement `MarbleService` interface
2. No duplicated stopCh management code
3. Consistent `/health` and `/info` response structure
4. All existing tests pass
5. New unit tests for BaseService at 80%+ coverage

## Timeline

| Phase     | Duration        | Dependencies |
| --------- | --------------- | ------------ |
| Phase 1   | 2-3 hours       | None         |
| Phase 2   | 3-6 hours       | Phase 1      |
| Phase 3   | 6-9 hours       | Phase 2      |
| Phase 4   | 1 hour          | Phase 3      |
| Testing   | 2-3 hours       | All phases   |
| **Total** | **14-22 hours** |              |

## Appendix: File-by-File Changes

### services/common/service/base.go

- Add `stopOnce sync.Once` field
- Add `statsFn func() map[string]any` field
- Add `WithStats()` method
- Add `AddTickerWorker()` method
- Update `Stop()` to use `sync.Once`

### services/common/service/interfaces.go (NEW)

- Define `MarbleService` interface
- Define `StatisticsProvider` interface
- Define `Hydratable` interface
- Define `ChainIntegrated` interface

### services/common/service/routes.go (NEW)

- Define `ServiceInfoResponse` struct
- Implement `InfoHandler()` function
- Implement `RegisterStandardRoutes()` method

### services/neoXXX/marble/service.go (each service)

- Change embedding to `*commonservice.BaseService`
- Remove manual `stopCh` field
- Use `WithStats()` for statistics
- Use `AddWorker()` or `AddTickerWorker()` for background tasks

### services/neoXXX/marble/lifecycle.go (each service)

- Simplify to delegate to `BaseService.Start/Stop`
- Remove duplicated ticker loop implementations

---

## Implementation Progress (Sprint 4-12)

### Completed Work

#### Sprint 4: TxSubmitter Service (TEE) ✅

- `services/txsubmitter/marble/types.go` - Service types and authorization config
- `services/txsubmitter/marble/service.go` - Core service with queue processing
- `services/txsubmitter/marble/handlers.go` - HTTP handlers and routes
- `services/txsubmitter/marble/ratelimit.go` - Dual-bucket token rate limiter
- `services/txsubmitter/supabase/models.go` - ChainTxRecord models
- `services/txsubmitter/supabase/repository.go` - Repository for chain_txs audit
- `services/txsubmitter/client/client.go` - Client SDK for other services
- `services/txsubmitter/README.md` - Documentation
- `internal/chain/rpcpool.go` - RPC pool with health checking and failover

#### Sprint 5-6: GlobalSigner Service (TEE) ✅

- `services/globalsigner/marble/types.go` - Service type definitions
- `services/globalsigner/marble/service.go` - Core with key rotation and signing
- `services/globalsigner/marble/handlers.go` - HTTP handler routes
- `services/globalsigner/supabase/repository.go` - Repository interface and mock
- `services/globalsigner/client/client.go` - Client SDK
- `services/globalsigner/README.md` - Documentation

#### Sprint 7-12: Business Service Migration ✅

- `services/neofeeds/marble/txsubmitter_adapter.go` - TxSubmitter adapter for price push
- `services/neorand/marble/txsubmitter_adapter.go` - TxSubmitter adapter for VRF fulfillment
- `services/neovault/marble/txsubmitter_adapter.go` - Combined TxSubmitter + GlobalSigner adapter
- `services/gasaccounting/marble/types.go` - GAS ledger type definitions
- `services/gasaccounting/marble/service.go` - Core ledger and reservation service
- `services/gasaccounting/marble/handlers.go` - HTTP handlers
- `services/gasaccounting/supabase/repository.go` - Repository interface and mock
- `services/gasaccounting/client/client.go` - Client SDK
- `services/gasaccounting/README.md` - Documentation
- `services/common/service/gasaccounting.go` - Common GasAccounting adapter

### Architecture Changes

#### Centralized Chain Write Authority

All services now submit transactions through TxSubmitter instead of direct TEEFulfiller calls:

```
Before: Service → TEEFulfiller → Chain
After:  Service → TxSubmitter → Chain
```

#### Centralized Signing Authority

Services use GlobalSigner for cryptographic operations:

```
Before: Service → Local masterKey signing
After:  Service → GlobalSigner → Domain-separated signing
```

#### GAS Accounting Integration

Services can now track GAS usage through GasAccounting:

```
Service → GasAccounting.Reserve() → Operation → GasAccounting.Release()
```

### New Service Dependencies

```
┌─────────────────────────────────────────────────────────────────┐
│                     Business Services                            │
│  NeoFeeds  NeoRand  NeoVault  NeoOracle  NeoFlow  NeoCompute    │
└─────────────────────────────────────────────────────────────────┘
                    │           │           │
                    ▼           ▼           ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Infrastructure Services                      │
│         TxSubmitter      GlobalSigner      GasAccounting         │
└─────────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Chain Layer                                  │
│              Neo N3 RPC Pool (with failover)                    │
└─────────────────────────────────────────────────────────────────┘
```

#### Sprint 13-14: Hardening & Operations ✅

- `services/common/service/healthcheck.go` - Deep health check framework
  - Component-level health checks with parallel execution
  - HTTP and database health check helpers
  - Aggregated health status (healthy/degraded/unhealthy)

- `services/common/service/probes.go` - Kubernetes probe support
  - Liveness probe (`/healthz`)
  - Readiness probe (`/readyz`)
  - Startup probe (`/startupz`)
  - Startup grace period handling

- `services/common/service/metrics.go` - Service metrics collection
  - Request counters (total/success/failed)
  - Latency histogram buckets
  - Error tracking by type
  - Custom gauges
  - Metrics middleware for automatic tracking

### Remaining Work

1. **Integration Testing**
   - End-to-end tests for TxSubmitter flow
   - GlobalSigner key rotation tests
   - GasAccounting reservation lifecycle tests

2. **Production Deployment**
   - Kubernetes manifests update
   - Monitoring dashboards
   - Alerting rules
