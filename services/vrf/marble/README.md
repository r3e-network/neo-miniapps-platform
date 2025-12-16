# NeoRand Marble Service

TEE-secured Verifiable Random Function (VRF) service running inside MarbleRun enclave.

## Overview

The NeoRand Marble service generates cryptographically verifiable random numbers using ECDSA P-256 VRF construction. It implements the **Request-Callback Pattern**:
1. User contract requests randomness on-chain
2. TEE generates VRF proof and random words
3. TEE fulfills request via callback to user contract

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                      │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   Event     │    │     VRF     │    │  Fulfiller  │        │
│  │  Listener   │───>│   Engine    │───>│  (Callback) │        │
│  └─────────────┘    └─────────────┘    └──────┬──────┘        │
│                                               │               │
│  ┌─────────────┐                              │               │
│  │  Supabase   │                              │               │
│  │ Repository  │                              │               │
│  └─────────────┘                              │               │
└───────────────────────────────────────────────┼───────────────┘
                                                │
                              ┌─────────────────┼─────────────────┐
                              ▼                 ▼                 │
                       ┌─────────────┐   ┌─────────────┐          │
                       │ VRF Contract│   │User Contract│          │
                       │ (Request)   │   │ (Callback)  │          │
                       └─────────────┘   └─────────────┘          │
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization and configuration |
| `core.go` | VRF generation logic |
| `fulfiller.go` | On-chain fulfillment |
| `listener.go` | Event listener for requests |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `types.go` | Data structures |

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    mu sync.RWMutex

    privateKey *ecdsa.PrivateKey  // VRF signing key

    repo neorandsupabase.RepositoryInterface

    chainClient   *chain.Client
    teeFulfiller  *chain.TEEFulfiller
    eventListener *chain.EventListener

    requests        map[string]*Request
    pendingRequests chan *Request
}
```

### Configuration

```go
type Config struct {
    Marble        *marble.Marble
    DB            database.RepositoryInterface
    NeoRandRepo   neorandsupabase.RepositoryInterface
    ChainClient   *chain.Client
    TEEFulfiller  *chain.TEEFulfiller
    EventListener *chain.EventListener
}
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status and statistics |
| `/pubkey` | GET | Get VRF public key |
| `/random` | POST | Direct randomness generation |
| `/verify` | POST | Verify VRF proof |
| `/request` | POST | Create on-chain request |
| `/request/{id}` | GET | Get request status |
| `/requests` | GET | List user's requests |

## VRF Algorithm

The service uses ECDSA P-256 with a deterministic VRF construction:

```go
// VRF Generation
1. Input: seed (from user), privateKey (TEE secret)
2. Generate: proof = VRF_prove(privateKey, seed)
3. Derive: randomWords = VRF_hash(proof)
4. Output: (randomWords, proof) for on-chain verification
```

## Background Workers

### Event Listener

Monitors Neo N3 blockchain for `VRFRequest` events:

```go
func (s *Service) runEventListener(ctx context.Context) {
    // Subscribe to VRFRequest events
    // Parse events and queue for fulfillment
}
```

### Request Fulfiller

Processes pending requests and submits fulfillments:

```go
func (s *Service) runRequestFulfiller(ctx context.Context) error {
    // Generate VRF proof for each request
    // Call fulfillRandomness on user contract
}
```

## Required Secrets

| Secret Name | Description |
|-------------|-------------|
| `VRF_PRIVATE_KEY` | ECDSA P-256 private key (32 bytes) |

**Note**: Key remains constant across enclave upgrades as it's injected from MarbleRun manifest, not derived from SGX sealing keys.

## Dependencies

### Infrastructure Packages

| Package | Purpose |
|---------|---------|
| `infrastructure/chain` | Neo N3 blockchain interaction |
| `infrastructure/crypto` | VRF key generation |
| `infrastructure/marble` | MarbleRun TEE utilities |
| `infrastructure/service` | Base service implementation |
| `services/vrf/supabase` | Service-specific repository |

## Testing

```bash
go test ./services/vrf/marble/... -v -cover
```

## Related Documentation

- [NeoRand Service Overview](../README.md)
- [Chain Integration](../../../infrastructure/chain/README.md)
- [Smart Contract](../contract/README.md)
- [Database Layer](../supabase/README.md)
