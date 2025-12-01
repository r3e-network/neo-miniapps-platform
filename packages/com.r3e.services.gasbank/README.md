# Gas Bank Service

## Overview

The Gas Bank Service provides gas fee management and sponsorship capabilities for the R3E Network service layer. It manages service-owned gas accounts, handles deposits and withdrawals, enforces spending limits, supports multi-signature approvals, and provides automated settlement of blockchain transactions.

**Package ID:** `com.r3e.services.gasbank`
**Service Name:** `gasbank`
**Domain:** `gasbank`

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Gas Bank Service                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐  │
│  │   Service    │──────│ Settlement   │──────│  Withdrawal  │  │
│  │   (Core)     │      │   Poller     │      │  Resolver    │  │
│  └──────┬───────┘      └──────────────┘      └──────────────┘  │
│         │                                                         │
│         │              ┌──────────────┐                          │
│         ├──────────────│ FeeCollector │                          │
│         │              └──────────────┘                          │
│         │                                                         │
│         │              ┌──────────────┐                          │
│         └──────────────│    Store     │                          │
│                        │ (Postgres)   │                          │
│                        └──────────────┘                          │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  AccountManager  │
                    │  Smart Contract  │
                    │   (Neo N3 BC)    │
                    └──────────────────┘
```

### Data Flow

1. **Deposit Flow**: External blockchain transaction → Service records deposit → Updates account balance
2. **Withdrawal Flow**: User requests withdrawal → Approval checks → Settlement poller → Blockchain execution → Completion
3. **Fee Collection**: Oracle/Service request → Fee deduction → Lock funds → Service completion → Settle fee

## Key Components

### Service (`service.go`)

Core business logic implementing gas account management, transaction processing, and withdrawal lifecycle management.

**Responsibilities:**
- Gas account creation and configuration
- Deposit and withdrawal processing
- Balance tracking (available, pending, locked)
- Approval workflow management
- Scheduled withdrawal handling
- Dead letter queue management

### Settlement Poller (`settlement.go`)

Background worker that monitors pending withdrawals and coordinates blockchain settlement.

**Responsibilities:**
- Poll pending withdrawals at configurable intervals
- Invoke withdrawal resolver for settlement status
- Retry failed settlements with exponential backoff
- Move failed withdrawals to dead letter queue after max attempts
- Record settlement attempts for observability

### Withdrawal Resolver (`resolver_http.go`)

Interface for determining withdrawal settlement status. HTTP implementation polls external endpoint.

**Responsibilities:**
- Query blockchain/external system for transaction status
- Return settlement decision (done/pending, success/failure)
- Provide retry timing guidance

### Fee Collector (`fee_collector.go`)

Adapter implementing `engine.FeeCollector` interface for oracle and service fee management.

**Responsibilities:**
- Deduct fees from gas accounts for service usage
- Lock funds during service execution
- Refund fees on service failure
- Settle fees on service completion

### Store (`store.go`, `store_postgres.go`)

Data access layer with PostgreSQL implementation.

**Responsibilities:**
- Persist gas accounts, transactions, approvals, schedules
- Query pending withdrawals and due schedules
- Manage dead letter entries
- Record settlement attempts

## Domain Types

### GasBankAccount

Represents a gas wallet owned by an application account. Aligned with `AccountManager.cs` contract `Wallet` struct.

```go
type GasBankAccount struct {
    ID                    string        // Internal identifier
    AccountID             string        // Owner account ID
    WalletAddress         string        // Neo N3 wallet address (UInt160)
    Status                AccountStatus // active | revoked
    Balance               float64       // Total balance
    Available             float64       // Available for withdrawal
    Pending               float64       // Pending withdrawal
    Locked                float64       // Locked for fees/operations
    MinBalance            float64       // Minimum balance threshold
    DailyLimit            float64       // Daily withdrawal limit
    DailyWithdrawal       float64       // Today's withdrawal total
    NotificationThreshold float64       // Low balance alert threshold
    RequiredApprovals     int           // Multi-sig approval count
    LastWithdrawal        time.Time     // Last withdrawal timestamp
    CreatedAt             time.Time
    UpdatedAt             time.Time
}
```

### Transaction

Records deposits, withdrawals, fees, and refunds.

```go
type Transaction struct {
    ID               string
    AccountID        string        // Gas account ID
    UserAccountID    string        // Owner account ID
    Type             string        // deposit | withdrawal | fee | refund
    Status           string        // pending | scheduled | awaiting_approval |
                                   // approved | dispatched | completed |
                                   // failed | cancelled | dead_letter
    Amount           float64
    NetAmount        float64       // Final settled amount
    BlockchainTxID   string        // On-chain transaction hash
    FromAddress      string
    ToAddress        string
    ScheduleAt       time.Time     // Deferred execution time
    CronExpression   string        // Recurring schedule (future)
    ApprovalPolicy   ApprovalPolicy
    ResolverAttempt  int           // Settlement retry count
    ResolverError    string        // Last resolver error
    LastAttemptAt    time.Time
    NextAttemptAt    time.Time
    DeadLetterReason string
    Error            string
    CompletedAt      time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

**Transaction Types:**
- `deposit`: Funds added to gas account
- `withdrawal`: Funds removed from gas account
- `fee`: Service usage fee deduction
- `refund`: Fee refund on service failure

**Transaction Statuses:**
- `pending`: Awaiting settlement
- `scheduled`: Deferred to future time
- `awaiting_approval`: Requires multi-sig approval
- `approved`: Approved, ready for execution
- `dispatched`: Sent to blockchain
- `completed`: Successfully settled
- `failed`: Settlement failed
- `cancelled`: User/system cancelled
- `dead_letter`: Exceeded retry limit, requires manual intervention

### WithdrawalApproval

Multi-signature approval record for withdrawal transactions.

```go
type WithdrawalApproval struct {
    TransactionID string
    Approver      string
    Status        string    // pending | approved | rejected
    Signature     string
    Note          string
    DecidedAt     time.Time
}
```

### WithdrawalSchedule

Tracks deferred withdrawals scheduled for future execution.

```go
type WithdrawalSchedule struct {
    TransactionID  string
    ScheduleAt     time.Time
    CronExpression string    // Reserved for recurring schedules
    NextRunAt      time.Time
    LastRunAt      time.Time
}
```

### SettlementAttempt

Observability record for withdrawal settlement attempts.

```go
type SettlementAttempt struct {
    TransactionID string
    Attempt       int
    StartedAt     time.Time
    CompletedAt   time.Time
    Latency       time.Duration
    Status        string    // retry | succeeded | failed | error
    Error         string
}
```

### DeadLetter

Failed withdrawals requiring manual intervention.

```go
type DeadLetter struct {
    TransactionID string
    AccountID     string
    Reason        string
    LastError     string
    LastAttemptAt time.Time
    Retries       int
}
```

## API Methods

The service exposes HTTP API endpoints through automatic discovery via method naming convention (`HTTP{Method}{Path}`). The following methods are available:

### Account Management

#### EnsureAccount
```go
func (s *Service) EnsureAccount(ctx context.Context, accountID string, walletAddress string) (GasBankAccount, error)
```
Creates or retrieves a gas account for the specified owner account. Wallet addresses are normalized (trimmed, lowercased) and validated for uniqueness.

#### EnsureAccountWithOptions
```go
func (s *Service) EnsureAccountWithOptions(ctx context.Context, accountID string, opts EnsureAccountOptions) (GasBankAccount, error)
```
Creates or updates a gas account with configuration options:
- `MinBalance`: Minimum balance threshold
- `DailyLimit`: Daily withdrawal limit
- `NotificationThreshold`: Low balance alert threshold
- `RequiredApprovals`: Multi-signature approval count

#### GetAccount
```go
func (s *Service) GetAccount(ctx context.Context, id string) (GasBankAccount, error)
```
Retrieves a gas account by ID.

#### ListAccounts
```go
func (s *Service) ListAccounts(ctx context.Context, ownerAccountID string) ([]GasBankAccount, error)
```
Lists all gas accounts for the specified owner account.

#### Summary
```go
func (s *Service) Summary(ctx context.Context, ownerAccountID string) (Summary, error)
```
Aggregates balances, pending withdrawals, and recent activity for an account.

### Transaction Operations

#### Deposit
```go
func (s *Service) Deposit(ctx context.Context, gasAccountID string, amount float64, txID string, from string, to string) (GasBankAccount, Transaction, error)
```
Records a deposit transaction and credits the gas account balance. Includes automatic rollback on transaction creation failure.

**Validations:**
- Amount must be positive
- Account must exist

#### Withdraw
```go
func (s *Service) Withdraw(ctx context.Context, accountID, gasAccountID string, amount float64, to string) (GasBankAccount, Transaction, error)
```
Initiates a withdrawal transaction with standard options.

#### WithdrawWithOptions
```go
func (s *Service) WithdrawWithOptions(ctx context.Context, accountID, gasAccountID string, opts WithdrawOptions) (GasBankAccount, Transaction, error)
```
Initiates a withdrawal with advanced options:
- `Amount`: Withdrawal amount
- `ToAddress`: Destination wallet address
- `ScheduleAt`: Deferred execution time
- `CronExpression`: Recurring schedule (not yet supported)

**Validations:**
- Amount must be positive
- Sufficient available balance
- Minimum balance threshold enforcement
- Daily withdrawal limit enforcement
- Account ownership verification

**Status Determination:**
- `StatusScheduled`: If scheduled for future time
- `StatusAwaitingApproval`: If multi-sig approvals required
- `StatusPending`: Otherwise

#### CompleteWithdrawal
```go
func (s *Service) CompleteWithdrawal(ctx context.Context, txID string, success bool, errMsg string) (GasBankAccount, Transaction, error)
```
Finalizes a pending withdrawal. On success, deducts from balance. On failure, returns funds to available balance.

#### CancelWithdrawal
```go
func (s *Service) CancelWithdrawal(ctx context.Context, accountID, transactionID, reason string) (Transaction, error)
```
Cancels a pending withdrawal and returns funds to available balance.

### Transaction Queries

#### ListTransactions
```go
func (s *Service) ListTransactions(ctx context.Context, gasAccountID string, limit int) ([]Transaction, error)
```
Lists transactions for a gas account with pagination.

#### ListTransactionsFiltered
```go
func (s *Service) ListTransactionsFiltered(ctx context.Context, gasAccountID, txType, status string, limit int) ([]Transaction, error)
```
Lists transactions filtered by type and/or status.

#### GetWithdrawal
```go
func (s *Service) GetWithdrawal(ctx context.Context, accountID, transactionID string) (Transaction, error)
```
Retrieves a specific withdrawal transaction with ownership verification.

### Approval Workflow

#### SubmitApproval
```go
func (s *Service) SubmitApproval(ctx context.Context, transactionID, approver, signature, note string, approve bool) (WithdrawalApproval, Transaction, error)
```
Records an approval or rejection for a withdrawal. Automatically promotes to `StatusPending` when required approvals are met. Cancels withdrawal on rejection.

#### ListApprovals
```go
func (s *Service) ListApprovals(ctx context.Context, transactionID string) ([]WithdrawalApproval, error)
```
Lists all approval records for a withdrawal transaction.

### Scheduled Withdrawals

#### ActivateDueSchedules
```go
func (s *Service) ActivateDueSchedules(ctx context.Context, limit int) error
```
Promotes scheduled withdrawals whose execution time has arrived. Called automatically by settlement poller.

### Settlement Monitoring

#### ListSettlementAttempts
```go
func (s *Service) ListSettlementAttempts(ctx context.Context, accountID, transactionID string, limit int) ([]SettlementAttempt, error)
```
Lists settlement attempts for a withdrawal transaction.

### Dead Letter Queue

#### ListDeadLetters
```go
func (s *Service) ListDeadLetters(ctx context.Context, accountID string, limit int) ([]DeadLetter, error)
```
Lists failed withdrawals requiring manual intervention.

#### RetryDeadLetter
```go
func (s *Service) RetryDeadLetter(ctx context.Context, accountID, transactionID string) (Transaction, error)
```
Requeues a dead-lettered withdrawal for retry.

#### DeleteDeadLetter
```go
func (s *Service) DeleteDeadLetter(ctx context.Context, accountID, transactionID string) error
```
Cancels and removes a dead-lettered withdrawal.

#### MarkDeadLetter
```go
func (s *Service) MarkDeadLetter(ctx context.Context, tx Transaction, reason, lastErr string) error
```
Moves a withdrawal to the dead letter queue after exceeding retry limits.

## Configuration

### Service Configuration

Defined in `manifest.yaml`:

```yaml
package_id: com.r3e.services.gasbank
version: "1.0.0"
display_name: "Gas Bank Service"

resources:
  max_storage_bytes: 52428800        # 50 MB
  max_concurrent_requests: 1000
  max_requests_per_second: 5000
  max_events_per_second: 1000

dependencies:
  - module: store
    required: true
  - module: svc-accounts
    required: true
```

### Settlement Poller Configuration

```go
poller := NewSettlementPoller(store, service, resolver, logger)

// Configure retry policy
poller.WithRetryPolicy(
    maxAttempts: 5,           // Max settlement attempts before dead letter
    interval: 15 * time.Second, // Polling interval
)

// Configure observability
poller.WithTracer(tracer)
poller.WithObservationHooks(hooks)
```

### HTTP Withdrawal Resolver Configuration

```go
resolver, err := NewHTTPWithdrawalResolver(
    client,                    // *http.Client
    "https://api.example.com/settle", // Endpoint URL
    "api-key-here",           // API key for authentication
    logger,
)
```

**Resolver Response Format:**
```json
{
  "done": true,
  "success": true,
  "message": "Transaction confirmed on blockchain",
  "retry_after_seconds": 5.0
}
```

## Dependencies

### Internal Dependencies

- `github.com/R3E-Network/service_layer/system/core` - Core engine interfaces
- `github.com/R3E-Network/service_layer/system/framework` - Service framework
- `github.com/R3E-Network/service_layer/system/framework/core` - Framework utilities
- `github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts` - Account validation
- `github.com/R3E-Network/service_layer/pkg/logger` - Structured logging

### External Dependencies

- PostgreSQL database for persistence
- Neo N3 blockchain (AccountManager.cs contract)
- HTTP endpoint for withdrawal settlement (optional)

### Required Permissions

- `system.api.storage` - Data persistence (required)
- `system.api.bus` - Event publishing (optional)

## Error Handling

### Common Errors

```go
var (
    errInvalidAmount     = errors.New("amount must be positive")
    errInsufficientFunds = errors.New("insufficient funds")
    errMinBalance        = errors.New("insufficient funds to maintain minimum balance")
    errDailyLimit        = errors.New("daily withdrawal limit exceeded")
    errCronUnsupported   = errors.New("cron expressions are not supported yet")
    ErrWalletInUse       = errors.New("wallet address already assigned to another account")
)
```

### Rollback Behavior

The service implements automatic rollback for critical operations:

1. **Deposit Failure**: If transaction creation fails, account balance is rolled back
2. **Withdrawal Failure**: If transaction creation fails, locked funds are returned to available balance
3. **Fee Collection Failure**: If transaction recording fails, fee deduction is rolled back

## Testing

### Running Tests

```bash
# Run all tests
go test ./packages/com.r3e.services.gasbank/...

# Run with coverage
go test -cover ./packages/com.r3e.services.gasbank/...

# Run specific test
go test -run TestService_DepositWithdraw ./packages/com.r3e.services.gasbank/
```

### Test Coverage

- `service_test.go` - Core service logic tests
- `settlement_test.go` - Settlement poller tests
- `resolver_http_test.go` - HTTP resolver tests
- `testing.go` - Mock implementations for testing

### Mock Store

```go
store := newMockStore()
accounts := store // Implements AccountChecker
service := New(accounts, store, logger)
```

## Observability

### Metrics

The service emits the following metrics:

- `gasbank_accounts_ensured_total` - Gas accounts created
- `gasbank_accounts_updated_total` - Gas accounts updated
- `gasbank_deposits_total` - Deposit transactions
- `gasbank_deposit_amount` - Deposit amount histogram
- `gasbank_withdrawals_total` - Withdrawal requests
- `gasbank_withdraw_amount` - Withdrawal amount histogram
- `gasbank_withdrawals_cancelled_total` - Cancelled withdrawals
- `gasbank_withdrawal_completions_total` - Completed withdrawals (success/failed)
- `gasbank_withdrawal_approvals_total` - Approval decisions
- `gasbank_deadletter_marked_total` - Dead letter promotions
- `gasbank_deadletter_retries_total` - Dead letter retry attempts
- `gasbank_deadletter_deleted_total` - Dead letter deletions

### Logging

Structured logging with contextual fields:

```go
s.Logger().WithField("gas_account_id", id).
    WithField("account_id", accountID).
    WithField("amount", amount).
    Info("gas deposit recorded")
```

### Tracing

Settlement poller supports distributed tracing:

```go
poller.WithTracer(tracer)
// Emits spans: "gasbank.settlement"
```

## Smart Contract Integration

The service aligns with the Neo N3 `AccountManager.cs` smart contract:

### Wallet Struct Mapping

```csharp
// AccountManager.cs
public struct Wallet {
    public UInt160 Address;  // → GasBankAccount.WalletAddress
    public byte Status;      // → GasBankAccount.Status (0=active, 1=revoked)
}
```

### Account Status Mapping

```go
const (
    AccountStatusActive  AccountStatus = "active"  // Contract: 0
    AccountStatusRevoked AccountStatus = "revoked" // Contract: 1
)
```

## Usage Examples

### Basic Deposit and Withdrawal

```go
// Create gas account
gasAcct, err := service.EnsureAccount(ctx, userAccountID, walletAddress)

// Record deposit from blockchain
updated, tx, err := service.Deposit(ctx, gasAcct.ID, 100.0, blockchainTxID, fromAddr, toAddr)

// Request withdrawal
gasAcct, tx, err := service.Withdraw(ctx, userAccountID, gasAcct.ID, 50.0, destinationAddr)

// Settlement poller will automatically process the withdrawal
```

### Withdrawal with Approval

```go
// Configure account with multi-sig
minBal := 10.0
dailyLimit := 100.0
approvals := 2
gasAcct, err := service.EnsureAccountWithOptions(ctx, userAccountID, EnsureAccountOptions{
    WalletAddress:     walletAddr,
    MinBalance:        &minBal,
    DailyLimit:        &dailyLimit,
    RequiredApprovals: &approvals,
})

// Request withdrawal (will be in awaiting_approval status)
_, tx, err := service.Withdraw(ctx, userAccountID, gasAcct.ID, 75.0, destAddr)

// Submit approvals
approval1, tx, err := service.SubmitApproval(ctx, tx.ID, "approver1", "sig1", "approved", true)
approval2, tx, err := service.SubmitApproval(ctx, tx.ID, "approver2", "sig2", "approved", true)
// Transaction automatically promoted to pending status
```

### Scheduled Withdrawal

```go
scheduleTime := time.Now().Add(24 * time.Hour)
gasAcct, tx, err := service.WithdrawWithOptions(ctx, userAccountID, gasAcct.ID, WithdrawOptions{
    Amount:     25.0,
    ToAddress:  destAddr,
    ScheduleAt: &scheduleTime,
})
// Transaction will be activated by settlement poller at scheduled time
```

### Fee Collection for Oracle Requests

```go
feeCollector := NewFeeCollector(service)

// Collect fee for oracle request
err := feeCollector.CollectFee(ctx, userAccountID, 1000000, "oracle-request-123")

// On success, settle the fee
err = feeCollector.SettleFee(ctx, userAccountID, 1000000, "oracle-request-123")

// On failure, refund the fee
err = feeCollector.RefundFee(ctx, userAccountID, 1000000, "oracle-request-123")
```

## Security Considerations

1. **Account Ownership**: All withdrawal operations verify account ownership
2. **Wallet Uniqueness**: Wallet addresses cannot be assigned to multiple accounts
3. **Balance Validation**: Withdrawals enforce available balance, minimum balance, and daily limits
4. **Multi-Signature**: Configurable approval requirements for high-value withdrawals
5. **Rollback Protection**: Critical operations include automatic rollback on failure
6. **Dead Letter Queue**: Failed withdrawals are isolated for manual review

## Future Enhancements

1. **Cron Expressions**: Recurring withdrawal schedules (currently unsupported)
2. **Batch Operations**: Bulk deposit/withdrawal processing
3. **Fee Optimization**: Dynamic fee calculation based on network conditions
4. **Advanced Approval Policies**: Role-based approval workflows
5. **Webhook Notifications**: Real-time alerts for balance thresholds and transaction events

## License

MIT License - Copyright (c) R3E Network

## Support

For issues and questions, please refer to the R3E Network service layer documentation or contact the development team.
