# TEE-Centric + Supabase Architecture for Neo N3 Service Layer

## Overview

This document describes the refactored architecture where **TEE (Trusted Execution Environment)** is the **trust root** of the entire system, and **Self-hosted Supabase** provides the infrastructure layer. The design follows **Android OS architecture patterns**.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           APPLICATIONS                                       │
│  cmd/server, CLI tools, Frontend, SDKs                                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                    ↑
┌─────────────────────────────────────────────────────────────────────────────┐
│                      SERVICES (Android Apps)                                 │
│  Oracle, Mixer, Secrets, VRF, DataFeeds, GasBank, Accounts, etc.           │
│  - Request capabilities from ServiceOS                                       │
│  - ALL sensitive operations delegated to TEE via ServiceOS                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    ↑ ServiceOS API
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SERVICE OS (Android OS Layer)                             │
│  - Capability-based access control (Android permissions model)              │
│  - Routes ALL sensitive operations to TEE                                    │
│  - Routes ALL data operations to Supabase                                    │
│  - Service lifecycle management                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                          ↙                    ↘
┌────────────────────────────────┐    ┌────────────────────────────────────────┐
│     TEE (Trust Root)           │    │     SUPABASE (Infrastructure)          │
│  ┌──────────────────────────┐  │    │  ┌──────────────────────────────────┐  │
│  │   ENCLAVE (SGX/TDX)      │  │    │  │  PostgreSQL (Data)               │  │
│  │  ┌────────────────────┐  │  │    │  │  - Service state                 │  │
│  │  │ Sealed Secrets     │  │  │    │  │  - Transaction logs              │  │
│  │  │ Key Management     │  │  │    │  │  - Audit trails                  │  │
│  │  │ Secure Network     │  │  │    │  └──────────────────────────────────┘  │
│  │  │ Confidential Compute│  │  │    │  ┌──────────────────────────────────┐  │
│  │  │ Attestation        │  │  │    │  │  Auth (Identity)                 │  │
│  │  │ Neo N3 Signing     │  │  │    │  │  - JWT tokens                    │  │
│  │  └────────────────────┘  │  │    │  │  - User/Tenant management        │  │
│  └──────────────────────────┘  │    │  └──────────────────────────────────┘  │
│                                │    │  ┌──────────────────────────────────┐  │
│  NEVER LEAVES ENCLAVE:         │    │  │  Storage (Files)                 │  │
│  - Private keys               │    │  │  - Encrypted blobs               │  │
│  - Decrypted secrets          │    │  └──────────────────────────────────┘  │
│  - Plaintext credentials      │    │  ┌──────────────────────────────────┐  │
│                                │    │  │  Realtime (Events)               │  │
│                                │    │  │  - Service events                │  │
│                                │    │  │  - Blockchain events             │  │
│                                │    │  └──────────────────────────────────┘  │
└────────────────────────────────┘    └────────────────────────────────────────┘
                          ↘                    ↙
                    ┌──────────────────────────────┐
                    │      NEO N3 BLOCKCHAIN       │
                    │  - Smart contracts           │
                    │  - Transaction submission    │
                    │  - Event monitoring          │
                    └──────────────────────────────┘
