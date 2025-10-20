# Triggers Module

## Responsibilities

- Store cron, event, and webhook rules tied to accounts and functions.
- Validate account and function existence before registration.
- Toggle trigger enablement status.
- Expose read/list operations for automation/HTTP callers.

## Key Components

- `internal/app/domain/trigger` – trigger model.
- `internal/app/services/triggers/service.go` – validation & business logic.
- Storage contracts in `internal/app/storage/interfaces.go`.
- Memory/PostgreSQL implementations.

## Interactions

- Used by Functions service (`RegisterTrigger`).
- HTTP endpoints: `/accounts/{account_id}/triggers`.
- Automation scheduler consumes triggers indirectly through future runtime components.

## Usage

```go
trgSvc := triggers.New(accountsStore, functionsStore, triggerStore, log)
trg, _ := trgSvc.Register(ctx, trigger.Trigger{AccountID: acctID, FunctionID: fnID, Rule: "cron:@hourly"})
```

## Checklist

- [x] Create trigger with validation.
- [x] Update trigger enablement.
- [x] Get/list triggers.
- [x] Support cron, event, and webhook trigger types.
- [ ] Additional integrations (queue bindings, retries).
