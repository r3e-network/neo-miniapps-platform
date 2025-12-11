# Architecture Documentation

## Overview

The Neo Service Layer is a production-ready, TEE-protected service platform built on:

- **MarbleRun**: NeoCompute computing orchestration
- **EGo**: Go MarbleRun TEE runtime
- **Supabase**: PostgreSQL database with RLS
- **Netlify**: Frontend hosting

## Core Principles

### 1. Defense in Depth

Every layer provides security:
- Network: mTLS between all services
- Compute: MarbleRun TEEs for all code execution
- Storage: Encrypted at rest, RLS policies
- Secrets: Never leave TEE memory

### 2. Zero Trust Architecture

- All services authenticate via MarbleRun attestation
- No implicit trust between components
- Secrets injected only after attestation

### 3. Minimal Attack Surface

- Services have minimal capabilities
- Network access restricted by manifest
- File system access limited to memfs

## Component Details

### MarbleRun Coordinator

The Coordinator is the trust anchor:

```
┌─────────────────────────────────────┐
│         COORDINATOR                  │
│                                      │
│  ┌─────────────┐  ┌─────────────┐   │
│  │  Manifest   │  │   Secrets   │   │
│  │   Store     │  │    Store    │   │
│  └─────────────┘  └─────────────┘   │
│                                      │
│  ┌─────────────┐  ┌─────────────┐   │
│  │ Attestation │  │    PKI      │   │
│  │   Engine    │  │   Manager   │   │
│  └─────────────┘  └─────────────┘   │
└─────────────────────────────────────┘
```

**Responsibilities:**
- Verify Marble attestation quotes
- Inject secrets based on manifest
- Issue TLS certificates
- Maintain cluster state

### Marble SDK

Each service uses the Marble SDK:

```go
type Marble struct {
    // Identity
    marbleType string
    uuid       string

    // TLS credentials from Coordinator
    cert       tls.Certificate
    rootCA     *x509.CertPool
    tlsConfig  *tls.Config

    // Secrets injected by Coordinator
    secrets    map[string][]byte

    // TEE report
    report     *enclave.Report
}
```

**Key Features:**
- Automatic TLS configuration
- Secret access via callback pattern
- TEE self-report for attestation

### Service Architecture

Each service follows a consistent pattern:

```
┌─────────────────────────────────────┐
│           SERVICE                    │
│                                      │
│  ┌─────────────────────────────┐    │
│  │        HTTP Router          │    │
│  └─────────────────────────────┘    │
│              │                       │
│  ┌───────────┴───────────┐          │
│  │                       │          │
│  ▼                       ▼          │
│  ┌─────────┐      ┌─────────┐       │
│  │ Handler │      │ Handler │       │
│  └─────────┘      └─────────┘       │
│       │                │            │
│       ▼                ▼            │
│  ┌─────────────────────────────┐    │
│  │      Core Logic             │    │
│  │  (Crypto, Business Logic)   │    │
│  └─────────────────────────────┘    │
│              │                       │
│              ▼                       │
│  ┌─────────────────────────────┐    │
│  │      Database Layer         │    │
│  │      (Supabase)             │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

## Security Model

### Attestation Flow

```
1. Marble starts inside EGo TEE
2. Marble generates attestation quote
3. Marble connects to Coordinator
4. Coordinator verifies quote against manifest
5. Coordinator injects secrets and certificates
6. Marble begins serving requests
```

### Secret Management

Secrets follow the "Use" callback pattern:

```go
// Secrets never leave TEE memory
err := marble.UseSecret("API_KEY", func(secret []byte) error {
    // Use secret here
    // Automatically zeroed after callback
    return nil
})
```

### Network Security

- All inter-service communication uses mTLS
- Certificates auto-provisioned by MarbleRun
- External TLS terminates inside TEE

## Data Flow

### Request Flow

```
Client → Gateway → Service → Database
   │        │         │         │
   │        │         │         └── RLS enforced
   │        │         └── TEE protected
   │        └── JWT validated
   └── HTTPS
```

### Secret Flow

```
Manifest → Coordinator → Marble → Memory
    │           │           │        │
    │           │           │        └── Zeroed after use
    │           │           └── Decrypted in TEE
    │           └── Encrypted at rest
    └── Defines access
