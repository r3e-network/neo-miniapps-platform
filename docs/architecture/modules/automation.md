# Automation Module

## Responsibilities

- Persist cron/interval jobs tied to functions and accounts.
- Validate account and function ownership for each job.
- Update job metadata (name, schedule, description, next run).
- Toggle job enablement.
- Provide a lifecycle-managed scheduler (tick loop) as `system.Service`.

## Key Components

- `internal/app/domain/automation` – job model.
- `internal/app/services/automation/service.go` – business logic.
- `internal/app/services/automation/scheduler.go` – background runner.
- Store contract (`AutomationStore`) with memory/PostgreSQL adapters.
- HTTP endpoints: `/accounts/{id}/automation/jobs`.

## Interactions

- Functions service delegates `ScheduleAutomationJob`, `UpdateAutomationJob`, `SetAutomationEnabled`.
- Scheduler will later integrate with runtime executors to queue jobs.

## Usage

```go
autoSvc := automation.New(accountsStore, functionsStore, automationStore, log)
job, _ := autoSvc.CreateJob(ctx, accountID, functionID, "daily", "0 0 * * *", "nightly run")
scheduler := automation.NewScheduler(autoSvc, log)
scheduler.Start(ctx)
```

## Checklist

- [x] Create/update/list automation jobs with validation.
- [x] Toggle job enablement.
- [x] Track last/next run metadata.
- [x] Lifecycle-managed scheduler service.
- [x] Execution dispatch via function dispatcher (runtime queue integration pending).
