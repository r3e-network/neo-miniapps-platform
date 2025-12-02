# Service Engine Architecture Redesign

## Android OS Architecture Reference

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ANDROID OS ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│  Applications        │ System Apps │ User Apps │ Third-party Apps           │
├─────────────────────────────────────────────────────────────────────────────┤
│  Application         │ Activity Manager │ Window Manager │ Content Provider │
│  Framework           │ Package Manager  │ Notification   │ Resource Manager │
├─────────────────────────────────────────────────────────────────────────────┤
│  System Services     │ System Server │ Media Server │ Surface Flinger       │
├─────────────────────────────────────────────────────────────────────────────┤
│  Android Runtime     │ ART/Dalvik │ Core Libraries │ Native Libraries       │
├─────────────────────────────────────────────────────────────────────────────┤
│  HAL                 │ Hardware Abstraction Layer                           │
├─────────────────────────────────────────────────────────────────────────────┤
│  Linux Kernel        │ Drivers │ Power │ Memory │ IPC (Binder)              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Current Architecture Analysis

### Strengths
1. Good separation between `system/` and `packages/`
2. ServiceEngine provides common functionality
3. Environment abstraction exists
4. TEE subsystem is well-structured

### Weaknesses
1. **No clear System Server** - Missing centralized service orchestration
2. **Scattered Managers** - ActivityManager, PackageManager equivalents are fragmented
3. **No Binder IPC** - Services communicate directly, not through IPC
4. **Missing HAL** - Hardware abstraction is mixed with business logic
5. **No Intent System** - No standardized inter-service communication
6. **Lifecycle Management** - Incomplete service lifecycle (no pause/resume)

---

## Proposed Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      NEO SERVICE LAYER ARCHITECTURE                          │
│                        (Android OS Style)                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                        APPLICATIONS LAYER                               │ │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐  │ │
│  │  │  HTTP API    │ │  gRPC API    │ │  WebSocket   │ │  Dashboard   │  │ │
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                     APPLICATION FRAMEWORK                               │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Service    │ │  Package    │ │  Intent     │ │  Content    │      │ │
│  │  │  Manager    │ │  Manager    │ │  Router     │ │  Resolver   │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Account    │ │  Permission │ │  Quota      │ │  Notification│      │ │
│  │  │  Manager    │ │  Manager    │ │  Manager    │ │  Manager    │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                        SYSTEM SERVICES                                  │ │
│  │  ┌──────────────────────────────────────────────────────────────────┐  │ │
│  │  │                      SYSTEM SERVER                                │  │ │
│  │  │  - Service Lifecycle Management                                   │  │ │
│  │  │  - Dependency Resolution                                          │  │ │
│  │  │  - Health Monitoring                                              │  │ │
│  │  │  - Resource Allocation                                            │  │ │
│  │  └──────────────────────────────────────────────────────────────────┘  │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Secrets    │ │  TEE        │ │  Scheduler  │ │  Event      │      │ │
│  │  │  Service    │ │  Service    │ │  Service    │ │  Service    │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Audit      │ │  Metrics    │ │  Tracing    │ │  Cache      │      │ │
│  │  │  Service    │ │  Service    │ │  Service    │ │  Service    │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                     SERVICE PACKAGES (APKs)                             │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │ │
│  │  │accounts │ │functions│ │ oracle  │ │  vrf    │ │ gasbank │ ...      │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘          │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                      RUNTIME LAYER                                      │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Service    │ │  Sandbox    │ │  Script     │ │  Contract   │      │ │
│  │  │  Runtime    │ │  Runtime    │ │  Runtime    │ │  Runtime    │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                   HARDWARE ABSTRACTION LAYER (HAL)                      │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  Database   │ │  TEE/SGX    │ │  Network    │ │  Crypto     │      │ │
│  │  │  HAL        │ │  HAL        │ │  HAL        │ │  HAL        │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                         │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                         PLATFORM LAYER                                  │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │  PostgreSQL │ │  SGX HW     │ │  Neo N3     │ │  External   │      │ │
│  │  │  Driver     │ │  Driver     │ │  RPC        │ │  APIs       │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure Redesign

