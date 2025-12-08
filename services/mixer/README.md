# Mixer Service

Privacy-preserving transaction mixing service for the Neo Service Layer.

## Overview

The Mixer service provides privacy mixing for Neo N3 tokens (GAS, NEO). It uses an off-chain mixing approach with TEE proofs and on-chain dispute resolution only when needed.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Mixer Service│     │ AccountPool  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Request Mix        │                    │
       │───────────────────>│                    │
       │                    │                    │
       │ RequestProof       │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Deposit to GasBank │                    │
       │────────────────────────────────────────>│
       │                    │                    │
       │                    │ Lock Pool Account  │
       │                    │───────────────────>│
       │                    │                    │
       │                    │ Execute Mixing     │
       │                    │                    │
       │ Tokens Delivered   │                    │
       │<───────────────────│                    │
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status and pool statistics |
| `/request` | POST | Create mix request |
| `/status/{id}` | GET | Get mix request status |
| `/request/{id}` | GET | Get full request details |
| `/request/{id}/deposit` | POST | Confirm deposit |
| `/request/{id}/resume` | POST | Resume mixing |
| `/request/{id}/dispute` | POST | Submit dispute |
| `/request/{id}/proof` | GET | Get completion proof |
| `/requests` | GET | List user's requests |

## Supported Tokens

| Token | Script Hash | Min Amount | Max Amount | Fee Rate |
|-------|-------------|------------|------------|----------|
| GAS | `0xd2a4cff31913016155e38e474a2c06d08be276cf` | 0.001 | 1.0 | 0.5% |
| NEO | `0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5` | 1 | 1000 | 0.5% |

## Request Flow

1. **Create Request**: User submits mix request with targets
2. **Receive Proof**: Service returns `RequestProof` (TEE-signed commitment)
3. **Deposit**: User deposits tokens to provided address
4. **Confirm Deposit**: User confirms deposit with tx hash
5. **Mixing**: Service executes mixing through pool accounts
6. **Delivery**: Tokens delivered to target addresses
7. **Completion**: `CompletionProof` generated (stored, not on-chain)

## Request/Response Types

### Create Request

```json
POST /request
{
    "version": 1,
    "token_type": "GAS",
    "user_address": "NAddr...",
    "targets": [
        {"address": "NTarget1...", "amount": 50000000},
        {"address": "NTarget2...", "amount": 50000000}
    ],
    "mix_option": 1800000,
    "timestamp": 1733616000
}
```

### Create Response

```json
{
    "request_id": "uuid",
    "request_hash": "0x...",
    "tee_signature": "0x...",
    "deposit_address": "NDeposit...",
    "total_amount": 100000000,
    "service_fee": 500000,
    "net_amount": 99500000,
    "deadline": 1733702400,
    "expires_at": "2025-12-09T00:00:00Z"
}
```

## Status Values

| Status | Description |
|--------|-------------|
| `pending` | Awaiting deposit |
| `deposited` | Deposit confirmed, mixing queued |
| `mixing` | Mixing in progress |
| `delivered` | Tokens delivered to targets |
| `failed` | Mix failed |
| `refunded` | Tokens refunded |

## Dispute Mechanism

If mixing is not completed by the deadline:
1. User can submit dispute via `/request/{id}/dispute`
2. Service submits `CompletionProof` on-chain (if completed)
3. If not completed, user can claim refund via on-chain dispute contract

## Configuration

### Required Secrets

| Secret | Description |
|--------|-------------|
| `MIXER_MASTER_KEY` | HMAC signing key for proofs |

## Testing

```bash
go test ./services/mixer/... -v -cover
```

Current test coverage: **22.5%**

## Version

- Service ID: `mixer`
- Version: `3.2.0`
