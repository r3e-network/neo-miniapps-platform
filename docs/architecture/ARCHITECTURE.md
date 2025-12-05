# Service Layer Architecture

> **Note**: This document describes the internal layered architecture. For the complete production architecture with MarbleRun, EGo, Supabase, and Netlify, see [MarbleRun Architecture](MARBLERUN_ARCHITECTURE.md).

## Overview

The Service Layer uses a **MarbleRun + EGo + Supabase + Netlify** architecture for production deployment, with an internal layered design for service isolation.

### Production Architecture

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

### Internal Layered Architecture

Each service Marble follows an internal layered design:

```
┌─────────────────────────────────────────────────────────────────┐
│                      SERVICES (14 Marbles)                      │
│  Oracle, Mixer, Secrets, DataFeeds, VRF, GasBank, etc.         │
│  - Run as EGo SGX enclaves                                      │
│  - Activated by MarbleRun Coordinator                           │
│  - Located in: services/                                        │
└─────────────────────────────────────────────────────────────────┘
                              ↑ Service APIs
┌─────────────────────────────────────────────────────────────────┐
│                    PLATFORM LAYER                               │
│  Runtime, Sandbox, Events, Storage, API                        │
│  - Located in: platform/                                        │
└─────────────────────────────────────────────────────────────────┘
                              ↑ TEE APIs
┌─────────────────────────────────────────────────────────────────┐
│                    TEE (EGo Runtime)                            │
│  AES-256-GCM Sealing, DCAP Quotes, Attestation, Keys           │
│  - Foundation of all secure operations                          │
│  - Located in: tee/, ego/                                       │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
service_layer/
├── tee/                          # Layer 1: Trust Root (底层)
│   ├── types/                    # All interfaces and types
│   │   └── types.go              # TrustRoot, SecureVault, etc.
│   ├── enclave/                  # Enclave runtime
│   │   ├── runtime.go            # Enclave abstraction
│   │   └── sealing.go            # Data sealing
│   ├── vault/                    # Secret management
│   │   ├── vault.go              # SecureVault implementation
│   │   ├── policy.go             # Access policies
│   │   └── audit.go              # Audit logging
│   ├── network/                  # Secure networking
│   │   └── http.go               # TLS inside enclave
│   ├── keys/                     # Key management
│   │   └── manager.go            # HSM-like key ops
│   ├── compute/                  # Confidential compute
│   │   └── engine.go             # Script execution
│   ├── attestation/              # Remote attestation
│   │   └── attestor.go           # Quote generation
│   ├── bridge/                   # Untrusted bridge
│   │   ├── socket.go             # Raw I/O
│   │   └── storage.go            # Encrypted storage
│   └── trust_root.go             # Main TrustRoot impl
│
├── platform/                     # Layer 2: Platform Services
│   ├── os/                       # ServiceOS (Android OS)
│   │   ├── types.go              # ServiceOS interface
│   │   ├── impl.go               # Implementation
│   │   ├── capability.go         # Capabilities (permissions)
│   │   ├── manifest.go           # Service manifests
│   │   ├── context.go            # Service context
│   │   └── api/                  # OS APIs
│   │       ├── secrets.go        # SecretsAPI
│   │       ├── network.go        # NetworkAPI
│   │       ├── keys.go           # KeysAPI
│   │       ├── storage.go        # StorageAPI
│   │       ├── events.go         # EventsAPI
│   │       └── compute.go        # ComputeAPI
│   │
│   ├── runtime/                  # Package runtime
│   │   ├── runtime.go            # Runtime abstraction
│   │   ├── loader.go             # Package loader
│   │   └── sandbox.go            # Sandbox execution
│   │
│   ├── events/                   # Event system
│   │   ├── bus.go                # Event bus
│   │   ├── dispatcher.go         # Event dispatch
│   │   └── store.go              # Event persistence
│   │
│   ├── storage/                  # Storage abstraction
│   │   ├── store.go              # Store interface
│   │   ├── postgres.go           # PostgreSQL impl
│   │   └── memory.go             # In-memory impl
│   │
│   ├── api/                      # HTTP API layer
│   │   ├── handler.go            # HTTP handlers
│   │   └── router.go             # Request routing
│   │
│   └── bootstrap/                # System bootstrap
│       └── bootstrap.go          # Service initialization
│
├── services/                     # Layer 3: Services (Android Apps)
│   ├── base/                     # Base service components
│   │   ├── service.go            # BaseService
│   │   ├── enclave.go            # BaseEnclave
│   │   └── store.go              # BaseStore
│   │
│   ├── oracle/                   # Oracle Service
│   │   ├── service.go            # Service implementation
│   │   ├── enclave.go            # TEE operations
│   │   ├── store.go              # Data persistence
│   │   └── domain.go             # Domain models
│   │
│   ├── mixer/                    # Mixer Service
│   ├── secrets/                  # Secrets Service
│   ├── vrf/                      # VRF Service
│   ├── datafeeds/                # DataFeeds Service
│   ├── gasbank/                  # GasBank Service
│   ├── accounts/                 # Accounts Service
│   ├── automation/               # Automation Service
│   ├── confidential/             # Confidential Service
│   ├── ccip/                     # CCIP Service
│   ├── cre/                      # CRE Service
│   ├── datalink/                 # DataLink Service
│   ├── datastreams/              # DataStreams Service
│   └── dta/                      # DTA Service
│
├── cmd/                          # Applications
│   ├── appserver/                # Main server
│   └── slctl/                    # CLI tool
│
├── sdk/                          # Client SDKs
│   ├── go/                       # Go SDK
│   └── js/                       # JavaScript SDK
│
└── docs/                         # Documentation
    └── architecture/             # Architecture docs
```

