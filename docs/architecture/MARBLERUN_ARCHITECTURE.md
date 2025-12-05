# Neo Service Layer - MarbleRun Architecture

## Overview

Neo Service Layer uses a modern confidential computing architecture based on:

- **MarbleRun** - Confidential computing control plane
- **EGo** - Go framework for Intel SGX enclaves
- **Supabase** - Backend-as-a-Service (PostgreSQL + Auth + Realtime)
- **Netlify** - Frontend hosting and deployment
- **Neo N3** - Target blockchain

```
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (React + Netlify)                   │
│  • TypeScript + Vite                                            │
│  • Supabase Auth (OAuth, Email/Password)                        │
│  • Supabase Realtime (WebSocket subscriptions)                  │
│  • 14 Service Helper Functions                                  │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS (Supabase API)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Supabase (PostgreSQL)                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ service_requests table                                    │  │
│  │ • 14 service types (oracle, vrf, secrets, gasbank, ...)   │  │
│  │ • Status: pending → processing → completed/failed         │  │
│  │ • RLS policies (user isolation)                           │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ realtime_notifications table                              │  │
│  │ • Push notifications via Supabase Realtime                │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────────────────────┬────────────────────────────────────┘
                             │ Polling (Service Key)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              Service Layer (MarbleRun + EGo)                    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Coordinator (MarbleRun)                     │   │
│  │  • Manifest management                                   │   │
│  │  • Secret provisioning                                   │   │
│  │  • TLS certificate signing                               │   │
│  │  • Quote verification (SGX attestation)                  │   │
│  │  • Recovery key management                               │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │ Activation                           │
│  ┌───────────┬───────────┼───────────┬───────────┬─────────┐   │
│  │  Oracle   │    VRF    │  Secrets  │  GasBank  │  ...    │   │
│  │  Marble   │  Marble   │  Marble   │  Marble   │         │   │
│  │           │           │           │           │         │   │
│  │ EGo SGX   │ EGo SGX   │ EGo SGX   │ EGo SGX   │         │   │
│  │ Runtime   │ Runtime   │ Runtime   │ Runtime   │         │   │
│  └───────────┴───────────┴───────────┴───────────┴─────────┘   │
│                                                                 │
│  Features:                                                      │
│  • AES-256-GCM sealing (not XOR)                               │
│  • Structured SGX quotes (DCAP compatible)                      │
│  • Retry logic with exponential backoff                         │
│  • Heartbeat mechanism                                          │
│  • Connection pooling                                           │
│  • Circuit breaker pattern                                      │
└────────────────────────────┬────────────────────────────────────┘
                             │ RPC
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Neo N3 Blockchain                         │
│  • Smart contracts for each service                             │
│  • On-chain verification of TEE signatures                      │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Frontend (Netlify)

**Location**: `frontend/`

**Technology Stack**:
- React 18 + TypeScript
- Vite build tool
- TailwindCSS
- Supabase JS Client

**Key Files**:
- `frontend/src/lib/supabase.ts` - Supabase client and service helpers
- `frontend/netlify.toml` - Netlify deployment configuration

**Features**:
- OAuth authentication (GitHub, Google, Discord, Twitter)
- Email/password authentication
- Real-time request status updates
- Service-specific helper functions for all 14 services

### 2. Supabase (Database + Auth + Realtime)

**Location**: `supabase/`

**Migrations**:
- `0001_core_schema.sql` - Core tables (coordinator state, packages, marbles, secrets)
- `0002_request_queue.sql` - Request queue architecture

**Key Tables**:
```sql
-- Generic service request queue
service_requests (
  id, user_id, service_type, operation, payload,
  status, priority, result, error_message,
  tee_signature, created_at, completed_at
)

-- Real-time notifications
realtime_notifications (
  id, user_id, type, title, message, data,
  reference_type, reference_id, read, created_at
)
```

**RLS Policies**:
- Users can only see their own requests
- Users can only insert their own requests
- Users can only cancel their own pending requests

### 3. MarbleRun Coordinator

**Location**: `coordinator/`

**Responsibilities**:
- Verify SGX quotes from Marbles
- Provision secrets to authenticated Marbles
- Sign TLS certificates for Marble-to-Marble communication
- Manage manifest (service definitions)
- Handle recovery keys for disaster recovery

**API Endpoints**:
```
POST /api/v2/marble/activate  - Marble activation
POST /api/v2/marble/heartbeat - Marble heartbeat
GET  /api/v2/manifest         - Get manifest
POST /api/v2/manifest         - Set manifest
GET  /api/v2/status           - Coordinator status
```

### 4. Marble SDK

**Location**: `marble/sdk/`

**Features**:
- Coordinator activation with retry logic (5 attempts, exponential backoff)
- EGo runtime integration
- TLS certificate management
- Secret access (copy semantics for security)
- Heartbeat mechanism
- Health status and metrics

**Usage**:
```go
marble, err := sdk.New(sdk.Config{
    MarbleType:      "oracle",
    CoordinatorAddr: "coordinator:4433",
    SimulationMode:  false,
})

if err := marble.Activate(ctx); err != nil {
    log.Fatal(err)
}

// Access secrets
secret, _ := marble.GetSecret("api_key")

