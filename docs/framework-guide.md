# Service Framework Deep Dive

Complete guide to the Service Framework layer (`internal/framework/`), the SDK for building services.

## Overview

The Framework layer provides developer tools and abstractions for building services:

```
internal/framework/
├── base.go              # ServiceBase - thread-safe state management
├── builder.go           # ServiceBuilder - fluent service construction
├── manifest.go          # Manifest - service contract declaration
├── bus.go               # BusClient - inter-service communication
├── bus_impl.go          # Bus implementation
├── errors.go            # Framework error types
├── lifecycle/
│   ├── hooks.go         # Lifecycle hooks (pre/post start/stop)
│   └── graceful.go      # Graceful shutdown utilities
└── testing/
    └── mock_bus.go      # MockBusClient for testing
```

---

## ServiceBase

Thread-safe base class for all services. Embed this in your service struct.

### Basic Usage

```go
package myservice

import (
    "context"
    "github.com/R3E-Network/service_layer/internal/framework"
)

type Service struct {
    framework.ServiceBase  // Embed ServiceBase

    store  Store
    config Config
}

func New(store Store, cfg Config) *Service {
    svc := &Service{
        store:  store,
        config: cfg,
    }
    svc.SetName("myservice")      // Set service name
    svc.SetDomain("custom")       // Set domain
    return svc
}

func (s *Service) Name() string   { return s.ServiceBase.Name() }
func (s *Service) Domain() string { return s.ServiceBase.Domain() }
```

### State Management

ServiceBase provides atomic state transitions:

```go
// Available states
const (
    StateUnknown      State = ""
    StateInitializing State = "initializing"
    StateReady        State = "ready"
    StateStopped      State = "stopped"
    StateFailed       State = "failed"
)

// Set state directly
svc.SetState(framework.StateInitializing)

// Atomic compare-and-swap
svc.CompareAndSwapState(framework.StateInitializing, framework.StateReady)

// Convenience methods
svc.MarkStarted()   // Sets ready + records start time
svc.MarkStopped()   // Sets stopped + records stop time
svc.MarkFailed(err) // Sets failed + stores error

// Query state
svc.State()         // Current state
svc.IsReady()       // true if StateReady
svc.IsStopped()     // true if StateStopped or StateFailed
svc.LastError()     // Most recent error
```

### Lifecycle Timing

```go
// Timing information
svc.StartedAt()     // When service started
svc.StoppedAt()     // When service stopped
svc.Uptime()        // Duration since start

// Ready check (implements engine.ReadyChecker)
func (s *Service) Ready(ctx context.Context) error {
    return s.ServiceBase.Ready(ctx)  // Returns error if not ready
}
```

### Metadata Storage

```go
// Store arbitrary metadata
svc.SetMetadata("version", "1.0.0")
svc.SetMetadata("region", "us-east-1")

// Retrieve metadata
version, ok := svc.GetMetadata("version")

// Get all metadata
all := svc.AllMetadata()
```

### Complete Example

```go
package myservice

import (
    "context"
    "time"

    "github.com/R3E-Network/service_layer/internal/framework"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

type Service struct {
    framework.ServiceBase

    store  Store
    log    *logger.Logger
    cancel context.CancelFunc
}

func New(store Store, log *logger.Logger) *Service {
    svc := &Service{
        store: store,
        log:   log,
    }
    svc.SetName("myservice")
    svc.SetDomain("custom")
    svc.SetMetadata("version", "1.0.0")
    return svc
}

func (s *Service) Start(ctx context.Context) error {
    s.SetState(framework.StateInitializing)

    // Initialize resources
    if err := s.store.Connect(ctx); err != nil {
        s.MarkFailed(err)
        return err
    }

    // Start background workers
    ctx, s.cancel = context.WithCancel(ctx)
    go s.backgroundWorker(ctx)

    s.MarkStarted()
    s.log.Info("service started")
    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    s.log.Info("stopping service...")

    // Signal background workers to stop
    if s.cancel != nil {
        s.cancel()
    }

    // Close resources
    if err := s.store.Close(ctx); err != nil {
        s.log.WithError(err).Warn("error closing store")
    }

    s.MarkStopped()
    s.log.WithField("uptime", s.Uptime()).Info("service stopped")
    return nil
}

func (s *Service) Ready(ctx context.Context) error {
    if err := s.ServiceBase.Ready(ctx); err != nil {
        return err
    }
    // Additional readiness checks
    return s.store.Ping(ctx)
}
```

---

## ServiceBuilder

Fluent API for constructing services with minimal boilerplate.

