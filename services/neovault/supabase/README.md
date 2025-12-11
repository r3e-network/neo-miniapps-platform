# NeoVault Supabase Repository

Database layer for the NeoVault privacy mixing service.

## Overview

This package provides NeoVault-specific data access through a repository pattern that wraps the generic `internal/database` package. It handles persistence of:
- Mix requests and their status
- Target addresses and amounts
- TEE proofs and signatures
- Completion records

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                  NeoVault Supabase Package                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐              ┌─────────────────┐          │
│  │ RepositoryInterface           │     Models      │          │
│  ├─────────────────┤              ├─────────────────┤          │
│  │ Create           │              │ RequestRecord  │          │
│  │ Update           │              │ TargetAddress  │          │
│  │ GetByID          │              └─────────────────┘          │
│  │ GetByDepositAddr │                                          │
│  │ ListByUser       │                                          │
│  │ ListByStatus     │                                          │
│  └────────┬─────────┘                                          │
│           │                                                     │
└───────────┼─────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    internal/database                             │
│  (Repository, GenericCreate, GenericUpdate, QueryBuilder)       │
└─────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Supabase                                   │
│                 (neovault_requests table)                        │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Data models and types |

## Data Models

### RequestRecord

Main entity representing a mix request.

```go
type RequestRecord struct {
    // Identity
    ID          string `json:"id"`
    UserID      string `json:"user_id"`
    UserAddress string `json:"user_address,omitempty"`

    // Token
    TokenType string `json:"token_type"` // GAS, NEO, etc.

    // Status
    Status string `json:"status"` // pending, deposited, mixing, delivered, failed, refunded

    // Amounts
    TotalAmount int64 `json:"total_amount"`
    ServiceFee  int64 `json:"service_fee"`
    NetAmount   int64 `json:"net_amount"`

    // Targets
    TargetAddresses []TargetAddress `json:"target_addresses"`

    // Mixing Configuration
    InitialSplits         int   `json:"initial_splits"`
    MixingDurationSeconds int64 `json:"mixing_duration_seconds"`

    // Deposit
    DepositAddress string `json:"deposit_address"`
    DepositTxHash  string `json:"deposit_tx_hash,omitempty"`

    // Pool Management
    PoolAccounts []string `json:"pool_accounts"`

    // TEE Commitment (for disputes)
    RequestHash  string `json:"request_hash,omitempty"`
    TEESignature string `json:"tee_signature,omitempty"`
    Deadline     int64  `json:"deadline,omitempty"`

    // Completion
    OutputTxIDs         []string `json:"output_tx_ids,omitempty"`
    CompletionProofJSON string   `json:"completion_proof_json,omitempty"`

    // Timestamps
    CreatedAt     time.Time `json:"created_at"`
    DepositedAt   time.Time `json:"deposited_at,omitempty"`
    MixingStartAt time.Time `json:"mixing_start_at,omitempty"`
    DeliveredAt   time.Time `json:"delivered_at,omitempty"`

    // Error
    Error string `json:"error,omitempty"`
}
```

### TargetAddress

Represents a delivery target.

```go
type TargetAddress struct {
    Address string `json:"address"`
    Amount  int64  `json:"amount,omitempty"`
}
```

## Repository Interface

```go
type RepositoryInterface interface {
    // Create/Update
    Create(ctx context.Context, req *RequestRecord) error
    Update(ctx context.Context, req *RequestRecord) error

    // Read by Key
    GetByID(ctx context.Context, id string) (*RequestRecord, error)
    GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error)

    // List Queries
    ListByUser(ctx context.Context, userID string) ([]RequestRecord, error)
    ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)
}
```

## Repository Implementation

### Create

Creates a new mix request.

```go
func (r *Repository) Create(ctx context.Context, req *RequestRecord) error
```

**Validation**:
- Request cannot be nil
- UserID cannot be empty

**Behavior**:
- Uses `GenericCreate` from base repository
- Updates request with server-generated ID

### Update

Updates an existing request by ID.

```go
func (r *Repository) Update(ctx context.Context, req *RequestRecord) error
```

**Validation**:
- Request cannot be nil
- ID cannot be empty

### GetByID

Fetches a request by its ID.

```go
func (r *Repository) GetByID(ctx context.Context, id string) (*RequestRecord, error)
```

**Returns**: `NotFoundError` if not found.

### GetByDepositAddress

Fetches a request by its deposit address.

```go
func (r *Repository) GetByDepositAddress(ctx context.Context, addr string) (*RequestRecord, error)
```

