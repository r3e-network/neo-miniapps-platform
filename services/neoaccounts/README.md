# NeoAccounts Service

HD-derived pool account management service for the Neo Service Layer.

## Overview

The NeoAccounts service manages a pool of Neo N3 accounts derived from a master key using HD (Hierarchical Deterministic) derivation. Other services (like NeoVault) can request accounts from the pool for their operations.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ NeoVault     │     │ AccountPool  │     │   Database   │
│ Service      │     │ Service      │     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Request Accounts   │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Lock Accounts      │
       │                    │───────────────────>│
       │                    │                    │
       │ Accounts + LockID  │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Sign Transaction   │                    │
       │───────────────────>│                    │
       │                    │                    │
       │ Signature          │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Release Accounts   │                    │
       │───────────────────>│                    │
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Pool statistics |
| `/request` | POST | Request and lock accounts |
| `/release` | POST | Release locked accounts |
| `/sign` | POST | Sign transaction hash |
| `/batch-sign` | POST | Sign multiple transactions |
| `/balance` | POST | Update account balance |

## Request/Response Types

### Request Accounts

```json
POST /request
{
    "service_id": "neovault",
    "count": 5,
    "purpose": "mixing operation"
}
```

### Request Response

```json
{
    "accounts": [
        {
            "id": "acc-1",
            "address": "NAddr1...",
            "balance": 0,
            "locked_by": "neovault"
        },
        ...
    ],
    "lock_id": "lock-123"
}
```

### Sign Transaction

```json
POST /sign
{
    "service_id": "neovault",
    "account_id": "acc-1",
    "tx_hash": "base64-encoded-hash"
}
```

### Sign Response

```json
{
    "account_id": "acc-1",
    "signature": "base64-encoded-signature",
    "public_key": "base64-encoded-pubkey"
}
```

### Batch Sign

```json
POST /batch-sign
{
    "service_id": "neovault",
    "requests": [
        {"account_id": "acc-1", "tx_hash": "..."},
        {"account_id": "acc-2", "tx_hash": "..."}
    ]
}
```

### Release Accounts

```json
POST /release
{
    "service_id": "neovault",
    "account_ids": ["acc-1", "acc-2"]
}
```

### Update Balance

```json
POST /balance
{
    "service_id": "neovault",
    "account_id": "acc-1",
    "delta": 1000000
}
```

## Pool Info Response

```json
GET /info

{
    "total_accounts": 100,
    "active_accounts": 80,
    "locked_accounts": 15,
    "retiring_accounts": 5,
    "total_balance": 1000000000
}
```

## Key Derivation

Accounts are derived using HKDF from the master key:

```
account_key = HKDF(master_key, salt=account_id, info="neo-account")
private_key = account_key mod (curve_order - 1) + 1
```

This ensures:
- Deterministic derivation (same account ID = same key)
- Keys can be regenerated from master key
- No key storage needed (derived on demand)

## Security

- Master key never leaves MarbleRun TEE
- Private keys derived on-demand, zeroed after use
- Signatures computed inside TEE
- Only public info (address, balance) exposed via API

## Configuration

### Required Secrets

| Secret | Description |
|--------|-------------|
| `POOL_MASTER_KEY` | HD wallet master key (32 bytes) |

## Data Layer

The NeoAccounts service uses a service-specific Supabase repository for database operations.

### Package Structure

```
services/accountpool/
├── supabase/
│   ├── repository.go    # AccountPool-specific repository interface
│   └── models.go        # AccountPool data models (Account, Lock)
├── accountpool.go       # Service implementation
└── README.md
```

### Repository Interface

```go
import accountpoolsupabase "github.com/R3E-Network/service_layer/services/accountpool/supabase"

// Create repository
poolRepo := accountpoolsupabase.NewRepository(baseRepo)

// Operations
err := poolRepo.Create(ctx, &accountpoolsupabase.Account{...})
accounts, err := poolRepo.ListAvailable(ctx, 10)
err := poolRepo.Update(ctx, account)
err := poolRepo.Lock(ctx, accountID, serviceID)
err := poolRepo.Unlock(ctx, accountID)
```

### Data Models

| Model | Description |
|-------|-------------|
| `Account` | Pool account with address, balance, status |
| `Lock` | Account lock record with service ID, timestamp |

## Testing

```bash
go test ./services/accountpool/... -v -cover
```

Current test coverage: **11.4%**

## Version

- Service ID: `accountpool`
- Version: `1.0.0`