```

## Core Principles

### 1. TEE as Trust Root

**ALL sensitive operations MUST happen inside the TEE enclave:**

| Operation | Location | Rationale |
|-----------|----------|-----------|
| Secret storage/retrieval | TEE Enclave | Secrets never leave enclave in plaintext |
| Key generation/derivation | TEE Enclave | Private keys never exported |
| Transaction signing | TEE Enclave | Signing happens inside enclave |
| Credential injection | TEE Enclave | API keys injected into requests inside enclave |
| TLS termination | TEE Enclave | Sensitive network traffic decrypted inside enclave |
| Confidential computation | TEE Enclave | User code runs with secrets inside enclave |

### 2. Supabase as Infrastructure

**Supabase provides the data and identity layer:**

| Component | Purpose | Security |
|-----------|---------|----------|
| PostgreSQL | Service state, logs, audit | Row-level security (RLS) |
| Auth | User/tenant identity | JWT tokens verified by TEE |
| Storage | Encrypted file storage | Encryption keys in TEE |
| Realtime | Event streaming | Authenticated subscriptions |

### 3. Android Architecture Pattern

**Services are like Android apps:**

- Declare required capabilities in manifest
- Request permissions from ServiceOS
- Cannot access TEE or Supabase directly
- All access mediated by ServiceOS

## Directory Structure

```
service_layer/
├── tee/                              # Layer 1: Trust Root
│   ├── types/                        # All TEE interfaces
│   │   └── types.go                  # TrustRoot, SecureVault, KeyManager, etc.
│   ├── enclave/                      # Enclave runtime
│   │   ├── runtime.go                # SGX/TDX abstraction
│   │   └── simulation.go             # Simulation mode for development
│   ├── vault/                        # Secret management
│   │   └── vault.go                  # Sealed secret storage
│   ├── keys/                         # Key management
│   │   ├── manager.go                # HD key derivation
│   │   └── neo.go                    # Neo N3 specific key ops
│   ├── network/                      # Secure networking
│   │   └── http.go                   # TLS inside enclave
│   ├── compute/                      # Confidential compute
│   │   └── engine.go                 # JavaScript/WASM execution
│   ├── attestation/                  # Remote attestation
│   │   └── attestor.go               # Quote generation/verification
│   ├── neo/                          # Neo N3 integration (NEW)
│   │   ├── signer.go                 # Transaction signing
│   │   ├── wallet.go                 # Wallet management
│   │   └── rpc.go                    # Secure RPC client
│   ├── bridge/                       # Untrusted I/O
│   │   ├── socket.go                 # Network I/O
│   │   └── storage.go                # Encrypted file I/O
│   └── trust_root.go                 # Main TrustRoot implementation
│
├── infra/                            # Layer 2: Infrastructure (NEW)
│   └── supabase/                     # Supabase integration
│       ├── client.go                 # Base client (TEE-aware)
│       ├── auth.go                   # Auth module
│       ├── database.go               # Database module
│       ├── storage.go                # Storage module
│       ├── realtime.go               # Realtime module
│       └── types.go                  # Supabase types
│
├── platform/                         # Layer 3: ServiceOS
│   ├── os/                           # ServiceOS (Android OS)
│   │   ├── types.go                  # ServiceOS interface, Capabilities
│   │   ├── context.go                # ServiceContext implementation
│   │   ├── events.go                 # Event bus
│   │   └── api/                      # OS APIs
│   │       ├── secrets.go            # SecretsAPI -> TEE Vault
│   │       ├── network.go            # NetworkAPI -> TEE Network
│   │       ├── keys.go               # KeysAPI -> TEE Keys
│   │       ├── compute.go            # ComputeAPI -> TEE Compute
│   │       ├── storage.go            # StorageAPI -> Supabase Storage
│   │       ├── database.go           # DatabaseAPI -> Supabase DB (NEW)
│   │       └── neo.go                # NeoAPI -> TEE Neo (NEW)
│   ├── bootstrap/                    # System bootstrap
│   │   └── foundation.go             # Initialize TEE + Supabase
│   └── runtime/                      # Service runtime
│       └── runtime.go                # Service lifecycle
│
├── services/                         # Layer 4: Services (Android Apps)
│   ├── base/                         # Base components
│   │   ├── service.go                # BaseService
│   │   ├── enclave.go                # BaseEnclave helpers
│   │   └── store.go                  # BaseStore (Supabase-backed)
│   ├── oracle/                       # Oracle Service
│   ├── mixer/                        # Mixer Service
│   ├── secrets/                      # Secrets Service
│   ├── vrf/                          # VRF Service
│   ├── datafeeds/                    # DataFeeds Service
│   ├── gasbank/                      # GasBank Service
│   └── ...                           # Other services
│
├── sealed_store/                     # Sealed data directory
│   └── .gitkeep                      # Encrypted secrets stored here
│
└── cmd/                              # Applications
    ├── server/                       # Main server
    │   └── main.go                   # Entry point
    └── seed_supabase/                # Database seeding
        └── main.go                   # Seed script