Useful for matching on-chain deposits to requests.

### ListByUser

Lists all requests for a user.

```go
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]RequestRecord, error)
```

### ListByStatus

Lists requests with a specific status.

```go
func (r *Repository) ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)
```

**Valid statuses**: `pending`, `deposited`, `mixing`, `delivered`, `failed`, `refunded`

## Usage Examples

### Creating Repository

```go
import (
    "github.com/R3E-Network/service_layer/internal/database"
    neovaultsupabase "github.com/R3E-Network/service_layer/services/neovault/supabase"
)

// Create base repository
baseRepo, err := database.NewRepository(supabaseURL, supabaseKey)
if err != nil {
    return err
}

// Create service-specific repository
repo := neovaultsupabase.NewRepository(baseRepo)
```

### Creating a Mix Request

```go
req := &neovaultsupabase.RequestRecord{
    UserID:      userID,
    UserAddress: userAddr,
    TokenType:   "GAS",
    Status:      "pending",
    TotalAmount: 100000000,
    ServiceFee:  500000,
    NetAmount:   99500000,
    TargetAddresses: []neovaultsupabase.TargetAddress{
        {Address: "NTarget1...", Amount: 50000000},
        {Address: "NTarget2...", Amount: 49500000},
    },
    DepositAddress: poolAccount,
    RequestHash:    hex.EncodeToString(reqHash),
    TEESignature:   hex.EncodeToString(sig),
    Deadline:       time.Now().Add(7 * 24 * time.Hour).Unix(),
}

err := repo.Create(ctx, req)
if err != nil {
    return fmt.Errorf("create request: %w", err)
}

fmt.Printf("Created request: %s\n", req.ID)
```

### Updating Request Status

```go
req, err := repo.GetByID(ctx, requestID)
if err != nil {
    return err
}

req.Status = "deposited"
req.DepositTxHash = txHash
req.DepositedAt = time.Now()

err = repo.Update(ctx, req)
```

### Finding Request by Deposit

```go
// When detecting on-chain deposit, find matching request
req, err := repo.GetByDepositAddress(ctx, depositAddress)
if err != nil {
    if database.IsNotFoundError(err) {
        // No matching request
        return nil
    }
    return err
}

// Update status
req.Status = "deposited"
```

### Listing Pending Requests

```go
pending, err := repo.ListByStatus(ctx, "deposited")
if err != nil {
    return err
}

for _, req := range pending {
    // Process requests ready for mixing
    if time.Since(req.DepositedAt) > cooldownPeriod {
        startMixing(req)
    }
}
```

## Database Schema

### Table: `neovault_requests`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key (auto-generated) |
| `user_id` | TEXT | User identifier |
| `user_address` | TEXT | User's Neo address |
| `token_type` | TEXT | Token type (GAS, NEO) |
| `status` | TEXT | Request status |
| `total_amount` | BIGINT | Total deposit amount |
| `service_fee` | BIGINT | Calculated fee |
| `net_amount` | BIGINT | Amount to deliver |
| `target_addresses` | JSONB | Array of targets |
| `initial_splits` | INT | Mixing splits |
| `mixing_duration_seconds` | BIGINT | Duration option |
| `deposit_address` | TEXT | Pool account address |
| `deposit_tx_hash` | TEXT | On-chain deposit tx |
| `pool_accounts` | JSONB | Array of pool accounts |
| `request_hash` | TEXT | TEE commitment hash |
| `tee_signature` | TEXT | TEE signature |
| `deadline` | BIGINT | Dispute deadline timestamp |
| `output_tx_ids` | JSONB | Delivery transaction IDs |
| `completion_proof_json` | TEXT | Completion proof |
| `created_at` | TIMESTAMP | Creation time |
| `deposited_at` | TIMESTAMP | Deposit confirmation time |
| `mixing_start_at` | TIMESTAMP | Mixing start time |
| `delivered_at` | TIMESTAMP | Delivery completion time |
| `error` | TEXT | Error message if failed |

## Status Flow

```
pending ───> deposited ───> mixing ───> delivered
    │            │            │
    │            │            └──────> failed
    │            │
    │            └──────────────────> refunded
    │
    └────────────────────────────────> (expired/cancelled)
```

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/database` | Generic repository and query builder |

## Testing

```bash
# Run unit tests
go test ./services/neovault/supabase/... -v

# Run with coverage
go test ./services/neovault/supabase/... -v -cover
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
- [Generic Repository](../../../internal/database/README.md)