```
system/
├── server/                      # System Server (NEW)
│   ├── server.go                # Main system server
│   ├── lifecycle.go             # Service lifecycle management
│   ├── dependency.go            # Dependency resolution
│   ├── health.go                # Health monitoring
│   └── resource.go              # Resource allocation
│
├── managers/                    # Application Framework Managers (NEW)
│   ├── service_manager.go       # ServiceManager (like ActivityManager)
│   ├── package_manager.go       # PackageManager
│   ├── intent_router.go         # IntentRouter (inter-service communication)
│   ├── content_resolver.go      # ContentResolver (data access)
│   ├── account_manager.go       # AccountManager
│   ├── permission_manager.go    # PermissionManager
│   ├── quota_manager.go         # QuotaManager
│   └── notification_manager.go  # NotificationManager
│
├── services/                    # System Services (NEW)
│   ├── secrets/                 # SecretsService
│   ├── scheduler/               # SchedulerService
│   ├── audit/                   # AuditService
│   ├── metrics/                 # MetricsService
│   ├── tracing/                 # TracingService
│   └── cache/                   # CacheService
│
├── runtime/                     # Runtime Layer (ENHANCED)
│   ├── service_runtime.go       # Service execution runtime
│   ├── sandbox_runtime.go       # Sandbox execution runtime
│   ├── script_runtime.go        # JavaScript execution runtime
│   ├── contract_runtime.go      # Smart contract runtime
│   └── loader.go                # Package loader
│
├── hal/                         # Hardware Abstraction Layer (NEW)
│   ├── hal.go                   # HAL interfaces
│   ├── database/                # Database HAL
│   │   ├── interface.go
│   │   ├── postgres.go
│   │   └── memory.go
│   ├── tee/                     # TEE HAL
│   │   ├── interface.go
│   │   ├── sgx.go
│   │   └── simulation.go
│   ├── network/                 # Network HAL
│   │   ├── interface.go
│   │   └── http.go
│   └── crypto/                  # Crypto HAL
│       ├── interface.go
│       └── native.go
│
├── ipc/                         # Inter-Process Communication (NEW)
│   ├── binder.go                # Binder-style IPC
│   ├── intent.go                # Intent system
│   ├── messenger.go             # Message passing
│   └── broadcast.go             # Broadcast system
│
├── framework/                   # Service Framework (REORGANIZED)
│   ├── base/                    # Base classes
│   │   ├── service.go           # ServiceBase
│   │   ├── component.go         # ComponentBase
│   │   └── provider.go          # ProviderBase
│   ├── context/                 # Context system
│   │   ├── context.go           # ServiceContext
│   │   ├── application.go       # ApplicationContext
│   │   └── engine.go            # EngineContext
│   ├── manifest/                # Manifest system
│   │   ├── manifest.go
│   │   ├── parser.go
│   │   └── validator.go
│   └── lifecycle/               # Lifecycle management
│       ├── state.go
│       ├── callbacks.go
│       └── observer.go
│
├── tee/                         # TEE Subsystem (KEEP)
│   ├── types/
│   ├── sdk/
│   ├── registry/
│   └── ...
│
├── core/                        # Core Interfaces (REORGANIZED)
│   ├── engines.go               # Engine interfaces
│   ├── capabilities.go          # Capability interfaces
│   ├── registry.go              # Service registry
│   └── types.go                 # Core types
│
├── platform/                    # Platform Drivers (KEEP)
│   ├── database/
│   ├── driver.go
│   └── migrations/
│
├── api/                         # API Layer (KEEP)
│   └── ...
│
├── events/                      # Event System (KEEP)
│   └── ...
│
├── sandbox/                     # Sandbox System (KEEP)
│   └── ...
│
└── bootstrap/                   # Bootstrap (KEEP)
    └── ...
```

