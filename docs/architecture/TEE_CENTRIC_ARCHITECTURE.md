# TEE-Centric Architecture Refactoring Plan

## Executive Summary

This document outlines a complete architectural refactoring to make TEE (Trusted Execution Environment) the **trust root** of the entire service layer. All sensitive operations - secrets management, secure network connections, data fetching, and confidential computing - must happen exclusively within the TEE enclave.

## Current Architecture Problems

### Critical Security Gaps

```
CURRENT (INSECURE):
┌─────────────────────────────────────────────────────────────────┐
│                    TEE Enclave (Trusted)                         │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  JavaScript Runtime                                          │ │
│  │  - Can request secrets                                       │ │
│  │  - Can request HTTP via OCALL                               │ │
│  └──────────────────────────┬──────────────────────────────────┘ │
└─────────────────────────────│────────────────────────────────────┘
                              │ OCALL (secrets/credentials LEAK here)
┌─────────────────────────────▼────────────────────────────────────┐
│              Go Service Engine (UNTRUSTED)                        │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  HTTP Client - API keys exposed here!                        │ │
│  │  Database Access - credentials exposed here!                 │ │
│  │  Blockchain RPC - private keys exposed here!                 │ │
│  └─────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

**Problems:**
1. **Secrets leak via OCALL**: When TEE requests HTTP, credentials are passed to untrusted layer
2. **Network in untrusted layer**: All HTTP/RPC happens outside TEE - responses can be tampered
3. **No data confidentiality**: Data fetched from network is processed in untrusted layer first
4. **Keys exposed**: API keys, private keys visible in Go memory (untrusted)

## Target Architecture: TEE as Trust Root

```
TARGET (SECURE):
┌──────────────────────────────────────────────────────────────────────────┐
│                         TEE ENCLAVE (Trust Root)                          │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                      Secure Network Stack                           │  │
│  │  - TLS termination inside enclave                                  │  │
│  │  - Certificate pinning                                             │  │
│  │  - Request signing with enclave key                                │  │
│  │  - Response verification                                           │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                      Secret Vault (Sealed)                          │  │
│  │  - All secrets encrypted with enclave sealing key                  │  │
│  │  - Secrets NEVER leave enclave in plaintext                        │  │
│  │  - Access audit logging                                            │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                      Confidential Compute                           │  │
│  │  - All sensitive computation in enclave                            │  │
│  │  - Encrypted state persistence                                     │  │
│  │  - Continuous attestation                                          │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                      Key Management (HSM-like)                      │  │
│  │  - Master key sealed to enclave                                    │  │
│  │  - HD key derivation inside enclave                                │  │
│  │  - Signing operations inside enclave                               │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
                              │
                              │ Encrypted Channel Only
                              │ (No plaintext secrets cross this boundary)
                              ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    Untrusted Layer (Minimal)                              │
