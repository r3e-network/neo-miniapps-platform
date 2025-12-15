# Neo Service Layer

A production-ready, TEE-protected service layer for Neo N3 blockchain, built with **MarbleRun**, **EGo**, **Supabase**, and **Vercel**.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         VERCEL (Frontend)                                    │
│                    React + TypeScript + Vite + TailwindCSS                  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │ HTTPS
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      MARBLERUN COORDINATOR                                   │
│  • Manifest-based topology verification                                      │
│  • Remote attestation for all Marbles                                       │
│  • Secrets injection & certificate management                               │
│  • Single attestation statement for entire cluster                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │ mTLS (auto-provisioned)
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│   GATEWAY MARBLE    │ │  SERVICE MARBLES    │ │   WORKER MARBLES    │
│     (EGo TEE)       │ │    (EGo TEEs)       │ │     (EGo TEEs)      │
│                     │ │                     │ │                     │
│ • API Gateway       │ │ • NeoOracle         │ │ • NeoFlow Jobs      │
│ • Auth/JWT          │ │ • NeoRand (VRF)     │ │ • NeoFeeds Push     │
│ • Rate Limiting     │ │ • NeoVault          │ │ • Event Processing  │
│ • Request Routing   │ │ • NeoStore          │ │                     │
│ • GasBank (via DB)  │ │ • NeoFeeds          │ │                     │
│                     │ │ • NeoFlow           │ │                     │
│                     │ │ • NeoCompute        │ │                     │
│                     │ │ • NeoAccounts       │ │                     │
└─────────────────────┘ └─────────────────────┘ └─────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SUPABASE (Database)                                  │
│  • PostgreSQL with Row Level Security                                       │
│  • Real-time subscriptions                                                  │
│  • Auth (backup, primary is TEE-based)                                      │
│  • Storage for encrypted blobs                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Node.js 20+
- EGo SDK (for MarbleRun development)
- MarbleRun CLI

### Development (Simulation Mode)

```bash
# Clone the repository
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer

# Start services in simulation mode
make docker-up

# (Optional) Re-apply the MarbleRun manifest
make marblerun-manifest

# Start frontend development server
make frontend-dev
```

### Production (MarbleRun Hardware)

```bash
# Build with EGo
make build-ego

# Sign enclave binaries
make sign-enclaves

# Start with MarbleRun hardware
make docker-up-tee
```

## 📦 Services

| Service         | Description                          | Port |
| --------------- | ------------------------------------ | ---- |
| **Gateway**     | API Gateway with JWT auth            | 8080 |
| **NeoOracle**   | External data fetching (allowlisted) | 8088 |
| **NeoRand**     | Verifiable randomness (VRF)          | 8081 |
| **NeoVault**    | Privacy-preserving transactions      | 8082 |
| **NeoStore**    | Secrets + encrypted data management  | 8087 |
| **NeoFeeds**    | Price feed aggregation               | 8083 |
| **NeoFlow**     | Task automation                      | 8084 |
| **NeoCompute**  | Confidential computation             | 8086 |
| **NeoAccounts** | Account pool / key management        | 8085 |

### Internal Services

| Service         | Description                                                                          |
| --------------- | ------------------------------------------------------------------------------------ |
| **NeoAccounts** | Shared account pool service (owns HD keys; other services request/lock/sign via API) |

## 📜 Smart Contracts

Neo N3 smart contracts for on-chain service integration:

```
contracts/
├── gateway/                # ServiceLayerGateway contract
├── common/                 # Shared contract utilities
├── examples/               # Example consumer contracts
└── build.sh                # Build script (nccs)
services/
├── neooracle/contract/     # NeoOracleService contract
├── neorand/contract/       # NeoRandService contract
├── neovault/contract/      # NeoVaultService contract
├── neofeeds/contract/      # NeoFeedsService contract
├── neoflow/contract/       # NeoFlowService contract
└── neocompute/contract/    # NeoComputeService contract
```

### Contract Workflow

The Service Layer supports three different service patterns:

