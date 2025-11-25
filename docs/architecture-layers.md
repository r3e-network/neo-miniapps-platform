# Service Layer Architecture

## Overview

The Neo N3 Service Layer follows a **clean layered architecture** inspired by the Android operating system design. This architecture provides clear separation of concerns, explicit dependency management, and a predictable structure for adding new capabilities.

The system is organized into **four primary layers** (bottom to top):

1. **Platform Layer** (HAL/Drivers) - Hardware abstraction for external systems
2. **Framework Layer** (API/SDK) - Developer tools and service utilities
3. **Engine Layer** (OS Kernel) - Service orchestration and lifecycle management
4. **Services Layer** (Applications) - Business logic and domain services

Additionally, an **Application Composition Layer** (`internal/app/`) sits above the Services Layer to wire everything together into a runnable application.

### Android OS Analogy

```
┌─────────────────────────────────────────┬────────────────────────────┐
│ Service Layer                           │ Android OS                 │
├─────────────────────────────────────────┼────────────────────────────┤
│ Services (VRF, Oracle, Functions, etc.) │ Apps (Gmail, Chrome, etc.) │
│ Engine (Registry, Bus, Lifecycle)       │ Android Framework          │
│ Framework (ServiceBase, BusClient)      │ Android SDK/APIs           │
│ Platform (RPC, Storage, Cache, Queue)   │ HAL (Camera, GPS, Radio)   │
└─────────────────────────────────────────┴────────────────────────────┘
```

Just as Android apps use standard APIs without knowing about hardware details, our services use framework interfaces without directly touching databases or RPC clients.

---

## Layer Definitions

### Layer 1: Platform (HAL/Drivers)

**Location**: `internal/platform/`

**Purpose**: Provide low-level drivers and adapters for external systems. This is the Hardware Abstraction Layer that isolates the rest of the system from infrastructure implementation details.

**Responsibilities**:
- Abstract blockchain RPC connectivity (Neo N3, Ethereum, etc.)
- Provide database drivers (PostgreSQL, SQLite)
- Handle cache operations (Redis, in-memory)
- Manage message queue integration (RocketMQ, Kafka)
- Expose cryptographic operations (key management, signing, encryption)
- Provide HTTP/gRPC client wrappers
- Connect to external data sources (oracle feeds, APIs)

**Key Interfaces**:
```go
type Driver interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Ping(ctx context.Context) error
}

type RPCDriver interface { ... }      // Blockchain RPC
type StorageDriver interface { ... }  // Database
type CacheDriver interface { ... }    // Redis/Cache
type QueueDriver interface { ... }    // Message queues
type CryptoDriver interface { ... }   // Cryptography
type HTTPDriver interface { ... }     // HTTP clients
type OracleDriver interface { ... }   // External data
```

**Key Files**:
- `driver.go` - Core driver interfaces and types
- `doc.go` - Package documentation and architecture overview
- `database/` - Database connectivity
- `migrations/` - Schema migrations

**Design Principles**:
- Each driver handles one external system
- Drivers are stateless adapters (state lives in external systems)
- Configuration passed during construction
- Lifecycle managed by engine

**Dependencies**: None (bottom layer, depends only on external systems)

---

### Layer 2: Framework (API/SDK)

**Location**: `internal/framework/`

**Purpose**: Provide developer-friendly tools and abstractions for building services. This is the SDK that service developers use.

**Responsibilities**:
- Provide base service implementation (`ServiceBase`)
- Define service manifest and metadata contracts
- Expose bus client for inter-service communication
- Offer lifecycle hooks (PreStart, PostStart, PreStop, PostStop)
- Supply testing utilities and mocks
- Provide builder patterns for simple service creation

**Key Interfaces**:
```go
type ServiceBase struct { ... }  // Embed in services for readiness tracking

type BusClient interface {
    PublishEvent(ctx context.Context, event string, payload any) error
    PushData(ctx context.Context, topic string, payload any) error
    InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error)
}

type Manifest struct {
    Name         string
    Domain       string
    Description  string
    Layer        string
    Capabilities []string
    DependsOn    []string
    RequiresAPIs []APISurface
}
```

