# GasAccounting Service

GAS ledger and accounting service for the R3E Network service layer.

## Overview

GasAccounting provides a double-entry ledger system for tracking GAS token usage across all services. It maintains immutable audit trails and supports reservation-based accounting for pending operations.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    GasAccounting Service                     │
├─────────────────────────────────────────────────────────────┤
│  Ledger Operations    │  Reservation System                  │
│  - Deposit            │  - Reserve (lock GAS)                │
│  - Withdraw           │  - Release (unlock/consume)          │
│  - Consume            │  - Auto-expiration cleanup           │
│  - Refund             │                                      │
├─────────────────────────────────────────────────────────────┤
│                    Supabase Repository                       │
│  - gas_ledger_entries (immutable)                           │
│  - gas_account_balances                                     │
│  - gas_reservations                                         │
└─────────────────────────────────────────────────────────────┘
```

## API Endpoints

### Balance Operations

#### GET /balance

Get user's current balance.

```bash
curl "http://localhost:8080/balance?user_id=123"
```

Response:

```json
{
  "user_id": 123,
  "available_balance": 1000000,
  "reserved_balance": 50000,
  "total_balance": 1050000,
  "as_of": "2025-01-15T10:30:00Z"
}
```

#### POST /deposit

Record a GAS deposit from on-chain transaction.

```bash
curl -X POST http://localhost:8080/deposit \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 123,
    "amount": 100000,
    "tx_hash": "0xabc123..."
  }'
```

#### POST /consume

Deduct GAS for a service operation.

```bash
curl -X POST http://localhost:8080/consume \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 123,
    "amount": 5000,
    "service_id": "neorand",
    "request_id": "req-456",
    "description": "VRF request fulfillment"
  }'
```

### Reservation Operations

#### POST /reserve

Reserve GAS for a pending operation.

```bash
curl -X POST http://localhost:8080/reserve \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 123,
    "amount": 10000,
    "service_id": "neovault",
    "request_id": "mix-789",
    "ttl": "600s"
  }'
```

Response:

```json
{
  "reservation_id": "res:neovault:mix-789:1705312200000000000",
  "amount": 10000,
  "expires_at": "2025-01-15T10:40:00Z",
  "new_available": 990000
}
```

#### POST /release

Release or consume a reservation.

```bash
# Release (return to available)
curl -X POST http://localhost:8080/release \
  -H "Content-Type: application/json" \
  -d '{
    "reservation_id": "res:neovault:mix-789:1705312200000000000",
    "consume": false
  }'

# Consume (deduct from balance)
curl -X POST http://localhost:8080/release \
  -H "Content-Type: application/json" \
  -d '{
    "reservation_id": "res:neovault:mix-789:1705312200000000000",
    "consume": true,
    "actual_amount": 8000
  }'
```

### History

#### GET /history

Get ledger history for a user.

```bash
curl "http://localhost:8080/history?user_id=123&limit=50&offset=0&type=consume"
```

## Entry Types

| Type         | Description                            |
| ------------ | -------------------------------------- |
| `deposit`    | User deposits GAS from on-chain        |
| `withdraw`   | User withdraws GAS to on-chain         |
| `consume`    | Service consumes GAS for operation     |
| `refund`     | Refund unused GAS                      |
| `reserve`    | Reserve GAS for pending operation      |
| `release`    | Release reserved GAS back to available |
| `fee`        | Service fee deduction                  |
| `adjustment` | Manual adjustment (admin only)         |

## Balance Model

```
┌─────────────────────────────────────┐
│         Account Balance             │
├─────────────────────────────────────┤
│  Available Balance                  │  ← Can be used immediately
│  + Reserved Balance                 │  ← Locked for pending ops
│  ─────────────────────              │
│  = Total Balance                    │
└─────────────────────────────────────┘
```

- **Available**: GAS that can be consumed or reserved
- **Reserved**: GAS locked for pending operations (with TTL)
- **Total**: Sum of available and reserved

## Reservation Flow

```
1. Service calls Reserve(amount, ttl)
   └─> Available -= amount
   └─> Reserved += amount
   └─> Returns reservation_id

2a. Operation succeeds → Release(consume=true, actual_amount)
    └─> Reserved -= original_amount
    └─> Ledger entry created for actual_amount consumed
    └─> Excess returned to Available

2b. Operation fails → Release(consume=false)
    └─> Reserved -= amount
    └─> Available += amount

2c. TTL expires → Auto-cleanup worker
    └─> Same as 2b (release without consume)
```

## Idempotency

All ledger operations use idempotency keys to prevent duplicate entries:

- Deposits: `deposit:{tx_hash}`
- Consumes: `consume:{service_id}:{request_id}`
- Reservation releases: `release:res:{reservation_id}`

## Integration Example

```go
import (
    gasaccounting "github.com/R3E-Network/service_layer/services/gasaccounting/marble"
)

// In your service
func (s *MyService) processRequest(ctx context.Context, userID int64, requestID string) error {
    // 1. Reserve GAS for the operation
    reservation, err := s.gasAccounting.Reserve(ctx, &gasaccounting.ReserveRequest{
        UserID:    userID,
        Amount:    estimatedCost,
        ServiceID: "myservice",
        RequestID: requestID,
        TTL:       10 * time.Minute,
    })
    if err != nil {
        return fmt.Errorf("insufficient balance: %w", err)
    }

    // 2. Perform the operation
    actualCost, err := s.doWork(ctx)

    // 3. Release the reservation
    _, err = s.gasAccounting.Release(ctx, &gasaccounting.ReleaseRequest{
        ReservationID: reservation.ReservationID,
        Consume:       err == nil,
        ActualAmount:  actualCost,
    })

    return err
}
```

## Configuration

The service uses the standard BaseService configuration pattern:

```go
cfg := gasaccounting.Config{
    Marble:     marble,
    DB:         db,
    Repository: supabase.NewRepository(db),
}

svc, err := gasaccounting.New(cfg)
```

## Database Schema

```sql
-- Immutable ledger entries
CREATE TABLE gas_ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    entry_type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reference_id VARCHAR(255),
    reference_type VARCHAR(50),
    service_id VARCHAR(50),
    description TEXT,
    metadata JSONB,
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Current balances (materialized view of ledger)
CREATE TABLE gas_account_balances (
    user_id BIGINT PRIMARY KEY,
    available_balance BIGINT NOT NULL DEFAULT 0,
    reserved_balance BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- Active reservations
CREATE TABLE gas_reservations (
    id VARCHAR(255) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    service_id VARCHAR(50) NOT NULL,
    request_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ledger_user_id ON gas_ledger_entries(user_id);
CREATE INDEX idx_ledger_created_at ON gas_ledger_entries(created_at);
CREATE INDEX idx_reservations_expires ON gas_reservations(expires_at);
```