| Pattern                | Services                          | Description                                                           |
| ---------------------- | --------------------------------- | --------------------------------------------------------------------- |
| **Request-Response**   | Oracle, VRF, NeoVault, NeoCompute | User initiates request → TEE processes → Callback                     |
| **Push (Auto-Update)** | NeoFeeds                          | TEE periodically updates on-chain data, no user request needed        |
| **Trigger-Based**      | NeoFlow                           | User registers trigger → TEE monitors conditions → Periodic callbacks |

#### Pattern 1: Request-Response (Oracle, VRF, NeoVault)

Complete request flow from User to Callback:

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

**Step-by-Step Flow:**

| Step | Component           | Action             | Description                               |
| ---- | ------------------- | ------------------ | ----------------------------------------- |
| 1    | User                | Initiate           | User calls their contract method          |
| 2    | User Contract       | `RequestPrice()`   | Builds payload, calls Gateway             |
| 3    | ServiceLayerGateway | `RequestService()` | Validates, charges fee, routes to service |
| 4    | Service Contract    | `OnRequest()`      | Stores request, emits event               |
| 5    | Service Layer (TEE) | Monitor            | Listens for on-chain events               |
| 6    | Service Layer (TEE) | Process            | Executes off-chain logic                  |
| 7    | Service Layer (TEE) | Sign               | Signs result with TEE key                 |
| 8    | Service Contract    | `OnFulfill()`      | Receives fulfillment from Gateway         |
| 9    | ServiceLayerGateway | `FulfillRequest()` | Verifies TEE signature, routes callback   |
| 10   | User Contract       | `Callback()`       | Receives result, updates state            |
| 11   | User                | Complete           | Transaction confirmed                     |

#### Pattern 2: Push / Auto-Update (NeoFeeds)

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

#### Pattern 3: Trigger-Based (NeoFlow)

Users register triggers, TEE monitors conditions and invokes callbacks:

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

| Trigger Type | Example                 | Use Case                        |
| ------------ | ----------------------- | ------------------------------- |
| Time-based   | `cron: "0 0 * * FRI"`   | Weekly token distribution       |
| Price-based  | `price: BTC > 100000`   | Auto-sell when price target hit |
| Threshold    | `balance < 10 GAS`      | Auto-refill gas bank            |
| Event-based  | `event: LiquidityAdded` | React to on-chain events        |

### NeoVault Service (v4.1) - Deterministic Shared Seed

The NeoVault uses **standard single-sig addresses** (identical to ordinary users) for maximum privacy:

- **No on-chain pool registration** - Pool accounts managed entirely off-chain
- **Standard single-sig addresses** - No multisig fingerprint, indistinguishable from regular users
- **Deterministic shared seed** - `Shared_Seed = HKDF(Master_Secret, TEE_Attestation_Hash)`
- **Master recovery** - Master can reconstruct seed and recover all accounts if TEE fails

```
1. Admin: RegisterService + DepositBond
2. User: CreateRequest(encryptedTargets, mixOption) + GAS
3. TEE: ClaimRequest → funds to standard single-sig pool accounts
4. TEE: Random transfers + noise transactions (off-chain)
5. TEE: SubmitCompletion(outputsHash)
6. User (timeout): ClaimRefundByUser → refund from bond
```

## 🔐 Security Features

### TEE Protection (Intel MarbleRun)

- All services run inside EGo MarbleRun TEE
- Remote attestation via MarbleRun
- Secrets never leave the TEE
- TLS termination inside TEE

### MarbleRun Integration

- Manifest-based topology verification
- Automatic certificate provisioning
- Secrets injection at runtime
- Single attestation for entire cluster

### Cryptographic Operations

- ECDSA secp256r1 (Neo N3 compatible)
- AES-256-GCM encryption
- HKDF key derivation
- VRF (ECVRF-P256-SHA256-TAI)

## 📁 Project Structure

