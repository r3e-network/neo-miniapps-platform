# NeoAccounts Marble Service

TEE-secured HD-derived account pool management service running inside MarbleRun enclave.

## Overview

The NeoAccounts Marble service manages a pool of Neo N3 accounts derived from a master key:
1. Accounts are derived on-demand using HKDF from master key
2. Other services (NeoVault, etc.) can request and lock accounts
3. Private keys never leave the TEE - signing done internally
4. Automatic account rotation and pool maintenance

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                      │
│                                                                 │
│    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│    │   Handler   │    │   Pool      │    │  Key        │        │
│    │  (REST API) │───>│  Manager    │<──>│  Deriver    │        │
│    └─────────────┘    └──────┬──────┘    └──────┬──────┘        │
│           │                  │                  │               │
│    ┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐        │
│    │   Signing   │    │  Account    │    │  Master Key │        │
│    │   Service   │    │  Rotation   │    │   (Sealed)  │        │
│    └─────────────┘    └─────────────┘    └─────────────┘        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │    Supabase     │
                    │  (Account Pool) │
                    └─────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization, key derivation |
| `lifecycle.go` | Service lifecycle (Start/Stop) |
| `pool.go` | Pool management, request/release |
| `signing.go` | Transaction signing |
| `masterkey.go` | Master key handling |
| `attestation.go` | TEE attestation |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `types.go` | Request/response types |

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    mu sync.RWMutex

    // Secrets
    masterKey              []byte
    masterPubKey           []byte
    masterKeyHash          []byte
    masterKeyAttestationID string

    // Service-specific repository
    repo neoaccountssupabase.RepositoryInterface

    // Chain interaction
    chainClient *chain.Client
}
```

### HD Key Derivation

Accounts are derived deterministically from master key:

```go
func (s *Service) deriveAccountKey(accountID string) ([]byte, error) {
    return crypto.DeriveKey(s.masterKey, []byte(accountID), "pool-account", 32)
}
```

**Upgrade Safety**: Key derivation uses only:
- `masterKey`: From MarbleRun injection (stable across upgrades)
- `accountID`: Business identifier (stable)
- `"pool-account"`: Service context (code constant)

NO enclave identity (MRENCLAVE/MRSIGNER) is used in derivation.

## Pool Configuration

| Constant | Value | Description |
|----------|-------|-------------|
| `MinPoolAccounts` | 200 | Minimum pool size |
| `MaxPoolAccounts` | 10000 | Maximum pool size |
| `RotationRate` | 10% | Daily rotation rate |
| `RotationMinAge` | 24h | Minimum age before rotation |
| `LockTimeout` | 24h | Stale lock timeout |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Pool statistics |
| `/accounts` | GET | List accounts by service |
| `/request` | POST | Request and lock accounts |
| `/release` | POST | Release locked accounts |
| `/sign` | POST | Sign transaction hash |
| `/batch-sign` | POST | Sign multiple transactions |
| `/balance` | POST | Update account balance |

## Request/Response Types

### RequestAccountsInput

```go
type RequestAccountsInput struct {
    ServiceID string `json:"service_id"` // ID of requesting service
    Count     int    `json:"count"`      // Number of accounts (1-100)
    Purpose   string `json:"purpose"`    // Audit description
}
```

### AccountInfo

```go
type AccountInfo struct {
    ID         string    `json:"id"`
    Address    string    `json:"address"`
    Balance    int64     `json:"balance"`
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at"`
    TxCount    int64     `json:"tx_count"`
    IsRetiring bool      `json:"is_retiring"`
    LockedBy   string    `json:"locked_by,omitempty"`
    LockedAt   time.Time `json:"locked_at,omitempty"`
}
```

### SignTransactionInput

```go
type SignTransactionInput struct {
    ServiceID string `json:"service_id"`
    AccountID string `json:"account_id"`
    TxHash    []byte `json:"tx_hash"`
}
```

### SignTransactionResponse

```go
type SignTransactionResponse struct {
    AccountID string `json:"account_id"`
    Signature []byte `json:"signature"`
    PublicKey []byte `json:"public_key"`
}
```

### PoolInfoResponse

```go
type PoolInfoResponse struct {
    TotalAccounts    int   `json:"total_accounts"`
    ActiveAccounts   int   `json:"active_accounts"`
    LockedAccounts   int   `json:"locked_accounts"`
    RetiringAccounts int   `json:"retiring_accounts"`
    TotalBalance     int64 `json:"total_balance"`
}
```

## Configuration

```go
type Config struct {
    Marble          *marble.Marble
    DB              database.RepositoryInterface
    NeoAccountsRepo neoaccountssupabase.RepositoryInterface
    ChainClient     *chain.Client
}
```

### Required Secrets

| Secret | Description |
|--------|-------------|
| `POOL_MASTER_KEY` | 32-byte HD wallet master key |

## Security Features

### Private Key Protection

- Master key never leaves MarbleRun TEE
- Private keys derived on-demand, zeroed after use
- Signatures computed inside TEE
- Only public info (address, balance) exposed via API

### Account Locking

- Services must lock accounts before use
- Only locking service can sign or modify balance
- Stale locks automatically cleaned up after 24h

### Account Rotation

- 10% of accounts rotated daily
- Locked accounts NEVER rotated
- Retiring accounts deleted when balance reaches zero
- Ensures fresh, unlinkable accounts

## Background Workers

### Account Rotation Worker

Runs hourly to:
- Mark old, low-balance accounts as retiring
- Create new accounts to maintain minimum pool size
- Delete empty retiring accounts

### Lock Cleanup Worker

Runs hourly to:
- Detect stale locks (>24h)
- Force-release abandoned accounts

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/chain` | Neo N3 blockchain interaction |
| `internal/crypto` | Key derivation, signing |
| `internal/marble` | MarbleRun TEE utilities |
| `internal/database` | Base repository |
| `services/common/service` | Base service framework |
| `services/neoaccounts/supabase` | Account repository |

### External Packages

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP router |
| `github.com/google/uuid` | Account/Lock ID generation |

## Related Documentation

- [NeoAccounts Service Overview](../README.md)
- [Database Layer](../supabase/README.md)
