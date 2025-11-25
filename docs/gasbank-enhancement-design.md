# Gas Bank Enhancement Design

This document translates the high-level brief into a concrete design that can be implemented across the service layer, CLI, Devpack runtime, and dashboard.

## 1. Objectives Recap
- Provide each workspace with a production-grade gas treasury: clear balances, pending withdrawals, and remediation controls.
- Support programmatic orchestration from Devpack/CLI/API, including approvals, scheduling, and limits.
- Increase reliability through retry/DLQ handling, structured telemetry, and resolver mocks for staging.

## 2. Domain Model
### 2.1 Gas Account
```
GasAccount {
  ID string
  AccountID string
  WalletAddress string
  Balance float64
  Available float64
  Pending float64
  Locked float64 // scheduled or approval-blocked amounts
  MinBalance float64
  DailyLimit float64
  NotificationThreshold float64
  RequiredApprovals int
  Flags map[string]bool // e.g. auto_approve_small_withdrawals
  Metadata map[string]string
  LastWithdrawal time.Time
  CreatedAt time.Time
  UpdatedAt time.Time
}
```

### 2.2 Gas Transaction
```
GasTransaction {
  ID string
  AccountID string        // gas account
  UserAccountID string    // workspace
  Type enum               // deposit | withdrawal | refund
  State enum              // pending | scheduled | awaiting_approval | approved | dispatched | settled | failed | dead_letter | cancelled
  Amount float64
  NetAmount float64
  BlockchainTxID string
  FromAddress string
  ToAddress string
  ScheduleAt *time.Time
  Cron         string
  ApprovalSet  []WithdrawalApproval
  ApprovalPolicy struct {
    Required int
    Approvers []string
  }
  ResolverAttempt int
  ResolverError string
  ResolverMetadata map[string]string
  LastAttemptAt *time.Time
  NextAttemptAt *time.Time
  DeadLetterReason string
  Notes string
  Error string
  CreatedAt time.Time
  UpdatedAt time.Time
  CompletedAt *time.Time
  DispatchedAt *time.Time
  ResolvedAt *time.Time
}
```

### 2.3 Additional Tables
- `gas_withdrawal_approvals`: transaction_id, approver, status (pending/approved/rejected), signature, note, decided_at.
- `gas_withdrawal_schedules`: transaction_id, schedule_at timestamp, next_fire_at, last_fire_at. (Cron expressions deferred; not supported in current implementation.)
- `gas_withdrawal_attempts`: transaction_id, attempt, started_at, latency_ms, resolver_status, error.
- `gas_withdrawal_deadletters`: transaction_id, reason, last_error, last_attempt_at, retries.

## 3. State Machine
1. **pending** – created, awaiting schedule/approval.
2. **scheduled** – schedule_at future date set (cron not supported yet).
3. **awaiting_approval** – approvals outstanding.
4. **approved** – all approvals collected; waiting for settlement.
5. **dispatched** – submitted to resolver, awaiting completion.
6. **settled** – success path, account balances finalized.
7. **failed** – resolver error, funds returned to available.
8. **dead_letter** – exceeded retry budget. Manual intervention required.
9. **cancelled** – user cancelled before dispatch.

Transitions are driven by API actions (approve/deny/cancel), scheduler (moves scheduled → awaiting_approval/approved), and poller (approved → dispatched → settled/failed/dead_letter). Cron-based scheduling is not available yet.

## 4. Storage Schema Changes
1. Extend `app_gas_accounts` with `locked`, `min_balance`, `daily_limit`, `notification_threshold`, `required_approvals`, `flags JSONB`, `metadata JSONB`.
2. Extend `app_gas_transactions` with:
   - `state`, `schedule_at`, `approval_policy JSONB`, `resolver_attempt`, `resolver_error`, `last_attempt_at`, `next_attempt_at`, `dead_letter_reason`, `dispatched_at`, `resolved_at`. (Cron field omitted until scheduler support exists.)
3. Create tables:
   - `app_gas_withdrawal_approvals` (PK transaction_id + approver).
   - `app_gas_withdrawal_schedules`.
   - `app_gas_withdrawal_attempts`.
   - `app_gas_dead_letters`.
4. Indexes:
   - `idx_gas_transactions_account_state` on (account_id, state).
   - `idx_gas_withdrawals_pending` on (state, schedule_at).
   - `idx_gas_dead_letters_status`.

## 5. Service Layer APIs
### Account & Summary
- `EnsureAccount(ctx, accountID, EnsureOptions)` – upserts thresholds/limits/wallet.
- `ListAccounts(ctx, accountID)`
- `Summary(ctx, accountID)` – returns per-account stats, alerts, pending counts per state, last deposit/withdrawal, DLQ totals.

### Transactions
- `Deposit(ctx, gasAccountID, DepositParams)` – records metadata (tx hash, confirmations, origin). Supports pending/confirmed statuses.
- `Withdraw(ctx, accountID, gasAccountID, WithdrawParams)` – creates withdrawal with schedule & approval metadata.
- `ApproveWithdrawal(ctx, txID, approver, signature, note, decision)` – updates ApprovalSet.
- `CancelWithdrawal(ctx, txID, reason)`
- `RetryWithdrawal(ctx, txID)` – clears DLQ metadata and requeues.
- `ListTransactions(ctx, Filter)` – filter by type/status/date, returns cursor pagination.
- `ListApprovals(ctx, accountID, approver, Filter)` – for operator dashboards.

### Settlement Hooks
- `ScheduleReadyWithdrawals(ctx)` – invoked by the poller to move due scheduled items into approval queue (no cron support yet).
- `CompleteWithdrawal(ctx, txID, success, resolverMessage)` – existing method extended to update state fields and metrics.

