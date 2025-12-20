# NeoGasBank Service

GasBank service for managing user gas balances within the Neo N3 Mini-App Platform.

## Overview

The NeoGasBank service provides:

- **Deposit Verification**: Monitors Neo N3 chain for confirmed GAS deposits
- **Balance Management**: Credit/debit operations for user gas balances
- **Service Fee Deduction**: Called by other TEE services (neofeeds, neoflow, etc.)
- **Transaction History**: Tracks all balance changes

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│  Edge Functions │────▶│   NeoGasBank    │
│  (gasbank-*)    │     │    Service      │
└─────────────────┘     └────────┬────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
        ▼                        ▼                        ▼
┌───────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   Supabase    │      │   Neo N3 RPC    │      │  Other Services │
│   (Storage)   │      │   (Verify Tx)   │      │  (Fee Deduct)   │
└───────────────┘      └─────────────────┘      └─────────────────┘
```

## API Endpoints

### User-Facing (via Edge Gateway)

| Endpoint        | Method | Description                 |
| --------------- | ------ | --------------------------- |
| `/account`      | GET    | Get user's gas bank account |
| `/transactions` | GET    | Get transaction history     |
| `/deposits`     | GET    | Get deposit history         |

### Service-to-Service (mTLS)

| Endpoint   | Method | Description                          |
| ---------- | ------ | ------------------------------------ |
| `/deduct`  | POST   | Deduct service fee from user balance |
| `/reserve` | POST   | Reserve funds for pending operation  |
| `/release` | POST   | Release or commit reserved funds     |

## Configuration

| Environment Variable       | Description                                    | Default  |
| -------------------------- | ---------------------------------------------- | -------- |
| `NEO_RPC_URL`              | Neo N3 RPC endpoint                            | Required |
| `SUPABASE_URL`             | Supabase API URL                               | Required |
| `SUPABASE_SERVICE_KEY`     | Supabase service key                           | Required |
| `GASBANK_DEPOSIT_ADDRESS`  | Platform deposit address used for verification | Required |

## Deposit Flow

1. User creates deposit request via `gasbank-deposit` edge function
2. User sends GAS to the platform deposit address (`GASBANK_DEPOSIT_ADDRESS`)
3. User updates deposit with `tx_hash`
4. NeoGasBank worker verifies transaction on-chain
5. Upon confirmation, balance is credited

## Service Fee Integration

Other TEE services call NeoGasBank to deduct fees:

```go
// Example: NeoFeeds deducting service fee
resp, err := gasbankClient.DeductFee(ctx, &client.DeductFeeRequest{
    UserID:      userID,
    Amount:      ServiceFeePerUpdate, // 0.0001 GAS
    ServiceID:   "neofeeds",
    ReferenceID: requestID,
})
if !resp.Success {
    return fmt.Errorf("insufficient balance: %s", resp.Error)
}
```

## Database Schema

### gasbank_accounts

- `id`: UUID
- `user_id`: User ID (unique)
- `balance`: Current balance (smallest GAS unit)
- `reserved`: Reserved amount for pending operations
- `created_at`, `updated_at`: Timestamps

### deposit_requests

- `id`: UUID
- `user_id`: User ID
- `account_id`: GasBank account ID
- `amount`: Deposit amount
- `tx_hash`: Neo N3 transaction hash
- `from_address`: Sender address
- `status`: pending | confirming | confirmed | failed | expired
- `confirmations`: Current confirmation count
- `created_at`, `confirmed_at`, `expires_at`: Timestamps

### gasbank_transactions

- `id`: UUID
- `account_id`: GasBank account ID
- `tx_type`: deposit | withdraw | service_fee | refund
- `amount`: Transaction amount (positive or negative)
- `balance_after`: Balance after transaction
- `reference_id`: Related entity ID
- `created_at`: Timestamp
