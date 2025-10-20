# Accounts Module

## Responsibilities

- Provide tenant-level metadata (owner, labels, quotas).
- Validate account existence for dependent services (functions, gas bank, automation).
- Expose CRUD operations via `internal/app/services/accounts`.
- Persist data through `storage.AccountStore` (memory or PostgreSQL).

## Key Components

- `internal/app/domain/account` – domain model.
- `internal/app/services/accounts/service.go` – business logic.
- `internal/app/storage/interfaces.go` – store contract.
- `internal/app/storage/memory/memory.go` / `storage/postgres/store.go` – implementations.

## Interactions

- Gas bank, functions, triggers, automation services call `GetAccount` for validation.
- HTTP endpoints under `/accounts` list/create accounts.

## Usage

```go
acctSvc := accounts.New(store, log)
acct, _ := acctSvc.Create(ctx, "owner@example.com", nil)
```

## Notes

- Account deletion is not yet exposed; to reset, remove via store directly.

## Checklist

- [x] Account creation with owner/metadata.
- [x] Account retrieval and listing.
- [x] Validation for dependent services.
- [x] Account deletion.