**Key Files**:
- `base.go` - ServiceBase with state management and readiness
- `bus.go`, `bus_impl.go` - Bus client interface and implementation
- `manifest.go` - Service manifest definition
- `builder.go` - ServiceBuilder pattern for simple services
- `errors.go` - Framework error types
- `lifecycle/` - Lifecycle hook utilities
- `testing/` - Mock implementations and test helpers

**Design Principles**:
- Services embed `ServiceBase` for common functionality
- Manifest-driven configuration (declare capabilities, dependencies)
- Bus provides publish/subscribe and request/response patterns
- Testing utilities enable isolated unit tests

**Dependencies**:
- Platform Layer (indirectly through engine injection)

---

### Layer 3: Engine (OS Kernel)

**Location**: `internal/engine/`

**Purpose**: Orchestrate services, manage lifecycle, and provide system-level capabilities. This is the operating system kernel.

**Responsibilities**:
- Register and discover services via Registry
- Manage service lifecycle (Start/Stop with dependency ordering)
- Provide event/data/compute bus systems
- Monitor service health and readiness
- Enforce API surface contracts
- Handle graceful shutdown

**Key Interfaces**:
```go
type ServiceModule interface {
    Name() string
    Domain() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

// Capability-based interfaces
type AccountEngine interface { ... }
type StoreEngine interface { ... }
type ComputeEngine interface { ... }
type DataEngine interface { ... }
type EventEngine interface { ... }
type LedgerEngine interface { ... }
type IndexerEngine interface { ... }
type RPCEngine interface { ... }
```

**Key Files**:
- `interfaces.go` - Core engine interfaces (ServiceModule, typed engines)
- `apis.go` - API surface definitions
- `runtime/` - Runtime adapters and module wrappers
  - `application.go` - Application runtime
  - `service_modules.go` - Service wrappers
  - `infrastructure_modules.go` - Infrastructure adapters
  - `config_bridge.go` - Configuration bridging
  - `engine_adapters.go` - Adapter implementations

**Engine Components** (conceptual, not yet fully decomposed):
- **Registry**: Service registration and lookup
- **Lifecycle Manager**: Dependency-aware start/stop
- **Bus System**: Event/data/compute fan-out
- **Health Monitor**: Readiness tracking and probing
- **API Surface**: Capability advertisement and verification

**Design Principles**:
- Services are pluggable via interfaces
- Dependency resolution ensures correct startup order
- Bus enables loose coupling between services
- Typed engine interfaces provide compile-time safety

**Dependencies**:
- Framework Layer (for ServiceBase, Manifest)
- Platform Layer (for infrastructure drivers)

---

### Layer 4: Services (Applications)

**Location**: `internal/services/`

**Purpose**: Implement business logic and domain-specific functionality. These are the applications running on top of the operating system.

**Responsibilities**:
- Implement domain-specific business logic
- Expose domain APIs (accounts, functions, VRF, oracle, etc.)
- Manage domain state via storage interfaces
- Publish domain events via bus
- Handle request validation and transformation

**Service Categories**:

**Core Services**:
- `accounts/` - Account lifecycle and tenancy
- `functions/` - Serverless function execution
- `secrets/` - Secret management and encryption

**Blockchain Services**:
- `vrf/` - Verifiable Random Function
- `oracle/` - External data requests
- `gasbank/` - Gas payment and settlement
- `automation/` - Scheduled function execution
- `triggers/` - Event-driven function triggers

**Data Services**:
- `pricefeed/` - Price feed aggregation
- `datafeeds/` - Generic data feed management
- `datastreams/` - Data streaming
- `datalink/` - Data pipeline orchestration
- `dta/` - Direct token access

**Advanced Services**:
- `cre/` - Compute runtime environment
- `ccip/` - Cross-chain interoperability
- `confidential/` - Confidential computing
- `random/` - Random number generation

