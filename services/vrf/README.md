# NeoVRF Service

TEE-backed verifiable randomness service for the Neo Mini-App Platform.

## Overview

NeoVRF generates deterministic randomness derived from a TEE-held signing key.
Each request signs the `request_id`, derives randomness from the signature, and
returns the signature + public key for verification. Results can be anchored on
Neo N3 via `RandomnessLog` using `txproxy`.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Supabase Edge│     │   NeoVRF     │     │  Randomness  │
│   Gateway    │     │  Service     │     │   Log (N3)   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ RNG Request        │                    │
       │───────────────────>│                    │
       │                    │ Sign + Derive RNG  │
       │                    │───────────────────>│
       │                    │                    │
       │ Randomness + Proof │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Optional anchor via txproxy             │
       │────────────────────────────────────────>│
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status + attestation hash |
| `/random` | POST | Generate randomness + signature |
| `/pubkey` | GET | Fetch the VRF public key |

### Random Request

```json
POST /random
{
  "request_id": "uuid-optional"
}
```

### Random Response

```json
{
  "request_id": "uuid",
  "randomness": "<hex>",
  "signature": "<hex>",
  "public_key": "<hex>",
  "attestation_hash": "<hex>",
  "timestamp": 1715352000
}
```

## Configuration

| Variable | Description |
|----------|-------------|
| `NEOVRF_SIGNING_KEY` | 32+ byte secret used to derive the VRF signing key |

In production/SGX mode, `NEOVRF_SIGNING_KEY` must be injected by MarbleRun.

## Testing

```bash
go test ./services/vrf/... -v -cover
```

## Version

- Service ID: `neovrf`
- Version: `1.0.0`
