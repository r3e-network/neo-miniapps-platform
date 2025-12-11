# NeoAccounts Supabase Repository

Database layer for the NeoAccounts pool management service.

## Overview

This package provides NeoAccounts-specific data access for pool account management, including account creation, locking, and lifecycle management.

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Account data model |

## Data Models

### Account

Represents a pool account with locking support.

```go
type Account struct {
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

### Field Descriptions

| Field | Description |
|-------|-------------|
| `ID` | Unique account identifier (UUID) |
| `Address` | Neo N3 address |
| `Balance` | Current balance in GAS units |
| `CreatedAt` | Account creation timestamp |
| `LastUsedAt` | Last activity timestamp |
| `TxCount` | Total transaction count |
| `IsRetiring` | Marked for rotation/deletion |
| `LockedBy` | Service ID holding the lock |
| `LockedAt` | Lock acquisition timestamp |

## Repository Interface

```go
type RepositoryInterface interface {
    Create(ctx context.Context, acc *Account) error
    Update(ctx context.Context, acc *Account) error
    GetByID(ctx context.Context, id string) (*Account, error)
    List(ctx context.Context) ([]Account, error)
    ListAvailable(ctx context.Context, limit int) ([]Account, error)
    ListByLocker(ctx context.Context, lockerID string) ([]Account, error)
    Delete(ctx context.Context, id string) error
}
```

## Database Table

| Table | Purpose |
|-------|---------|
| `pool_accounts` | Pool account storage |

## Usage

```go
import neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"

repo := neoaccountssupabase.NewRepository(baseRepo)

// Create account
err := repo.Create(ctx, &neoaccountssupabase.Account{
    ID:         uuid.New().String(),
    Address:    "NAddr...",
    Balance:    0,
    CreatedAt:  time.Now(),
    LastUsedAt: time.Now(),
})

// Get available accounts (unlocked, non-retiring)
available, err := repo.ListAvailable(ctx, 10)

// List accounts locked by a service
locked, err := repo.ListByLocker(ctx, "neovault")

// Update account
account.Balance += 1000000
account.TxCount++
err := repo.Update(ctx, account)

// Delete retiring account
err := repo.Delete(ctx, accountID)
```

## Query Methods

### ListAvailable

Returns unlocked, non-retiring accounts ordered by least recently used:

```go
query := database.NewQuery().
    IsFalse("is_retiring").
    IsNull("locked_by").
    OrderAsc("last_used_at").
    Limit(limit).
    Build()
```

This ensures:
- Fair distribution (LRU ordering)
- No conflict with locked accounts
- No retiring accounts assigned

### ListByLocker

Returns all accounts currently locked by a specific service:

```go
return database.GenericListByField[Account](r.base, ctx, tableName, "locked_by", lockerID)
```

## Account States

```
                    ┌─────────────┐
       Create       │   Active    │
    ────────────>   │ (unlocked)  │
                    └──────┬──────┘
                           │
              Lock         │         Release
    ┌──────────────────────┼──────────────────────┐
    │                      ▼                      │
    │              ┌─────────────┐                │
    │              │   Locked    │                │
    │              │ (by service)│                │
    │              └──────┬──────┘                │
    │                     │                       │
    └─────────────────────┼───────────────────────┘
                          │
              Rotation    │
    ┌─────────────────────▼───────────────────────┐
    │              ┌─────────────┐                │
    │              │  Retiring   │                │
    │              │ (draining)  │                │
    │              └──────┬──────┘                │
    │                     │                       │
    │        Balance = 0  │                       │
    │                     ▼                       │
    │              ┌─────────────┐                │
    │              │  Deleted    │                │
    │              └─────────────┘                │
    └─────────────────────────────────────────────┘
```

## Locking Semantics

1. **Request**: Service requests N accounts
2. **Lock**: Accounts marked with `locked_by` and `locked_at`
3. **Use**: Only locking service can sign/modify
4. **Release**: Service releases when done
5. **Timeout**: Stale locks (>24h) force-released

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
