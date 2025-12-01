# Automation Service

**Package ID:** `com.r3e.services.automation`
**Version:** 1.0.0
**Layer:** Service

## Overview

The Automation Service provides scheduled task execution capabilities for the R3E Network service layer. It enables users to create, manage, and execute automation jobs that trigger serverless functions on a defined schedule using cron expressions or interval-based timing.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Automation Service                       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │   Service    │◄─────┤  HTTP API    │                     │
│  │   (Core)     │      │  Endpoints   │                     │
│  └──────┬───────┘      └──────────────┘                     │
│         │                                                     │
│         │ manages                                             │
│         ▼                                                     │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │    Store     │      │   Schedule   │                     │
│  │ (Postgres)   │      │   Parser     │                     │
│  └──────────────┘      └──────────────┘                     │
│                                                               │
└───────────────────────────┬───────────────────────────────────┘
                            │
                            │ dispatches
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Automation Scheduler                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐      ┌──────────────┐                     │
│  │  Scheduler   │─────►│  Dispatcher  │                     │
│  │  (Polling)   │      │  (Executor)  │                     │
│  └──────────────┘      └──────┬───────┘                     │
│                                │                              │
│                                │ invokes                      │
│                                ▼                              │
│                        ┌──────────────┐                      │
│                        │   Function   │                      │
│                        │   Runner     │                      │
│                        └──────────────┘                      │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Execution Flow

1. **Job Creation**: User creates automation job via HTTP API
2. **Schedule Parsing**: Service parses cron/interval schedule and calculates next run time
3. **Job Storage**: Job metadata persisted to PostgreSQL
4. **Polling**: Scheduler polls for jobs due for execution (1-second interval)
5. **Dispatch**: Eligible jobs dispatched to FunctionDispatcher
6. **Check Phase**: Function executed with `phase: "check"` payload
7. **Perform Phase**: If check returns `shouldPerform: true`, function executed with `phase: "perform"`
8. **Recording**: Execution metadata recorded, next run time calculated

## Key Components

### Service (`service.go`)

Core service implementation providing job lifecycle management.

**Responsibilities:**
- Job CRUD operations (Create, Read, Update, Delete)
- Schedule validation and next-run calculation
- Account ownership verification
- Observability (logging, metrics, tracing)
- HTTP API endpoint handlers

**Key Methods:**
- `CreateJob()` - Provision new automation job
- `UpdateJob()` - Modify job properties
- `SetEnabled()` - Toggle job active state
- `RecordExecution()` - Update execution metadata
- `GetJob()` / `ListJobs()` - Query operations

### Scheduler (`scheduler.go`)

Background polling service that identifies and dispatches jobs due for execution.

**Responsibilities:**
- Poll job store at 1-second intervals
- Filter enabled jobs with `NextRun <= now`
- Dispatch jobs to registered JobDispatcher
- Lifecycle management (Start/Stop/Ready)
- Concurrent job execution

**Configuration:**
- Polling interval: 1 second (configurable)
- Timeout per tick: 2 seconds
- Concurrent dispatch: Unlimited (goroutine per job)

### FunctionDispatcher (`function_dispatcher.go`)

Executes automation jobs by invoking serverless functions with check/perform pattern.

**Responsibilities:**
- Two-phase execution (check → perform)
- Function invocation via FunctionRunner interface
- Execution metrics recording
- Distributed tracing support

**Check Phase:**
```json
{
  "automation_job": "job-id",
  "phase": "check"
}
```

**Perform Phase:**
```json
{
  "automation_job": "job-id",
  "phase": "perform",
  ...additional payload from check phase
}
```

### Schedule Parser (`schedule.go`)

Parses cron expressions and interval specifications to calculate next execution time.

**Supported Formats:**

**Interval-based:**
- `@every <duration>` - e.g., `@every 15m`, `@every 1h30m`

**Named schedules:**
- `@hourly` - Every hour at minute 0
- `@daily` / `@midnight` - Every day at 00:00
- `@weekly` - Every Sunday at 00:00
- `@monthly` - First day of month at 00:00
- `@annually` / `@yearly` - January 1st at 00:00

