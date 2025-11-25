# Service Engine (Android-Style)

The Service Engine acts like a lightweight OS. It owns lifecycle, readiness, and
shared system APIs, while services behave like applications that plug into those
surfaces. This keeps every module consistent and lets operators reason about the
platform the same way they would on a phone OS: the engine is the system, and
services are apps built against the system APIs.

## Responsibilities
- **Engine (the OS)**
  - Manages lifecycle (`Start/Stop/Ready`) with deterministic ordering and
    rollback on failure.
  - Exposes common system APIs: store, accounts, compute, data bus, event bus.
  - Enforces bus permissions and dependency wiring across modules.
  - Surfaces health, readiness, timings, and API participation via
    `/system/status` (`modules`, `modules_summary`, `modules_api_summary`).
  - Provides fan-out endpoints (`/system/events|data|compute`) as stable system
    entry points used by the dashboard, CLI, and SDKs.
  - Lives in `internal/engine` with runtime wiring in `internal/engine/runtime`.
- **Services (the apps)**
  - Implement the standard interfaces instead of bespoke lifecycles:
    `ServiceModule` + `Store|Account|Compute|Data|Event` engines.
  - Optionally advertise extra surfaces by implementing `engine.APIDescriber`.
  - Respect engine-provided permissions/deps rather than hardcoding topology.
  - Stay portable: no direct coupling between services; communicate over the
    engine buses.
  - Live together under `internal/services` (shared helpers in `internal/services/core`).

## Standard API surfaces
These are the OS-level APIs a module can expose. The engine auto-populates them
based on implemented interfaces and bus permissions; `/system/status` lists them
per module (`apis`) and grouped in `modules_api_summary`.

| Surface       | How to expose                                 | Summary                                             |
|---------------|-----------------------------------------------|-----------------------------------------------------|
| `lifecycle`   | Implement `ServiceModule`                     | Engine-managed start/stop                           |
| `readiness`   | Implement `Ready(ctx)`                        | Probe-able readiness hook                           |
| `store`       | Implement `StoreEngine`                       | Persistent store ping                               |
| `account`     | Implement `AccountEngine`                     | Account registry (create/list)                      |
| `compute`     | Implement `ComputeEngine` + allow compute bus | Function/job execution via compute bus              |
| `data`        | Implement `DataEngine` + allow data bus       | Data push fan-out via `/system/data`                |
| `event`       | Implement `EventEngine` + allow event bus     | Publish/subscribe fan-out via `/system/events`      |
| `crypto`      | Implement `CryptoEngine` or advertise via APIs| Engine crypto helpers (ZKP/FHE/MPC)                 |

## Adding a service (as an app)
1. Implement `Name`/`Domain` and the lifecycle hooks (`Start/Stop/Ready`).
2. Implement whichever engine interfaces match your capability
   (`AccountEngine`, `ComputeEngine`, `DataEngine`, `EventEngine`, `StoreEngine`).
3. If you need custom surfaces (e.g., telemetry/admin), implement
   `engine.APIDescriber` to append `APIDescriptor`s.
4. Register with the engine via the runtime adapter; avoid bespoke wiring so
   ordering, permissions, and status are handled centrally.
5. Verify `/system/status` shows your module under `modules` with the expected
   `interfaces`, `apis`, and that `modules_api_summary` lists it under the right
   surfaces.
6. Advertise a descriptor/manifest with a `layer` hint (`service`, `runner`, `infra`) plus capabilities and required APIs; `/system/descriptors` and `/system/status` use this for operator insight.

## Operator view
- `/system/status` is the single pane for engine health:
  - `modules`: lifecycle/readiness, timings, permissions, `apis`.
  - `modules_summary`: quick view of data/event/compute-capable modules.
  - `modules_api_summary`: modules grouped by system API surface.
  - `modules_waiting_*`, `modules_slow`, timings/uptime for observability.
  - `modules_layers`: modules grouped by `service|runner|infra`.
  - `modules_capabilities`/`modules_quotas`: infra metadata for RPC hubs, service bank limits, and other OS services.
  - `modules_requires_apis`: surfaces each module expects from the OS; `modules_requires_missing` flags any gaps.
- `/system/events|data|compute` are the OS buses; services should prefer these
  instead of custom cross-calls.
- `/system/descriptors` merges service manifests with engine/infra modules (layer=`service|runner|infra`) so operators can see both the apps and the OS services in one place.

