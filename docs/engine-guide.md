# Service Engine Deep Dive

Complete guide to the Service Engine (`internal/engine/`), the OS kernel that orchestrates all services.

## Overview

The Engine is the "operating system" for services:

```
internal/engine/
├── engine.go           # Core coordinator
├── interfaces.go       # All interface definitions
├── registry.go         # Module registration and lookup
├── lifecycle.go        # Start/Stop management
├── bus.go              # Event/Data/Compute buses
├── health.go           # Health checks and readiness
├── dependency.go       # Dependency resolution
├── apis.go             # API Surface definitions
├── options.go          # Engine configuration
├── metadata.go         # Module metadata
├── state/              # State machine
├── events/             # Event system
├── metrics/            # Metrics collection
├── recovery/           # Failure recovery
├── bridge/             # External integrations
├── bus/                # Bus extensions (limiter)
├── domains/            # Domain modules (defi, gamefi, nft)
└── runtime/            # Runtime adapters
    ├── application.go      # App runtime
    ├── service_modules.go  # Service wrappers
    ├── infrastructure_modules.go
    ├── config_bridge.go    # Config integration
    └── secrets.go          # Secret management
```

---

## Core Concepts

### Android-Style Model

The Engine behaves like a mobile OS:

| Concept | Engine Equivalent |
|---------|------------------|
| OS Kernel | Engine (`internal/engine/`) |
| Applications | Services (`internal/services/`) |
| System APIs | API Surfaces (store, compute, event, data) |
| App Manifest | Service Manifest |
| Intent System | Event Bus |
| Background Services | Runners |

### API Surfaces

Standard "system APIs" that modules can expose:

```go
const (
    APISurfaceLifecycle APISurface = "lifecycle"  // Start/Stop
    APISurfaceReadiness APISurface = "readiness"  // Ready check
    APISurfaceStore     APISurface = "store"      // Persistence
    APISurfaceAccount   APISurface = "account"    // Account ops
    APISurfaceCompute   APISurface = "compute"    // Execution
    APISurfaceData      APISurface = "data"       // Data push
    APISurfaceEvent     APISurface = "event"      // Pub/sub
    APISurfaceCrypto    APISurface = "crypto"     // Cryptography
)
```

---

## ServiceModule Interface

Every module must implement:

```go
type ServiceModule interface {
    Name() string                      // Unique module name
    Domain() string                    // Domain/category
    Start(ctx context.Context) error   // Lifecycle start
    Stop(ctx context.Context) error    // Lifecycle stop
}
```

### Specialized Interfaces

Modules implement additional interfaces based on capabilities:

```go
// Account operations
type AccountEngine interface {
    ServiceModule
    CreateAccount(ctx context.Context, owner string, meta map[string]string) (string, error)
    ListAccounts(ctx context.Context) ([]any, error)
}

// Persistence
type StoreEngine interface {
    ServiceModule
    Ping(ctx context.Context) error
}

// Function execution
type ComputeEngine interface {
    ServiceModule
    Invoke(ctx context.Context, payload any) (any, error)
}

// Data push
type DataEngine interface {
    ServiceModule
    Push(ctx context.Context, topic string, payload any) error
}

// Event pub/sub
type EventEngine interface {
    ServiceModule
    Publish(ctx context.Context, event string, payload any) error
    Subscribe(ctx context.Context, event string, handler func(context.Context, any) error) error
}

// Readiness
type ReadyChecker interface {
    Ready(ctx context.Context) error
}

// API description
type APIDescriber interface {
    APIs() []APIDescriptor
}
```

---

## Creating the Engine

### Basic Setup

```go
import "github.com/R3E-Network/service_layer/internal/engine"

// Create engine with options
eng := engine.New(
    engine.WithLogger(log),
    engine.WithOrder("store-postgres", "svc-accounts", "svc-functions"),
)

// Register modules
eng.Register(postgresStore)
eng.Register(accountsService)
eng.Register(functionsService)

// Start all modules
if err := eng.Start(ctx); err != nil {
    log.Fatalf("engine start: %v", err)
}
defer eng.Stop(context.Background())
```

### Engine Options