```

## Component Details

### TEE Layer (`tee/`)

#### TrustRoot Interface

```go
// TrustRoot is the foundation of all secure operations.
type TrustRoot interface {
    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) error

    // Mode
    Mode() EnclaveMode

    // Core capabilities - ALL sensitive operations
    Vault() SecureVault           // Secret storage
    Network() SecureNetwork       // Secure HTTP/RPC
    Keys() KeyManager             // Key management
    Compute() ConfidentialCompute // Script execution
    Attestation() Attestor        // Remote attestation
    Neo() NeoSigner               // Neo N3 operations (NEW)
}
```

#### SecureVault - Secrets NEVER Leave Enclave

```go
// SecureVault manages secrets inside the TEE enclave.
type SecureVault interface {
    // Store encrypts and stores a secret (sealed to enclave)
    Store(ctx context.Context, namespace, name string, value []byte) error

    // Use executes a function with access to a secret
    // The secret is ONLY available inside the callback
    // This is the ONLY way to access secret values
    Use(ctx context.Context, namespace, name string, fn func(secret []byte) error) error

    // Delete removes a secret
    Delete(ctx context.Context, namespace, name string) error

    // List returns secret names (not values)
    List(ctx context.Context, namespace string) ([]string, error)
}
```

#### NeoSigner - Neo N3 Integration (NEW)

```go
// NeoSigner handles Neo N3 blockchain operations inside the enclave.
type NeoSigner interface {
    // CreateWallet creates a new Neo wallet (keys stay in enclave)
    CreateWallet(ctx context.Context, path string) (WalletHandle, error)

    // ImportWallet imports a wallet from WIF (decrypted inside enclave)
    ImportWallet(ctx context.Context, wif string) (WalletHandle, error)

    // GetAddress returns the Neo address (safe to export)
    GetAddress(ctx context.Context, handle WalletHandle) (string, error)

    // GetPublicKey returns the public key (safe to export)
    GetPublicKey(ctx context.Context, handle WalletHandle) ([]byte, error)

    // SignTransaction signs a Neo transaction (signing inside enclave)
    SignTransaction(ctx context.Context, handle WalletHandle, tx *NeoTransaction) ([]byte, error)

    // SignMessage signs arbitrary data
    SignMessage(ctx context.Context, handle WalletHandle, message []byte) ([]byte, error)

    // InvokeContract invokes a Neo smart contract
    InvokeContract(ctx context.Context, handle WalletHandle, req ContractInvocation) (*InvocationResult, error)
}
```

### Infrastructure Layer (`infra/supabase/`)

#### Supabase Client (TEE-Aware)

```go
// Client is the main Supabase client.
// API keys are stored in TEE and injected into requests inside the enclave.
type Client struct {
    os       serviceos.ServiceOS
    config   Config
    auth     *AuthClient
    database *DatabaseClient
    storage  *StorageClient
    realtime *RealtimeClient
}

// Config for Supabase client.
type Config struct {
    ProjectURL      string
    AnonKeySecret   string // Secret name in TEE vault
    ServiceKeySecret string // Secret name in TEE vault (for admin ops)
}

// New creates a Supabase client.
// API keys are retrieved from TEE vault and never exposed.
func New(os serviceos.ServiceOS, cfg Config) (*Client, error)
```

#### Auth Module

```go
// AuthClient handles Supabase Auth operations.
type AuthClient struct {
    client *Client
}

// SignUp creates a new user.
func (a *AuthClient) SignUp(ctx context.Context, email, password string) (*User, error)

// SignIn authenticates a user.
func (a *AuthClient) SignIn(ctx context.Context, email, password string) (*Session, error)