### Basic Usage

```go
import (
    "context"
    "github.com/R3E-Network/service_layer/internal/framework"
    "github.com/R3E-Network/service_layer/internal/engine"
)

svc, err := framework.NewService("my-service", "domain").
    WithDescription("My awesome service").
    WithVersion("1.0.0").
    WithLayer("service").
    WithCapabilities("read", "write").
    DependsOn("store-postgres", "svc-accounts").
    RequiresAPI(engine.APISurfaceStore, engine.APISurfaceEvent).
    OnStart(func(ctx context.Context) error {
        // Initialize resources
        return nil
    }).
    OnStop(func(ctx context.Context) error {
        // Cleanup resources
        return nil
    }).
    Build()

if err != nil {
    log.Fatal(err)
}
```

### All Builder Options

```go
framework.NewService("name", "domain").
    // Metadata
    WithDescription("Service description").
    WithVersion("1.2.3").
    WithLayer("service").              // service|runner|infra

    // Dependencies
    DependsOn("dep1", "dep2").         // Service dependencies
    RequiresAPI("store", "compute").   // Required API surfaces

    // Capabilities
    WithCapabilities("cap1", "cap2").  // Advertised capabilities

    // Resource quotas
    WithQuotas(map[string]string{
        "rpc": "my-quota",
        "gas": "1000",
    }).
    WithQuota("key", "value").         // Single quota

    // Tags/metadata
    WithTags(map[string]string{
        "env":  "prod",
        "tier": "premium",
    }).
    WithTag("key", "value").           // Single tag

    // Enable/disable
    Enabled(true).

    // Lifecycle hooks
    OnPreStart(func(ctx context.Context) error { return nil }).
    OnStart(func(ctx context.Context) error { return nil }).
    OnStop(func(ctx context.Context) error { return nil }).
    OnPostStop(func(ctx context.Context) error { return nil }).

    // Custom readiness check
    WithReadyCheck(func(ctx context.Context) error {
        return pingDatabase()
    }).

    // Validation
    WithValidator(myValidator).
    WithValidatorFunc(func(m *framework.Manifest) error {
        if m.Name == "" {
            return errors.New("name required")
        }
        return nil
    }).

    // Merge another manifest
    MergeManifest(baseManifest).

    Build()
```

### Named Lifecycle Hooks

For debugging and observability:

```go
svc, _ := framework.NewService("my-service", "domain").
    OnPreStartNamed("validate-config", func(ctx context.Context) error {
        return validateConfig()
    }).
    OnPreStartNamed("connect-database", func(ctx context.Context) error {
        return db.Connect(ctx)
    }).
    OnStartNamed("start-workers", func(ctx context.Context) error {
        return startBackgroundWorkers(ctx)
    }).
    OnPreStopNamed("drain-connections", func(ctx context.Context) error {
        return drainConnections(ctx)
    }).
    OnPostStopNamed("close-database", func(ctx context.Context) error {
        return db.Close(ctx)
    }).
    Build()
```

---

## Manifest

Declares service contracts with the engine.

### Structure

```go
type Manifest struct {
    Name         string              // Service name (required)
    Domain       string              // Domain/category (required)
    Description  string              // Human-readable description
    Version      string              // Semantic version
    Layer        string              // service|runner|infra
    Capabilities []string            // Advertised capabilities
    DependsOn    []string            // Service dependencies
    RequiresAPIs []engine.APISurface // Required API surfaces
    Quotas       map[string]string   // Resource quotas
    Tags         map[string]string   // Metadata tags
    Enabled      bool                // Enable/disable flag
}
```

### Creating Manifests

```go
// Direct construction
m := &framework.Manifest{
    Name:         "my-service",
    Domain:       "custom",
    Description:  "My service description",
    Version:      "1.0.0",
    Layer:        "service",
    Capabilities: []string{"read", "write"},
    DependsOn:    []string{"store-postgres"},
    RequiresAPIs: []engine.APISurface{
        engine.APISurfaceStore,
        engine.APISurfaceEvent,
    },
    Quotas: map[string]string{"rpc": "my-quota"},
    Tags:   map[string]string{"env": "prod"},
    Enabled: true,
}

// Normalize (clean up whitespace, dedupe)
m.Normalize()

// Validate
if err := m.Validate(); err != nil {
    return err
}
```

### Query Methods

```go
m.HasCapability("read")           // true
m.RequiresAPI("store")            // true
m.DependsOnService("store-postgres") // true
m.GetQuota("rpc")                 // "my-quota", true
m.GetTag("env")                   // "prod", true
m.IsEnabled()                     // true
```