```

## Upgrade Safety

TEE upgrades (changing MRENCLAVE) must NOT affect business keys. All cryptographic keys remain stable across upgrades.

### Design Principles

1. **Global Master Keys from MarbleRun** - All business keys derived from manifest-defined secrets
2. **No TEE Identity in Derivation** - HKDF context uses only business identifiers (account IDs, service names)
3. **No MarbleRun Sealing Keys** - Application never uses `sgx_seal_data()` for business data
4. **Manifest-Defined Persistence** - Secrets defined in manifest persist across TEE versions

### Key Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MarbleRun Coordinator                         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  manifest.json                                           │    │
│  │  - VRF_PRIVATE_KEY (persist across upgrades)            │    │
│  │  - MIXER_MASTER_KEY (persist across upgrades)           │    │
│  │  - DATAFEEDS_SIGNING_KEY                                │    │
│  └─────────────────────────────────────────────────────────┘    │
└───────────────────────────┬─────────────────────────────────────┘
                            │ Secret Injection (after attestation)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TEE TEE (Marble)                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Marble.Secret("VRF_PRIVATE_KEY")      → VRF signing      │  │
│  │  Marble.Secret("MIXER_MASTER_KEY")     → Pool derivation  │  │
│  │  Marble.Secret("DATAFEEDS_SIGNING_KEY") → Price signing   │  │
│  └───────────────────────────────────────────────────────────┘  │
│                            │                                     │
│                            ▼                                     │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  HKDF Derivation (no TEE identity!)                   │  │
│  │  DeriveKey(masterKey, accountID, "neovault-account", 32)     │  │
│  │                    ↓                                       │  │
│  │  Derived keys ONLY depend on:                              │  │
│  │  - Master key (from MarbleRun)                             │  │
│  │  - Business identifiers (account ID, service name)         │  │
│  │  NOT on: MRENCLAVE, MRSIGNER, sealing keys                │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### HKDF Key Derivation

The `DeriveKey` function uses HKDF-SHA256 without any TEE identity:

```go
func DeriveKey(masterKey []byte, salt []byte, info string, keyLen int) ([]byte, error) {
    hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(info))
    key := make([]byte, keyLen)
    _, err := io.ReadFull(hkdfReader, key)
    return key, err
}
```

| Parameter | Source | Upgrade Impact |
|-----------|--------|----------------|
| masterKey | MarbleRun injection | Stable (manifest-defined) |
| salt | Business identifier (accountID) | Stable |
| info | Service name ("neovault-account") | Stable |
| keyLen | Constant (32) | Stable |

### Service Key Sources

| Service | Key | Source | Derivation |
|---------|-----|--------|------------|
| VRF | Private Key | `Marble.Secret("VRF_PRIVATE_KEY")` | Direct use |
| NeoVault | Pool Keys | `Marble.Secret("MIXER_MASTER_KEY")` | HKDF with accountID |
| NeoFeeds | Signing Key | `Marble.Secret("DATAFEEDS_SIGNING_KEY")` | Direct use |
| NeoFlow | Signing Key | `Marble.Secret("AUTOMATION_SIGNING_KEY")` | Direct use |
| TLS | Certificates | MarbleRun PKI | Auto-provisioned |

### Upgrade Process

```
1. Build new TEE binary (new MRENCLAVE)
2. Update manifest.json with new MRENCLAVE/MRSIGNER
3. Deploy new Marble instances
4. Coordinator verifies new attestation
5. Same secrets injected → Same derived keys
6. Service continues with identical cryptographic identity
```

### What Breaks Upgrade Safety

| Operation | Risk Level | Impact |
|-----------|------------|--------|
| Using `sgx_seal_data()` for business keys | CRITICAL | Keys lost on upgrade |
| Including MRENCLAVE in HKDF context | CRITICAL | Keys change on upgrade |
| Hardcoding keys in TEE binary | HIGH | Keys change on rebuild |
| Using TEE report fields in derivation | HIGH | Keys change on upgrade |

## Deployment

### Simulation Mode

For development without MarbleRun hardware:

```bash
OE_SIMULATION=1 docker compose up
```

### Production Mode

With MarbleRun hardware:

```bash
OE_SIMULATION=0 docker compose up
```

### Kubernetes

MarbleRun supports Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  template:
    spec:
      containers:
      - name: gateway
        image: service-layer/gateway
        env:
        - name: EDG_MARBLE_COORDINATOR_ADDR
          value: "coordinator:2001"
        - name: EDG_MARBLE_TYPE
          value: "gateway"
```

