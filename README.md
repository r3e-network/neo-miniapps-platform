# Neo Service Layer

A production-grade confidential computing platform for Neo N3 blockchain services.

## Architecture

**Stack**: MarbleRun + EGo + Supabase + Netlify + Neo N3

```
┌─────────────────────────────────────────────────────────────────┐
│                    Frontend (React + Netlify)                   │
│  • TypeScript + Vite + TailwindCSS                              │
│  • Supabase Auth (OAuth, Email/Password)                        │
│  • Supabase Realtime (WebSocket subscriptions)                  │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Supabase (PostgreSQL)                        │
│  • service_requests table (14 service types)                    │
│  • Row-Level Security (user isolation)                          │
│  • Realtime notifications                                       │
└────────────────────────────┬────────────────────────────────────┘
                             │ Polling
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│              Service Layer (MarbleRun + EGo)                    │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Coordinator (MarbleRun)                     │   │
│  │  • Quote verification • Secret provisioning • mTLS       │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌───────────┬───────────┬───────────┬───────────┬─────────┐   │
│  │  Oracle   │    VRF    │  Secrets  │  GasBank  │  ...    │   │
│  │  Marble   │  Marble   │  Marble   │  Marble   │ (14)    │   │
│  │  EGo SGX  │  EGo SGX  │  EGo SGX  │  EGo SGX  │         │   │
│  └───────────┴───────────┴───────────┴───────────┴─────────┘   │
└────────────────────────────┬────────────────────────────────────┘
                             │ RPC
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Neo N3 Blockchain                         │
│  • Smart contracts • On-chain TEE signature verification        │
└─────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
service_layer/
├── coordinator/           # MarbleRun Coordinator
│   └── coordinator.go     # Quote verification, secret provisioning
│
├── marble/                # Marble SDK
│   └── sdk/               # Activation, heartbeat, TLS management
│
├── ego/                   # EGo Runtime
│   └── ego.go             # SGX sealing (AES-256-GCM), quotes
│
├── services/              # 14 Service Implementations
│   ├── common/            # Shared interfaces and registry
│   ├── oracle/            # External data fetching
│   ├── vrf/               # Verifiable random functions
│   ├── secrets/           # Encrypted secret storage
│   ├── gasbank/           # Gas fee management
│   ├── mixer/             # Privacy mixing
│   ├── datafeeds/         # Price feed aggregation
│   ├── accounts/          # Account management
│   ├── automation/        # Task scheduling
│   ├── ccip/              # Cross-chain messaging
│   ├── confidential/      # Confidential compute
│   ├── cre/               # Chainlink runtime
│   ├── datalink/          # Data linking
│   ├── datastreams/       # Real-time streaming
│   └── dta/               # Data trust authority
│
├── supabase/              # Supabase Integration
│   ├── client/            # Go client with resilience patterns
│   └── migrations/        # Database schema
│
├── frontend/              # React Frontend (Netlify)
│   └── src/
│       ├── lib/           # Supabase client
│       ├── pages/         # Service pages
│       └── components/    # UI components
│
├── tee/                   # TEE Abstractions
│   ├── enclave/           # Enclave runtime
│   ├── vault/             # Secret management
│   └── attestation/       # Remote attestation
│
└── docs/                  # Documentation
    ├── architecture/      # Architecture docs
    └── services/          # Per-service documentation
```

## 14 Services

| Service | Description | Operations |
|---------|-------------|------------|
| **oracle** | External data fetching | fetch, callback |
| **vrf** | Verifiable random functions | random, verify |
| **secrets** | Encrypted secret storage | store, get, delete |
| **gasbank** | Gas fee management | deposit, withdraw, balance |
| **mixer** | Privacy mixing | create_request, get_status |
| **datafeeds** | Price feed aggregation | get_value, create_feed |
| **accounts** | Account management | create, get, list |
| **automation** | Task scheduling | create_task, run_task |
| **ccip** | Cross-chain messaging | send_message, get_status |
| **confidential** | Confidential compute | execute, get_result |
| **cre** | Chainlink runtime | deploy_function, invoke |
| **datalink** | Data linking | create, fetch |
| **datastreams** | Real-time streaming | create, subscribe |
| **dta** | Data trust authority | register, query, verify |

## Quick Start

### Development (Simulation Mode)

```bash
# 1. Start Supabase
docker-compose up -d supabase

# 2. Start Coordinator
SIMULATION_MODE=true go run ./cmd/coordinator

# 3. Start a Service Marble
SIMULATION_MODE=true MARBLE_TYPE=oracle go run ./cmd/marble

# 4. Start Frontend
cd frontend && npm install && npm run dev
```

### Production (SGX Hardware)

```bash
# Build EGo enclave
ego-go build -o marble ./cmd/marble
ego sign marble

# Deploy with MarbleRun
kubectl apply -f devops/kubernetes/

# Deploy Frontend to Netlify
netlify deploy --prod
```

## Key Features

### Security
- **SGX Enclaves**: All sensitive operations run in Intel SGX
- **AES-256-GCM Sealing**: Secure data encryption (not mock XOR)
- **DCAP Attestation**: Structured quote generation and verification
- **mTLS**: Marble-to-Marble and Marble-to-Coordinator encryption
- **RLS**: Row-Level Security for user data isolation

### Resilience
- **Retry Logic**: Exponential backoff with jitter
- **Circuit Breaker**: Closed → Open → Half-Open pattern
- **Connection Pooling**: 100 connections, 10 per host
- **Heartbeat**: Regular health checks to Coordinator

### Architecture
- **KISS**: Clear separation of concerns
- **DRY**: Shared base components
- **SOLID**: Interface segregation (SecretsAPI, NetworkAPI, etc.)

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

**Frontend** (Netlify):
```bash
VITE_SUPABASE_URL=https://xxx.supabase.co
VITE_SUPABASE_ANON_KEY=eyJ...
VITE_NEO_RPC_URL=https://mainnet1.neo.coz.io:443
```

## Documentation

- [Architecture Overview](docs/architecture/MARBLERUN_ARCHITECTURE.md)
- [Service Documentation](docs/services/README.md)
- [Deployment Guide](docs/DEPLOYMENT.md)

## Building

```bash
# Build all Go packages
go build ./...

# Run tests
go test ./...

# Build frontend
cd frontend && npm run build
```

## License

See [LICENSE](LICENSE) file.