// Generate attestation quote
quote, _ := marble.GenerateQuote([]byte("data"))

// Seal/unseal data
sealed, _ := marble.Seal(data)
unsealed, _ := marble.Unseal(sealed)
```

### 5. EGo Runtime

**Location**: `ego/`

**Features**:
- SGX enclave detection
- AES-256-GCM sealing (secure, not XOR mock)
- Structured quote generation (DCAP compatible format)
- Quote verification
- Attestation info retrieval
- Legacy format migration support

**Sealing Policies**:
- `SealPolicyUnique` - Seal to MRENCLAVE (same binary)
- `SealPolicyProduct` - Seal to MRSIGNER + ProductID (same signer)

### 6. Supabase Client (with Resilience)

**Location**: `supabase/client/`

**Features**:
- PostgREST query builder
- Realtime WebSocket subscriptions
- Connection pooling (100 connections, 10 per host)
- Retry logic with exponential backoff
- Circuit breaker pattern (Closed → Open → Half-Open)
- Request metrics

**Circuit Breaker States**:
```
Closed (normal) → 5 failures → Open (reject all)
                                    ↓ 30s timeout
                              Half-Open (test)
                                    ↓ 2 successes
                              Closed (normal)
```

## 14 Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **oracle** | External data fetching | fetch, callback |
| **vrf** | Verifiable random functions | random, verify |
| **secrets** | Encrypted secret storage | store, get, delete, list |
| **gasbank** | Gas fee management | deposit, withdraw, balance |
| **mixer** | Privacy mixing | create_request, get_status |
| **datafeeds** | Price feed aggregation | get_value, create_feed |
| **accounts** | Account management | create, get, list |
| **automation** | Task scheduling | create_task, run_task |
| **ccip** | Cross-chain messaging | send_message, get_status |
| **confidential** | Confidential compute | execute, get_result |
| **cre** | Chainlink runtime | deploy_function, invoke |
| **datalink** | Data linking | create, fetch |
| **datastreams** | Real-time streaming | create, subscribe, get_latest |
| **dta** | Data trust authority | register_source, query, verify |

## Request Flow

```
1. User submits request via Frontend
   └─→ supabase.submitServiceRequest('oracle', 'fetch', {url: '...'})

2. Request stored in Supabase
   └─→ INSERT INTO service_requests (status='pending')

3. Service Layer polls Supabase
   └─→ SELECT * FROM service_requests WHERE status='pending'

4. Marble claims and processes request
   └─→ UPDATE service_requests SET status='processing'
   └─→ Execute operation in SGX enclave
   └─→ Sign result with TEE key

5. Result stored in Supabase
   └─→ UPDATE service_requests SET status='completed', result=...

6. Frontend receives update via Realtime
   └─→ WebSocket notification
   └─→ UI updates automatically
```

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
    Supabase (Encrypted at Rest)
        ↓
    Frontend (User Authentication)
```

### Key Security Features

1. **Attestation**: All Marbles must prove they're running in genuine SGX enclaves
2. **Sealing**: Sensitive data encrypted with enclave-derived keys (AES-256-GCM)
3. **TLS Everywhere**: Marble-to-Marble and Marble-to-Coordinator use mTLS
4. **RLS**: Row-Level Security ensures user data isolation
5. **Secret Provisioning**: Secrets only available inside attested enclaves

## Deployment

### Development (Simulation Mode)

```bash
# Start Supabase
docker-compose up -d supabase

# Start Coordinator (simulation)
SIMULATION_MODE=true go run ./cmd/coordinator

# Start Service Marble (simulation)
SIMULATION_MODE=true MARBLE_TYPE=oracle go run ./cmd/marble

# Start Frontend
cd frontend && npm run dev
```

### Production (SGX Hardware)

```bash
# Build EGo enclave
ego-go build -o marble ./cmd/marble
ego sign marble

# Deploy to Kubernetes with MarbleRun
kubectl apply -f devops/kubernetes/

# Deploy Frontend to Netlify
netlify deploy --prod
```

## Configuration

### Environment Variables

**Coordinator**:
```
COORDINATOR_ADDR=0.0.0.0:4433
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=eyJ...
SIMULATION_MODE=false
```

**Marble**:
```
MARBLE_TYPE=oracle
COORDINATOR_ADDR=coordinator:4433
SIMULATION_MODE=false
```

**Frontend** (Netlify):
```
VITE_SUPABASE_URL=https://xxx.supabase.co
VITE_SUPABASE_ANON_KEY=eyJ...
VITE_NEO_RPC_URL=https://mainnet1.neo.coz.io:443
```

## Monitoring

### Health Endpoints

```
GET /health          - Service health
GET /info            - Service info
GET /attestation     - TEE attestation info
GET /quote?data=...  - Generate SGX quote
```

### Metrics

- `requests_total` - Total requests processed
- `requests_failed` - Failed requests
- `activation_attempts` - Marble activation attempts
- `heartbeats_sent` - Heartbeats sent to Coordinator
- `circuit_state` - Circuit breaker state

## Related Documentation

- [TEE Supabase Architecture](TEE_SUPABASE_ARCHITECTURE.md)
- [Deployment Guide](../DEPLOYMENT.md)
- [API Reference](../api/README.md)
- [Service Documentation](../services/README.md)