## Monitoring

### Health Checks

Each service exposes `/health`:

```json
{
  "status": "healthy",
  "service": "gateway",
  "version": "1.0.0",
  "enclave": true,
  "timestamp": "2024-12-05T12:00:00Z"
}
```

### Attestation Endpoint

Gateway exposes `/attestation`:

```json
{
  "enclave": true,
  "security_version": 1,
  "debug": false
}
```

## Smart Contract Integration

The Service Layer integrates with Neo N3 smart contracts for on-chain operations.

### Service Patterns

The Service Layer supports three different service patterns:

| Pattern | Services | Description |
|---------|----------|-------------|
| **Request-Response** | VRF, NeoVault, NeoCompute | User initiates request → TEE processes → Callback |
| **Push (Auto-Update)** | NeoFeeds | TEE periodically updates on-chain data, no user request needed |
| **Trigger-Based** | NeoFlow | User registers trigger → TEE monitors conditions → Periodic callbacks |

### Pattern 1: Request-Response Flow

The following diagram shows the complete flow from User to Callback:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           REQUEST FLOW (Steps 1-4)                            │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐ │
│  │ User │───►│ User Contract │───►│ ServiceLayerGateway │───►│  Service   │ │
│  └──────┘    │               │    │     (Gateway)       │    │  Contract  │ │
│     1        │ RequestPrice()│    │  RequestService()   │    │ OnRequest()│ │
│              └───────────────┘    └─────────────────────┘    └─────┬──────┘ │
│                     2                       3                      4 │      │
│                                                                      ▼      │
│                                                              ┌────────────┐ │
│                                                              │   Event    │ │
│                                                              │ (on-chain) │ │
│                                                              └─────┬──────┘ │
└────────────────────────────────────────────────────────────────────┼────────┘
                                                                     │
┌────────────────────────────────────────────────────────────────────┼────────┐
│                        SERVICE LAYER (Off-chain TEE)               │        │
├────────────────────────────────────────────────────────────────────┼────────┤
│                                                                    ▼        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Service Layer (TEE TEE)                       │   │
│  │  5. Monitor blockchain events                                        │   │
│  │  6. Process request (HTTP fetch / VRF compute / Mix execution)       │   │
│  │  7. Sign result with TEE private key                                 │   │
│  └──────────────────────────────────┬──────────────────────────────────┘   │
│                                     │                                       │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┼───────────────────────────────────────┐
│                        CALLBACK FLOW (Steps 8-11)                │          │
├─────────────────────────────────────┼───────────────────────────────────────┤
│                                     ▼                                       │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐│
│  │ User │◄───│ User Contract │◄───│ ServiceLayerGateway │◄───│  Service   ││
│  └──────┘    │               │    │     (Gateway)       │    │  Contract  ││
│    11        │   Callback()  │    │  FulfillRequest()   │    │ OnFulfill()││
│              └───────────────┘    └─────────────────────┘    └────────────┘│
│                    10                       9                      8        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Step-by-Step Flow

| Step | Component | Method | Description |
|------|-----------|--------|-------------|
| 1 | User | - | User initiates transaction to their contract |
| 2 | User Contract | `RequestPrice()` | Builds payload, calls Gateway |
| 3 | ServiceLayerGateway | `RequestService()` | Validates request, charges fee, routes to service |
| 4 | Service Contract | `OnRequest()` | Stores request data, emits service-specific event |
| 5 | Service Layer (TEE) | - | Monitors blockchain for events |
| 6 | Service Layer (TEE) | - | Processes request off-chain (HTTP/VRF/Mix) |
| 7 | Service Layer (TEE) | - | Signs result with TEE private key |
| 8 | Service Contract | `OnFulfill()` | Receives fulfillment, cleans up request data |
| 9 | ServiceLayerGateway | `FulfillRequest()` | Verifies TEE signature, updates request status |
| 10 | User Contract | `Callback()` | Receives result, updates application state |
| 11 | User | - | Transaction confirmed on blockchain |