---

## Key Components Design

### 1. System Server

```go
// system/server/server.go
package server

// SystemServer is the central orchestrator for all system services.
// It manages service lifecycle, dependency resolution, and health monitoring.
// Equivalent to Android's SystemServer.
type SystemServer struct {
    mu sync.RWMutex

    // Core managers
    serviceManager    *ServiceManager
    packageManager    *PackageManager
    permissionManager *PermissionManager

    // System services
    services map[string]SystemService

    // State
    phase   BootPhase
    ready   chan struct{}

    // Configuration
    config *ServerConfig
}

type BootPhase int

const (
    BootPhaseInit BootPhase = iota
    BootPhaseSystemServices
    BootPhaseCoreServices
    BootPhasePackageServices
    BootPhaseReady
)

// Boot starts the system server and all services in order.
func (s *SystemServer) Boot(ctx context.Context) error {
    // Phase 1: Initialize core managers
    s.phase = BootPhaseInit
    if err := s.initManagers(ctx); err != nil {
        return fmt.Errorf("init managers: %w", err)
    }

    // Phase 2: Start system services
    s.phase = BootPhaseSystemServices
    if err := s.startSystemServices(ctx); err != nil {
        return fmt.Errorf("start system services: %w", err)
    }

    // Phase 3: Start core services
    s.phase = BootPhaseCoreServices
    if err := s.startCoreServices(ctx); err != nil {
        return fmt.Errorf("start core services: %w", err)
    }

    // Phase 4: Load and start package services
    s.phase = BootPhasePackageServices
    if err := s.startPackageServices(ctx); err != nil {
        return fmt.Errorf("start package services: %w", err)
    }

    // Phase 5: Ready
    s.phase = BootPhaseReady
    close(s.ready)

    return nil
}
```

### 2. Service Manager

```go
// system/managers/service_manager.go
package managers

// ServiceManager manages service lifecycle and provides service discovery.
// Equivalent to Android's ActivityManager + ServiceManager.
type ServiceManager struct {
    mu sync.RWMutex

    // Service registry
    services    map[string]ServiceRecord
    byInterface map[string][]string  // interface -> service names

    // Lifecycle
    lifecycle *LifecycleManager

    // Dependencies
    resolver *DependencyResolver
}

// ServiceRecord contains metadata about a registered service.
type ServiceRecord struct {
    Name        string
    Service     Service
    Manifest    *Manifest
    State       ServiceState
    StartedAt   time.Time
    StoppedAt   time.Time
    RestartCount int
    LastError   error
}

type ServiceState int

const (
    ServiceStateCreated ServiceState = iota
    ServiceStateInitializing
    ServiceStateStarting
    ServiceStateRunning
    ServiceStatePausing
    ServiceStatePaused
    ServiceStateResuming
    ServiceStateStopping
    ServiceStateStopped
    ServiceStateDestroyed
    ServiceStateFailed
)

// GetService retrieves a service by name.
func (m *ServiceManager) GetService(name string) (Service, error)

// GetServiceByInterface retrieves services implementing an interface.
func (m *ServiceManager) GetServiceByInterface(iface string) ([]Service, error)

// StartService starts a service by name.
func (m *ServiceManager) StartService(ctx context.Context, name string) error

// StopService stops a service by name.
func (m *ServiceManager) StopService(ctx context.Context, name string) error

// RestartService restarts a service.
func (m *ServiceManager) RestartService(ctx context.Context, name string) error
```

### 3. Intent Router (IPC)