**Standard Service Structure**:
```
internal/services/{service-name}/
├── service.go          # Main service implementation
├── service_test.go     # Service tests
├── handlers.go         # Optional request handlers
├── store.go            # Optional storage interface
└── events.go           # Optional event definitions
```

**Service Implementation Pattern**:
```go
type Service struct {
    framework.ServiceBase
    manifest *framework.Manifest
    bus      framework.BusClient
    store    Store
}

func (s *Service) Name() string   { return s.manifest.Name }
func (s *Service) Domain() string { return s.manifest.Domain }

func (s *Service) Start(ctx context.Context) error {
    // Initialize resources
    s.MarkReady(true)
    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    s.MarkReady(false)
    // Cleanup resources
    return nil
}
```

**Design Principles**:
- Services are independent and composable
- Each service owns its domain logic
- Services communicate via bus, not direct calls
- Services declare dependencies in manifest

**Dependencies**:
- Framework Layer (ServiceBase, BusClient)
- Engine Layer (ServiceModule interfaces)

---

### Application Composition Layer

**Location**: `internal/app/`

**Purpose**: Wire services together and compose them into a runnable application. This is the "main assembly" layer.

**Responsibilities**:
- Instantiate all services with their dependencies
- Configure runtime settings from environment/config
- Provide HTTP API handlers (`internal/app/httpapi/`)
- Define domain models (`internal/app/domain/`)
- Implement storage interfaces (`internal/app/storage/`)
- Manage system lifecycle (`internal/app/system/`)
- Expose metrics and observability (`internal/app/metrics/`)

**Key Components**:

**Application Builder** (`application.go`):
- Constructs all services with proper wiring
- Applies runtime configuration
- Registers services with manager or engine
- Provides Start/Stop lifecycle

**Domain Models** (`domain/`):
- Shared domain types used across services
- DTOs and value objects
- Validation logic

**Storage Layer** (`storage/`):
- Storage interface definitions
- In-memory implementations
- PostgreSQL implementations (when applicable)

**HTTP API** (`httpapi/`):
- REST API handlers
- Authentication and authorization
- Tenant isolation middleware
- Request validation and transformation
- Error handling

**System Management** (`system/`):
- Service registry and discovery
- Lifecycle management
- Health monitoring
- Descriptor collection

**Key Files**:
- `application.go` - Main application builder
- `httpapi/handler.go` - HTTP handler router
- `httpapi/service.go` - HTTP service lifecycle
- `storage/interfaces.go` - Storage contracts
- `system/manager.go` - Legacy service manager
- `system/registry.go` - Service registry
- `metrics/metrics.go` - Metrics collection

**Design Principles**:
- Composition over inheritance
- Dependency injection via constructor parameters
- Configuration externalized via environment/config
- Clear separation between composition and business logic

**Dependencies**:
- All lower layers (Platform, Framework, Engine, Services)

---

## Dependency Rules

### Dependency Matrix

```
┌──────────────┬──────────┬───────────┬────────┬──────────┬─────────────┐
│ Layer        │ Platform │ Framework │ Engine │ Services │ Application │
├──────────────┼──────────┼───────────┼────────┼──────────┼─────────────┤
│ Platform     │    -     │     ✗     │   ✗    │    ✗     │      ✗      │
│ Framework    │    ✓     │     -     │   ✗    │    ✗     │      ✗      │
│ Engine       │    ✓     │     ✓     │   -    │    ✗     │      ✗      │
│ Services     │    ✗     │     ✓     │   ✓    │    ✗*    │      ✗      │
│ Application  │    ✓     │     ✓     │   ✓    │    ✓     │      -      │
└──────────────┴──────────┴───────────┴────────┴──────────┴─────────────┘

Legend:
  ✓ = Can depend on (import packages from)
  ✗ = Cannot depend on (dependency violation)
  * = Services should not directly depend on each other; use bus for communication
```

### Key Rules

