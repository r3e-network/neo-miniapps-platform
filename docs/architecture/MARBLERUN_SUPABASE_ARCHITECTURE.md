# Neo Service Layer - MarbleRun + Supabase Architecture

## Overview

This document defines the complete architecture for Neo Service Layer, built entirely on:
- **MarbleRun**: Confidential computing control plane for TEE orchestration
- **Supabase**: Backend-as-a-Service for data persistence, auth, and realtime

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Neo Service Layer                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     COORDINATOR (MarbleRun-based)                    │    │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐   │    │
│  │  │  Manifest   │ │   Quote     │ │   Secret    │ │   Recovery   │   │    │
│  │  │  Manager    │ │  Verifier   │ │   Manager   │ │   Manager    │   │    │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘   │    │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐   │    │
│  │  │    TLS      │ │    User     │ │   Update    │ │    State     │   │    │
│  │  │  Manager    │ │   Manager   │ │   Manager   │ │   Machine    │   │    │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                    ┌───────────────┼───────────────┐                        │
│                    │               │               │                        │
│  ┌─────────────────▼───┐ ┌────────▼────────┐ ┌────▼─────────────────┐      │
│  │   MARBLE: Oracle    │ │  MARBLE: VRF    │ │  MARBLE: Secrets     │      │
│  │   ┌─────────────┐   │ │  ┌───────────┐  │ │  ┌───────────────┐   │      │
│  │   │ SGX Enclave │   │ │  │SGX Enclave│  │ │  │  SGX Enclave  │   │      │
│  │   │ ┌─────────┐ │   │ │  │┌─────────┐│  │ │  │ ┌───────────┐ │   │      │
│  │   │ │ Service │ │   │ │  ││ Service ││  │ │  │ │  Service  │ │   │      │
│  │   │ │  Logic  │ │   │ │  ││  Logic  ││  │ │  │ │   Logic   │ │   │      │
│  │   │ └─────────┘ │   │ │  │└─────────┘│  │ │  │ └───────────┘ │   │      │
│  │   └─────────────┘   │ │  └───────────┘  │ │  └───────────────┘   │      │
│  └─────────────────────┘ └─────────────────┘ └──────────────────────┘      │
│                                    │                                         │
│  ┌─────────────────────────────────▼───────────────────────────────────┐    │
│  │                        SUPABASE LAYER                                │    │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌──────────────┐   │    │
│  │  │  PostgreSQL │ │    Auth     │ │   Storage   │ │   Realtime   │   │    │
│  │  │  (RLS)      │ │  (JWT/RBAC) │ │   (Blobs)   │ │  (WebSocket) │   │    │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └──────────────┘   │    │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │    │
│  │  │   Edge      │ │  PostgREST  │ │   GoTrue    │                    │    │
│  │  │  Functions  │ │   (API)     │ │   (Auth)    │                    │    │
│  │  └─────────────┘ └─────────────┘ └─────────────┘                    │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
              ┌─────▼─────┐   ┌──────▼──────┐  ┌─────▼─────┐
              │  Neo N3   │   │   Frontend  │  │  External │
              │ Blockchain│   │   (React)   │  │   APIs    │
              └───────────┘   └─────────────┘  └───────────┘
```

## Core Components

### 1. Coordinator (MarbleRun-based)

The Coordinator is the control plane that:
- Verifies Marble integrity via SGX remote attestation
- Manages the service mesh manifest
- Distributes secrets to verified Marbles
- Issues TLS certificates for inter-service communication
- Handles multi-party recovery

**State Machine:**
```
┌──────────────────┐
│  Uninitialized   │
└────────┬─────────┘
         │ Start()
         ▼
┌──────────────────┐     Recovery needed
│ AcceptingManifest│◄────────────────────┐
└────────┬─────────┘                     │
         │ SetManifest()                 │
         ▼                               │
┌──────────────────┐                     │
│ AcceptingMarbles │─────────────────────┘
└────────┬─────────┘     Sealed state lost
         │ Activate()
         ▼
