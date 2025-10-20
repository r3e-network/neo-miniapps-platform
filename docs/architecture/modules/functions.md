# Functions Module

## Responsibilities

- Manage function definitions (name, description, source code, secrets).
- Serve as orchestration façade for dependent modules:
  - Registers triggers.
  - Schedules automation jobs.
  - Records price feed data.
  - Creates oracle requests.
  - Ensures gas accounts.
  - Executes definitions via pluggable executors.
- Validate account ownership for each function.

## Key Components

- `internal/app/domain/function` – domain model.
- `internal/app/services/functions/service.go` – business logic & orchestration helpers.
- `internal/app/storage/interfaces.go` – `FunctionStore`.
- Memory/PostgreSQL implementations in storage package.

## Interactions

- Relies on `AccountStore` for ownership validation.
- Delegates to `triggers.Service`, `automation.Service`, `pricefeed.Service`, `oracle.Service`, `gasbank.Service` via `AttachDependencies`.
- HTTP routes:
  - `/accounts/{account_id}/functions`

## Usage

```go
stores := app.Stores{Accounts: acctStore, Functions: fnStore, Triggers: trgStore}
app, _ := app.New(stores, log)
fn, _ := app.Functions.Create(ctx, function.Definition{AccountID: acctID, Name: "hello", Source: "() => 1"})
_ = app.Functions.RegisterTrigger(ctx, trigger.Trigger{AccountID: acctID, FunctionID: fn.ID, Rule: "cron:@hourly"})
```

## Notes

- Ensure `AttachDependencies` is called (handled in `application.go`).
- Secrets integration is stubbed; when expanded, update orchestration helpers accordingly.
- Simple executor currently echoes payload; wire real runtime when available.

## Checklist

- [x] Create/update/get/list function definitions.
- [x] Account ownership validation on create/update.
- [x] Trigger registration helper.
- [x] Automation job helpers (create/update/toggle).
- [x] Price feed snapshot recording helper.
- [x] Oracle request lifecycle helpers (create/complete).
- [x] Gas bank account ensure helper.
- [x] Basic function execution endpoint (pluggable executor).
- [ ] Full runtime integration with TEE (future work).
