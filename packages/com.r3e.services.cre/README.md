# CRE Service (Composable Run Engine)

## Overview

The CRE (Composable Run Engine) service provides a declarative orchestration framework for executing multi-step workflows (playbooks) within the R3E service layer. It enables users to define, manage, and execute complex automation sequences with configurable executors and comprehensive observability.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CRE Service                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │   Playbook   │    │     Run      │    │  Executor   │  │
│  │  Management  │    │  Management  │    │ Management  │  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬──────┘  │
│         │                   │                    │         │
│         └───────────────────┼────────────────────┘         │
│                             │                              │
│                      ┌──────▼──────┐                       │
│                      │   Runner    │                       │
│                      │  Dispatcher │                       │
│                      └──────┬──────┘                       │
│                             │                              │
├─────────────────────────────┼──────────────────────────────┤
│         Service Engine      │                              │
│  ┌──────────────────────────▼───────────────────────────┐  │
│  │ Observability │ Events │ Accounts │ Validation      │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────┬──────────────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   Store Interface │
                    │   (Persistence)   │
                    └───────────────────┘
```

## Key Components

### 1. Service Core (`Service`)
The main service struct that orchestrates all CRE operations:
- Manages playbook lifecycle (create, update, list, get)
- Handles run execution and tracking
- Manages executor registration and configuration
- Provides observability through metrics and logging
- Enforces account-based access control

### 2. Runner Interface
Pluggable execution backend for dispatching playbook runs:
- `Runner` interface allows custom execution strategies
- `RunnerFunc` adapter for function-based implementations
- Default no-op implementation (can be replaced via `WithRunner`)

### 3. Store Interface
Persistence layer abstraction for all CRE entities:
- Playbook CRUD operations
- Run lifecycle management
- Executor registration and updates
- Account-scoped queries

### 4. HTTP API Layer
Declarative HTTP endpoints using naming convention:
- `HTTP{Method}{Path}` pattern for automatic route discovery
- RESTful resource management
- Account-scoped authorization

## Domain Types

### Playbook
Declarative workflow definition containing ordered steps.

```go
type Playbook struct {
    ID          string            // Unique identifier
    AccountID   string            // Owner account
    Name        string            // Human-readable name (required)
    Description string            // Optional description
    Tags        []string          // Searchable tags
    Steps       []Step            // Ordered execution steps (min: 1)
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last modification timestamp
    Metadata    map[string]string // Custom key-value pairs
}
```

### Step
Individual unit of work within a playbook.

```go
type Step struct {
    Name           string            // Step identifier
    Type           StepType          // Execution type (required)
    Config         map[string]any    // Type-specific configuration
    TimeoutSeconds int               // Execution timeout (0 = no timeout)
    RetryLimit     int               // Retry attempts (0 = no retry)
    Metadata       map[string]string // Custom metadata
    Tags           []string          // Step-level tags
}
```

**Supported Step Types:**
- `function_call` - Invoke a Chainlink Function
- `automation_job` - Trigger an automation workflow
- `http_request` - Execute HTTP request

### Run
Execution instance of a playbook with runtime state.

```go
type Run struct {
    ID          string            // Unique run identifier
    AccountID   string            // Owner account
    PlaybookID  string            // Source playbook
    ExecutorID  string            // Optional executor override
    Status      RunStatus         // Current execution state
    Parameters  map[string]any    // Runtime parameters
    Tags        []string          // Run-specific tags
    CreatedAt   time.Time         // Start timestamp
    UpdatedAt   time.Time         // Last state change
    CompletedAt *time.Time        // Completion timestamp
    Results     []StepResult      // Per-step execution results
    Metadata    map[string]string // Runtime metadata
}
```

**Run Status Values:**
- `pending` - Queued for execution
- `running` - Currently executing
- `succeeded` - Completed successfully
- `failed` - Execution failed
- `canceled` - Manually canceled

### Executor
Configurable backend for executing playbook runs.

```go
type Executor struct {
    ID        string            // Unique identifier
    AccountID string            // Owner account
    Name      string            // Human-readable name (required)
    Type      string            // Executor type (default: "generic")
    Endpoint  string            // Execution endpoint URL (required)
    Metadata  map[string]string // Custom configuration
    Tags      []string          // Searchable tags
    CreatedAt time.Time         // Registration timestamp
    UpdatedAt time.Time         // Last update timestamp
}
```

### StepResult
Captures execution outcome for individual steps.

```go
type StepResult struct {
    RunID       string            // Parent run identifier
    StepIndex   int               // Step position in playbook
    Name        string            // Step name
    Type        StepType          // Step type
    Status      RunStatus         // Execution status
    Logs        []string          // Execution logs
    Error       string            // Error message (if failed)
    StartedAt   time.Time         // Step start time
    CompletedAt *time.Time        // Step completion time
    Metadata    map[string]string // Result metadata
}
```

## API Endpoints

All endpoints are automatically registered via the declarative HTTP method naming convention.

### Playbooks

| Method | Path                | Handler                    | Description                |
|--------|---------------------|----------------------------|----------------------------|
| GET    | /playbooks          | `HTTPGetPlaybooks`         | List account playbooks     |
| POST   | /playbooks          | `HTTPPostPlaybooks`        | Create new playbook        |
| GET    | /playbooks/{id}     | `HTTPGetPlaybooksById`     | Get specific playbook      |

**POST /playbooks Request Body:**
```json
{
  "name": "Daily Data Sync",
  "description": "Synchronize data feeds daily",
  "steps": [
    {
      "name": "fetch-data",
      "type": "http_request",
      "config": {
        "url": "https://api.example.com/data",
        "method": "GET"
      },
      "timeout_seconds": 30,
      "retry_limit": 3
    },
    {
      "name": "process-data",
      "type": "function_call",
      "config": {
        "function_id": "func_123"
      }
    }
  ],
  "tags": ["automation", "daily"],
  "metadata": {
    "schedule": "0 0 * * *"
  }
}
```

### Runs

| Method | Path            | Handler                | Description              |
|--------|-----------------|------------------------|--------------------------|
| GET    | /runs           | `HTTPGetRuns`          | List account runs        |
| POST   | /runs           | `HTTPPostRuns`         | Create new run           |
| GET    | /runs/{id}      | `HTTPGetRunsById`      | Get specific run         |

**POST /runs Request Body:**
```json
{
  "playbook_id": "pb_abc123",
  "executor_id": "exec_xyz789",
  "parameters": {
    "target_chain": "ethereum",
    "batch_size": 100
  },
  "tags": ["manual", "test"]
}
```

**Query Parameters:**
- `limit` - Maximum results to return (default: 50, max: 500)

### Executors

| Method | Path                | Handler                    | Description                |
|--------|---------------------|----------------------------|----------------------------|
| GET    | /executors          | `HTTPGetExecutors`         | List account executors     |
| POST   | /executors          | `HTTPPostExecutors`        | Register new executor      |
| GET    | /executors/{id}     | `HTTPGetExecutorsById`     | Get specific executor      |

**POST /executors Request Body:**
```json
{
  "name": "Production Runner",
  "type": "kubernetes",
  "endpoint": "https://k8s.example.com/api/v1/run",
  "tags": ["production", "high-priority"],
  "metadata": {
    "namespace": "cre-prod",
    "resource_limits": "2cpu-4gb"
  }
}
```

## Configuration

### Service Dependencies
- `store` - Persistence layer
- `svc-accounts` - Account validation service

### Required API Surfaces
- `APISurfaceStore` - Data persistence
- `APISurfaceEvent` - Event publishing
- `APISurfaceCompute` - Execution backend

### Service Capabilities
- `cre` - Composable Run Engine operations

### Constructor Parameters
```go
func New(
    accounts AccountChecker,  // Account validation interface
    store Store,              // Persistence implementation
    log *logger.Logger        // Structured logger
) *Service
```

### Optional Configuration
```go
// Inject custom runner implementation
service.WithRunner(customRunner)
```

## Events

### Published Events

**cre.run.created**
Emitted when a new run is created.

```json
{
  "run_id": "run_abc123",
  "account_id": "acc_xyz789",
  "playbook_id": "pb_def456"
}
```

## Observability

### Metrics

**Counters:**
- `cre_playbooks_created_total{account_id}` - Total playbooks created
- `cre_playbooks_updated_total{account_id}` - Total playbook updates
- `cre_runs_created_total{account_id}` - Total runs created
- `cre_executors_created_total{account_id}` - Total executors registered

**Observations:**
All operations include distributed tracing spans with attributes:
- `account_id` - Account identifier
- `resource` - Resource type (playbook, run, executor)
- `playbook_id` - Playbook identifier (when applicable)
- `run_id` - Run identifier (when applicable)
- `executor_id` - Executor identifier (when applicable)

### Logging

Structured logs include:
- Playbook creation/updates
- Run creation and dispatch
- Executor registration/updates
- Runner dispatch failures
- Event bus unavailability warnings

## Validation Rules

### Playbook Validation
- `name` - Required, non-empty after trimming
- `steps` - Must contain at least one step
- Each step must have a valid `type` (function_call, automation_job, http_request)
- Step names auto-generated if empty (format: `step-{index}`)
- Negative timeout/retry values normalized to 0

### Executor Validation
- `name` - Required, non-empty after trimming
- `endpoint` - Required, non-empty after trimming
- `type` - Defaults to "generic" if empty

### Run Validation
- `playbook_id` - Must reference existing playbook owned by account
- `executor_id` - If provided, must reference existing executor owned by account
- Account ownership enforced for all resources

## Security

### Access Control
- All operations are account-scoped
- Ownership validation via `core.EnsureOwnership`
- Account existence verified before resource creation
- Cross-account access denied

### Data Normalization
- Metadata sanitized via `core.NormalizeMetadata`
- Tags normalized via `core.NormalizeTags`
- String fields trimmed of whitespace
- Deep cloning of configuration maps to prevent mutation

## Testing

### Unit Tests
```bash
# Run all CRE service tests
go test ./packages/com.r3e.services.cre/...