┌──────────────────┐
│     Running      │
└──────────────────┘
```

### 2. Marbles (Services)

Each service runs as a Marble inside an SGX enclave:

| Marble | Purpose | Capabilities |
|--------|---------|--------------|
| oracle | External data fetching | network, secrets, contract |
| vrf | Verifiable random function | keys, contract |
| secrets | Secret management | secrets.write, attestation |
| gasbank | Gas sponsorship | neo.sign, contract |
| datafeeds | Price feeds | network, contract |
| automation | Task scheduling | scheduler, contract |
| mixer | Privacy mixing | keys, neo.sign |

**Marble Lifecycle:**
```
1. Marble starts inside SGX enclave
2. Marble generates attestation quote
3. Marble sends activation request to Coordinator
4. Coordinator verifies quote against Package definition
5. Coordinator returns secrets, TLS certs, and config
6. Marble initializes with received credentials
7. Marble registers with service mesh
8. Marble begins processing requests
```

### 3. Supabase Layer

Supabase provides all data persistence and auth:

**PostgreSQL Tables:**
```sql
-- Core tables
accounts          -- User accounts
api_keys          -- API key management
service_requests  -- Request tracking
service_responses -- Response tracking

-- Service-specific tables
oracle_requests   -- Oracle request queue
vrf_requests      -- VRF request queue
secrets_store     -- Encrypted secrets metadata
gasbank_deposits  -- Gas deposits
datafeeds_prices  -- Price feed data
automation_tasks  -- Scheduled tasks

-- Audit tables
audit_logs        -- All operations audit trail
```

**Row Level Security (RLS):**
- All tables have RLS enabled
- Policies enforce tenant isolation
- Service accounts have specific permissions

**Auth Flow:**
```
1. User authenticates via Supabase Auth (email/OAuth)
2. JWT token issued with user claims
3. API requests include JWT in Authorization header
4. Supabase validates JWT and applies RLS
5. Marbles use service role for internal operations
```

## Data Flow

### Request Processing Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│ Supabase │────▶│Coordinator────▶│  Marble  │
│          │     │   API    │     │          │     │ (Enclave)│
└──────────┘     └──────────┘     └──────────┘     └──────────┘
     │                │                │                │
     │ 1. Request     │                │                │
     │───────────────▶│                │                │
     │                │ 2. Validate    │                │
     │                │    JWT/RLS     │                │
     │                │                │                │
     │                │ 3. Route to    │                │
     │                │    Coordinator │                │
     │                │───────────────▶│                │
     │                │                │ 4. Select      │
     │                │                │    Marble      │
     │                │                │───────────────▶│
     │                │                │                │ 5. Process
     │                │                │                │    in TEE
     │                │                │◀───────────────│
     │                │◀───────────────│ 6. Response    │
     │◀───────────────│                │                │
     │ 7. Result      │                │                │
```

### Secret Access Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Marble  │────▶│Coordinator────▶│  Vault   │
│ (Enclave)│     │          │     │ (Sealed) │
└──────────┘     └──────────┘     └──────────┘
     │                │                │
     │ 1. Need secret │                │
     │   "api_key"    │                │
     │───────────────▶│                │
     │                │ 2. Verify      │
     │                │    Marble ID   │
     │                │                │
     │                │ 3. Check       │
     │                │    permissions │
     │                │                │
     │                │ 4. Unseal      │
     │                │───────────────▶│
     │                │◀───────────────│
     │◀───────────────│ 5. Return      │
     │                │    (encrypted) │
     │ 6. Decrypt     │                │
     │    in enclave  │                │
```

## Directory Structure

```
service_layer/
├── cmd/
│   ├── coordinator/        # Coordinator binary
│   │   └── main.go
│   └── marble/             # Generic marble binary
│       └── main.go
├── coordinator/            # Coordinator implementation
│   ├── api/                # REST API handlers
│   │   ├── manifest.go
│   │   ├── quote.go
│   │   ├── secrets.go
│   │   ├── status.go
│   │   └── recovery.go
│   ├── core/               # Core coordinator logic
│   │   ├── coordinator.go
│   │   ├── state.go
│   │   └── activation.go
│   ├── crypto/             # Cryptographic operations
│   │   ├── sealing.go
│   │   ├── tls.go
│   │   └── recovery.go
│   └── store/              # State persistence
│       ├── sealed.go
│       └── supabase.go
├── marble/                 # Marble SDK
│   ├── sdk/                # Marble SDK for services
│   │   ├── marble.go
│   │   ├── activation.go
│   │   └── secrets.go
│   └── premain/            # Pre-main injection
│       └── premain.go
├── services/               # Service implementations (Marbles)
│   ├── oracle/
│   ├── vrf/
│   ├── secrets/
│   ├── gasbank/
│   ├── datafeeds/
│   └── automation/
├── supabase/               # Supabase integration
│   ├── client/             # Supabase client
│   ├── migrations/         # Database migrations
│   └── functions/          # Edge functions
├── manifest/               # Manifest definitions
│   ├── manifest.go         # Manifest types
│   ├── validate.go         # Validation
│   └── examples/           # Example manifests
├── tee/                    # TEE abstractions
│   ├── sgx/                # SGX implementation
│   ├── simulation/         # Simulation mode
│   └── attestation/        # Attestation
└── configs/
    └── manifest.yaml       # Production manifest
