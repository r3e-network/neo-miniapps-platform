# VRF Service

Verifiable Random Function (VRF) service for the Neo Service Layer.

## Overview

The VRF service provides cryptographically verifiable random numbers for smart contracts on Neo N3. It uses ECDSA P-256 with a deterministic VRF construction to generate random numbers that can be verified on-chain.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ User Contract│     │ VRF Contract │     │ TEE (VRF Svc)│
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ requestRandomness  │                    │
       │───────────────────>│                    │
       │                    │ RandomnessRequested│
       │                    │───────────────────>│
       │                    │                    │
       │                    │                    │ Generate VRF
       │                    │                    │
       │                    │ fulfillRandomness  │
       │                    │<───────────────────│
       │ Callback           │                    │
       │<───────────────────│                    │
```

## API Endpoints

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

## Request/Response Types

### Direct Random Request

```json
POST /random
{
    "seed": "user-provided-seed-string",
    "num_words": 3
}
```

### Direct Random Response

```json
{
    "request_id": "uuid",
    "seed": "user-provided-seed-string",
    "random_words": ["0x...", "0x...", "0x..."],
    "proof": "0x...",
    "public_key": "0x...",
    "timestamp": "2025-12-08T00:00:00Z"
}
```

## Configuration

### Required Secrets

| Secret | Description |
|--------|-------------|
| `VRF_PRIVATE_KEY` | ECDSA P-256 private key (32 bytes hex) |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SUPABASE_URL` | Supabase project URL |
| `SUPABASE_SERVICE_KEY` | Supabase service role key |

## Usage Example

### Off-Chain (Direct API)

```bash
curl -X POST http://localhost:8080/api/v1/vrf/random \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"seed": "my-unique-seed", "num_words": 1}'
```

### On-Chain (Smart Contract)

```csharp
// In your Neo N3 smart contract
public static void RequestRandom(byte[] seed) {
    Contract.Call(VRFServiceHash, "requestRandomness", CallFlags.All, seed, 1);
}

// Callback method (called by VRF service)
public static void fulfillRandomness(BigInteger requestId, byte[][] randomWords) {
    // Use random words...
}
```

## Security

- Private key never leaves the TEE enclave
- VRF proof is deterministic and verifiable
- Requests are signed with TEE attestation

## Data Layer

The VRF service uses a service-specific Supabase repository for database operations.

### Package Structure

```
services/vrf/
├── supabase/
│   ├── repository.go    # VRF-specific repository interface
│   └── models.go        # VRF data models (Request, Response)
├── vrf.go               # Service implementation
└── README.md
```

### Repository Interface

```go
import vrfsupabase "github.com/R3E-Network/service_layer/services/vrf/supabase"

// Create repository
vrfRepo := vrfsupabase.NewRepository(baseRepo)

// Operations
err := vrfRepo.Create(ctx, &vrfsupabase.Request{...})
req, err := vrfRepo.GetByRequestID(ctx, "vrf-123")
requests, err := vrfRepo.ListByStatus(ctx, "pending")
err := vrfRepo.UpdateStatus(ctx, id, "completed")
```

### Data Models

| Model | Description |
|-------|-------------|
| `Request` | VRF request with seed, status, timestamps |
| `Response` | VRF response with random words, proof |

## Testing

```bash
go test ./services/vrf/... -v -cover
```

Current test coverage: **28.6%**

## Version

- Service ID: `vrf`
- Version: `2.0.0`