# Run with coverage
go test -cover ./packages/com.r3e.services.cre/...

# Run with race detection
go test -race ./packages/com.r3e.services.cre/...
```

### Integration Tests
Requires:
- Mock or real Store implementation
- Mock AccountChecker
- Test logger instance

Example test setup:
```go
mockStore := &MockStore{}
mockAccounts := &MockAccountChecker{}
logger := logger.NewTest()

service := cre.New(mockAccounts, mockStore, logger)

// Inject test runner
testRunner := cre.RunnerFunc(func(ctx context.Context, run cre.Run, pb cre.Playbook, exec *cre.Executor) error {
    // Custom test logic
    return nil
})
service.WithRunner(testRunner)
```

## Dependencies

### Internal Dependencies
- `github.com/R3E-Network/service_layer/pkg/logger` - Structured logging
- `github.com/R3E-Network/service_layer/system/core` - Core engine types
- `github.com/R3E-Network/service_layer/system/framework` - Service framework
- `github.com/R3E-Network/service_layer/system/framework/core` - Framework utilities

### External Dependencies
None (standard library only)

## Error Handling

### Common Errors
- `core.RequiredError("field")` - Required field missing
- `core.ErrNotFound` - Resource not found
- `core.ErrUnauthorized` - Account ownership violation
- `core.ErrBusUnavailable` - Event bus unavailable (non-fatal)

### Error Propagation
- Store errors propagated directly
- Validation errors returned before persistence
- Runner dispatch errors logged but don't fail run creation
- Event publishing failures logged as warnings if bus unavailable

## Best Practices

### Playbook Design
1. Keep steps focused and single-purpose
2. Use descriptive step names for debugging
3. Set appropriate timeouts for external calls
4. Use retry limits for transient failures
5. Tag playbooks for easy filtering

### Run Management
1. Pass runtime parameters instead of hardcoding in playbooks
2. Use tags to track run context (manual, scheduled, etc.)
3. Monitor run status via observability metrics
4. Handle failed runs with appropriate retry logic

### Executor Configuration
1. Use dedicated executors for different environments
2. Configure resource limits in metadata
3. Implement health checks at executor endpoints
4. Use tags to route runs to appropriate executors

## Future Enhancements

Potential areas for extension:
- Conditional step execution (if/else logic)
- Parallel step execution
- Step output passing between steps
- Scheduled playbook execution
- Run cancellation API
- Step result pagination
- Playbook versioning
- Executor health monitoring

## License

Copyright R3E Network. All rights reserved.