```

## Manifest Structure

```yaml
# Neo Service Layer Manifest (MarbleRun-compatible)

Packages:
  neo-service-pkg:
    SignerID: "..."
    ProductID: 1
    SecurityVersion: 1

Marbles:
  oracle:
    Package: neo-service-pkg
    Parameters:
      Env:
        SUPABASE_URL: "{{ .Secrets.supabase_url }}"
        SUPABASE_KEY: "{{ .Secrets.supabase_service_key }}"
      Files:
        /config/service.json: "{{ raw .Secrets.oracle_config }}"
    TLS:
      Incoming:
        - Port: "8443"
          Cert: oracle-cert

Secrets:
  supabase_url:
    Type: plain
    UserDefined: true
  supabase_service_key:
    Type: plain
    UserDefined: true
  oracle-cert:
    Type: cert-ecdsa
    Cert:
      Subject:
        CommonName: "oracle.neo-service-layer"

Users:
  admin:
    Certificate: "..."
    Roles: [admin]

Roles:
  admin:
    ResourceType: "*"
    Actions: ["*"]
```

## API Endpoints

### Coordinator API (MarbleRun-compatible)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v2/manifest` | GET | Get current manifest |
| `/api/v2/manifest` | POST | Set manifest |
| `/api/v2/quote` | GET | Get coordinator quote |
| `/api/v2/secrets` | GET/POST | Manage secrets |
| `/api/v2/status` | GET | Get coordinator status |
| `/api/v2/recover` | POST | Recovery operation |
| `/api/v2/update` | POST | Update manifest |

### Service API (via Supabase)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rest/v1/oracle/request` | POST | Create oracle request |
| `/rest/v1/vrf/request` | POST | Create VRF request |
| `/rest/v1/secrets` | GET/POST | Manage user secrets |
| `/rest/v1/gasbank/deposit` | POST | Deposit gas |
| `/rest/v1/datafeeds` | GET | Get price feeds |

## Security Model

### Trust Hierarchy

```
┌─────────────────────────────────────────┐
│           Intel SGX Root of Trust       │
│  (Hardware attestation, sealing keys)   │
└────────────────────┬────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│            Coordinator Enclave          │
│  (Manifest validation, secret mgmt)     │
└────────────────────┬────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│            Marble Enclaves              │
│  (Service logic, data processing)       │
└────────────────────┬────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│              Supabase                   │
│  (Encrypted data at rest, RLS)          │
└─────────────────────────────────────────┘
```

### Data Protection

| Data Type | Protection |
|-----------|------------|
| Secrets | Sealed in TEE, never in plaintext outside enclave |
| User data | Encrypted at rest in Supabase, RLS enforced |
| API keys | Hashed in database, plaintext only in TEE |
| TLS keys | Generated and stored only in TEE |
| Audit logs | Immutable, signed by TEE |

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Coordinator core implementation
- [ ] Marble SDK
- [ ] Supabase schema and migrations
- [ ] Basic attestation (simulation mode)

### Phase 2: Service Migration
- [ ] Migrate Oracle service to Marble
- [ ] Migrate VRF service to Marble
- [ ] Migrate Secrets service to Marble
- [ ] Migrate GasBank service to Marble

### Phase 3: Production Hardening
- [ ] SGX hardware mode support
- [ ] Multi-party recovery
- [ ] Manifest signing
- [ ] Audit logging

### Phase 4: Advanced Features
- [ ] Service mesh networking
- [ ] Auto-scaling
- [ ] Monitoring and alerting
- [ ] Disaster recovery