1. **Platform Layer**:
   - Depends on: Nothing (external systems only)
   - Depended on by: Framework, Engine, Application

2. **Framework Layer**:
   - Depends on: Platform (indirectly via injection)
   - Depended on by: Engine, Services, Application
   - Never imports from: Engine, Services, Application

3. **Engine Layer**:
   - Depends on: Platform, Framework
   - Depended on by: Services, Application
   - Never imports from: Services, Application

4. **Services Layer**:
   - Depends on: Framework, Engine (interfaces only)
   - Depended on by: Application
   - Never imports from: Platform (use engine), other Services (use bus)

5. **Application Layer**:
   - Depends on: All layers (composition root)
   - Depended on by: None (top of the stack)

### Import Path Guidelines

```
✓ GOOD:
  internal/framework/base.go  → (no internal imports)
  internal/engine/interfaces.go → internal/framework
  internal/services/vrf/service.go → internal/framework, internal/engine
  internal/app/application.go → internal/services/vrf

✗ BAD:
  internal/framework/base.go → internal/engine
  internal/engine/registry.go → internal/services/vrf
  internal/services/vrf/service.go → internal/services/oracle
  internal/services/vrf/service.go → internal/platform/driver
```

---

## Architecture Diagrams

### Full Stack Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     APPLICATION COMPOSITION LAYER                        │
│                        (internal/app/)                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │   HTTP API   │  │    Domain    │  │   Storage    │  │   System   │ │
│  │   Handlers   │  │    Models    │  │  Interfaces  │  │  Manager   │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │         Application Builder (wiring & composition)               │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
┌─────────────────────────────────────────────────────────────────────────┐
│                         SERVICES LAYER (Applications)                    │
│                        (internal/services/)                              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │   VRF   │ │ Oracle  │ │Functions│ │ Gasbank │ │Automation│          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │PriceFeed│ │DataFeeds│ │DataLink │ │  CCIP   │ │   CRE   │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                      │
│  │Accounts │ │ Secrets │ │ Random  │ │   DTA   │   ... (17+ services) │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘                      │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
┌─────────────────────────────────────────────────────────────────────────┐
│                    ENGINE LAYER (OS Kernel)                              │
│                        (internal/engine/)                                │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Service Engine                               │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐            │  │
│  │  │ Registry │ │Lifecycle │ │   Bus    │ │  Health  │            │  │
│  │  │          │ │  Manager │ │  System  │ │ Monitor  │            │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘            │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐                         │  │
│  │  │   Deps   │ │API Surface│ │  Config  │                         │  │
│  │  │ Resolver │ │ Verifier  │ │  Bridge  │                         │  │
│  │  └──────────┘ └──────────┘ └──────────┘                         │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
┌─────────────────────────────────────────────────────────────────────────┐
│                   FRAMEWORK LAYER (API/SDK)                              │
│                        (internal/framework/)                             │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐  │
│  │ ServiceBase  │ │  BusClient   │ │   Manifest   │ │  Lifecycle   │  │
│  │  (state)     │ │  (pub/sub)   │ │  (contract)  │ │   (hooks)    │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                   │
│  │   Builder    │ │   Testing    │ │    Errors    │                   │
│  │  (patterns)  │ │   (mocks)    │ │  (standard)  │                   │
│  └──────────────┘ └──────────────┘ └──────────────┘                   │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
┌─────────────────────────────────────────────────────────────────────────┐
│                  PLATFORM LAYER (HAL/Drivers)                            │
│                        (internal/platform/)                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│  │Blockchain│ │ Storage  │ │  Cache   │ │  Queue   │ │  Crypto  │    │
│  │   RPC    │ │ (Postgres│ │ (Redis)  │ │(RocketMQ)│ │  (HSM)   │    │
│  │(Neo/ETH) │ │ /SQLite) │ │          │ │          │ │          │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                 │
│  │   HTTP   │ │   gRPC   │ │WebSocket │ │  Oracle  │                 │
│  │  Client  │ │  Client  │ │  Client  │ │  Feeds   │                 │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘                 │
└─────────────────────────────────────────────────────────────────────────┘
                                    ▲
                                    │
                         External Systems
        (Neo N3 Network, Postgres, Redis, RocketMQ, APIs)