### Contract Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Contract                            │
│                    (ExampleConsumer)                             │
│  • RequestPrice()      • RequestRandom()     • OnServiceCallback │
└─────────────────────────┬───────────────────────────────────────┘
                          │ RequestService()
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   ServiceLayerGateway                            │
│  • Fee Management      • TEE Account Management                 │
│  • Service Registry    • Request Routing                        │
│  • Callback Execution  • Replay Protection (Nonce)              │
└─────────────────────────┬───────────────────────────────────────┘
                          │ OnRequest()
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Service Contracts                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │   VRF    │ │  NeoVault   │ │ NeoFeeds│ │NeoFlow│           │
│  │ Service  │ │ Service  │ │ Service  │ │ Service  │           │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘           │
└───────┼────────────┼────────────┼────────────┼──────────────────┘
        │            │            │            │
        └────────────┴────────────┴────────────┘
                          │ Events (VRFRequest, NeoVaultRequest, etc.)
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TEE (Service Layer)                           │
│  • Event Listening   • Request Processing   • Signed Callbacks  │
└─────────────────────────┬───────────────────────────────────────┘
                          │ FulfillRequest()
                          ▼
                   ServiceLayerGateway
                          │ Callback
                          ▼
                       User Contract
```

### Pattern 2: Push / Auto-Update (NeoFeeds)

NeoFeeds service automatically updates on-chain price data without user requests:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SERVICE LAYER (TEE) - Continuous Loop                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  1. Fetch prices from multiple sources (Binance, Coinbase, etc.)    │   │
│  │  2. Aggregate and validate data (median, outlier removal)           │   │
│  │  3. Sign aggregated price with TEE key                              │   │
│  │  4. Submit to NeoFeedsService contract periodically                │   │
│  └──────────────────────────────────┬──────────────────────────────────┘   │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │ UpdatePrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      NeoFeedsService Contract                               │
│  • Stores latest prices (BTC/USD, ETH/USD, NEO/USD, GAS/USD, etc.)         │
│  • Verifies TEE signature                                                   │
│  • Emits PriceUpdated event                                                 │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │ getLatestPrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         User Contracts (Read Only)                           │
│  • DeFi protocols read prices directly                                      │
│  • No callback needed - just query current price                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Pattern 3: Trigger-Based (NeoFlow)

Users register triggers, TEE monitors conditions and invokes callbacks periodically:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      TRIGGER REGISTRATION (One-time)                         │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐│
│  │ User │───►│ User Contract │───►│ ServiceLayerGateway │───►│ NeoFlow ││
│  └──────┘    │               │    │  RequestService()   │    │  Service   ││
│              │RegisterTrigger│    └─────────────────────┘    │ OnRequest()││
│              └───────────────┘                               └─────┬──────┘│
│                                                                    │       │
│  Trigger Types:                                                    ▼       │
│  • Time-based: "Every Friday 00:00 UTC"                    ┌────────────┐  │
│  • Price-based: "When BTC > $100,000"                      │  Trigger   │  │
│  • Event-based: "When contract X emits event Y"            │ Registered │  │
│                                                            └────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┼───────────────────────────────────────┐
│              SERVICE LAYER (TEE) - Continuous Monitoring    │               │
├─────────────────────────────────────┼───────────────────────────────────────┤
│                                     ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Loop: Check all registered triggers                                 │   │
│  │  • Time triggers: Compare current time                               │   │
│  │  • Price triggers: Check NeoFeeds prices                            │   │
│  │  • Event triggers: Monitor blockchain events                         │   │
│  │  When condition met → Execute callback                               │   │
│  └──────────────────────────────────┬──────────────────────────────────┘   │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │ Condition Met
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         CALLBACK EXECUTION (Periodic)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐│
│  │ User │◄───│ User Contract │◄───│ ServiceLayerGateway │◄───│ NeoFlow ││
│  └──────┘    │   Callback()  │    │  FulfillRequest()   │    │  Service   ││
│              │ (e.g. rebase) │    └─────────────────────┘    └────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
```

**NeoFlow Trigger Examples:**

| Trigger Type | Example | Use Case |
|--------------|---------|----------|
| Time-based | `cron: "0 0 * * FRI"` | Weekly token distribution |
| Price-based | `price: BTC > 100000` | Auto-sell when price target hit |
| Threshold | `balance < 10 GAS` | Auto-refill gas bank |
| Event-based | `event: LiquidityAdded` | React to on-chain events |