│  - Raw socket I/O only (encrypted data passthrough)                      │
│  - Storage backend (encrypted blobs only)                                │
│  - Logging (sanitized, no secrets)                                       │
└──────────────────────────────────────────────────────────────────────────┘
```

## Core Principles

### 1. Secrets Never Leave TEE in Plaintext
- All secrets stored encrypted with enclave sealing key
- When a service needs an API key for HTTP, the HTTP request is made INSIDE the enclave
- Credentials are injected into requests inside enclave, never exposed to Go layer

### 2. Network Operations Inside TEE
- TLS handshake and termination inside enclave
- Certificate verification inside enclave
- Only encrypted TCP packets cross enclave boundary
- Response data decrypted and processed inside enclave

### 3. All Sensitive Computation in TEE
- Price calculations, VRF generation, oracle aggregation - all in enclave
- State encrypted before leaving enclave
- Results signed by enclave key for verification

### 4. Minimal Untrusted Surface
- Untrusted layer only handles:
  - Raw socket I/O (encrypted bytes)
  - Encrypted blob storage
  - System resource management
  - Sanitized logging

## New Directory Structure

```
system/
├── tee/                          # TEE Trust Root (NEW - replaces os/tee)
│   ├── enclave/                  # Core enclave implementation
│   │   ├── runtime.go            # Enclave runtime (SGX/simulation)
│   │   ├── attestation.go        # Remote attestation
│   │   └── sealing.go            # Data sealing
│   │
│   ├── vault/                    # Secret management (INSIDE enclave)
│   │   ├── vault.go              # Sealed secret storage
│   │   ├── policy.go             # Access control policies
│   │   └── audit.go              # Access audit logging
│   │
│   ├── network/                  # Secure networking (INSIDE enclave)
│   │   ├── tls.go                # TLS stack inside enclave
│   │   ├── http.go               # HTTP client inside enclave
│   │   ├── rpc.go                # JSON-RPC client inside enclave
│   │   └── pinning.go            # Certificate pinning
│   │
│   ├── compute/                  # Confidential compute
│   │   ├── engine.go             # Script execution engine
│   │   ├── state.go              # Encrypted state management
│   │   └── proof.go              # Execution proofs
│   │
│   ├── keys/                     # Key management (HSM-like)
│   │   ├── master.go             # Master key (sealed)
│   │   ├── derivation.go         # HD key derivation
│   │   ├── signer.go             # Signing operations
│   │   └── verifier.go           # Signature verification
│   │
│   └── bridge/                   # Minimal untrusted bridge
│       ├── socket.go             # Raw socket I/O (encrypted only)
│       ├── storage.go            # Encrypted blob storage
│       └── syscall.go            # System calls
│
├── os/                           # Service OS (uses TEE as foundation)
│   ├── core/                     # Core interfaces
│   ├── framework/                # Service framework
│   ├── runtime/                  # Service runtime
│   └── sandbox/                  # Service sandboxing
│
└── engine/                       # Engine adapters
```

## Interface Definitions

### 1. TEE Root Interface

```go
// Package tee provides the Trust Root for the entire system.
package tee

// TrustRoot is the foundation of all secure operations.
// All sensitive operations MUST go through this interface.
type TrustRoot interface {
    // Vault provides sealed secret storage
    Vault() SecureVault

    // Network provides secure networking (TLS inside enclave)
    Network() SecureNetwork

    // Compute provides confidential computation
    Compute() ConfidentialCompute

    // Keys provides key management (HSM-like)
    Keys() KeyManager

    // Attestation provides remote attestation
    Attestation() Attestor
}
```

### 2. Secure Vault (Secrets NEVER leave enclave)

```go
// SecureVault manages secrets inside the TEE enclave.
// Secrets are sealed with enclave key and NEVER exposed in plaintext outside.
type SecureVault interface {
    // Store stores a secret (encrypted with enclave sealing key)
    Store(ctx context.Context, namespace, name string, value []byte) error

    // Use executes a function with access to a secret.
    // The secret value is ONLY available inside the callback.
    // This ensures secrets never leave the enclave.
    Use(ctx context.Context, namespace, name string, fn SecretConsumer) error

    // Delete removes a secret
    Delete(ctx context.Context, namespace, name string) error

    // List returns secret names (not values)
    List(ctx context.Context, namespace string) ([]string, error)
}

// SecretConsumer is called with the secret value inside the enclave.
// The secret is zeroed after the function returns.
type SecretConsumer func(secret []byte) error
```

### 3. Secure Network (TLS inside enclave)

```go
// SecureNetwork provides networking with TLS termination inside enclave.
// All credentials are injected inside the enclave, never exposed to untrusted layer.
type SecureNetwork interface {
    // Fetch performs an HTTP request with TLS inside enclave.
    // If auth is provided, credentials are retrieved from vault and injected inside enclave.
    Fetch(ctx context.Context, req SecureHTTPRequest) (*SecureHTTPResponse, error)

    // RPC performs a JSON-RPC call with TLS inside enclave.
    RPC(ctx context.Context, endpoint string, method string, params any) (json.RawMessage, error)

    // AddPinnedCert adds a pinned certificate for a host
    AddPinnedCert(host string, certHash []byte) error
}

// SecureHTTPRequest includes optional auth that's resolved inside enclave
type SecureHTTPRequest struct {
    Method  string
    URL     string
    Headers map[string]string
    Body    []byte

    // Auth specifies how to authenticate (resolved inside enclave)
    Auth *RequestAuth
}