// VerifyToken verifies a JWT token (verification inside TEE).
func (a *AuthClient) VerifyToken(ctx context.Context, token string) (*Claims, error)

// RefreshToken refreshes an access token.
func (a *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*Session, error)
```

#### Database Module

```go
// DatabaseClient handles Supabase Database operations.
type DatabaseClient struct {
    client *Client
}

// From starts a query on a table.
func (d *DatabaseClient) From(table string) *QueryBuilder

// RPC calls a Postgres function.
func (d *DatabaseClient) RPC(ctx context.Context, fn string, params any) ([]byte, error)

// QueryBuilder for fluent queries.
type QueryBuilder struct {
    // Select, Insert, Update, Delete, Filter, Order, Limit, etc.
}
```

#### Storage Module

```go
// StorageClient handles Supabase Storage operations.
// Files are encrypted with keys from TEE before upload.
type StorageClient struct {
    client *Client
}

// Upload uploads a file (encrypted with TEE key).
func (s *StorageClient) Upload(ctx context.Context, bucket, path string, data []byte) error

// Download downloads a file (decrypted with TEE key).
func (s *StorageClient) Download(ctx context.Context, bucket, path string) ([]byte, error)

// Delete removes a file.
func (s *StorageClient) Delete(ctx context.Context, bucket, path string) error
```

#### Realtime Module

```go
// RealtimeClient handles Supabase Realtime subscriptions.
type RealtimeClient struct {
    client *Client
}

// Subscribe subscribes to database changes.
func (r *RealtimeClient) Subscribe(ctx context.Context, channel string, handler EventHandler) (Subscription, error)

// Broadcast sends a message to a channel.
func (r *RealtimeClient) Broadcast(ctx context.Context, channel string, event string, payload any) error
```

### ServiceOS Layer (`platform/os/`)

#### ServiceOS Interface

```go
// ServiceOS is the main interface for services (like Android Context).
type ServiceOS interface {
    // Identity
    ServiceID() string

    // Capability checking (Android permissions)
    HasCapability(cap Capability) bool
    RequireCapability(cap Capability) error

    // TEE-backed APIs
    Secrets() SecretsAPI       // -> TEE Vault
    Network() NetworkAPI       // -> TEE Network
    Keys() KeysAPI             // -> TEE Keys
    Compute() ComputeAPI       // -> TEE Compute
    Attestation() AttestationAPI // -> TEE Attestation
    Neo() NeoAPI               // -> TEE Neo (NEW)

    // Supabase-backed APIs
    Database() DatabaseAPI     // -> Supabase Database (NEW)
    Storage() StorageAPI       // -> Supabase Storage
    Events() EventsAPI         // -> Supabase Realtime

    // Lifecycle
    Context() context.Context
    Logger() Logger
}
```

#### Capabilities (Android Permissions)

```go
const (
    // TEE capabilities
    CapSecrets     Capability = "secrets"      // Access TEE vault
    CapNetwork     Capability = "network"      // Secure network requests
    CapKeys        Capability = "keys"         // Key management
    CapCompute     Capability = "compute"      // Confidential compute
    CapAttestation Capability = "attestation"  // Remote attestation
    CapNeo         Capability = "neo"          // Neo N3 operations (NEW)

    // Supabase capabilities
    CapDatabase    Capability = "database"     // Database access (NEW)
    CapStorage     Capability = "storage"      // File storage
    CapEvents      Capability = "events"       // Event bus

    // Extended capabilities
    CapNetworkExternal Capability = "network.external"  // External APIs
    CapSecretsWrite    Capability = "secrets.write"     // Write secrets
    CapNeoSign         Capability = "neo.sign"          // Sign transactions
)
```

### Service Layer (`services/`)

#### Service Manifest

```go
// Manifest declares a service's identity and required capabilities.
type Manifest struct {
    ServiceID            string       `json:"service_id"`
    Version              string       `json:"version"`
    Description          string       `json:"description"`
    RequiredCapabilities []Capability `json:"required_capabilities"`
    OptionalCapabilities []Capability `json:"optional_capabilities"`
    AllowedHosts         []string     `json:"allowed_hosts,omitempty"`
    ResourceLimits       ResourceLimits `json:"resource_limits"`
}