### NeoVault Service (v3.0 - Off-Chain First)

The NeoVault implements an **Off-Chain Mixing with On-Chain Dispute** pattern:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│         MIXER FLOW (Privacy-First, Off-Chain First, Dispute Only On-Chain)   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  PRIVACY-FIRST FEE MODEL:                                                    │
│  User NEVER connects to any known service layer account.                     │
│  Fee is deducted from delivery (user receives NetAmount = TotalAmount - Fee) │
│                                                                              │
│  1. USER → CLI/API → MIXER SERVICE (direct, NO on-chain)                    │
│     └── Returns: RequestProof + Deposit Address (anonymous pool account)     │
│                                                                              │
│  2. USER → DIRECT DEPOSIT to Pool Account on-chain                           │
│     └── NOT gasbank, NOT any known service layer address                     │
│     └── Anonymous pool account (no public association with service layer)    │
│                                                                              │
│  3. MIXER SERVICE (TEE) processes off-chain:                                 │
│     ├── Split funds across AccountPool-managed HD-derived accounts           │
│     ├── Random mixing transactions between pool accounts                     │
│     └── Deliver NetAmount to target addresses (fee deducted)                 │
│                                                                              │
│  4. FEE HANDLING: ServiceFee stays in pool accounts                          │
│     └── No explicit fee transfer to any known address                        │
│                                                                              │
│  5. MIXER SERVICE generates CompletionProof (stored, NOT submitted)          │
│     └── CompletionProof = outputsHash + outputTxIDs + TEE signature          │
│                                                                              │
│  6. NORMAL PATH: User happy, ZERO on-chain link to service layer             │
│                                                                              │
│  7. DISPUTE PATH (if mix not done within 7 days):                            │
│     └── User calls /dispute → NeoVault submits CompletionProof on-chain         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Design Principles:**
- **Privacy-First**: User NEVER transacts with any known service layer account
- **Implicit Fee**: Fee deducted from delivery, no separate fee transaction
- **Off-Chain First**: Normal flow has ZERO on-chain link between user and service layer
- **Proof System**: Both RequestProof (for user) and CompletionProof (for dispute)
- **Privacy by Default**: On-chain data only exposed during dispute
- **7-Day Deadline**: User can claim refund via dispute contract if not completed
- Pool accounts are **standard single-sig addresses** (identical to ordinary users)
- **AccountPool-owned keys**: AccountPool service derives/holds pool keys; neovault only locks/updates balances via API

**NeoVault API Endpoints:**
```
POST /request              - Create mix request, returns RequestProof
POST /request/{id}/deposit - Confirm deposit, start mixing
GET  /request/{id}         - Get request details
GET  /request/{id}/proof   - Get CompletionProof (after delivery)
POST /request/{id}/dispute - Submit dispute (ONLY on-chain call)
```

**Proofs:**

| Proof | When Generated | Purpose | On-Chain? |
|-------|---------------|---------|-----------|
| RequestProof | On request creation | User can prove they requested mix | Only if disputed |
| CompletionProof | On delivery | TEE can prove mix was completed | Only if disputed |

### Event Listening

The TEE listens for contract events using the chain package (for services that use on-chain requests):

```go
listener := chain.NewEventListener(chain.ListenerConfig{
    Client:    client,
    Contracts: contractAddresses,
    StartBlock: startBlock,
})

listener.On("VRFRequest", func(event *chain.ContractEvent) error {
    req, _ := chain.ParseVRFRequestEvent(event)
    // Process VRF request...
    return nil
})

// Note: NeoVault requests are now handled directly via API, not on-chain events
// The neovault uses the off-chain-first pattern with dispute-only on-chain

listener.Start(ctx)
```

## Future Enhancements

1. **Multi-region deployment** with geo-distributed Coordinators
2. **Hardware key management** integration (HSM)
3. **Audit logging** with tamper-proof storage
4. **Rate limiting** per user/API key
5. **WebSocket support** for real-time updates
6. **Cross-chain support** via CCIP integration

## Database Architecture

The Service Layer uses Supabase (PostgreSQL) for persistent storage with a modular, service-specific repository pattern.

### Repository Pattern