```

### Request Flow Diagram

```
HTTP Request
    │
    ▼
┌────────────────────────────────────────────────────┐
│ Application Layer: HTTP Handler                    │
│ • Authentication/Authorization                     │
│ • Tenant isolation                                 │
│ • Request validation                               │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Services Layer: Domain Service                     │
│ • Business logic                                   │
│ • State validation                                 │
│ • Event publishing                                 │
└──────────────────┬─────────────────────────────────┘
                   │
           ┌───────┴───────┐
           │               │
           ▼               ▼
┌──────────────────┐  ┌──────────────────┐
│ Framework: Bus   │  │ Engine: Registry │
│ • Publish event  │  │ • Route to deps  │
└──────────────────┘  └──────────────────┘
           │
           ▼
┌────────────────────────────────────────────────────┐
│ Platform Layer: Storage Driver                     │
│ • Database operations                              │
│ • Transaction management                           │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
              PostgreSQL
```

### Lifecycle Flow Diagram

```
Application Start
    │
    ▼
┌────────────────────────────────────────────────────┐
│ Application Layer                                  │
│ • Load configuration                               │
│ • Instantiate services                             │
│ • Wire dependencies                                │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Engine Layer                                       │
│ • Register services                                │
│ • Resolve dependencies                             │
│ • Calculate startup order                          │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Platform Layer                                     │
│ • Start drivers (Storage, RPC, Cache, etc.)        │
│ • Verify connectivity                              │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Services Layer                                     │
│ • Start services in dependency order               │
│ • Mark ready when initialized                      │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Application Layer                                  │
│ • Start HTTP API                                   │
│ • Begin serving requests                           │
└────────────────────────────────────────────────────┘
```

---

## Adding New Components

### Adding a New Service

Follow these steps to add a new domain service:

**Step 1: Create Service Directory**
```bash
mkdir -p internal/services/myservice
cd internal/services/myservice
```

**Step 2: Implement Service**
```go
// service.go
package myservice

import (
    "context"
    "github.com/R3E-Network/service_layer/internal/framework"
    "github.com/R3E-Network/service_layer/internal/engine"
)

type Service struct {
    framework.ServiceBase
    manifest *framework.Manifest
    bus      framework.BusClient
    store    Store
}

func New(store Store, bus framework.BusClient) *Service {
    s := &Service{
        manifest: &framework.Manifest{
            Name:         "myservice",
            Domain:       "myservice",
            Description:  "My new domain service",
            Layer:        "service",
            Capabilities: []string{"myservice.read", "myservice.write"},
            DependsOn:    []string{"svc-accounts"},
            RequiresAPIs: []engine.APISurface{engine.APISurfaceStore},
        },
        store: store,
        bus:   bus,
    }
    s.ServiceBase = *framework.NewServiceBase("myservice", "myservice")
    return s
}

func (s *Service) Name() string { return s.manifest.Name }
func (s *Service) Domain() string { return s.manifest.Domain }
func (s *Service) Manifest() *framework.Manifest { return s.manifest }

func (s *Service) Start(ctx context.Context) error {
    // Initialize resources
    s.MarkStarted()
    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    // Cleanup resources
    s.MarkStopped()
    return nil
}

// Domain methods
func (s *Service) DoSomething(ctx context.Context, input Input) (*Output, error) {
    if err := s.Ready(ctx); err != nil {
        return nil, err
    }
    // Business logic...
    return &Output{}, nil
}
```

**Step 3: Define Storage Interface**
```go
// store.go
package myservice

import "context"

type Store interface {
    Create(ctx context.Context, entity *Entity) error
    Get(ctx context.Context, id string) (*Entity, error)
    List(ctx context.Context, accountID string) ([]*Entity, error)
    Update(ctx context.Context, entity *Entity) error
    Delete(ctx context.Context, id string) error
}

