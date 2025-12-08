# Neo Service Layer - Functionality Documentation

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Technology Stack](#technology-stack)
3. [Services](#services)
4. [Integration Workflows](#integration-workflows)
5. [Security Model](#security-model)
6. [Deployment Architecture](#deployment-architecture)

---

## Architecture Overview

The Neo Service Layer is a **TEE-Centric (Trusted Execution Environment)** platform built on **MarbleRun + EGo** for confidential computing on the Neo N3 blockchain. It provides oracle services, verifiable randomness, privacy mixing, automated task execution, and price feeds through a secure enclave-based architecture.

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           FRONTEND (Netlify)                                │
│                    React + TypeScript + Tailwind CSS                        │
│                         User Dashboard & API Console                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          API GATEWAY (EGo Enclave)                          │
│                    Authentication, Routing, Rate Limiting                   │
│                         JWT + Neo Wallet Signature Auth                     │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│   VRF Service         │ │   Mixer Service       │ │   DataFeeds Service   │
│   (EGo Enclave)       │ │   (EGo Enclave)       │ │   (EGo Enclave)       │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│   Automation Service  │ │   AccountPool Service │ │   Confidential Service│
│   (EGo Enclave)       │ │   (EGo Enclave)       │ │   (EGo Enclave)       │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│   Supabase            │ │   Neo N3 Blockchain   │ │   MarbleRun           │
│   (PostgreSQL + Auth) │ │   (Smart Contracts)   │ │   Coordinator         │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
```

### Core Design Principles

1. **TEE-First Security**: All sensitive operations execute inside SGX enclaves
2. **Secrets Never Leave Enclave**: Private keys and secrets are managed via MarbleRun Coordinator
3. **Request-Callback Pattern**: On-chain requests trigger off-chain TEE processing with on-chain callbacks
4. **Capability-Based Access**: Services declare required capabilities in their manifest

---

## Technology Stack

### Backend (Go)

| Component | Technology | Purpose |
|-----------|------------|---------|
| TEE Runtime | EGo (Edgeless Systems) | SGX enclave execution |
| Orchestration | MarbleRun | Multi-enclave coordination, secrets management |
| Database | Supabase (PostgreSQL) | Persistent storage, real-time subscriptions |
| HTTP Router | Gorilla Mux | REST API routing |
| Blockchain | Neo N3 | Smart contract interaction |

### Frontend (TypeScript)

| Component | Technology | Purpose |
|-----------|------------|---------|
| Framework | React 18 | UI components |
| Styling | Tailwind CSS | Responsive design |
| State | Zustand | State management |
| Hosting | Netlify | CDN, serverless functions |

### Infrastructure

| Component | Technology | Purpose |
|-----------|------------|---------|
| Secrets | MarbleRun Coordinator | Enclave secret injection |
| TLS | mTLS (MarbleRun) | Inter-service communication |
| Attestation | Intel SGX | Remote attestation |

---

## Services

### 1. VRF Service (Verifiable Random Function)

**Purpose**: Provides cryptographically verifiable random numbers for smart contracts.

**Architecture Pattern**: Request-Callback

#### Workflow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ User Contract│     │ VRF Contract │     │ TEE (VRF Svc)│     │ User Contract│
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ requestRandomness  │                    │                    │
       │ (seed, numWords)   │                    │                    │
       │───────────────────>│                    │                    │
       │                    │                    │                    │
       │                    │ RandomnessRequested│                    │
       │                    │ (event)            │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │                    │                    │ Generate VRF Proof │
       │                    │                    │ (ECDSA P-256)      │
       │                    │                    │                    │
       │                    │                    │ fulfillRandomness  │
       │                    │                    │───────────────────>│
       │                    │                    │                    │
```

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status and statistics |
| `/pubkey` | GET | Get VRF public key |
| `/random` | POST | Direct randomness generation (off-chain) |
| `/verify` | POST | Verify VRF proof |
| `/request` | POST | Create on-chain request |
| `/request/{id}` | GET | Get request status |
| `/requests` | GET | List user's requests |

#### Request/Response Types

```go
// Direct Random Request
type DirectRandomRequest struct {
    Seed     string `json:"seed"`
    NumWords int    `json:"num_words,omitempty"` // Default: 1, Max: 10
}

// Direct Random Response
type DirectRandomResponse struct {
    RequestID   string   `json:"request_id"`
    Seed        string   `json:"seed"`
    RandomWords []string `json:"random_words"`  // 32-byte hex strings
    Proof       string   `json:"proof"`         // VRF proof
    PublicKey   string   `json:"public_key"`    // Compressed P-256 public key
    Timestamp   string   `json:"timestamp"`
}
```

#### Security

- **Private Key**: Injected by MarbleRun Coordinator (`VRF_PRIVATE_KEY`)
- **Algorithm**: ECDSA P-256 with deterministic VRF construction
- **Upgrade Safety**: Key persists across enclave upgrades (MRENCLAVE changes)

---

### 2. Mixer Service (Privacy Mixing)

**Purpose**: Privacy-preserving transaction mixing for Neo N3 tokens.

**Architecture Pattern**: Off-Chain Mixing with TEE Proofs + On-Chain Dispute Only

#### Workflow

**Privacy-First Fee Model**: User NEVER connects to any known service layer account.
Fee is deducted from delivery (user receives NetAmount = TotalAmount - ServiceFee).

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Mixer Service│     │ AccountPool  │     │  Blockchain  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ Request Mix        │                    │                    │
       │ (amount, token)    │                    │                    │
       │───────────────────>│                    │                    │
       │                    │ Lock Pool Account  │                    │
       │                    │───────────────────>│                    │
       │ RequestProof +     │                    │                    │
       │ Deposit Address    │                    │                    │
       │<───────────────────│                    │                    │
       │                    │                    │                    │
       │ Direct Deposit to Pool Account (NOT gasbank)                 │
       │ (anonymous, no service layer link)                           │
       │─────────────────────────────────────────────────────────────>│
       │                    │                    │                    │
       │                    │ Execute Mixing     │                    │
       │                    │ (off-chain hops)   │                    │
       │                    │                    │                    │
       │                    │ Release Account    │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │ NetAmount Delivered│                    │                    │
       │ (Fee Deducted)     │                    │                    │
       │<───────────────────│                    │                    │
       │                    │                    │                    │
       │ [DISPUTE ONLY]     │                    │                    │
       │ Submit Dispute     │                    │                    │
       │───────────────────>│                    │                    │
       │                    │ Submit Proof       │                    │
       │                    │────────────────────────────────────────>│
```

#### Supported Tokens

| Token | Script Hash | Min Amount | Max Amount | Fee Rate |
|-------|-------------|------------|------------|----------|
| GAS | `0xd2a4cff31913016155e38e474a2c06d08be276cf` | 0.001 | 1.0 | 0.5% |
| NEO | `0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5` | 1 | 1000 | 0.5% |

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status and pool statistics |
| `/mix` | POST | Create mix request |
| `/mix/{id}` | GET | Get mix request status |
| `/mix/{id}/deposit` | POST | Confirm deposit |
| `/tokens` | GET | List supported tokens |
| `/dispute/{id}` | POST | Submit dispute (triggers on-chain proof) |

#### Security Features

- **RequestProof**: `Hash256(request) + TEE HMAC signature`
- **CompletionProof**: `Hash256(outputs) + TEE HMAC signature`
- **Dispute Period**: 7 days
- **Compliance Limits**: ≤10,000 per request, ≤100,000 total pool

---

### 3. DataFeeds Service (Price Oracle)

**Purpose**: Aggregated price feeds from multiple sources with TEE attestation.

**Architecture Pattern**: Push/Auto-Update

#### Workflow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Price Sources│     │ DataFeeds Svc│     │ DataFeeds    │     │ User Contract│
│ (Chainlink,  │     │ (TEE)        │     │ Contract     │     │              │
│  Binance)    │     │              │     │              │     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ Fetch Prices       │                    │                    │
       │<───────────────────│                    │                    │
       │                    │                    │                    │
       │ Price Data         │                    │                    │
       │───────────────────>│                    │                    │
       │                    │                    │                    │
       │                    │ Aggregate & Sign   │                    │
       │                    │ (Median + ECDSA)   │                    │
       │                    │                    │                    │
       │                    │ updatePrice()      │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │                    │                    │ getLatestPrice()   │
       │                    │                    │<───────────────────│
       │                    │                    │                    │
       │                    │                    │ Price + Timestamp  │
       │                    │                    │───────────────────>│
```

#### Data Sources

| Source | Type | Priority | Weight |
|--------|------|----------|--------|
| Chainlink (Arbitrum) | On-chain | 1 (Primary) | 3 |
| Binance | HTTP API | 2 (Fallback) | 1 |

#### Supported Feeds

- BTC/USD, ETH/USD, NEO/USD, GAS/USD, NEO/GAS

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service configuration |
| `/price/{pair}` | GET | Get single price |
| `/prices` | GET | Get all prices |
| `/feeds` | GET | List available feeds |
| `/sources` | GET | List data sources |
| `/config` | GET | Get full configuration |

#### Configuration (YAML)

```yaml
update_interval: 60s
feeds:
  - id: "BTC/USD"
    pair: "BTCUSDT"
    base: "btc"
    quote: "usd"
    decimals: 8
    enabled: true
    sources: ["binance", "chainlink"]
sources:
  - id: "binance"
    name: "Binance"
    url: "https://api.binance.com/api/v3/ticker/price?symbol={pair}"
    json_path: "price"
    weight: 1
```

---

### 4. Automation Service (Task Automation)

**Purpose**: Trigger-based task automation for smart contracts.

**Architecture Pattern**: Trigger-Based Execution

#### Trigger Types

| Type | ID | Description | Example |
|------|-----|-------------|---------|
| Time | 1 | Cron expressions | "Every Friday 00:00 UTC" |
| Price | 2 | Price thresholds | "When BTC > $100,000" |
| Event | 3 | On-chain events | "When contract X emits event Y" |
| Threshold | 4 | Balance thresholds | "When balance < 10 GAS" |

#### Workflow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Automation   │     │ Condition    │     │ User Contract│
│              │     │ Service (TEE)│     │ Monitor      │     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ Register Trigger   │                    │                    │
       │ (type, condition,  │                    │                    │
       │  callback)         │                    │                    │
       │───────────────────>│                    │                    │
       │                    │                    │                    │
       │                    │ Monitor Condition  │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │                    │ Condition Met!     │                    │
       │                    │<───────────────────│                    │
       │                    │                    │                    │
       │                    │ Execute Callback   │                    │
       │                    │───────────────────────────────────────>│
       │                    │                    │                    │
```

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/triggers` | GET | List user's triggers |
| `/triggers` | POST | Create trigger |
| `/triggers/{id}` | GET | Get trigger details |
| `/triggers/{id}` | PUT | Update trigger |
| `/triggers/{id}` | DELETE | Delete trigger |
| `/triggers/{id}/enable` | POST | Enable trigger |
| `/triggers/{id}/disable` | POST | Disable trigger |
| `/triggers/{id}/executions` | GET | List executions |

---

### 5. AccountPool Service

**Purpose**: HD-derived pool account management for mixer and other services.

**Architecture Pattern**: Account Lending

#### Features

- HD wallet derivation from master key
- Account locking/unlocking for exclusive use
- Multi-token balance tracking
- Automatic account rotation

#### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Pool statistics |
| `/accounts` | GET | List available accounts |
| `/accounts/lock` | POST | Lock account for use |
| `/accounts/{id}/unlock` | POST | Release account |
| `/accounts/{id}/balance` | GET | Get account balance |

---

### 6. Confidential Service

**Purpose**: Confidential computing for sensitive data processing.

**Architecture Pattern**: Sealed Computation

#### Features

- Data encryption at rest and in transit
- Computation inside SGX enclave
- Result attestation

---

## Integration Workflows

### MarbleRun + EGo Integration

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MarbleRun Coordinator                               │
│                                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Manifest    │  │ Secrets     │  │ TLS Certs   │  │ Attestation │        │
│  │ Validation  │  │ Injection   │  │ Distribution│  │ Verification│        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│   EGo Enclave         │ │   EGo Enclave         │ │   EGo Enclave         │
│   (VRF Service)       │ │   (Mixer Service)     │ │   (Gateway)           │
│                       │ │                       │ │                       │
│ MARBLE_CERT           │ │ MARBLE_CERT           │ │ MARBLE_CERT           │
│ MARBLE_KEY            │ │ MARBLE_KEY            │ │ MARBLE_KEY            │
│ MARBLE_ROOT_CA        │ │ MARBLE_ROOT_CA        │ │ MARBLE_ROOT_CA        │
│ MARBLE_SECRETS        │ │ MARBLE_SECRETS        │ │ MARBLE_SECRETS        │
│ VRF_PRIVATE_KEY       │ │ MIXER_MASTER_KEY      │ │ JWT_SECRET            │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
```

### Enclave Configuration (enclave.json)

```json
{
  "exe": "marble",
  "key": "private.pem",
  "debug": true,
  "heapSize": 512,
  "productID": 1,
  "securityVersion": 1,
  "mounts": [
    {"source": "/etc/ssl/certs", "target": "/etc/ssl/certs", "type": "hostfs", "readOnly": true}
  ],
  "env": [
    {"name": "SERVICE_TYPE", "fromHost": true},
    {"name": "MARBLE_CERT", "fromHost": true},
    {"name": "MARBLE_KEY", "fromHost": true},
    {"name": "MARBLE_ROOT_CA", "fromHost": true},
    {"name": "MARBLE_SECRETS", "fromHost": true},
    {"name": "SUPABASE_URL", "fromHost": true},
    {"name": "SUPABASE_SERVICE_KEY", "fromHost": true}
  ]
}
```

### Supabase Integration

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Supabase                                       │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │
│  │   PostgreSQL    │  │   Auth          │  │   Realtime      │             │
│  │                 │  │                 │  │                 │             │
│  │ - users         │  │ - JWT tokens    │  │ - Subscriptions │             │
│  │ - vrf_requests  │  │ - OAuth         │  │ - Webhooks      │             │
│  │ - mix_requests  │  │ - API keys      │  │                 │             │
│  │ - price_feeds   │  │                 │  │                 │             │
│  │ - triggers      │  │                 │  │                 │             │
│  │ - executions    │  │                 │  │                 │             │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         internal/database                                   │
│                                                                             │
│  supabase_client.go      - REST API client                                  │
│  supabase_repository.go  - Data access layer                                │
│  supabase_models.go      - Data models                                      │
│  supabase_vrf.go         - VRF-specific queries                             │
│  supabase_automation.go  - Automation-specific queries                      │
│  mixer.go                - Mixer-specific queries                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Netlify Frontend Deployment

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Netlify                                        │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │
│  │   CDN           │  │   Build         │  │   Functions     │             │
│  │                 │  │                 │  │   (Optional)    │             │
│  │ - Static assets │  │ - Vite build    │  │ - API proxy     │             │
│  │ - SPA routing   │  │ - TypeScript    │  │ - Auth helpers  │             │
│  │ - HTTPS         │  │ - Tailwind      │  │                 │             │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Security Model

### TEE Security Guarantees

1. **Confidentiality**: Code and data inside enclave are encrypted
2. **Integrity**: Tampering is detected via hardware attestation
3. **Attestation**: Remote parties can verify enclave identity

### Secret Management

| Secret | Service | Injection Method |
|--------|---------|------------------|
| VRF_PRIVATE_KEY | VRF | MarbleRun Coordinator |
| MIXER_MASTER_KEY | Mixer | MarbleRun Coordinator |
| POOL_MASTER_KEY | AccountPool | MarbleRun Coordinator |
| DATAFEEDS_SIGNING_KEY | DataFeeds | MarbleRun Coordinator |
| JWT_SECRET | Gateway | MarbleRun Coordinator |

### Authentication Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │   Gateway    │     │   Supabase   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ 1. Get Nonce       │                    │
       │───────────────────>│                    │
       │                    │                    │
       │ 2. Sign with       │                    │
       │    Neo Wallet      │                    │
       │                    │                    │
       │ 3. Login           │                    │
       │ (address, sig,     │                    │
       │  pubkey)           │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ 4. Verify Sig      │
       │                    │ 5. Create/Get User │
       │                    │───────────────────>│
       │                    │                    │
       │ 6. JWT Token       │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ 7. API Request     │                    │
       │ (Bearer token)     │                    │
       │───────────────────>│                    │
```

---

## Deployment Architecture

### Production Deployment

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Production Environment                              │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    SGX-Enabled Kubernetes Cluster                    │   │
│  │                                                                      │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│  │  │ MarbleRun    │  │ Gateway Pod  │  │ Service Pods │               │   │
│  │  │ Coordinator  │  │ (EGo)        │  │ (EGo)        │               │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│  │                                                                      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │
│  │ Supabase Cloud  │  │ Neo N3 Mainnet  │  │ Netlify CDN     │             │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| SUPABASE_URL | Supabase project URL | Yes |
| SUPABASE_SERVICE_KEY | Supabase service role key | Yes |
| NEO_RPC_URL | Neo N3 RPC endpoint | Yes |
| PORT | Service port | No (default: 8080) |
| SERVICE_TYPE | Service identifier | Yes |

---

## API Gateway Routes

### Public Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/health` | GET | Health check |
| `/attestation` | GET | SGX attestation report |
| `/api/v1/auth/nonce` | POST | Get auth nonce |
| `/api/v1/auth/register` | POST | Register user |
| `/api/v1/auth/login` | POST | Login with Neo wallet |
| `/api/v1/auth/google` | GET | Google OAuth |
| `/api/v1/auth/github` | GET | GitHub OAuth |

### Protected Routes (Require JWT)

| Route | Method | Description |
|-------|--------|-------------|
| `/api/v1/me` | GET | Get user profile |
| `/api/v1/apikeys` | GET/POST | Manage API keys |
| `/api/v1/wallets` | GET/POST | Manage wallets |
| `/api/v1/gasbank/*` | * | Gas bank operations |
| `/api/v1/vrf/*` | * | VRF service proxy |
| `/api/v1/mixer/*` | * | Mixer service proxy |
| `/api/v1/datafeeds/*` | * | DataFeeds service proxy |
| `/api/v1/automation/*` | * | Automation service proxy |
| `/api/v1/confidential/*` | * | Confidential service proxy |

---

## Version Information

| Service | Version | Status |
|---------|---------|--------|
| VRF Service | 2.0.0 | Production |
| Mixer Service | 3.2.0 | Production |
| DataFeeds Service | 3.0.0 | Production |
| Automation Service | 2.0.0 | Production |
| AccountPool Service | 1.0.0 | Production |
| Confidential Service | 1.0.0 | Beta |

---

## References

- [MarbleRun Documentation](https://docs.edgeless.systems/marblerun)
- [EGo Documentation](https://docs.edgeless.systems/ego)
- [Neo N3 Documentation](https://docs.neo.org/)
- [Supabase Documentation](https://supabase.com/docs)