```go
// system/ipc/intent.go
package ipc

// Intent represents an inter-service communication message.
// Equivalent to Android's Intent system.
type Intent struct {
    // Action to perform
    Action string `json:"action"`

    // Target service (explicit intent)
    Component string `json:"component,omitempty"`

    // Data URI
    Data string `json:"data,omitempty"`

    // MIME type
    Type string `json:"type,omitempty"`

    // Category
    Categories []string `json:"categories,omitempty"`

    // Extra data
    Extras map[string]any `json:"extras,omitempty"`

    // Flags
    Flags IntentFlags `json:"flags,omitempty"`
}

// Standard actions
const (
    ActionView     = "neo.intent.action.VIEW"
    ActionEdit     = "neo.intent.action.EDIT"
    ActionSend     = "neo.intent.action.SEND"
    ActionQuery    = "neo.intent.action.QUERY"
    ActionExecute  = "neo.intent.action.EXECUTE"
    ActionSchedule = "neo.intent.action.SCHEDULE"
)

// IntentRouter routes intents to appropriate services.
type IntentRouter struct {
    mu sync.RWMutex

    // Intent filters registered by services
    filters map[string][]IntentFilter

    // Service manager for service lookup
    serviceManager *ServiceManager
}

// IntentFilter defines what intents a service can handle.
type IntentFilter struct {
    Actions    []string
    Categories []string
    DataSchemes []string
    DataTypes  []string
    Priority   int
}

// Route finds the best service to handle an intent.
func (r *IntentRouter) Route(ctx context.Context, intent *Intent) (Service, error)

// Broadcast sends an intent to all matching services.
func (r *IntentRouter) Broadcast(ctx context.Context, intent *Intent) error

// StartService starts a service with an intent.
func (r *IntentRouter) StartService(ctx context.Context, intent *Intent) error
```

### 4. Content Resolver

```go
// system/managers/content_resolver.go
package managers

// ContentResolver provides unified data access across services.
// Equivalent to Android's ContentResolver.
type ContentResolver struct {
    mu sync.RWMutex

    // Registered content providers
    providers map[string]ContentProvider

    // URI scheme -> provider mapping
    schemes map[string]string
}

// ContentProvider interface for data providers.
type ContentProvider interface {
    // Query retrieves data matching the criteria.
    Query(ctx context.Context, uri string, projection []string,
          selection string, selectionArgs []any, sortOrder string) (Cursor, error)

    // Insert adds new data.
    Insert(ctx context.Context, uri string, values map[string]any) (string, error)

    // Update modifies existing data.
    Update(ctx context.Context, uri string, values map[string]any,
           selection string, selectionArgs []any) (int, error)

    // Delete removes data.
    Delete(ctx context.Context, uri string,
           selection string, selectionArgs []any) (int, error)

    // GetType returns the MIME type for a URI.
    GetType(uri string) string
}

// URI format: content://{authority}/{path}
// Example: content://accounts/users/123
//          content://oracle/prices/NEO-USD
```

### 5. Hardware Abstraction Layer

```go
// system/hal/hal.go
package hal

// HAL provides hardware abstraction interfaces.
// Services interact with HAL, not directly with hardware/drivers.

// DatabaseHAL abstracts database operations.
type DatabaseHAL interface {
    // Connection management
    Connect(ctx context.Context, config DatabaseConfig) error
    Disconnect(ctx context.Context) error
    Ping(ctx context.Context) error

    // Query execution
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    Exec(ctx context.Context, query string, args ...any) (Result, error)

    // Transaction support
    Begin(ctx context.Context) (Transaction, error)
}

// TEEHAL abstracts TEE operations.
type TEEHAL interface {
    // Lifecycle
    Initialize(ctx context.Context) error
    Shutdown(ctx context.Context) error

    // Execution
    Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)

    // Attestation
    GetAttestation(ctx context.Context) (*AttestationReport, error)

    // Sealing
    Seal(ctx context.Context, data []byte) ([]byte, error)
    Unseal(ctx context.Context, sealed []byte) ([]byte, error)

    // Mode
    Mode() TEEMode
}

// NetworkHAL abstracts network operations.
type NetworkHAL interface {
    // HTTP
    HTTPRequest(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

    // WebSocket
    WebSocketConnect(ctx context.Context, url string) (WebSocketConn, error)

    // RPC
    RPCCall(ctx context.Context, endpoint string, method string, params any) (any, error)
}

// CryptoHAL abstracts cryptographic operations.
type CryptoHAL interface {
    // Hashing
    Hash(algorithm string, data []byte) ([]byte, error)

    // Signing
    Sign(key []byte, data []byte) ([]byte, error)
    Verify(key []byte, data []byte, signature []byte) (bool, error)

    // Encryption
    Encrypt(key []byte, plaintext []byte) ([]byte, error)
    Decrypt(key []byte, ciphertext []byte) ([]byte, error)

    // Key generation
    GenerateKey(keyType string, bits int) ([]byte, []byte, error)

    // Random
    RandomBytes(n int) ([]byte, error)
}
```