// RequestAuth specifies authentication (credentials stay in enclave)
type RequestAuth struct {
    Type      AuthType // Bearer, Basic, APIKey, etc.
    SecretRef string   // Reference to secret in vault (e.g., "oracle/api_key")
}
```

### 4. Confidential Compute

```go
// ConfidentialCompute executes code with data confidentiality guarantees.
type ConfidentialCompute interface {
    // Execute runs code inside the enclave with:
    // - Secrets resolved inside enclave
    // - Network requests made inside enclave
    // - State encrypted before leaving enclave
    Execute(ctx context.Context, req ComputeRequest) (*ComputeResult, error)

    // LoadState loads encrypted state into enclave
    LoadState(ctx context.Context, stateID string) error

    // SaveState saves encrypted state from enclave
    SaveState(ctx context.Context, stateID string) error
}

// ComputeRequest specifies what to execute
type ComputeRequest struct {
    ServiceID  string
    Script     string
    EntryPoint string
    Input      map[string]any

    // Secrets are resolved INSIDE enclave, never exposed
    SecretRefs []string

    // Network requests are made INSIDE enclave
    AllowedHosts []string
}
```

### 5. Key Manager (HSM-like)

```go
// KeyManager provides HSM-like key management inside enclave.
// Private keys NEVER leave the enclave.
type KeyManager interface {
    // GenerateMasterKey generates and seals the master key
    GenerateMasterKey(ctx context.Context) error

    // DeriveKey derives a child key (stays in enclave)
    DeriveKey(ctx context.Context, path string) (KeyHandle, error)

    // Sign signs data with a key (signing happens inside enclave)
    Sign(ctx context.Context, keyHandle KeyHandle, data []byte) ([]byte, error)

    // Verify verifies a signature
    Verify(ctx context.Context, publicKey, data, signature []byte) (bool, error)

    // GetPublicKey returns the public key (safe to export)
    GetPublicKey(ctx context.Context, keyHandle KeyHandle) ([]byte, error)
}

// KeyHandle is an opaque reference to a key inside the enclave.
// The actual key material never leaves the enclave.
type KeyHandle string
```

## Migration Strategy

### Phase 1: Foundation (Week 1-2)
1. Create new `system/tee/` structure
2. Implement `TrustRoot` interface
3. Implement `SecureVault` with proper sealing
4. Add comprehensive audit logging

### Phase 2: Secure Networking (Week 3-4)
1. Implement TLS stack inside enclave (using mbedTLS or similar)
2. Implement `SecureNetwork` interface
3. Add certificate pinning
4. Migrate HTTP operations to use secure network

### Phase 3: Key Management (Week 5)
1. Implement `KeyManager` interface
2. Migrate master key generation to enclave
3. Implement HD key derivation inside enclave
4. Migrate signing operations to enclave

### Phase 4: Service Migration (Week 6-8)
1. Migrate Oracle service to use TEE trust root
2. Migrate Mixer service to use TEE trust root
3. Migrate Secrets service to use TEE trust root
4. Migrate remaining services

### Phase 5: Verification (Week 9-10)
1. Security audit
2. Penetration testing
3. Performance optimization
4. Documentation

## Security Guarantees

After refactoring, the system will provide:

1. **Secret Confidentiality**: Secrets never exist in plaintext outside TEE
2. **Network Integrity**: All network data verified inside TEE
3. **Computation Confidentiality**: Sensitive computations isolated in TEE
4. **Key Protection**: Private keys never leave TEE
5. **Attestation**: Remote parties can verify TEE integrity

## Breaking Changes

This refactoring will require changes to:

1. **All services using secrets**: Must use `vault.Use()` pattern
2. **All services making HTTP requests**: Must use `SecureNetwork`
3. **All services with signing**: Must use `KeyManager`
4. **Service registration**: Must specify TEE requirements

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Performance overhead | Batch operations, connection pooling inside enclave |
| TLS library in enclave | Use well-audited mbedTLS, extensive testing |
| Migration complexity | Phased approach, backward compatibility layer |
| Hardware availability | Simulation mode for development, hardware for production |

## Success Criteria

1. No secrets visible in Go layer memory dumps
2. All HTTP requests made with TLS inside enclave
3. All signing operations inside enclave
4. Remote attestation verifiable by third parties
5. All existing tests pass
6. Performance within 20% of current baseline