## Layer Responsibilities

### Layer 1: TEE (Trust Root)
- **Location**: `tee/`
- **Responsibility**: Foundation of all secure operations
- **Key Principle**: Secrets and keys NEVER leave the enclave
- **Components**:
  - `TrustRoot`: Main interface to TEE
  - `SecureVault`: Secret storage with Use() callback pattern
  - `SecureNetwork`: TLS termination inside enclave
  - `KeyManager`: HSM-like key management
  - `ConfidentialCompute`: Script execution with secrets
  - `Attestor`: Remote attestation

### Layer 2: Platform (ServiceOS)
- **Location**: `platform/`
- **Responsibility**: Abstract TEE details, provide Android-style APIs
- **Key Principle**: Services interact with OS, not directly with TEE
- **Components**:
  - `ServiceOS`: Main interface (like Android Context)
  - `Capability`: Permission system
  - `Manifest`: Service declarations
  - Runtime, Events, Storage, API

### Layer 3: Services
- **Location**: `services/`
- **Responsibility**: Business logic implementation
- **Key Principle**: Depend ONLY on ServiceOS interface
- **Pattern**: Each service has:
  - `service.go`: Main service logic
  - `enclave.go`: TEE-protected operations
  - `store.go`: Data persistence
  - `domain.go`: Domain models

## Import Rules

```
services/* → platform/os (ServiceOS only)
platform/* → tee/types (types only)
tee/*      → (no external dependencies)
```

## Capability Model (Android-style Permissions)

Services declare required capabilities in their manifest:

```go
manifest := &Manifest{
    ServiceID: "oracle",
    RequiredCapabilities: []Capability{
        CapSecrets,   // Access to secret storage
        CapNetwork,   // Outbound network requests
        CapKeys,      // Key derivation and signing
    },
}
```

ServiceOS grants or denies access based on manifest.

## Security Principles

1. **Secrets Never Leave TEE**: Use callback pattern `Use(ctx, name, fn)`
2. **Keys Stay in Enclave**: Only KeyHandle references exported
3. **TLS Inside Enclave**: Network credentials injected inside TEE
4. **Capability Enforcement**: Services can only access granted APIs
5. **Audit Logging**: All sensitive operations logged

## Implementation Status

### TEE Layer (Complete)
```
tee/
├── types/types.go          ✅ All interfaces (TrustRoot, SecureVault, etc.)
├── enclave/runtime.go      ✅ Enclave runtime abstraction
├── vault/vault.go          ✅ SecureVault implementation
├── keys/manager.go         ✅ HSM-like key management
├── network/http.go         ✅ Secure HTTP client
├── compute/engine.go       ✅ Confidential compute engine
├── attestation/attestor.go ✅ Remote attestation
├── bridge/socket.go        ✅ Raw I/O bridge
├── bridge/storage.go       ✅ Encrypted storage bridge
└── trust_root.go           ✅ Main TrustRoot implementation
```

### Platform Layer (Complete)
```
platform/
└── os/
    ├── types.go            ✅ ServiceOS interface, Capabilities, APIs
    └── context.go          ✅ ServiceContext implementation
```

### Services Layer (Complete)
```
services/
├── base/
│   ├── service.go          ✅ BaseService, Registry
│   ├── enclave.go          ✅ BaseEnclave with helpers
│   └── store.go            ✅ BaseEntity, MemoryStore
└── oracle/
    ├── service.go          ✅ Oracle service implementation
    ├── enclave.go          ✅ TEE operations (SecureFetch, Sign)
    ├── store.go            ✅ Data persistence
    └── domain.go           ✅ Domain models (DataFeed, etc.)
```

## Usage Example

### Creating a New Service

```go
package myservice

import (
    "context"
    "github.com/R3E-Network/service_layer/platform/os"
    "github.com/R3E-Network/service_layer/services/base"
)

const ServiceID = "myservice"

// Manifest declares required capabilities
func Manifest() *os.Manifest {
    return &os.Manifest{
        ServiceID: ServiceID,
        RequiredCapabilities: []os.Capability{
            os.CapSecrets,
            os.CapNetwork,
        },
    }
}

// Service implements the service
type Service struct {
    *base.BaseService
    enclave *Enclave
}

// New creates the service
func New(serviceOS os.ServiceOS) (*Service, error) {
    return &Service{
        BaseService: base.NewBaseService(ServiceID, "My Service", "1.0.0", serviceOS),
        enclave:     NewEnclave(serviceOS),
    }, nil
}

// Enclave handles TEE operations
type Enclave struct {
    *base.BaseEnclave
}

func NewEnclave(serviceOS os.ServiceOS) *Enclave {
    return &Enclave{
        BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
    }
}

// SecureOperation uses secrets inside TEE
func (e *Enclave) SecureOperation(ctx context.Context) error {
    return e.UseSecret(ctx, "api_key", func(secret []byte) error {
        // Secret is ONLY available here, zeroed after return
        return nil
    })
}
```

## Migration Guide

To migrate existing services to the new architecture:

1. **Create service manifest** with required capabilities
2. **Extend BaseService** for lifecycle management
3. **Extend BaseEnclave** for TEE operations
4. **Use ServiceOS APIs** instead of direct TEE access
5. **Use callback pattern** for secrets: `Use(ctx, name, fn)`
