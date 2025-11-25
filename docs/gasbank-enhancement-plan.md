# Gas Bank Enhancement Plan

## Objectives
- Deliver production-grade gas treasury management for every workspace.
- Provide operators with clear visibility (balances, transactions, settlement health) and easy remediation controls.
- Enable programmatic orchestration (Devpack, CLI, API) for gas budgeting and withdrawals.
- Improve reliability/observability of settlement and withdrawal flows.

## Functional Extensions
### Domain Model Updates
- **GasAccount**: add `MinBalance`, `DailyLimit`, `NotificationThreshold`, `RequiredApprovals` fields.
- **GasTransaction**: extend with `Type` (deposit, withdraw, refund), `State` (pending, scheduled, approved, dispatched, settled, failed, dead_letter), `ApprovalSet`, `ScheduleAt`, `ResolvedAt`, `ResolverError`, `OnChainTxID`.
- **WithdrawalApproval** (new table): `TransactionID`, `Approver`, `SignedAt`, `Signature`, `Status`.
- **WithdrawalSchedule** (new table or embedded JSON): timestamp-based scheduling metadata. Cron rules are not supported in the current implementation and should be rejected until a scheduler is added.

### Workflows
1. **Account Ensure**: existing behavior + ability to set thresholds/limits. API accepts optional `min_balance`, `daily_limit`, `required_approvals`, `notification_threshold`.
2. **Deposit**: record metadata (source wallet, tx hash, confirmations). CLI/API returns pending status until confirmations met.
3. **Withdraw**:
   - **Immediate**: default path. Requires approvals based on `RequiredApprovals` (multi-sig). Auto-schedules settlement when approvals complete.
   - **Scheduled**: user supplies a future `schedule_at` timestamp. Cron rules are not supported yet. Settlement poller enqueues once due.
   - **On-chain Transfer**: optional path to call external resolver for actual blockchain transfer (needs `destination_chain`, `gas_limit`, `payload`).
4. **Approvals**: API to request approval, list approvals, approve/deny. CLI command `slctl gasbank approvals`.
5. **Retry & Dead-letter**: Poller automatically retries with exponential backoff. After N attempts, transaction marked `dead_letter` and operators can requeue or cancel.

### Devpack Actions
Expose new actions in the functions runtime:
- `gasbank.ensureAccount({wallet, minBalance, dailyLimit})`
- `gasbank.balance({wallet?})`
- `gasbank.listTransactions({status, limit})`
- `gasbank.withdraw({amount, toAddress, scheduleAt, approvals})`
- `gasbank.requestApproval({transactionId})`
Each action returns structured results and surfaces errors to Devpack scripts.

### HTTP API
New/extended endpoints under `/accounts/{id}/gasbank`:
1. `GET /summary` — balance, pending withdrawals, limits, alerts.
2. `POST /ensure` — existing ensure with new thresholds.
3. `POST /deposit` — record deposit metadata.
4. `POST /withdraw` — create withdrawal (immediate or scheduled).
5. `PATCH /withdraw/{tid}` — approve/deny, reschedule, cancel.
6. `GET /withdraw/{tid}` — details including approvals, resolver state.
7. `GET /transactions` — paginated list with filters (`type`, `status`, `cursor`, `limit`).
8. `GET /approvals` — list pending approvals for operator.
9. `POST /approvals/{tid}` — submit approval/denial.
10. `POST /settlement/retry` — manual retry for failed transaction (auth-protected).

### CLI Enhancements (`slctl gasbank`)
Subcommands:
- `slctl gasbank summary --account <id>`
- `slctl gasbank deposits create|list`
- `slctl gasbank withdrawals create|approve|list|retry`
- `slctl gasbank approvals list|approve|reject`
Flags for pagination (`--limit`, `--cursor`), filters (`--status`, `--type`), scheduling (`--schedule-at` only), approvals (`--approver`, `--note`).

### Dashboard / UI
React view (apps/dashboard):
1. **Overview Card**: balance, min balance, pending withdrawal count, alert banners for low balance or dead-letter items.
2. **Accounts Table**: columns for wallet, balance, limits, last activity, buttons for “Deposit”, “Withdraw”.
3. **Transactions Table**: filterable by status/type; show amount, destination, schedule time, approvals, resolver status, retry button.
4. **Settlement Monitor panel**: poller health (last run, avg latency), queue depth, DLQ count, manual retry controls.
5. **Approval Drawer**: list of pending approvals per operator with approve/reject actions and reason capture.
6. **Activity Timeline**: high-level log of recent deposits/withdrawals/resolver errors.

### Observability & Reliability
- **Metrics**: `gasbank_balance{account}`, `gasbank_pending_withdrawals`, `gasbank_settlement_latency_seconds`, `gasbank_resolver_failures_total`, `gasbank_dead_letter_total`.
- **Logs**: structured logs for each workflow step with correlation IDs.
- **Alerts**: trigger when pending queue > threshold, low balance, repeated resolver failures, DLQ growth.
- **Rate Limiting**: per-account throttle on withdrawals to prevent abuse.
- **Resolver Mock**: new package under `internal/services/gasbank/resolvermock` to simulate responses in staging/tests.

### Documentation & Examples
- **Tutorial**: Markdown guide covering ensure → deposit → withdraw (CLI + API). Include screenshots of CLI output and dashboard cards.
- **Devpack Example**: TypeScript function showing `gasbank.ensureAccount` + `gasbank.withdraw` + approval handling.
- **Troubleshooting Guide**: section for resolver errors, how to retry/cancel, reading metrics.

## Implementation Plan
1. **Design Review**: validate models/endpoints/UI with stakeholders.
2. **Backend Phase**
   - Update storage schemas (migration for new fields/tables).
   - Implement new service methods & Devpack actions.
   - Extend settlement poller with retry/DLQ.
   - Add metrics/logging.
3. **CLI & Docs**
   - Add new commands/flags.
   - Write tutorials/examples.
4. **Dashboard**
   - Build React components (overview, tables, drawers).
5. **Observability**
   - Wire metrics/exporter.
   - Implement alerts/config.
6. **Testing & QA**
   - Unit tests for new workflows.
   - Integration tests with resolver mock.
   - Documentation review.