### Mutation Methods

```go
m.SetEnabled(false)
m.SetQuota("gas", "500")
m.SetTag("tier", "premium")
m.Merge(otherManifest)            // Combine manifests
clone := m.Clone()                // Deep copy
```

### Engine Integration

```go
// Convert to engine Descriptor
desc := m.ToDescriptor()

// Convert from engine Descriptor
m2 := framework.ManifestFromDescriptor(desc)
```

### Service Implementation

```go
func (s *Service) Manifest() *framework.Manifest {
    return &framework.Manifest{
        Name:         s.Name(),
        Domain:       s.Domain(),
        Description:  "My service",
        Layer:        "service",
        DependsOn:    []string{"store-postgres", "svc-accounts"},
        RequiresAPIs: []engine.APISurface{
            engine.APISurfaceStore,
            engine.APISurfaceEvent,
        },
        Capabilities: []string{"my-capability"},
        Quotas:       map[string]string{"rpc": "my-quota"},
    }
}
```

---

## Lifecycle Hooks

Located in `internal/framework/lifecycle/`.

### Hook Types

```go
type HookFunc func(ctx context.Context) error

type Hooks struct {
    PreStart  []HookFunc  // Before Start() is called
    PostStart []HookFunc  // After Start() succeeds
    PreStop   []HookFunc  // Before Stop() is called
    PostStop  []HookFunc  // After Stop() completes (LIFO order)
}
```

### Using Hooks

```go
import "github.com/R3E-Network/service_layer/internal/framework/lifecycle"

hooks := &lifecycle.Hooks{}

// Add hooks
hooks.PreStart = append(hooks.PreStart, func(ctx context.Context) error {
    return validateConfig()
})

hooks.PostStart = append(hooks.PostStart, func(ctx context.Context) error {
    return registerWithDiscovery()
})

hooks.PreStop = append(hooks.PreStop, func(ctx context.Context) error {
    return drainConnections()
})

hooks.PostStop = append(hooks.PostStop, func(ctx context.Context) error {
    return closeDatabase()
})

// Run hooks
if err := hooks.RunPreStart(ctx); err != nil {
    return err
}

// Start service...

if err := hooks.RunPostStart(ctx); err != nil {
    return err
}
```

---

## Graceful Shutdown

Located in `internal/framework/lifecycle/graceful.go`.

### Basic Usage

```go
import "github.com/R3E-Network/service_layer/internal/framework/lifecycle"

gs := lifecycle.NewGracefulShutdown()

// Track in-flight operations
func (s *Service) HandleRequest(ctx context.Context) error {
    if !gs.Add() {
        return ErrShuttingDown  // Reject if shutting down
    }
    defer gs.Done()

    // Process request...
    return nil
}

// Shutdown
func (s *Service) Stop(ctx context.Context) error {
    gs.Shutdown()                           // Signal shutdown
    gs.WaitWithTimeout(5 * time.Second)     // Wait for in-flight
    return nil
}
```

### OperationGuard Pattern

RAII-style guard for cleaner code:

```go
func (s *Service) HandleRequest(ctx context.Context) error {
    guard := lifecycle.NewOperationGuard(s.graceful)
    if guard == nil {
        return ErrShuttingDown
    }
    defer guard.Close()

    // Process request - automatically tracked
    return nil
}
```

### Advanced Usage

```go
gs := lifecycle.NewGracefulShutdown()

// Check state
gs.IsShuttingDown()           // true if shutdown signaled
gs.InFlightCount()            // Number of active operations

// Wait options
<-gs.ShutdownCh()             // Channel that closes on shutdown
gs.Wait(ctx)                  // Wait with context
gs.WaitWithTimeout(timeout)   // Wait with timeout
gs.ShutdownAndWait(timeout)   // Signal + wait in one call
```

---

## BusClient

Interface for inter-service communication via the engine bus.

### Interface

```go
type BusClient interface {
    PublishEvent(ctx context.Context, event string, payload any) error
    PushData(ctx context.Context, topic string, payload any) error
    InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error)
}
```

### Using in Services

```go
type Service struct {
    framework.ServiceBase
    bus framework.BusClient
}

func (s *Service) NotifyCompletion(ctx context.Context, result Result) error {
    return s.bus.PublishEvent(ctx, "myservice.completed", map[string]any{
        "id":     result.ID,
        "status": "success",
    })
}

func (s *Service) BroadcastData(ctx context.Context, data Data) error {
    return s.bus.PushData(ctx, "myservice/updates", data)
}

func (s *Service) ExecuteCompute(ctx context.Context, payload any) ([]ComputeResult, error) {
    return s.bus.InvokeCompute(ctx, payload)
}
```