type Entity struct {
    ID        string
    AccountID string
    Data      map[string]any
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Step 4: Add Storage Implementation**

Update `internal/app/storage/interfaces.go`:
```go
type MyServiceStore interface {
    // Same methods as service Store interface
}
```

Update `internal/app/storage/memory/store.go` to implement the interface.

**Step 5: Wire in Application Layer**

Update `internal/app/application.go`:
```go
// Add to Stores struct
type Stores struct {
    // ... existing stores
    MyService storage.MyServiceStore
}

// Add to Application struct
type Application struct {
    // ... existing services
    MyService *myservice.Service
}

// Add to New() function
func New(stores Stores, log *logger.Logger, opts ...Option) (*Application, error) {
    // ... existing setup

    myService := myservice.New(stores.MyService, bus)

    if manager != nil {
        if err := manager.Register(myService); err != nil {
            return nil, fmt.Errorf("register myservice: %w", err)
        }
    }

    return &Application{
        // ... existing fields
        MyService: myService,
    }, nil
}
```

**Step 6: Add HTTP Handlers**

Create `internal/app/httpapi/handler_myservice.go`:
```go
func (h *Handler) handleMyServiceList(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    accountID := getAccountID(ctx)

    entities, err := h.app.MyService.List(ctx, accountID)
    if err != nil {
        h.error(w, r, err)
        return
    }

    h.json(w, r, http.StatusOK, entities)
}
```

Register routes in `handler.go` or `router.go`.

**Step 7: Write Tests**
```go
// service_test.go
func TestService_DoSomething(t *testing.T) {
    store := &mockStore{}
    bus := framework.NewMockBusClient()
    svc := New(store, bus)

    ctx := context.Background()
    if err := svc.Start(ctx); err != nil {
        t.Fatal(err)
    }
    defer svc.Stop(ctx)

    // Test business logic
    output, err := svc.DoSomething(ctx, Input{})
    if err != nil {
        t.Fatal(err)
    }
    // Assertions...
}
```

**Step 8: Update Documentation**
- Add service to this document's Services Layer section
- Update API documentation
- Add examples to `docs/examples/`

### Adding a New Platform Driver

**Step 1: Define Driver Interface**

Update `internal/platform/driver.go`:
```go
// MySystemDriver provides connectivity to My External System.
type MySystemDriver interface {
    Driver

    // Domain-specific methods
    Query(ctx context.Context, query string) ([]Result, error)
    Execute(ctx context.Context, command string) error
}
```

**Step 2: Implement Driver**
```go
// internal/platform/mysystem/driver.go
package mysystem

import (
    "context"
    "github.com/R3E-Network/service_layer/internal/platform"
)

type driver struct {
    endpoint string
    client   *http.Client
}

func NewDriver(endpoint string) platform.MySystemDriver {
    return &driver{
        endpoint: endpoint,
        client:   &http.Client{Timeout: 10 * time.Second},
    }
}

func (d *driver) Name() string { return "mysystem" }

func (d *driver) Start(ctx context.Context) error {
    // Initialize connection
    return nil
}

func (d *driver) Stop(ctx context.Context) error {
    // Cleanup
    return nil
}

func (d *driver) Ping(ctx context.Context) error {
    // Health check
    return nil
}

func (d *driver) Query(ctx context.Context, query string) ([]Result, error) {
    // Implementation
    return nil, nil
}
```

**Step 3: Register with Engine**

Update `internal/engine/runtime/infrastructure_modules.go` to wrap the driver if it needs to be lifecycle-managed by the engine.

**Step 4: Wire in Application**

Update application builder to instantiate and inject the driver where needed.

### Adding a New Framework Utility

**Step 1: Implement Utility**
```go
// internal/framework/myutil/utility.go
package myutil

func DoSomething() {
    // Utility logic
}
```

**Step 2: Document in Framework Package**

Update `internal/framework/doc.go` with usage examples.

**Step 3: Add Tests**
```go
// internal/framework/myutil/utility_test.go
func TestDoSomething(t *testing.T) {
    // Test utility
}
```

---

## Best Practices

### Layer Guidelines

1. **Platform Layer**:
   - Keep drivers thin; just adapt external systems
   - Use context for cancellation and timeouts
   - Return domain-agnostic errors
   - Make drivers reusable across services

2. **Framework Layer**:
   - Provide sensible defaults
   - Make utilities composable
   - Minimize dependencies
   - Document with examples

3. **Engine Layer**:
   - Keep orchestration logic separate from business logic
   - Use dependency injection
   - Fail fast on misconfiguration
   - Log lifecycle events clearly

4. **Services Layer**:
   - Keep services focused on single domain
   - Use bus for inter-service communication
   - Validate inputs thoroughly
   - Return structured errors

5. **Application Layer**:
   - Keep wiring logic separate from business logic
   - Externalize configuration
   - Make dependencies explicit
   - Provide builder options for testing

### Dependency Management

- **Never import upwards**: Lower layers cannot import upper layers
- **Services communicate via bus**: Avoid direct service-to-service dependencies
- **Use interfaces**: Depend on abstractions, not implementations
- **Inject dependencies**: Pass dependencies via constructors

### Testing Strategy

- **Unit tests**: Test each layer in isolation using mocks
- **Integration tests**: Test layer boundaries with real implementations
- **End-to-end tests**: Test full request flow through all layers

### Error Handling

- **Platform Layer**: Return wrapped errors with context
- **Framework Layer**: Define error types for common cases
- **Engine Layer**: Log and propagate errors with stack traces
- **Services Layer**: Return domain-specific errors
- **Application Layer**: Transform errors to HTTP responses

---

## Cross-Cutting Concerns

### Observability

- **Metrics**: Collected in Application Layer (`internal/app/metrics/`)
- **Logging**: Structured logging via `pkg/logger`
- **Tracing**: Context propagation through all layers
- **Health Checks**: Exposed via `/healthz` and `/system/status`

### Security

- **Authentication**: Handled in Application Layer (HTTP middleware)
- **Authorization**: Enforced in Service Layer (account validation)
- **Secrets**: Managed via Secrets service
- **Encryption**: Provided by Platform Layer (CryptoDriver)

### Configuration

- **Environment Variables**: Parsed in Application Layer
- **Config Files**: Loaded and validated at startup
- **Runtime Config**: Bridged to services via `RuntimeConfig`
- **Defaults**: Provided by Framework Layer

---

## Migration Path

### Current State

The codebase is transitioning from a mixed architecture to this clean layered design:

- Platform Layer: Well-defined (`internal/platform/`)
- Framework Layer: Well-defined (`internal/framework/`)
- Engine Layer: Partially decomposed (interfaces defined, needs refactoring)
- Services Layer: Well-defined (`internal/services/`)
- Application Layer: Well-defined (`internal/app/`)

### Future Work

1. **Engine Decomposition**: Extract registry, lifecycle, bus, and health monitoring into separate files
2. **Service Standardization**: Migrate all services to use ServiceBuilder pattern
3. **Testing Utilities**: Expand framework testing package with more mocks
4. **Documentation**: Add per-service README files with examples

---

## References

### Related Documentation

- `docs/service-engine-architecture.md` - Detailed engine design
- `docs/service-engine.md` - Engine implementation guide
- `docs/core-engine.md` - Core engine concepts
- `docs/neo-layering-summary.md` - NEO-specific layering
- `docs/system-architecture.md` - System-wide architecture overview
- `docs/examples/services.md` - Service usage examples
- `docs/examples/bus.md` - Bus usage examples

### External References

- Clean Architecture (Robert C. Martin)
- Hexagonal Architecture (Alistair Cockburn)
- Android Architecture Components
- Go Project Layout (golang-standards)

---

**Document Version**: 1.0
**Last Updated**: 2025-11-25
**Author**: Service Layer Team