```go
engine.New(
    // Logger
    engine.WithLogger(log),

    // Startup ordering (modules start in this order)
    engine.WithOrder("store-postgres", "core-application", "svc-*"),

    // Module dependencies
    engine.WithDependencies(map[string][]string{
        "svc-functions": {"store-postgres", "svc-accounts"},
        "svc-oracle":    {"store-postgres", "svc-accounts"},
    }),

    // Bus permissions per module
    engine.WithBusPermissions(map[string]engine.BusPermissions{
        "svc-functions": {Compute: true, Event: true, Data: false},
        "svc-datastreams": {Compute: false, Event: false, Data: true},
    }),

    // Slow module threshold (default 1s)
    engine.WithSlowThreshold(2 * time.Second),
)
```

---

## Registry

Module registration and lookup.

### Registration

```go
// Register module
if err := eng.Register(myModule); err != nil {
    // Handle duplicate name error
}

// Unregister (rarely needed)
eng.Unregister("module-name")
```

### Lookup

```go
// By name
mod := eng.Lookup("svc-functions")

// List all module names
names := eng.Modules()

// By domain
mods := eng.ModulesByDomain("functions")

// Typed lookups
stores := eng.StoreEngines()       // []StoreEngine
accounts := eng.AccountEngines()   // []AccountEngine
computes := eng.ComputeEngines()   // []ComputeEngine
datas := eng.DataEngines()         // []DataEngine
events := eng.EventEngines()       // []EventEngine
```

---

## Lifecycle Management

### Startup

```go
// Start all modules in order
if err := eng.Start(ctx); err != nil {
    // If any module fails, already-started modules are stopped (rollback)
    log.Fatalf("start failed: %v", err)
}
```

Startup process:
1. Resolve dependency order
2. Start modules in order
3. If a module fails, stop already-started modules in reverse order
4. Record start times per module

### Shutdown

```go
// Stop all modules in reverse order
if err := eng.Stop(ctx); err != nil {
    log.Errorf("stop errors: %v", err)
}
```

### Module Status

```go
// Get status for all modules
status := eng.ModulesStatus()

// Status structure
type ModuleStatus struct {
    Name        string
    Domain      string
    Status      string     // registered|starting|started|stopped|failed
    ReadyStatus string     // ready|not-ready|unknown
    ReadyError  string
    StartedAt   *time.Time
    StoppedAt   *time.Time
    StartNanos  int64      // Start duration
    StopNanos   int64      // Stop duration
    Interfaces  []string   // Implemented interfaces
    APIs        []APIDescriptor
    Permissions BusPermissions
}
```

### Manual Status Updates

For modules with external lifecycle:

```go
// Mark as started (without calling Start())
eng.MarkStarted("module-name")

// Mark as stopped
eng.MarkStopped("module-name")

// Mark as ready
eng.MarkReady("module-name", true, "")
eng.MarkReady("module-name", false, "database connection lost")
```

---

## Bus System

Fan-out communication to registered engines.

### Event Bus

```go
// Publish to all EventEngines + local subscribers
err := eng.PublishEvent(ctx, "order.created", map[string]any{
    "order_id": "ord-123",
    "amount":   99.99,
})

// Subscribe to events
eng.SubscribeEvent(ctx, "order.created", func(ctx context.Context, payload any) error {
    order := payload.(map[string]any)
    log.Infof("New order: %v", order["order_id"])
    return nil
})
```

### Data Bus

```go
// Push to all DataEngines
err := eng.PushData(ctx, "metrics/orders", map[string]any{
    "total":     1000,
    "timestamp": time.Now(),
})
```

### Compute Bus

```go
// Invoke all ComputeEngines
results, err := eng.InvokeComputeAll(ctx, map[string]any{
    "function_id": "fn-123",
    "input":       `{"x": 1}`,
})

// Process results
for _, r := range results {
    if r.Err != nil {
        log.Errorf("Module %s failed: %v", r.Module, r.Err)
        continue
    }
    log.Infof("Module %s result: %v", r.Module, r.Result)
}
```

### Bus Permissions

Control which modules can access which buses:

```go
type BusPermissions struct {
    Event   bool  // Can publish/subscribe events
    Data    bool  // Can push data
    Compute bool  // Can be invoked via compute bus
}

// Set permissions
eng.SetBusPermissions("svc-functions", engine.BusPermissions{
    Event:   true,
    Data:    false,
    Compute: true,
})

// Get permissions
perms := eng.GetBusPermissions("svc-functions")
```

