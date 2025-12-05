# Neo Service Layer Documentation

Production-grade confidential computing platform for Neo N3 blockchain.

**Stack**: MarbleRun + EGo + Supabase + Netlify + Neo N3

---

## Quick Navigation

| Category | Document | Description |
|----------|----------|-------------|
| **Start Here** | [README](../README.md) | Project overview and quick start |
| **Architecture** | [MarbleRun Architecture](architecture/MARBLERUN_ARCHITECTURE.md) | Complete system architecture |
| **Deployment** | [Deployment Guide](DEPLOYMENT.md) | Production deployment |
| **Services** | [Service Documentation](services/README.md) | All 14 services |

---

## Architecture

### Core Architecture Documents

| Document | Description |
|----------|-------------|
| [MarbleRun Architecture](architecture/MARBLERUN_ARCHITECTURE.md) | **Primary**: Complete architecture with MarbleRun, EGo, Supabase |
| [TEE Supabase Architecture](architecture/TEE_SUPABASE_ARCHITECTURE.md) | TEE integration with Supabase |
| [TEE Trust Root](architecture/TEE_TRUST_ROOT_IMPLEMENTATION.md) | Trust root implementation details |

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (React + Netlify)                   │
│  • Supabase Auth • Realtime subscriptions • 14 service helpers  │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Supabase (PostgreSQL)                        │
│  • service_requests • RLS policies • Realtime notifications     │
└────────────────────────────┬────────────────────────────────────┘
                             │ Polling
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              Service Layer (MarbleRun + EGo)                    │
│  Coordinator → Oracle/VRF/Secrets/GasBank/... Marbles (SGX)     │
└────────────────────────────┬────────────────────────────────────┘
                             │ RPC
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Neo N3 Blockchain                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 14 Services

All services run as EGo SGX Marbles, activated by the MarbleRun Coordinator.

| Service | Description | Documentation |
|---------|-------------|---------------|
| **oracle** | External data fetching | [Oracle Docs](services/oracle/README.md) |
| **vrf** | Verifiable random functions | [VRF Docs](services/vrf/README.md) |
| **secrets** | Encrypted secret storage | [Secrets Docs](services/secrets/README.md) |
| **gasbank** | Gas fee management | [GasBank Docs](services/gasbank/README.md) |
| **mixer** | Privacy mixing | [Mixer Docs](services/mixer/README.md) |
| **datafeeds** | Price feed aggregation | [DataFeeds Docs](services/datafeeds/README.md) |
| **accounts** | Account management | [Accounts Docs](services/accounts/README.md) |
| **automation** | Task scheduling | [Automation Docs](services/automation/README.md) |
| **ccip** | Cross-chain messaging | [CCIP Docs](services/ccip/README.md) |
| **confidential** | Confidential compute | [Confidential Docs](services/confidential/README.md) |
| **cre** | Chainlink runtime | [CRE Docs](services/cre/README.md) |
| **datalink** | Data linking | [DataLink Docs](services/datalink/README.md) |
| **datastreams** | Real-time streaming | [DataStreams Docs](services/datastreams/README.md) |
| **dta** | Data trust authority | [DTA Docs](services/dta/README.md) |

---

## Components

### MarbleRun Coordinator

Manages SGX enclave lifecycle and secret provisioning.

**Location**: `coordinator/`

**Endpoints**:
```
POST /api/v2/marble/activate  - Marble activation
POST /api/v2/marble/heartbeat - Health check
GET  /api/v2/manifest         - Get manifest
POST /api/v2/manifest         - Set manifest
GET  /api/v2/status           - Coordinator status
```

### Marble SDK

Go SDK for service Marbles.

**Location**: `marble/sdk/`

```go
marble, _ := sdk.New(sdk.Config{
    MarbleType:      "oracle",
    CoordinatorAddr: "coordinator:4433",
})
marble.Activate(ctx)
secret, _ := marble.GetSecret("api_key")
```

### EGo Runtime