// Example: Oracle Service Manifest
func Manifest() *os.Manifest {
    return &os.Manifest{
        ServiceID:   "oracle",
        Version:     "1.0.0",
        Description: "Price oracle service for Neo N3",
        RequiredCapabilities: []os.Capability{
            os.CapSecrets,         // Store API keys
            os.CapNetwork,         // Fetch prices from APIs
            os.CapNeo,             // Submit price updates to chain
            os.CapDatabase,        // Store price history
        },
        AllowedHosts: []string{
            "api.coingecko.com",
            "api.binance.com",
            "seed1.neo.org",
        },
    }
}
```

#### BaseService with Supabase Store

```go
// BaseService provides common functionality for all services.
type BaseService struct {
    id      string
    name    string
    version string
    state   ServiceState
    os      os.ServiceOS
    logger  os.Logger
}

// BaseStore provides Supabase-backed storage for services.
type BaseStore struct {
    os        os.ServiceOS
    tableName string
}

// Create inserts a record.
func (s *BaseStore) Create(ctx context.Context, record any) error {
    return s.os.Database().From(s.tableName).Insert(ctx, record)
}

// FindByID retrieves a record by ID.
func (s *BaseStore) FindByID(ctx context.Context, id string, dest any) error {
    return s.os.Database().From(s.tableName).
        Select("*").
        Eq("id", id).
        Single(ctx, dest)
}

// Update updates a record.
func (s *BaseStore) Update(ctx context.Context, id string, updates any) error {
    return s.os.Database().From(s.tableName).
        Update(ctx, updates).
        Eq("id", id).
        Execute(ctx)
}

// Delete removes a record.
func (s *BaseStore) Delete(ctx context.Context, id string) error {
    return s.os.Database().From(s.tableName).
        Delete(ctx).
        Eq("id", id).
        Execute(ctx)
}
```

## Data Flow Examples

### Example 1: Oracle Price Fetch

```
1. Oracle Service calls os.Network().FetchWithSecret(req, "coingecko_api_key")
2. ServiceOS checks CapNetwork capability
3. ServiceOS delegates to TEE Network
4. TEE Network:
   a. Retrieves "coingecko_api_key" from Vault (inside enclave)
   b. Injects API key into request headers (inside enclave)
   c. Makes HTTPS request (TLS inside enclave)
   d. Returns response to service
5. Oracle Service stores price in os.Database().From("prices").Insert(...)
6. ServiceOS delegates to Supabase Database
7. Price stored in PostgreSQL
```

### Example 2: Neo Transaction Signing

```
1. Mixer Service calls os.Neo().SignTransaction(walletHandle, tx)
2. ServiceOS checks CapNeo and CapNeoSign capabilities
3. ServiceOS delegates to TEE NeoSigner
4. TEE NeoSigner:
   a. Retrieves private key using walletHandle (inside enclave)
   b. Signs transaction (inside enclave)
   c. Returns signature (private key never leaves enclave)
5. Mixer Service broadcasts signed transaction
```

### Example 3: Secret Storage

```
1. Secrets Service calls os.Secrets().Store(ctx, "api_key", value)
2. ServiceOS checks CapSecrets and CapSecretsWrite capabilities
3. ServiceOS delegates to TEE Vault
4. TEE Vault:
   a. Encrypts value with enclave sealing key
   b. Stores encrypted blob to sealed_store/
   c. Records metadata in Supabase (encrypted, no plaintext)
5. Later retrieval uses Use() callback pattern:
   os.Secrets().Use(ctx, "api_key", func(secret []byte) error {
       // Secret only available here, inside enclave
       return nil
   })