## 6. Settlement Poller
- Configurable via `configs/config.yaml` (`gasbank.poll_interval`, `gasbank.max_attempts`, `gasbank.backoff.initial`, `gasbank.backoff.max`).
- Maintains per-transaction attempt metadata in storage (not in-memory map) so restarts resume correctly.
- Emits events:
  - `gasbank_settlement_attempts_total{result="success|failure"}`
  - `gasbank_settlement_latency_seconds`
  - `gasbank_dead_letter_total`
- Adds `ResolverMock` (under `internal/services/gasbank/resolvermock`) supporting scripted responses for staging/CI.

## 7. HTTP API
```
GET   /accounts/{id}/gasbank/summary
POST  /accounts/{id}/gasbank/ensure
GET   /accounts/{id}/gasbank/accounts
POST  /accounts/{id}/gasbank/deposits
GET   /accounts/{id}/gasbank/deposits?cursor&limit
POST  /accounts/{id}/gasbank/withdrawals
GET   /accounts/{id}/gasbank/withdrawals?status&type&cursor
GET   /accounts/{id}/gasbank/withdrawals/{txID}
PATCH /accounts/{id}/gasbank/withdrawals/{txID}   // approve/deny/cancel/reschedule payload w/ action enum
POST  /accounts/{id}/gasbank/settlement/retry     // requeue DLQ
GET   /accounts/{id}/gasbank/approvals?pending_only
POST  /accounts/{id}/gasbank/approvals/{txID}     // approve/deny with signature/note
GET   /accounts/{id}/gasbank/transactions?type&status&cursor&limit
```
Payloads use camelCase JSON fields and include correlation IDs through `X-Correlation-ID`.

## 8. CLI Surface (`slctl gasbank`)
- `summary --account <id>`
- `accounts ensure --account --wallet --min-balance --daily-limit --approvals`
- `accounts list --account`
- `deposits create|list`
- `withdrawals create|list|get|approve|deny|cancel|retry --account <id> [flags]`
- `approvals list|approve|deny`
- `transactions list --account <id> [--type --status --limit --cursor]`
- `settlement deadletters --account <id>` and `settlement retry --transaction <id>`

All commands support `--json` (default pretty JSON), `--status`, `--pending-only`, `--schedule-at` (timestamp only), `--approver`, `--note`, `--signature`, `--limit`, `--cursor`. Cron flags are intentionally out of scope.

## 9. Devpack & SDK
- Runtime actions:
  - `gasbank.ensureAccount({ wallet, minBalance, dailyLimit, requiredApprovals })`
  - `gasbank.balance({ wallet?, gasAccountId? })`
  - `gasbank.listTransactions({ gasAccountId?, status?, limit? })`
  - `gasbank.withdraw({ gasAccountId?, wallet?, amount, to, scheduleAt?, approvals? })` (cron not supported)
  - `gasbank.requestApproval({ transactionId })`
- Go service handlers translate each action into the new service methods and embed results in execution logs.
- Update `sdk/devpack/README.md` plus provide a TypeScript example demonstrating ensure + withdraw + approval handling.

## 10. Dashboard
Components (React/Vite):
1. **GasbankOverviewCard** – show total balance, available, pending, min balance, alert banners for low balance and DLQ > 0.
2. **GasAccountsTable** – wallet, balance, min balance, last activity, action buttons for deposit/withdraw.
3. **GasTransactionsTable** – filter controls (status/type/date), show approvals, schedule info, retry button.
4. **SettlementMonitorPanel** – poller status (last tick, latency histogram sparkline), pending queue depth, DLQ count, manual retry button.
5. **ApprovalDrawer** – pending approvals assigned to operator with approve/deny CTA.
6. **ActionDrawer** – forms for deposits/withdrawals with schedule + multi-sig fields and quick links to docs/CLI scripts.

API client updates (`apps/dashboard/src/api.ts`) mirror the HTTP endpoints above and return typed responses for new fields.

## 11. Observability & Alerts
- Metrics (Prometheus):
  - `gasbank_balance{workspace, gas_account}`
  - `gasbank_pending_withdrawals{state}`
  - `gasbank_settlement_latency_seconds`
  - `gasbank_resolver_failures_total`
  - `gasbank_dead_letter_total`
  - `gasbank_approvals_pending`
- Structured logs with correlation IDs for ensure/deposit/withdraw/resolver callbacks.
- Alert rules:
  - Pending queue > threshold for >5 min.
  - Resolver failures in consecutive attempts.
  - Low balance (balance < min_balance or < notification threshold).
  - Dead letters present.
- Config toggles via `configs/config.yaml` (`gasbank.alerts.*`).

## 12. Documentation & Examples
- `docs/gasbank/tutorial.md` – CLI + HTTP walkthrough (ensure → deposit → scheduled withdraw with approvals).
- `docs/gasbank/dashboard.md` – annotated screenshots/walkthrough of new UI.
- `docs/gasbank/troubleshooting.md` – resolver errors, retrying, interpreting metrics.
- Devpack sample under `examples/functions/devpack/js/gasbank_workflow.js`.

## 13. Work Breakdown
1. **Schema & Storage** – DB migrations, Postgres + in-memory store updates, domain structs.
2. **Service Layer** – business logic for withdrawals, approvals, scheduling, retries.
3. **API/CLI** – HTTP handlers + `slctl` parity with pagination/filters.
4. **Devpack/SDK** – runtime/handler updates + docs/examples.
5. **Dashboard** – API client + UI components.
6. **Observability** – metrics/logging/alerts + resolver mock.
7. **Docs & Tutorials** – markdown guides, README updates.
8. **QA** – integration tests with resolver mock, CLI/dash e2e walkthrough.

Each workstream should ship incremental slices (e.g., schema + service foundation, then approvals, then scheduling) to keep PRs reviewable.