## Migration tips
- If a service already implements lifecycle but not the engine interfaces, wrap
  it with the runtime adapter so the engine owns start/stop/ready and surfaces
  standard APIs automatically.
- Use `engine.SetBusPermissions` to restrict surfaces when an adapter stubs an
  interface (e.g., disable event bus for a compute-only module).
- Keep module names stable (`svc-*`, `runner-*`, `store-*`, `core-*`) so status
  payloads remain predictable for operators and dashboards.
- Expose a service manifest (name/domain/depends_on/required APIs/quotas) so the engine can surface capabilities and enforce basic wiring; see `internal/framework/manifest.go` for the contract.
- Required API surfaces from manifests are exposed via `/system/status` (`modules_requires_apis`, `modules_requires_missing`); enable `runtime.require_apis_strict=true` to fail startup when required surfaces are absent.

## Service Framework

The Service Framework (`internal/framework`) provides developer tools for building services:

### ServiceBuilder (Fluent API)
Build services with a fluent API that reduces boilerplate:
```go
svc, err := framework.NewService("my-service", "domain").
    WithDescription("My service description").
    WithVersion("1.0.0").
    WithLayer("service").
    WithCapabilities("capability1", "capability2").
    DependsOn("accounts", "functions").
    RequiresAPI("store", "compute").
    WithQuota("gas", "1000").
    WithTag("env", "prod").
    Enabled(true).
    OnPreStart(func(ctx context.Context) error { /* warmup */ return nil }).
    OnStart(func(ctx context.Context) error { /* main init */ return nil }).
    OnStop(func(ctx context.Context) error { /* cleanup */ return nil }).
    WithReadyCheck(func(ctx context.Context) error { /* custom check */ return nil }).
    Build()
```

Full builder options:
- `WithDescription(desc)` - Human-readable description
- `WithVersion(v)` - Semantic version for compatibility tracking
- `WithLayer(layer)` - service|runner|infra for categorization
- `WithCapabilities(caps...)` - Advertised capabilities
- `DependsOn(deps...)` - Service dependencies
- `RequiresAPI(apis...)` - Required API surfaces
- `WithQuotas(map)` / `WithQuota(k, v)` - Resource quotas
- `WithTags(map)` / `WithTag(k, v)` - Metadata for filtering
- `Enabled(bool)` - Enable/disable service
- `MergeManifest(m)` - Merge another manifest (for overrides)
- `WithValidator(v)` / `WithValidatorFunc(fn)` - Custom validation

### Manifest
Service manifests declare contracts with the engine:
```go
m := &framework.Manifest{
    Name:         "my-service",
    Domain:       "domain",
    Description:  "Service description",
    Version:      "1.0.0",
    Layer:        "service",
    Capabilities: []string{"cap1"},
    DependsOn:    []string{"dep1"},
    RequiresAPIs: []engine.APISurface{"store"},
    Quotas:       map[string]string{"gas": "1000"},
    Tags:         map[string]string{"env": "prod"},
}
m.Normalize() // Clean up whitespace, dedupe

// Query methods
m.HasCapability("cap1")      // true
m.RequiresAPI("store")       // true
m.DependsOnService("dep1")   // true
m.GetQuota("gas")            // "1000", true
m.GetTag("env")              // "prod", true
m.IsEnabled()                // true (default)

// Mutation methods
m.SetEnabled(false)
m.SetQuota("rpc", "500")
m.SetTag("tier", "premium")
m.Merge(otherManifest)       // Combine manifests
clone := m.Clone()           // Deep copy

// Engine integration
desc := m.ToDescriptor()     // Convert to service.Descriptor
m2 := framework.ManifestFromDescriptor(desc) // Reverse
```

### ServiceBase (Thread-Safe State)
Embed `ServiceBase` for thread-safe state tracking:
```go
type MyService struct {
    framework.ServiceBase
    // ... your fields
}

svc := &MyService{}
svc.SetName("my-service")
svc.SetDomain("domain")

// State management
svc.SetState(framework.StateInitializing)
svc.CompareAndSwapState(framework.StateInitializing, framework.StateReady)
svc.MarkStarted()  // Sets ready + records start time
svc.MarkStopped()  // Sets stopped + records stop time
svc.MarkFailed(err) // Sets failed + stores error

// Query state
svc.State()        // Current state
svc.IsReady()      // true if StateReady
svc.IsStopped()    // true if StateStopped or StateFailed
svc.LastError()    // Most recent error
svc.Uptime()       // Duration since start
svc.StartedAt()    // Start timestamp
svc.StoppedAt()    // Stop timestamp

// Metadata
svc.SetMetadata("key", "value")
v, ok := svc.GetMetadata("key")
all := svc.AllMetadata()
```

