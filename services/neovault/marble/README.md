# NeoVault Marble Service

TEE-secured privacy-preserving transaction mixing service running inside MarbleRun enclave.

## Overview

The NeoVault Marble service is the core TEE component that:
1. Receives mix requests from users via HTTP API
2. Generates TEE-signed request proofs for dispute claims
3. Manages pool account allocation via NeoAccounts service
4. Executes off-chain mixing through pool accounts
5. Delivers tokens (minus fees) to target addresses
6. Provides completion proofs for dispute resolution

This service implements an **Off-Chain First** architecture where:
- Normal operations happen entirely off-chain
- On-chain interaction only occurs during disputes
- User privacy is preserved by never linking to service layer accounts

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                    │
│                                                               │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   Request   │    │   Mixing    │    │   Dispute   │        │
│  │  Handlers   │───>│   Engine    │───>│  Resolver   │        │
│  └─────────────┘    └─────────────┘    └──────┬──────┘        │
│         │                  │                  │               │
│         │                  │                  │               │
│  ┌──────▼──────┐    ┌──────▼──────┐           │               │
│  │  Supabase   │    │ NeoAccounts │           │               │
│  │ Repository  │    │   Client    │           │               │
│  └─────────────┘    └─────────────┘           │               │
└───────────────────────────────────────────────┼───────────────┘
                                                │
                              ┌─────────────────┼───────────────┐
                              ▼                 ▼               │
                       ┌─────────────┐   ┌─────────────┐        │
                       │ Pool Accts  │   │NeoVaultSvc  │        │
                       │ (Off-Chain) │   │ (Dispute)   │        │
                       └─────────────┘   └─────────────┘        │
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization and configuration |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `mixing.go` | Mixing execution logic |
| `pool.go` | Pool account management |
| `proofs.go` | Request/completion proof generation |
| `types.go` | Data structures |

Lifecycle is handled by the shared `commonservice.BaseService` (start/stop hooks, workers, standard routes).

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    mu sync.RWMutex

    // TEE signing key for proofs
    masterKey []byte

    // Service-specific repository
    repo neovaultsupabase.RepositoryInterface

    // Per-token configuration (limits, fees)
    tokenConfigs map[string]*TokenConfig

    // Account pool integration
    neoAccountsURL string

    // Fee collection address
    feeCollectionAddress string

    // Chain interaction (for disputes only)
    chainClient  *chain.Client
    teeFulfiller *chain.TEEFulfiller
    gateway      *chain.GatewayContract
}
```

### Token Configuration

```go
type TokenConfig struct {
    TokenType        string  `json:"token_type"`
    ScriptHash       string  `json:"script_hash"`
    MinTxAmount      int64   `json:"min_tx_amount"`
    MaxTxAmount      int64   `json:"max_tx_amount"`
    MaxRequestAmount int64   `json:"max_request_amount"`
    MaxPoolBalance   int64   `json:"max_pool_balance"`
    ServiceFeeRate   float64 `json:"service_fee_rate"`
}
```

Default configurations:
- **GAS**: Min 0.001, Max 1.0, Fee 0.5%
- **NEO**: Min 1, Max 1000, Fee 0.5%

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status and pool statistics |
| `/request` | POST | Create new mix request |
| `/status/{id}` | GET | Get request status |
| `/request/{id}` | GET | Get full request details |
| `/request/{id}/deposit` | POST | Confirm user deposit |
| `/request/{id}/resume` | POST | Resume mixing |
| `/request/{id}/dispute` | POST | Submit dispute |
| `/request/{id}/proof` | GET | Get completion proof |
| `/requests` | GET | List user's requests |

## Privacy-First Fee Model

Users NEVER connect to any known service layer account:

```
User deposits:  100 GAS  →  Anonymous Pool Account
User receives:  99.5 GAS →  Target Address(es)
Fee collected:  0.5 GAS  →  Random Pool Account → Master Fee Address
```

Key privacy guarantees:
- User deposits to anonymous pool account (not linked to service)
- Fee is deducted from delivery amount
- Fee collection from random pool preserves privacy
- No on-chain link between user and service layer

## Request Flow

```
1. User: POST /request
   └── Service returns: RequestProof + DepositAddress

2. User: Deposit to pool account (on-chain, direct)
   └── No service layer account involved

3. User: POST /request/{id}/deposit
   └── Service verifies deposit via chain

4. Service: Execute mixing (off-chain)
   └── Tokens shuffled through pool accounts

5. Service: Deliver NetAmount to targets
   └── Fee deducted from delivery

6. Service: Collect fee from random pool account
   └── Sent to master fee address

7. Normal path: Complete
   └── Nothing on-chain links user to service

8. Dispute path: User submits dispute
   └── Service submits CompletionProof on-chain
```

## Status Values

| Status | Description |
|--------|-------------|
| `pending` | Awaiting user deposit |
| `deposited` | Deposit confirmed, queued for mixing |
| `mixing` | Mixing in progress |
| `delivered` | Tokens delivered to targets |
| `failed` | Mix failed |
| `refunded` | Tokens refunded to user |

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/chain` | Neo N3 blockchain interaction |
| `internal/marble` | MarbleRun TEE utilities |
| `internal/database` | Base repository interface |
| `services/common/service` | Base service implementation |
| `services/neovault/supabase` | Service-specific repository |

### External Services

| Service | Purpose |
|---------|---------|
| NeoAccounts | Pool account allocation |
| Supabase | Request persistence |

## Required Secrets

| Secret Name | Description |
|-------------|-------------|
| `NEOVAULT_MASTER_KEY` | HMAC key for signing proofs |
| `NEOVAULT_FEE_ADDRESS` | Master account for fee collection |

## Configuration

```go
type Config struct {
    Marble               *marble.Marble
    DB                   database.RepositoryInterface
    NeoVaultRepo         neovaultsupabase.RepositoryInterface
    ChainClient          *chain.Client
    TEEFulfiller         *chain.TEEFulfiller
    Gateway              *chain.GatewayContract
    TokenConfigs         map[string]*TokenConfig
    NeoAccountsURL       string
    FeeCollectionAddress string
}
```

## Mixing Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `MinMixingTxPerMinute` | 5 | Minimum transactions per minute |
| `MaxMixingTxPerMinute` | 20 | Maximum transactions per minute |
| `DisputeGracePeriod` | 7 days | Time window for user disputes |

## Security Model

### TEE Proofs

- **RequestProof**: `Hash256(request) + TEE signature`
  - Generated when user creates request
  - User stores for dispute claim

- **CompletionProof**: `Hash256(outputs) + TEE signature`
  - Generated when mixing completes
  - Stored in database (not on-chain)
  - Submitted on-chain only if disputed

### Compliance Limits

- Maximum ≤10,000 per request
- Maximum ≤100,000 total pool balance
- Configurable per token type

## Testing

```bash
# Run unit tests
go test ./services/neovault/marble/... -v

# Run with coverage
go test ./services/neovault/marble/... -v -cover

# Run integration tests
go test ./services/neovault/... -tags=integration -v
```

## Related Documentation

- [NeoVault Service Overview](../README.md)
- [Chain Integration](../chain/README.md)
- [Smart Contract](../contract/README.md)
- [Database Layer](../supabase/README.md)