### ComputeResult Helpers

```go
// Single result
r := framework.NewComputeResult("module", data)
r := framework.NewComputeResultError("module", err)

r.Success()                   // true if no error
r.Failed()                    // true if has error
r.Error()                     // error message string

// Type conversion
var s string
err := r.ResultAs(&s)         // Safe conversion
r.MustResultAs(&s)            // Panics on error

// Batch results
rs := framework.ComputeResults{r1, r2, r3}
rs.AllSuccessful()            // true if all succeeded
rs.AnyFailed()                // true if any failed
rs.Successful()               // Filter to successful
rs.Failed()                   // Filter to failed
rs.ByModule("name")           // Find by module name
rs.Modules()                  // List all module names
rs.FirstError()               // First error found
rs.Errors()                   // All errors
rs.Count()                    // Total count
rs.SuccessCount()             // Successful count
rs.FailedCount()              // Failed count
```

---

## Testing

### MockBusClient

Located in `internal/framework/testing/mock_bus.go`.

```go
import ftesting "github.com/R3E-Network/service_layer/internal/framework/testing"

func TestService_PublishesEvent(t *testing.T) {
    mock := ftesting.NewMockBusClient()

    svc := NewService(store, mock)

    // Do something that publishes
    svc.DoWork(context.Background())

    // Assert event was published
    mock.AssertEventPublished(t, "myservice.completed")
}

func TestService_PushesData(t *testing.T) {
    mock := ftesting.NewMockBusClient()

    svc := NewService(store, mock)
    svc.BroadcastUpdate(context.Background(), data)

    mock.AssertDataPushed(t, "myservice/updates")
}

func TestService_InvokesCompute(t *testing.T) {
    mock := ftesting.NewMockBusClient()

    // Setup expected results
    mock.SetInvokeResults([]framework.ComputeResult{
        framework.NewComputeResult("svc-functions", "result"),
    })

    svc := NewService(store, mock)
    results, err := svc.ExecuteCompute(context.Background(), payload)

    require.NoError(t, err)
    require.Len(t, results, 1)
}

func TestService_NoOperations(t *testing.T) {
    mock := ftesting.NewMockBusClient()

    svc := NewService(store, mock)
    // Do something that should NOT publish

    mock.AssertNoOperations(t)
}
```

### MockBusClient Methods

```go
mock := ftesting.NewMockBusClient()

// Inspect published events
mock.PublishedEvents       // []PublishedEvent

// Inspect pushed data
mock.PushedData            // []PushedData

// Configure invoke results
mock.SetInvokeResults(results)
mock.SetInvokeError(err)

// Assertions
mock.AssertEventPublished(t, "event.name")
mock.AssertDataPushed(t, "topic")
mock.AssertNoOperations(t)

// Reset
mock.Reset()
```

---

## Error Types

Located in `internal/framework/errors.go`.

### Sentinel Errors

```go
var (
    ErrServiceNotReady    = errors.New("service not ready")
    ErrServiceStartFailed = errors.New("service start failed")
    ErrServiceStopFailed  = errors.New("service stop failed")
    ErrMissingDependency  = errors.New("missing dependency")
    ErrDependencyCycle    = errors.New("dependency cycle detected")
    ErrTimeout            = errors.New("operation timeout")
)
```

### Structured Errors

```go
// Service error
err := &framework.ServiceError{
    Service: "myservice",
    Op:      "start",
    Err:     originalError,
}

// Config error
err := &framework.ConfigError{
    Field:   "database_url",
    Message: "required field missing",
}

// Dependency error
err := &framework.DependencyError{
    Service:    "myservice",
    Dependency: "store-postgres",
    Err:        originalError,
}

// Hook error
err := &framework.HookError{
    Hook:  "PreStart",
    Name:  "validate-config",
    Err:   originalError,
}
```

### Error Checking

```go
if framework.IsServiceNotReady(err) {
    // Handle not ready
}

if framework.IsTimeout(err) {
    // Handle timeout
}

if framework.IsCanceled(err) {
    // Handle cancellation
}
```

---

## Complete Service Example

Putting it all together:

