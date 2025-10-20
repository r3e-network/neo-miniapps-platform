# Gas Bank Module

## Responsibilities

- Maintain gas balances per account & wallet.
- Enforce wallet uniqueness.
- Record deposit/withdraw transactions.
- Provide queries for balances and transaction history.
- Run settlement poller to finalize withdrawals.

## Key Components

- `internal/app/domain/gasbank` – account & transaction models.
- `internal/app/services/gasbank/service.go` – operations and validation.
- Storage contract (`GasBankStore`) with memory/PostgreSQL adapters.
- HTTP routes: `/accounts/{id}/gasbank`, `/accounts/{id}/gasbank/deposit`, `/accounts/{id}/gasbank/withdraw`, `/accounts/{id}/gasbank/transactions`.

## Interactions

- Functions service can ensure gas accounts via `EnsureGasAccount`.
- Automation & oracle flows can query balances before scheduling executions.
- All operations require specifying `gas_account_id` to avoid ambiguity.

## Usage

```go
gasSvc := gasbank.New(accountsStore, gasStore, log)
acct, _ := gasSvc.EnsureAccount(ctx, accountID, \"wallet1\")
acct, tx, _ := gasSvc.Deposit(ctx, acct.ID, 10, \"txhash\", \"from\", \"to\")
```

## Notes

- Withdraws currently mark transactions as pending; settlement logic (blockchain callbacks) is handled elsewhere.

## Checklist

- [x] Ensure account with wallet uniqueness enforcement.
- [x] Deposit flow with balance updates and transaction record.
- [x] Withdraw flow with available balance check and pending tracking.
- [x] List accounts and transactions.
- [x] Settlement poller with timeout-based resolver.
- [ ] Blockchain confirmation integration (pending).