### 6. Enhanced Service Lifecycle

```go
// system/framework/lifecycle/state.go
package lifecycle

// ServiceLifecycle defines the complete service lifecycle.
// Follows Android's Activity/Service lifecycle pattern.
type ServiceLifecycle interface {
    // Creation
    OnCreate(ctx context.Context) error

    // Binding (for bound services)
    OnBind(ctx context.Context, intent *Intent) (IBinder, error)
    OnUnbind(ctx context.Context, intent *Intent) bool
    OnRebind(ctx context.Context, intent *Intent)

    // Start command (for started services)
    OnStartCommand(ctx context.Context, intent *Intent, flags int, startId int) int

    // Lifecycle transitions
    OnStart(ctx context.Context) error
    OnPause(ctx context.Context) error
    OnResume(ctx context.Context) error
    OnStop(ctx context.Context) error

    // Destruction
    OnDestroy(ctx context.Context)

    // Configuration changes
    OnConfigurationChanged(ctx context.Context, config *Configuration)

    // Low memory
    OnLowMemory(ctx context.Context)
    OnTrimMemory(ctx context.Context, level int)
}

// Start command return values
const (
    StartNotSticky        = 1 // Don't restart if killed
    StartSticky           = 2 // Restart with null intent
    StartRedeliverIntent  = 3 // Restart with last intent
    StartStickyCompatibility = 4
)

// Memory trim levels
const (
    TrimMemoryComplete       = 80
    TrimMemoryModerate       = 60
    TrimMemoryBackground     = 40
    TrimMemoryUIHidden       = 20
    TrimMemoryRunningCritical = 15
    TrimMemoryRunningLow     = 10
    TrimMemoryRunningModerate = 5
)
```

---

## Migration Plan

### Phase 1: Foundation (Week 1-2)
1. Create `system/server/` - System Server
2. Create `system/hal/` - Hardware Abstraction Layer
3. Create `system/ipc/` - Intent system

### Phase 2: Managers (Week 3-4)
1. Create `system/managers/service_manager.go`
2. Create `system/managers/package_manager.go`
3. Create `system/managers/intent_router.go`
4. Create `system/managers/content_resolver.go`

### Phase 3: Lifecycle (Week 5)
1. Enhance service lifecycle with pause/resume
2. Add OnBind/OnUnbind for bound services
3. Implement OnStartCommand for started services

### Phase 4: System Services (Week 6-7)
1. Migrate existing services to new architecture
2. Create system services (Secrets, Scheduler, Audit, etc.)
3. Implement HAL implementations

### Phase 5: Integration (Week 8)
1. Update all service packages to use new architecture
2. Update applications layer
3. Testing and documentation

---

## Benefits

1. **Clear Separation of Concerns** - Each layer has specific responsibilities
2. **Standardized IPC** - Intent system for inter-service communication
3. **Hardware Abstraction** - Services don't depend on specific implementations
4. **Complete Lifecycle** - Full lifecycle management with pause/resume
5. **Service Discovery** - ServiceManager provides service lookup
6. **Content Access** - ContentResolver for unified data access
7. **Boot Sequence** - Ordered boot with dependency resolution
8. **Testability** - HAL allows easy mocking for tests