```

## Bootstrap Sequence

```go
// cmd/server/main.go
func main() {
    // 1. Load configuration
    cfg := config.Load()

    // 2. Initialize TEE (Trust Root)
    trustRoot, err := tee.New(tee.Config{
        EnclaveID:      cfg.EnclaveID,
        Mode:           cfg.TEEMode,
        SealingKeyPath: cfg.SealingKeyPath,
        StoragePath:    cfg.SealedStorePath,
    })
    if err != nil {
        log.Fatal(err)
    }
    if err := trustRoot.Start(ctx); err != nil {
        log.Fatal(err)
    }

    // 3. Initialize Supabase (with TEE-backed secrets)
    supabase, err := infra.NewSupabase(trustRoot, infra.SupabaseConfig{
        ProjectURL:       cfg.SupabaseURL,
        AnonKeySecret:    "supabase_anon_key",    // Stored in TEE
        ServiceKeySecret: "supabase_service_key", // Stored in TEE
    })
    if err != nil {
        log.Fatal(err)
    }

    // 4. Create Foundation (TEE + Supabase)
    foundation := bootstrap.NewFoundation(trustRoot, supabase)

    // 5. Initialize ServiceOS
    serviceOS := platform.NewServiceOS(foundation)

    // 6. Register and start services
    registry := services.NewRegistry()
    registry.Register(oracle.New(serviceOS))
    registry.Register(mixer.New(serviceOS))
    registry.Register(secrets.New(serviceOS))
    // ... more services

    if err := registry.StartAll(ctx); err != nil {
        log.Fatal(err)
    }

    // 7. Start HTTP server
    server := api.NewServer(serviceOS, registry)
    server.ListenAndServe(cfg.HTTPAddr)
}
```

## Security Guarantees

### What NEVER Leaves the TEE Enclave

1. **Private Keys** - Only KeyHandle/WalletHandle references exported
2. **Decrypted Secrets** - Only accessible via Use() callback
3. **API Credentials** - Injected into requests inside enclave
4. **Plaintext Sensitive Data** - All encryption/decryption inside enclave

### What is Stored in Supabase (Encrypted or Non-Sensitive)

1. **Service State** - Transaction status, job queues
2. **Audit Logs** - Who did what, when
3. **Encrypted Blobs** - Files encrypted with TEE keys
4. **Public Data** - Prices, addresses, public keys

### Attestation Flow

```
1. Client requests attestation from Service
2. Service calls os.Attestation().GenerateQuote(userData)
3. TEE generates SGX/TDX quote
4. Quote includes:
   - MR_ENCLAVE (code measurement)
   - MR_SIGNER (signer measurement)
   - User data (binding to request)
5. Client verifies quote with Intel Attestation Service
6. Client trusts that operations happen inside verified enclave
```

## Migration Guide

### From Old Architecture

1. **Move secrets to TEE Vault**
   ```go
   // Old: secrets in config/env
   apiKey := os.Getenv("API_KEY")

   // New: secrets in TEE
   os.Secrets().Use(ctx, "api_key", func(secret []byte) error {
       // Use secret here
       return nil
   })
   ```

2. **Move database to Supabase**
   ```go
   // Old: direct PostgreSQL
   db.Query("SELECT * FROM prices")

   // New: via ServiceOS
   os.Database().From("prices").Select("*").Execute(ctx, &prices)
   ```

3. **Move signing to TEE**
   ```go
   // Old: keys in memory
   signature := wallet.Sign(tx)

   // New: signing in TEE
   signature, _ := os.Neo().SignTransaction(walletHandle, tx)
   ```

## Conclusion

This architecture ensures:

1. **Security** - All sensitive operations in TEE, secrets never exposed
2. **Scalability** - Supabase handles data layer, TEE handles security
3. **Maintainability** - Clean separation following Android patterns
4. **Auditability** - All operations logged, attestation available
5. **Neo N3 Native** - First-class support for Neo blockchain operations