Following the **Interface Segregation Principle (ISP)**, each service has its own database package:

```
internal/database/           # Shared database infrastructure
├── supabase_client.go       # Supabase HTTP client
├── supabase_repository.go   # Base repository implementation
├── repository_interface.go  # Core interface definitions
├── supabase_models.go       # Shared model definitions (User, APIKey, etc.)
└── mock_repository.go       # Test mocks

services/*/supabase/         # Service-specific database operations
├── repository.go            # Service-specific repository interface
└── models.go                # Service-specific data models
```

### Service-Specific Packages

| Service | Package | Models |
|---------|---------|--------|
| VRF | `services/vrf/supabase` | `Request`, `Response` |
| NeoVault | `services/neovault/supabase` | `Request`, `MixOperation` |
| NeoFlow | `services/neoflow/supabase` | `Trigger`, `Execution` |
| AccountPool | `services/accountpool/supabase` | `Account`, `Lock` |
| Secrets | `services/secrets/supabase` | `Secret`, `AllowedService` |

### Usage Pattern

```go
import (
    "github.com/R3E-Network/service_layer/internal/database"
    vrfsupabase "github.com/R3E-Network/service_layer/services/vrf/supabase"
)

// Create base repository
client, _ := database.NewClient(config)
baseRepo := database.NewRepository(client)

// Create service-specific repository
vrfRepo := vrfsupabase.NewRepository(baseRepo)

// Use service-specific methods
err := vrfRepo.Create(ctx, &vrfsupabase.Request{...})
req, err := vrfRepo.GetByRequestID(ctx, "vrf-123")
```

### Design Benefits

1. **Interface Segregation**: Services only depend on interfaces they use
2. **Encapsulation**: Service-specific models stay within service boundaries
3. **Testability**: Each service can mock its own repository interface
4. **Maintainability**: Changes to one service don't affect others

## Off-Chain Fee Management

All fee management has moved from on-chain smart contracts to the off-chain Supabase-based system for better flexibility and lower transaction costs.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    User Deposit Flow                             │
├─────────────────────────────────────────────────────────────────┤
│  1. User sends GAS to Service Layer deposit address             │
│  2. TEE monitors blockchain for deposit transactions            │
│  3. Upon confirmation, user's Supabase balance is credited      │
│  4. User can now use services (fees deducted from balance)      │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Sponsor Payment Flow                          │
├─────────────────────────────────────────────────────────────────┤
│  User A (Sponsor) → PayForContract(contract_address, amount)    │
│                   → PayForUser(user_B, amount)                  │
│                                                                  │
│  Result: Contract/User B's balance credited, Sponsor debited    │
└─────────────────────────────────────────────────────────────────┘
```

### Database Tables

| Table | Purpose |
|-------|---------|
| `gasbank_accounts` | User/contract balances (balance, reserved) |
| `gasbank_transactions` | Transaction history (deposits, fees, sponsors) |
| `deposit_requests` | Pending deposit confirmations |

### Transaction Types

| Type | Description |
|------|-------------|
| `deposit` | GAS deposited to account |
| `withdraw` | GAS withdrawn from account |
| `service_fee` | Fee charged for service usage |
| `refund` | Service fee refunded (on failure) |
| `sponsor` | Sponsor payment debit |
| `sponsor_credit` | Sponsor payment credit |

### CLI Usage

```bash
# Check balance
slcli balance <user_id>

# Credit deposit (admin)
slcli deposit <user_id> <tx_hash> <amount>

# Pay for a contract's fees
slcli pay-contract <user_id> <contract_address> <amount>

# Pay for another user's fees
slcli pay-user <user_id> <recipient_user_id> <amount>

# View transaction history
slcli transactions <user_id> [limit]

# List service fees
slcli fees
```

### Migration from On-Chain Fees

The on-chain fee methods in `GatewayContract` are deprecated:

| Deprecated Method | Replacement |
|-------------------|-------------|
| `GatewayContract.GetServiceFee()` | `gasbank.GetServiceFee()` |
| `GatewayContract.BalanceOf()` | `gasbank.Manager.GetBalance()` |
| `ServiceRequest.Fee` | Managed via `gasbank_transactions` |

Smart contracts should no longer handle fee logic. All fee operations are managed by the Service Layer via Supabase.