```
service_layer/
├── cmd/
│   └── gateway/          # API Gateway entry point
├── internal/
│   ├── marble/           # Marble SDK & service framework
│   ├── database/         # Supabase client & repository
│   ├── crypto/           # Cryptographic operations
│   └── secretstore/      # NeoStore (secrets) client
├── services/
│   ├── neooracle/        # NeoOracle service
│   ├── neorand/          # NeoRand (VRF) service
│   ├── neovault/         # NeoVault service
│   ├── neoaccounts/      # NeoAccounts (AccountPool) service
│   ├── neostore/         # NeoStore (Secrets) service
│   ├── neofeeds/         # NeoFeeds service
│   ├── neoflow/          # NeoFlow service
│   └── neocompute/       # NeoCompute service
├── manifests/
│   └── manifest.json     # MarbleRun manifest
├── migrations/
│   └── 001_initial_schema.sql
├── docker/
│   ├── Dockerfile.gateway
│   ├── Dockerfile.service
│   └── docker-compose.yaml
├── k8s/                   # Kubernetes manifests/overlays
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── api/
│   │   └── stores/
│   └── package.json
├── scripts/              # Dev/deploy scripts
└── Makefile
```

## 🛠️ Development

### Build Commands

```bash
make build          # Build all services
make build-ego      # Build with EGo for MarbleRun
make test           # Run tests
make lint           # Run linter
make fmt            # Format code
```

### Docker Commands

```bash
make docker-build   # Build Docker images
make docker-up      # Start in simulation mode
make docker-up-tee  # Start with MarbleRun hardware
make docker-down    # Stop all services
make docker-logs    # View logs
```

### MarbleRun Commands

```bash
make marblerun-manifest  # Set manifest
make marblerun-status    # Check status
make marblerun-recover   # Recover coordinator
```

## 🌐 API Reference

### Authentication

```bash
# 1) Request a nonce + message to sign
POST /api/v1/auth/nonce
{
  "address": "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
}

# 2) Sign the returned "message" with your Neo N3 wallet, then login (or register)
POST /api/v1/auth/login
{
  "address": "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
  "publicKey": "hex_or_base64_public_key",
  "signature": "hex_or_base64_signature",
  "message": "original_message_from_nonce_endpoint",
  "nonce": "nonce_from_nonce_endpoint"
}

# Note: when OAUTH_COOKIE_MODE=true, the gateway also sets an HttpOnly cookie
# (sl_auth_token) so browser clients can authenticate via `credentials: include`.
```

### Oracle

```bash
# Fetch external data
POST /api/v1/oracle/fetch
{
  "url": "https://api.example.com/data",
  "json_path": "data.price"
}
```

### VRF

```bash
# Generate random numbers
POST /api/v1/vrf/random
{
  "seed": "0x...",
  "num_words": 3
}
```

### Secrets

```bash
# Create secret
POST /api/v1/secrets/secrets
{
  "name": "API_KEY",
  "value": "secret_value"
}
```

## 📊 Environment Variables

```bash
# Runtime
MARBLE_ENV=development   # development|testing|production

# Gateway (required in production; >= 32 bytes)
JWT_SECRET=...
GATEWAY_TLS_MODE=off     # off|tls|mtls
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173

# Supabase
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=xxx

# Neo N3
NEO_RPC_URL=https://testnet1.neo.coz.io:443
NEO_NETWORK_MAGIC=894710606

# MarbleRun
# Marbles reach the coordinator via the mesh API address.
# - Docker Compose: coordinator:2001
# - Kubernetes: coordinator-mesh-api.marblerun.svc.cluster.local:2001 (or your domain)
COORDINATOR_MESH_ADDR=coordinator:2001
# marblerun CLI connects to the coordinator client API (usually exposed on localhost for dev)
COORDINATOR_CLIENT_ADDR=localhost:4433
OE_SIMULATION=1  # Set to 0 for MarbleRun hardware

# OAuth (optional)
FRONTEND_URL=http://localhost:3000
OAUTH_REDIRECT_BASE=http://localhost:8080
OAUTH_COOKIE_MODE=true
OAUTH_COOKIE_SAMESITE=lax  # strict|lax|none (none requires HTTPS)
```

See `.env.example` for a complete, annotated list (service URLs, allowlists, testnet settings).

## 🔄 Upgrade Notes

- AccountPool shared table: new deployments use `pool_accounts` (see `migrations/003_service_persistence.sql`). Existing deployments should apply `migrations/006_accountpool_schema.sql` to rename any legacy `neovault_pool_accounts` table and add lock columns/indexes expected by the AccountPool service.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