SGX enclave runtime with secure sealing.

**Location**: `ego/`

**Features**:
- AES-256-GCM sealing (not mock XOR)
- DCAP-compatible quote generation
- Quote verification
- Attestation info

### Supabase Client

Resilient PostgreSQL client.

**Location**: `supabase/client/`

**Features**:
- Connection pooling (100 connections)
- Retry with exponential backoff
- Circuit breaker pattern
- Request metrics

---

## Quick Start

### Development

```bash
# Start Supabase
docker-compose up -d supabase

# Start Coordinator (simulation)
SIMULATION_MODE=true go run ./cmd/coordinator

# Start Service Marble
SIMULATION_MODE=true MARBLE_TYPE=oracle go run ./cmd/marble

# Start Frontend
cd frontend && npm run dev
```

### Production

```bash
# Build EGo enclave
ego-go build -o marble ./cmd/marble
ego sign marble

# Deploy to Kubernetes
kubectl apply -f devops/kubernetes/

# Deploy Frontend
netlify deploy --prod
```

---

## Configuration

### Environment Variables

**Coordinator**:
```bash
COORDINATOR_ADDR=0.0.0.0:4433
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=eyJ...
SIMULATION_MODE=false
```

**Marble**:
```bash
MARBLE_TYPE=oracle
COORDINATOR_ADDR=coordinator:4433
SIMULATION_MODE=false
```

**Frontend**:
```bash
VITE_SUPABASE_URL=https://xxx.supabase.co
VITE_SUPABASE_ANON_KEY=eyJ...
VITE_NEO_RPC_URL=https://mainnet1.neo.coz.io:443
```

---

## Security Model

### Trust Hierarchy

```
Intel SGX CPU (Hardware Root of Trust)
        ↓
    EGo Runtime (Enclave)
        ↓
    MarbleRun Coordinator (Quote Verification)
        ↓
    Service Marbles (Attested Enclaves)
        ↓
    Supabase (Encrypted at Rest, RLS)
        ↓
    Frontend (User Authentication)
```

### Key Security Features

1. **SGX Attestation**: All Marbles prove genuine enclave execution
2. **AES-256-GCM Sealing**: Secure data encryption with enclave keys
3. **mTLS**: Encrypted Marble-to-Marble communication
4. **RLS**: Row-Level Security for user isolation
5. **Secret Provisioning**: Secrets only inside attested enclaves

---

## API Reference

### Service Request Flow

```
1. Frontend → Supabase
   INSERT INTO service_requests (service_type='oracle', operation='fetch')

2. Marble polls Supabase
   SELECT * FROM service_requests WHERE status='pending'

3. Marble processes in SGX
   Execute operation, sign result with TEE key

4. Result stored
   UPDATE service_requests SET status='completed', result=...

5. Frontend receives via Realtime
   WebSocket notification → UI update
```

### Health Endpoints

```bash
GET /health          # Service health
GET /info            # Service info
GET /attestation     # TEE attestation info
GET /quote?data=...  # Generate SGX quote
```

---

## Testing

```bash
# Unit tests
go test ./...

# Integration tests
go test -tags integration ./...

# Frontend tests
cd frontend && npm test
```

---

## Project Structure

```
service_layer/
├── coordinator/       # MarbleRun Coordinator
├── marble/sdk/        # Marble SDK
├── ego/               # EGo Runtime
├── services/          # 14 Service implementations
├── supabase/          # Supabase client + migrations
├── frontend/          # React + Netlify
├── tee/               # TEE abstractions
├── contracts/         # Neo N3 smart contracts
└── docs/              # Documentation
    ├── architecture/  # Architecture docs
    └── services/      # Per-service docs
```

---

## Document Status

| Document | Status | Updated |
|----------|--------|---------|
| MarbleRun Architecture | Current | 2025-12 |
| Service Documentation | Current | 2025-12 |
| Deployment Guide | Current | 2025-12 |
| README | Current | 2025-12 |