```go
package myservice

import (
    "context"
    "fmt"
    "time"

    "github.com/R3E-Network/service_layer/internal/engine"
    "github.com/R3E-Network/service_layer/internal/framework"
    "github.com/R3E-Network/service_layer/internal/framework/lifecycle"
    core "github.com/R3E-Network/service_layer/internal/services/core"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

type Service struct {
    framework.ServiceBase

    store    Store
    bus      framework.BusClient
    log      *logger.Logger
    graceful *lifecycle.GracefulShutdown
    cancel   context.CancelFunc
}

func New(store Store, bus framework.BusClient, log *logger.Logger) *Service {
    if log == nil {
        log = logger.NewDefault("myservice")
    }
    svc := &Service{
        store:    store,
        bus:      bus,
        log:      log,
        graceful: lifecycle.NewGracefulShutdown(),
    }
    svc.SetName("myservice")
    svc.SetDomain("custom")
    return svc
}

// Engine interface
func (s *Service) Name() string   { return "myservice" }
func (s *Service) Domain() string { return "custom" }

func (s *Service) Manifest() *framework.Manifest {
    return &framework.Manifest{
        Name:         s.Name(),
        Domain:       s.Domain(),
        Description:  "Custom service implementation",
        Version:      "1.0.0",
        Layer:        "service",
        DependsOn:    []string{"store-postgres", "svc-accounts"},
        RequiresAPIs: []engine.APISurface{
            engine.APISurfaceStore,
            engine.APISurfaceEvent,
        },
        Capabilities: []string{"custom-capability"},
        Quotas:       map[string]string{"rpc": "myservice-quota"},
    }
}

func (s *Service) Descriptor() core.Descriptor {
    m := s.Manifest()
    return core.Descriptor{
        Name:         m.Name,
        Domain:       m.Domain,
        Layer:        core.LayerService,
        Capabilities: m.Capabilities,
        DependsOn:    m.DependsOn,
        RequiresAPIs: toStringSlice(m.RequiresAPIs),
    }
}

func (s *Service) Start(ctx context.Context) error {
    s.log.Info("starting service")

    // Validate dependencies
    if s.store == nil {
        return &framework.DependencyError{
            Service:    s.Name(),
            Dependency: "store",
            Err:        fmt.Errorf("store is nil"),
        }
    }

    // Test store connection
    if err := s.store.Ping(ctx); err != nil {
        s.MarkFailed(err)
        return fmt.Errorf("store ping: %w", err)
    }

    // Start background worker
    ctx, s.cancel = context.WithCancel(ctx)
    go s.worker(ctx)

    s.MarkStarted()
    s.log.Info("service started")
    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    s.log.Info("stopping service")

    // Signal shutdown
    if s.cancel != nil {
        s.cancel()
    }

    // Wait for in-flight operations
    s.graceful.ShutdownAndWait(5 * time.Second)

    s.MarkStopped()
    s.log.WithField("uptime", s.Uptime()).Info("service stopped")
    return nil
}

func (s *Service) Ready(ctx context.Context) error {
    if err := s.ServiceBase.Ready(ctx); err != nil {
        return err
    }
    return s.store.Ping(ctx)
}

// Business logic
func (s *Service) DoWork(ctx context.Context, input Input) (*Output, error) {
    // Track operation for graceful shutdown
    guard := lifecycle.NewOperationGuard(s.graceful)
    if guard == nil {
        return nil, framework.ErrServiceNotReady
    }
    defer guard.Close()

    // Validate readiness
    if err := s.Ready(ctx); err != nil {
        return nil, err
    }

    // Do work...
    result, err := s.store.Process(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("process: %w", err)
    }

    // Publish completion event
    if s.bus != nil {
        _ = s.bus.PublishEvent(ctx, "myservice.completed", map[string]any{
            "id":     result.ID,
            "status": "success",
        })
    }

    return result, nil
}

// Background worker
func (s *Service) worker(ctx context.Context) {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.log.Debug("background tick")
        }
    }
}

// Health check
func (s *Service) HealthCheck(ctx context.Context) core.HealthCheck {
    hc := core.NewHealthCheck(s.Name())

    storeCheck := core.CheckStore(ctx, "myservice-store", func(ctx context.Context) error {
        return s.store.Ping(ctx)
    })
    hc = hc.WithComponent(storeCheck)

    return hc
}

func toStringSlice(apis []engine.APISurface) []string {
    out := make([]string, len(apis))
    for i, a := range apis {
        out[i] = string(a)
    }
    return out
}
```

---

## Related Documentation

- [Service Catalog](service-catalog.md) - All 17 services
- [Developer Guide](developer-guide.md) - Building services
- [Engine Deep Dive](engine-guide.md) - Engine internals
- [Architecture Layers](architecture-layers.md) - Overall architecture