---

## Health Monitoring

### Readiness Probing

```go
// Probe all modules
eng.ProbeReadiness(ctx)

// Get health for specific module
health := eng.GetHealth("svc-functions")

// Get all module health
healths := eng.ModulesHealth()
```

### Health Status

```go
type ModuleHealth struct {
    Name        string     `json:"name"`
    Domain      string     `json:"domain,omitempty"`
    Status      string     `json:"status"`        // started|stopped|failed
    ReadyStatus string     `json:"ready_status"`  // ready|not-ready
    ReadyError  string     `json:"ready_error,omitempty"`
    StartedAt   *time.Time `json:"started_at,omitempty"`
    StoppedAt   *time.Time `json:"stopped_at,omitempty"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

### Slow Module Detection

```go
// Get modules that took too long to start/stop
slowModules := eng.SlowModules()

// Configure threshold
eng.SetSlowThreshold(2 * time.Second)
```

---

## Dependency Management

### Declaring Dependencies

```go
// Via options
engine.New(
    engine.WithDependencies(map[string][]string{
        "svc-functions": {"store-postgres", "svc-accounts"},
        "svc-oracle":    {"store-postgres", "svc-accounts", "svc-functions"},
    }),
)

// Or programmatically
eng.SetDependencies("svc-functions", []string{"store-postgres", "svc-accounts"})
```

### Dependency Resolution

```go
// Verify all dependencies exist
if err := eng.VerifyDependencies(); err != nil {
    // Handle missing dependency
}

// Get startup order
order, err := eng.ResolveStartupOrder()
// Returns topologically sorted module names
```

### Dependency Checking at Runtime

```go
// Modules can check if dependencies are ready
func (s *Service) Ready(ctx context.Context) error {
    // Engine tracks dependency readiness
    for _, dep := range s.Manifest().DependsOn {
        health := eng.GetHealth(dep)
        if health.ReadyStatus != "ready" {
            return fmt.Errorf("dependency %s not ready", dep)
        }
    }
    return nil
}
```

---

## API Descriptors

Modules can advertise additional APIs.

### APIDescriptor Structure

```go
type APIDescriptor struct {
    Name        string     `json:"name"`
    Surface     APISurface `json:"surface"`
    Description string     `json:"description,omitempty"`
    Stability   string     `json:"stability,omitempty"` // stable|beta|alpha
    Version     string     `json:"version,omitempty"`
}
```

### Implementing APIDescriber

```go
func (s *Service) APIs() []engine.APIDescriptor {
    return []engine.APIDescriptor{
        {
            Name:        "admin",
            Surface:     "admin",
            Description: "Administrative operations",
            Stability:   "beta",
        },
        {
            Name:        "telemetry",
            Surface:     "telemetry",
            Description: "Telemetry data export",
            Stability:   "alpha",
        },
    }
}
```

### Getting Module APIs

```go
// Get APIs for a module
apis := eng.ModuleAPIs("svc-functions")

// Get all modules grouped by API surface
summary := eng.ModulesAPISummary()
// Returns: map[string][]string{
//   "compute": {"svc-functions", "svc-cre"},
//   "event":   {"svc-oracle", "svc-pricefeed"},
//   ...
// }
```

---

## HTTP Integration

The engine exposes system endpoints via HTTP.

### System Status

```bash
GET /system/status
```

Response:
```json
{
  "modules": [
    {
      "name": "svc-functions",
      "domain": "functions",
      "status": "started",
      "ready_status": "ready",
      "started_at": "2025-01-15T10:00:00Z",
      "start_nanos": 1200000,
      "interfaces": ["ComputeEngine", "ReadyChecker"],
      "apis": [{"name": "lifecycle", "surface": "lifecycle"}],
      "permissions": {"event": true, "data": false, "compute": true}
    }
  ],
  "modules_meta": {"total": 10, "started": 10, "failed": 0},
  "modules_summary": {"compute": ["svc-functions"], "event": ["svc-oracle"]},
  "modules_api_summary": {"compute": ["svc-functions", "svc-cre"]},
  "modules_slow": [],
  "modules_slow_threshold_ms": 1000,
  "listen_addr": "127.0.0.1:8080"
}
```

