# Gas Bank Architecture

The gas bank module manages per-account gas balances and transactions. It exposes
REST endpoints under `/accounts/{accountID}/gasbank/...`.

## Domain Types

```
internal/app/domain/gasbank/
  model.go   // Account, Transaction
```

## Services

```
internal/app/services/gasbank/
  service.go         // EnsureAccount, Deposit, Withdraw, CompleteWithdrawal
  settlement.go      // Poller that finalises pending withdrawals
```

## Storage

```
internal/app/storage/
  memory.go/memory/  // in-memory store implementing GasBankStore
  postgres/store.go  // Postgres-backed store
```

## HTTP Surface

```
POST   /accounts/{id}/gasbank                 -> EnsureAccount
POST   /accounts/{id}/gasbank/deposit         -> Deposit
POST   /accounts/{id}/gasbank/withdraw        -> Withdraw
GET    /accounts/{id}/gasbank                 -> List gas accounts for the owner
GET    /accounts/{id}/gasbank/transactions    -> List transactions
```

## Background Jobs

The settlement poller runs as a system service and periodically checks for
pending withdrawals that need completing.