### Lifecycle Hooks
Register hooks for different lifecycle phases:
- `OnPreStart` / `OnPreStartNamed` - Before service starts (warmup, validation)
- `OnPostStart` / `OnPostStartNamed` - After service starts successfully
- `OnPreStop` / `OnPreStopNamed` - Before service stops (drain connections)
- `OnPostStop` / `OnPostStopNamed` - After service stops (cleanup)

PostStop hooks run in LIFO order for proper resource cleanup.

### Graceful Shutdown
Coordinate in-flight operations during shutdown:
```go
gs := lifecycle.NewGracefulShutdown()

// Track operations
if gs.Add() {
    defer gs.Done()
    // ... do work ...
}

// RAII pattern with OperationGuard
guard := lifecycle.NewOperationGuard(gs)
if guard == nil {
    return ErrShuttingDown
}
defer guard.Close()

// Shutdown coordination
gs.Shutdown()                         // Signal shutdown
<-gs.ShutdownCh()                     // Wait for signal
gs.Wait(ctx)                          // Wait for in-flight to complete
gs.WaitWithTimeout(5 * time.Second)   // Wait with timeout
gs.ShutdownAndWait(5 * time.Second)   // Shutdown + wait
```

### ComputeResult Helpers
Work with compute bus results:
```go
// Single result
r := framework.NewComputeResult("module", data)
r := framework.NewComputeResultError("module", err)

r.Success()   // true if no error
r.Failed()    // true if has error
r.Error()     // error message string

// Type conversion
var s string
err := r.ResultAs(&s)       // Safe conversion
r.MustResultAs(&s)          // Panics on error

// Batch results
rs := framework.ComputeResults{r1, r2, r3}
rs.AllSuccessful()    // true if all succeeded
rs.AnyFailed()        // true if any failed
rs.Successful()       // Filter to successful
rs.Failed()           // Filter to failed
rs.ByModule("name")   // Find by module name
rs.Modules()          // List all module names
rs.FirstError()       // First error found
rs.Errors()           // All errors
rs.Count()            // Total count
rs.SuccessCount()     // Successful count
rs.FailedCount()      // Failed count
```

### MockBusClient (Testing)
Test services without a full Engine using `framework/testing.MockBusClient`:
```go
mock := testing.NewMockBusClient()
mock.SetInvokeResults([]framework.ComputeResult{
    framework.NewComputeResult("module", "result"),
})

// ... test your service ...

mock.AssertEventPublished(t, "my.event")
mock.AssertDataPushed(t, "topic")
mock.AssertNoOperations(t)
```

### Error Types
Comprehensive error types in `framework/errors.go`:
- `ErrServiceNotReady`, `ErrServiceStartFailed`, `ErrServiceStopFailed`
- `ErrMissingDependency`, `ErrDependencyCycle`, `ErrTimeout`
- `ServiceError`, `ConfigError`, `DependencyError`, `HookError` structs
- Helper functions: `IsServiceNotReady()`, `IsTimeout()`, `IsCanceled()`

### Service Registry
Self-registration pattern to eliminate hardcoded service lists:
```go
// In service init()
system.NewServiceBuilder("my-service").
    Domain("domain").
    Priority(10).
    Factory(func(deps system.ServiceDeps) (system.Service, error) {
        return NewMyService(deps), nil
    }).
    MustRegister()
```

## Migration note (legacy paths)
- Legacy service implementations under `internal/app/services` and the old
  engine at `internal/core/engine` have been moved. Use `internal/services`
  (apps) and `internal/engine` (OS) going forward. Shared service helpers live
  in `internal/services/core`; runtime wiring is in `internal/engine/runtime`.
- Infrastructure modules: enable Neo node/indexer (`svc-neo-node`, `svc-neo-indexer`), multi-chain RPC hub (`svc-chain-rpc`), shared data source hub (`svc-data-sources`), contracts module (`svc-contracts`), crypto engine (`svc-crypto`), RocketMQ-backed event bus (`svc-rocketmq`, configure name servers/topic prefix/consumer group/namespace/max_reconsume_times/consume_batch/consume_from), and service-owned GAS controller (`svc-service-bank`) via runtime config so they appear in `/system/status` with readiness and API descriptors.