### Bus Endpoints

```bash
# Publish event
POST /system/events
{"event": "my.event", "payload": {...}}

# Push data
POST /system/data
{"topic": "my.topic", "payload": {...}}

# Invoke compute
POST /system/compute
{"payload": {...}}
```

### Service Descriptors

```bash
GET /system/descriptors
```

Returns all service manifests/descriptors.

---

## Runtime Adapters

Located in `internal/engine/runtime/`.

### Service Module Adapter

Wraps existing services for engine integration:

```go
// Create adapter for a service
adapter := runtime.NewServiceAdapter(
    "svc-myservice",
    "custom",
    myService,
    runtime.WithAccountEngine(myService),  // If implements AccountEngine
    runtime.WithComputeEngine(myService),  // If implements ComputeEngine
    runtime.WithEventEngine(myService),    // If implements EventEngine
)

// Register with engine
eng.Register(adapter)
```

### Infrastructure Modules

Pre-built adapters for common infrastructure:

```go
// PostgreSQL store
storeModule := runtime.NewStoreModule(db)

// RocketMQ event bus
mqModule := runtime.NewRocketMQModule(config)

// Multi-chain RPC
rpcModule := runtime.NewMultiChainRPC(chains)
```

---

## Complete Example

Putting it all together:

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/R3E-Network/service_layer/internal/engine"
    "github.com/R3E-Network/service_layer/internal/engine/runtime"
    "github.com/R3E-Network/service_layer/internal/services/accounts"
    "github.com/R3E-Network/service_layer/internal/services/functions"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    log := logger.NewDefault("main")

    // Create engine
    eng := engine.New(
        engine.WithLogger(log),
        engine.WithOrder(
            "store-postgres",
            "svc-accounts",
            "svc-functions",
        ),
        engine.WithDependencies(map[string][]string{
            "svc-accounts":  {"store-postgres"},
            "svc-functions": {"store-postgres", "svc-accounts"},
        }),
        engine.WithSlowThreshold(2*time.Second),
    )

    // Create and register modules
    store := createStore()
    eng.Register(runtime.NewStoreModule(store))

    accountsSvc := accounts.New(store, log)
    eng.Register(runtime.NewServiceAdapter(
        "svc-accounts", "accounts", accountsSvc,
        runtime.WithAccountEngine(accountsSvc),
    ))

    functionsSvc := functions.New(store, accountsSvc, log)
    eng.Register(runtime.NewServiceAdapter(
        "svc-functions", "functions", functionsSvc,
        runtime.WithComputeEngine(functionsSvc),
    ))

    // Start engine
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := eng.Start(ctx); err != nil {
        log.Fatalf("Engine start failed: %v", err)
    }

    // Log module status
    for _, m := range eng.ModulesStatus() {
        log.Infof("Module %s: %s (ready: %s)", m.Name, m.Status, m.ReadyStatus)
    }

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    log.Info("Shutting down...")

    // Stop engine
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()

    if err := eng.Stop(shutdownCtx); err != nil {
        log.Errorf("Engine stop errors: %v", err)
    }

    log.Info("Shutdown complete")
}
```

---

## Monitoring

### Module Metrics

```go
// Get timing information
timings := eng.ModulesTimings()
// Returns: map[string]ModuleTiming{
//   "svc-functions": {StartMs: 1.2, StopMs: 0.5},
// }

// Get uptimes
uptimes := eng.ModulesUptime()
// Returns: map[string]float64{
//   "svc-functions": 3600.5,  // seconds
// }
```

### Health Aggregation

```go
// Get aggregate health
meta := eng.ModulesMeta()
// Returns: ModulesMeta{
//   Total:    10,
//   Started:  9,
//   Failed:   1,
//   StopError: 0,
//   NotReady:  1,
// }

// List modules waiting for dependencies
waiting := eng.ModulesWaitingForDeps()

// List modules in failed state
failed := eng.FailedModules()
```

---

## Related Documentation

- [Framework Guide](framework-guide.md) - Service SDK
- [Service Catalog](service-catalog.md) - All 17 services
- [Developer Guide](developer-guide.md) - Building services
- [Architecture Layers](architecture-layers.md) - Overall architecture