**Cron expressions (5 fields):**
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday=0 or 7)
│ │ │ │ │
* * * * *
```

**Examples:**
- `0 0 * * *` - Daily at midnight
- `*/15 * * * *` - Every 15 minutes
- `0 9-17 * * 1-5` - Every hour 9am-5pm, Monday-Friday
- `30 15 * * 5` - Every Friday at 3:30pm

## Domain Types

### Job (`domain.go`)

Represents a scheduled automation task.

```go
type Job struct {
    ID          string    // Unique identifier
    AccountID   string    // Owner account
    FunctionID  string    // Target function to execute
    Name        string    // Human-readable name (unique per account)
    Description string    // Optional description
    Schedule    string    // Cron expression or interval spec
    Status      JobStatus // active | completed | paused
    Enabled     bool      // Active execution flag
    RunCount    int       // Total executions performed
    MaxRuns     int       // Execution limit (0 = unlimited)
    LastRun     time.Time // Last execution timestamp
    NextRun     time.Time // Scheduled next execution
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### JobStatus

```go
type JobStatus string

const (
    JobStatusActive    JobStatus = "active"    // Contract: 0
    JobStatusCompleted JobStatus = "completed" // Contract: 1 (MaxRuns reached)
    JobStatusPaused    JobStatus = "paused"    // Contract: 2
)
```

### FunctionExecution

Result of function invocation during automation job execution.

```go
type FunctionExecution struct {
    ID          string
    AccountID   string
    FunctionID  string
    Input       map[string]any
    Output      map[string]any
    Logs        []string
    Error       string
    Status      string
    StartedAt   time.Time
    CompletedAt time.Time
    Duration    time.Duration
}
```

## HTTP API Endpoints

All endpoints require authentication and are scoped to the authenticated account.

### List Jobs

```http
GET /automation/jobs
```

**Response:**
```json
[
  {
    "id": "job-123",
    "account_id": "acc-456",
    "function_id": "fn-789",
    "name": "Daily Report",
    "description": "Generate daily analytics report",
    "schedule": "0 0 * * *",
    "status": "active",
    "enabled": true,
    "run_count": 42,
    "max_runs": 0,
    "last_run": "2025-12-01T00:00:00Z",
    "next_run": "2025-12-02T00:00:00Z",
    "created_at": "2025-11-01T10:00:00Z",
    "updated_at": "2025-12-01T00:00:05Z"
  }
]
```

### Create Job

```http
POST /automation/jobs
Content-Type: application/json

{
  "function_id": "fn-789",
  "name": "Hourly Sync",
  "description": "Sync data every hour",
  "schedule": "@hourly"
}
```

**Response:** Job object (201 Created)

**Validation:**
- `function_id` - Required, must reference valid function
- `name` - Required, unique per account
- `schedule` - Required, valid cron or interval expression

### Get Job

```http
GET /automation/jobs/{id}
```

**Response:** Job object (200 OK)

**Authorization:** Job must belong to authenticated account

### Update Job

```http
PATCH /automation/jobs/{id}
Content-Type: application/json

{
  "name": "Updated Name",
  "schedule": "*/30 * * * *",
  "description": "Updated description",
  "enabled": false
}
```

**Response:** Updated Job object (200 OK)

**Notes:**
- All fields optional
- `enabled` field triggers `SetEnabled()` operation
- Schedule changes recalculate `next_run`
- Name must remain unique per account

### Delete Job

```http
DELETE /automation/jobs/{id}
```

**Response:** Job object with `enabled: false` (200 OK)

**Implementation:** Soft delete via `SetEnabled(false)`

## Configuration

### Service Configuration

```go
framework.ServiceConfig{
    Name:         "automation",
    Description:  "Automation jobs and schedulers",
    DependsOn:    []string{"store", "svc-accounts", "svc-functions"},
    RequiresAPIs: []engine.APISurface{
        engine.APISurfaceStore,
        engine.APISurfaceEvent,
        engine.APISurfaceCompute,
    },
    Capabilities: []string{"automation"},
}
```

### Resource Limits (manifest.yaml)

```yaml
resources:
  max_storage_bytes: 314572800      # 300 MB
  max_concurrent_requests: 1000
  max_requests_per_second: 5000
  max_events_per_second: 1000
```

### Scheduler Configuration

- **Polling Interval:** 1 second
- **Tick Timeout:** 2 seconds
- **Concurrency:** Unlimited (goroutine per eligible job)
- **Search Window:** 5 years (cron next-run calculation)

## Dependencies

### Required Services

- **store** - PostgreSQL persistence layer
- **svc-accounts** - Account validation
- **svc-functions** - Function execution runtime

### Required APIs

- **APISurfaceStore** - Data persistence
- **APISurfaceEvent** - Event publishing
- **APISurfaceCompute** - Function execution

### External Packages

- `github.com/R3E-Network/service_layer/pkg/logger` - Structured logging
- `github.com/R3E-Network/service_layer/pkg/metrics` - Observability
- `github.com/R3E-Network/service_layer/system/framework` - Service framework

## Observability

### Metrics

**Counters:**
- `automation_jobs_created_total{account_id}` - Jobs created
- `automation_jobs_updated_total{account_id}` - Jobs updated
- `automation_job_runs_total{job_id,account_id}` - Executions performed

**Gauges:**
- `automation_job_enabled{job_id,account_id}` - Job enabled state (0 or 1)

**Histograms:**
- `automation_execution_duration{job_id,phase}` - Execution timing (check/perform)
- `automation_execution_success{job_id,phase}` - Success rate

### Logging

**Structured Fields:**
- `job_id` - Job identifier
- `account_id` - Owner account
- `function_id` - Target function
- `enabled` - Job state

**Log Events:**
- Job created/updated/enabled/disabled
- Execution started/completed/failed
- Schedule parsing errors
- Dispatch failures

### Tracing

Distributed tracing spans:
- `automation.dispatch` - Job dispatch operation
- `automation.job.check` - Check phase execution
- `automation.job.perform` - Perform phase execution

## Testing

### Unit Tests

```bash
# Run unit tests
go test ./packages/com.r3e.services.automation/...

# Run with coverage
go test -cover ./packages/com.r3e.services.automation/...

# Run specific test
go test -run TestNextRunFromSpec_Cron ./packages/com.r3e.services.automation/
```

### Integration Tests

Integration tests require PostgreSQL database:

```bash
# Set database connection
export DATABASE_URL="postgres://user:pass@localhost:5432/testdb"

# Run integration tests
go test -tags=integration ./packages/com.r3e.services.automation/...
```

### Test Coverage

- `schedule_test.go` - Schedule parsing and cron calculation
- `scheduler_test.go` - Scheduler lifecycle
- `service_test.go` - Service lifecycle and manifest
- `function_dispatcher_test.go` - Dispatcher check/perform logic

### Example Test

```go
func TestNextRunFromSpec_Every(t *testing.T) {
    now := time.Date(2025, 2, 10, 10, 0, 0, 0, time.UTC)
    next, err := nextRunFromSpec("@every 15m", now)
    if err != nil {
        t.Fatalf("next run: %v", err)
    }
    expected := now.Add(15 * time.Minute)
    if !next.Equal(expected) {
        t.Fatalf("expected %v, got %v", expected, next)
    }
}
```

## Usage Examples

### Creating a Job

```go
import "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"

// Initialize service
svc := automation.New(accountChecker, store, logger)

// Create hourly job
job, err := svc.CreateJob(
    ctx,
    "account-123",
    "function-456",
    "Hourly Backup",
    "@hourly",
    "Backup data every hour",
)
```

### Setting Up Scheduler

```go
// Create scheduler
scheduler := automation.NewScheduler(svc, logger)

// Configure dispatcher
dispatcher := automation.NewFunctionDispatcher(functionRunner, svc, logger)
scheduler.WithDispatcher(dispatcher)

// Start scheduler
if err := scheduler.Start(ctx); err != nil {
    log.Fatal(err)
}
defer scheduler.Stop(ctx)
```

### Implementing Function Runner

```go
type MyFunctionRunner struct {
    functionsService *functions.Service
}

func (r *MyFunctionRunner) Execute(
    ctx context.Context,
    functionID string,
    payload map[string]any,
) (automation.FunctionExecution, error) {
    // Execute function and return result
    result, err := r.functionsService.ExecuteFunction(ctx, functionID, payload)
    return automation.FunctionExecution{
        ID:          result.ID,
        FunctionID:  functionID,
        Output:      result.Output,
        Status:      result.Status,
        Duration:    result.Duration,
    }, err
}
```

### Check/Perform Pattern

Function implementation example:

```javascript
// Automation function handler
export async function handler(event) {
    const { phase, automation_job } = event;

    if (phase === "check") {
        // Determine if perform phase should run
        const shouldRun = await checkCondition();
        return {
            shouldPerform: shouldRun,
            performPayload: {
                timestamp: Date.now(),
                reason: "condition met"
            }
        };
    }

    if (phase === "perform") {
        // Execute actual work
        await doWork(event);
        return { success: true };
    }
}
```

## Error Handling

### Common Errors

**Validation Errors:**
- `required: account_id` - Missing account identifier
- `required: function_id` - Missing function identifier
- `required: name` - Missing job name
- `required: schedule` - Missing schedule specification

**Business Logic Errors:**
- `job with name "X" already exists` - Duplicate job name
- `account validation failed` - Invalid account
- `forbidden: job belongs to different account` - Authorization failure

**Schedule Parsing Errors:**
- `schedule "X" must contain 5 fields` - Invalid cron format
- `parse duration "X": invalid` - Invalid interval format
- `unable to find next run for schedule "X"` - Unsatisfiable schedule

### Error Response Format

```json
{
  "error": "validation error",
  "message": "required: function_id",
  "code": "VALIDATION_ERROR"
}
```

## Performance Considerations

### Scalability

- **Polling Overhead:** 1-second polling interval suitable for <10,000 jobs
- **Concurrent Execution:** Unlimited goroutines may require tuning for large deployments
- **Database Load:** List query on every tick; consider indexing on `enabled` and `next_run`

### Optimization Recommendations

1. **Index Strategy:**
   ```sql
   CREATE INDEX idx_jobs_enabled_nextrun
   ON automation_jobs(enabled, next_run)
   WHERE enabled = true;
   ```

2. **Scheduler Tuning:**
   - Increase polling interval for less time-sensitive workloads
   - Implement job batching for high-volume scenarios
   - Add rate limiting to dispatcher

3. **Function Execution:**
   - Implement timeout controls in FunctionRunner
   - Add circuit breaker for failing functions
   - Consider async execution for long-running jobs

## Security

### Authorization

- All API endpoints verify account ownership
- Jobs can only be accessed by owning account
- Function execution inherits job's account context

### Input Validation

- Schedule expressions validated before storage
- Job names sanitized and uniqueness enforced
- Function IDs validated against functions service

### Audit Trail

- All job operations logged with account context
- Execution history tracked via `LastRun` and `RunCount`
- Metrics provide observability into job activity

## License

MIT License - Copyright (c) R3E Network

## Support

For issues, questions, or contributions, please refer to the main service layer repository.
