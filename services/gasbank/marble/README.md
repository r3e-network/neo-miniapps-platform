# NeoGasBank Service

GasBank is a TEE service that manages user GAS balances for the Neo N3 Mini-App Platform. It provides deposit verification, balance management, and service fee deduction capabilities.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    NeoGasBank Service                       │
├─────────────────────────────────────────────────────────────┤
│  Deposit Verification Worker (15s interval)                 │
│  ├── Monitor pending deposits                               │
│  ├── Verify on-chain GAS transfers                          │
│  └── Credit user balances on confirmation                   │
├─────────────────────────────────────────────────────────────┤
│  Balance Operations                                         │
│  ├── GetAccount - Retrieve/create user account              │
│  ├── DeductFee - Service fee deduction (mTLS only)          │
│  ├── ReserveFunds - Reserve for pending operations          │
│  └── ReleaseFunds - Release/commit reserved funds           │
├─────────────────────────────────────────────────────────────┤
│  Transaction History                                        │
│  ├── Deposits - Track deposit requests and confirmations    │
│  └── Transactions - Track all balance changes               │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```
services/gasbank/marble/
├── service.go      # Main service, deposit verification worker
├── handlers.go     # HTTP request handlers
├── api.go          # Route registration
└── types.go        # Type definitions
```

## API Endpoints

### User-Facing Endpoints (JWT Auth)

| Method | Endpoint        | Description                |
| ------ | --------------- | -------------------------- |
| GET    | `/account`      | Get user's GasBank account |
| GET    | `/transactions` | List transaction history   |
| GET    | `/deposits`     | List deposit requests      |

### Service-to-Service Endpoints (mTLS Auth)

| Method | Endpoint   | Description                          |
| ------ | ---------- | ------------------------------------ |
| POST   | `/deduct`  | Deduct service fee from user balance |
| POST   | `/reserve` | Reserve funds for pending operation  |
| POST   | `/release` | Release or commit reserved funds     |

## Deposit Verification Flow

```
1. User initiates GAS transfer to platform deposit address (`GASBANK_DEPOSIT_ADDRESS`)
2. User calls /gasbank-deposit Edge Function with tx_hash
3. Edge Function creates pending deposit record
4. Deposit Verification Worker (every 15s):
   a. Fetches pending deposits from database
   b. Queries Neo N3 chain for transaction confirmation
   c. Verifies GAS transfer details (from, to, amount)
   d. On confirmation: credits user balance, records transaction
5. User can query /deposits to check status
```

## Service Fee Deduction

Other TEE services (neofeeds, neoflow) call GasBank to deduct fees:

```go
// Example: neofeeds deducting fee for price update
resp, err := gasbankClient.DeductFee(ctx, &client.DeductFeeRequest{
    UserID:      userID,
    Amount:      10000,  // 0.0001 GAS
    ServiceID:   "neofeeds",
    ReferenceID: requestID,
})
if !resp.Success {
    // Insufficient balance - reject operation
}
```

## Configuration

| Environment Variable      | Description                                    | Required          |
| ------------------------- | ---------------------------------------------- | ----------------- |
| `NEO_RPC_URL`             | Neo N3 RPC endpoint                            | Yes (strict mode) |
| `SUPABASE_URL`            | Supabase project URL                           | Yes               |
| `SUPABASE_SERVICE_KEY`    | Supabase service key                           | Yes               |
| `GASBANK_DEPOSIT_ADDRESS` | Platform deposit address used for verification | Yes (strict mode) |

## Constants

```go
RequiredConfirmations    = 1              // Blocks to wait
DepositCheckInterval     = 15 * time.Second
DepositExpirationTime    = 24 * time.Hour
MaxPendingDepositsPerRun = 100
GASContractHash          = "0xd2a4cff31913016155e38e474a2c06d08be276cf"
```

## Database Schema

### gasbank_accounts

| Column     | Type      | Description                         |
| ---------- | --------- | ----------------------------------- |
| id         | uuid      | Primary key                         |
| user_id    | text      | User identifier                     |
| balance    | bigint    | Current balance (smallest GAS unit) |
| reserved   | bigint    | Reserved for pending operations     |
| created_at | timestamp | Account creation time               |
| updated_at | timestamp | Last update time                    |

### gasbank_transactions

| Column        | Type   | Description                      |
| ------------- | ------ | -------------------------------- |
| id            | uuid   | Primary key                      |
| account_id    | uuid   | Foreign key to account           |
| tx_type       | text   | deposit, service_fee, withdrawal |
| amount        | bigint | Transaction amount (signed)      |
| balance_after | bigint | Balance after transaction        |
| reference_id  | text   | External reference               |
| tx_hash       | text   | On-chain transaction hash        |
| status        | text   | pending, completed, failed       |

### deposit_requests

| Column        | Type      | Description                            |
| ------------- | --------- | -------------------------------------- |
| id            | uuid      | Primary key                            |
| user_id       | text      | User identifier                        |
| amount        | bigint    | Expected deposit amount                |
| tx_hash       | text      | On-chain transaction hash              |
| from_address  | text      | Sender Neo address                     |
| status        | text      | pending, confirming, confirmed, failed, expired |
| confirmations | int       | Current confirmation count             |
| created_at    | timestamp | Request creation time                  |
| confirmed_at  | timestamp | Confirmation time                      |

## Security

- **mTLS Authentication**: Service-to-service endpoints require MarbleRun mTLS certificates
- **User Authentication**: User endpoints require valid JWT from Supabase Auth
- **Atomic Operations**: Balance updates use mutex locks to prevent race conditions
- **Strict Mode**: In production/enclave mode, chain client is required

## Architecture: Edge vs TEE Implementation

GasBank has a split architecture where user-facing read operations go through Edge Functions while write operations and service-to-service calls go through the TEE service.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         SDK / Frontend                               │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
┌─────────────────────────────┐     ┌─────────────────────────────────┐
│     Edge Functions          │     │      TEE Service (neogasbank)   │
│  (Supabase Edge Runtime)    │     │      (MarbleRun SGX Enclave)    │
├─────────────────────────────┤     ├─────────────────────────────────┤
│ /gasbank-account   (GET)    │     │ /deduct   (POST, mTLS)          │
│ /gasbank-deposits  (GET)    │     │ /reserve  (POST, mTLS)          │
│ /gasbank-transactions (GET) │     │ /release  (POST, mTLS)          │
│ /gasbank-deposit   (POST)   │     │ /account  (GET, internal)       │
├─────────────────────────────┤     ├─────────────────────────────────┤
│ Direct Supabase DB access   │     │ Deposit verification worker     │
│ JWT authentication          │     │ Balance atomicity guarantees    │
│ Read-optimized              │     │ Service fee deduction           │
└─────────────────────────────┘     └─────────────────────────────────┘
```

### Design Rationale

| Aspect      | Edge Functions                   | TEE Service                        |
| ----------- | -------------------------------- | ---------------------------------- |
| **Purpose** | User queries, deposit initiation | Balance mutations, service fees    |
| **Auth**    | JWT (Supabase Auth)              | mTLS (MarbleRun certificates)      |
| **Latency** | Low (edge-deployed)              | Higher (enclave overhead)          |
| **Trust**   | User-facing, read-mostly         | Service-to-service, write-critical |

### Important Notes

1. **Consistency**: Edge Functions read directly from Supabase; TEE writes go through the service. There may be brief read-after-write inconsistency.
2. **Type Alignment**: Both implementations must use identical JSON field names and types. See `types.go` for canonical definitions.
3. **Error Codes**: Edge returns Supabase errors; TEE returns structured `httputil` errors. SDK normalizes both.
