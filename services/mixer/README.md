# Mixer Service

Privacy-preserving transaction mixing service for the Neo Service Layer.

## Overview

The Mixer service provides privacy mixing for Neo N3 tokens (GAS, NEO). It uses an off-chain mixing approach with TEE proofs and on-chain dispute resolution only when needed.

## Privacy-First Fee Model

**Users NEVER connect to any known service layer account.** This is achieved by:

1. **Direct Pool Deposit**: User deposits directly to an anonymous pool account (not gasbank, not any known service layer address)
2. **Fee Deduction**: Fee is deducted from delivery amount (user receives `NetAmount = TotalAmount - ServiceFee`)
3. **Random Fee Collection**: After mixing, fee is collected from a **randomly selected pool account** to the master fee address
4. **Zero Direct Link**: No transaction directly connects user to service layer identity

```
User deposits:  100 GAS  →  Pool Account (anonymous)
User receives:  99.5 GAS →  Target Address
Fee collected:  0.5 GAS  →  Random Pool Account → Master Fee Address
```

## Fee Collection Mechanism

After mixing is complete and tokens are delivered to target addresses:

1. **Random Selection**: A pool account with sufficient balance is randomly selected
2. **Fee Transfer**: The service fee is transferred from the random pool account to the master fee address
3. **Privacy Preserved**: The fee transfer is not directly linked to the user's original deposit

This ensures:
- User's deposit address is never directly connected to the fee collection
- Fee collection happens from a random intermediate account
- The mixing process breaks the link between deposit and fee payment

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Mixer Service│     │ AccountPool  │     │ Master Fee   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ Request Mix        │                    │                    │
       │───────────────────>│                    │                    │
       │                    │ Lock Pool Account  │                    │
       │                    │───────────────────>│                    │
       │ RequestProof +     │                    │                    │
       │ Deposit Address    │                    │                    │
       │<───────────────────│                    │                    │
       │                    │                    │                    │
       │ Deposit to Pool    │                    │                    │
       │ (On-Chain, Direct) │                    │                    │
       │--------------------│------------------->│                    │
       │                    │                    │                    │
       │                    │ Execute Mixing     │                    │
       │                    │ (Off-Chain)        │                    │
       │                    │                    │                    │
       │ NetAmount Delivered│                    │                    │
       │ (Fee Deducted)     │                    │                    │
       │<───────────────────│                    │                    │
       │                    │                    │                    │
       │                    │ Collect Fee from   │                    │
       │                    │ Random Pool Account│                    │
       │                    │───────────────────>│───────────────────>│
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

1. **Create Request**: User submits mix request with target addresses
2. **Receive Proof**: Service returns `RequestProof` (TEE-signed commitment) + deposit address
3. **Direct Deposit**: User deposits tokens directly to pool account on-chain (NOT gasbank)
4. **Confirm Deposit**: User confirms deposit with tx hash
5. **Mixing**: Service executes mixing through pool accounts (off-chain)
6. **Delivery**: `NetAmount` (TotalAmount - Fee) delivered to target addresses
7. **Fee Collection**: ServiceFee collected from random pool account to master fee address
8. **Completion**: `CompletionProof` generated (stored, not on-chain unless disputed)

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

**Note**: User deposits `total_amount` but receives `net_amount` at target addresses. The `service_fee` is collected from a random pool account to the master fee address.

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

## Privacy Guarantees

- **No Direct Account Link**: User never directly transacts with any known service layer account
- **Anonymous Deposit**: Deposit address is a pool account with no public association
- **Random Fee Collection**: Fee is collected from random pool account, not user's deposit
- **Off-Chain Mixing**: All mixing happens off-chain, only dispute goes on-chain
- **TEE Proofs**: Cryptographic proofs without revealing transaction graph

## Configuration

### Required Secrets

| Secret | Description |
|--------|-------------|
| `MIXER_MASTER_KEY` | HMAC signing key for proofs |
| `MIXER_FEE_ADDRESS` | Master account address for fee collection |

### Config Options

| Option | Description |
|--------|-------------|
| `FeeCollectionAddress` | Master account address to receive collected fees |

## Data Layer

The Mixer service uses a service-specific Supabase repository for database operations.

### Package Structure

```
services/mixer/
├── supabase/
│   ├── repository.go    # Mixer-specific repository interface
│   └── models.go        # Mixer data models (Request, MixOperation)
├── service.go           # Service implementation
├── handlers.go          # HTTP handlers
├── mixing.go            # Mixing logic
├── pool.go              # Pool management
└── README.md
```

### Repository Interface

```go
import mixersupabase "github.com/R3E-Network/service_layer/services/mixer/supabase"

// Create repository
mixerRepo := mixersupabase.NewRepository(baseRepo)

// Operations
err := mixerRepo.Create(ctx, &mixersupabase.Request{...})
req, err := mixerRepo.GetByID(ctx, "mix-123")
requests, err := mixerRepo.ListByStatus(ctx, "mixing")
err := mixerRepo.UpdateStatus(ctx, id, "delivered")
```

### Data Models

| Model | Description |
|-------|-------------|
| `Request` | Mix request with targets, amounts, status |
| `MixOperation` | Individual mixing operation record |

## Testing

```bash
go test ./services/mixer/... -v -cover
```

Current test coverage: **22.5%**

## Version

- Service ID: `mixer`
- Version: `3.4.0`
