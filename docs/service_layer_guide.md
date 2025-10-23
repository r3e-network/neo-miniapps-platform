# Service Layer Guide

The service layer packages the business logic that powers the Neo N3 runtime.
Each service encapsulates a bounded domain, keeps persistence abstracted behind
interfaces, and exposes a focused API that can be driven programmatically or
through the HTTP surface in `internal/app/httpapi`.

This guide walks through every service, outlines the primary entry points, and
shows how the self-contained examples and tests exercise the behaviour. Treat it
as a map when integrating new features or reading the code base.

## Reading the Examples

Every service ships with a runnable example in `*_test.go`. Running
`go test ./...` will execute them, ensuring the snippets stay in sync with the
code. Each example uses an in-memory store (from `internal/app/storage/memory`)
and a logger that discards output, so the printed results are deterministic.

## Accounts Service

- File: `internal/app/services/accounts/service.go`
- Responsibilities: manage account lifecycle and metadata.
- Example: `ExampleService_Create` in `service_test.go` shows how to create an
  account with metadata using the in-memory store.
- Tests: `TestService` covers create, update, and list flows.
- Key interfaces: `storage.AccountStore`.

## Automation Service

- File: `internal/app/services/automation/service.go`
- Responsibilities: create, update, and manage scheduled jobs that will execute
  functions.
- Example: `ExampleService_CreateJob` in `service_test.go` creates a job wired to
  a function and demonstrates the default enabled state.
- Additional components:
  - `scheduler.go` polls due jobs and dispatches them.
  - `function_dispatcher.go` executes the linked function and records results.
- Tests: `TestService_CreateAndUpdateJob`, `TestScheduler_RespectsNextRun`,
  `TestFunctionDispatcher_Run`.
- Key interfaces: `storage.AutomationStore`, `JobDispatcher`, `FunctionRunner`.

## Functions Service

- File: `internal/app/services/functions/service.go`
- Responsibilities: store function definitions, execute code through an executor
  (default simple executor or the TEE Goja executor), bridge to auxiliary
  services (automation, price feed, oracle, gas bank, triggers), and record
  execution history.
- Example: `ExampleService_Execute` demonstrates creating and executing a
  function with the simple executor.
- Devpack runtime: functions can queue gas bank, oracle, trigger, and automation
  actions through the injected `Devpack` helper. The queued actions execute
  after the JavaScript finishes and their outcomes are captured in the execution
  record (`execution.Actions`). See `docs/function_devpack.md` for API details.
- Tests: coverage for CRUD (`TestService`), execution success/failure, secret
  validation (`TestService_UpdateValidatesSecrets`), and the TEE executor
  (`tee_executor_test.go`).
- Key interfaces: `storage.FunctionStore`, `FunctionExecutor`,
  `SecretResolver`.

## Gas Bank Service

- File: `internal/app/services/gasbank/service.go`
- Responsibilities: ensure gas accounts exist, handle deposits/withdrawals, and
  settle pending withdrawals with the settlement poller.
- Example: `ExampleService_Deposit` illustrates crediting an account and reading
  the resulting balance/transaction status.
- Tests: deposit/withdraw lifecycle, rollback scenarios,
  `TestService_WithdrawRollsBackOnTransactionFailure`, and the settlement poller
  (`settlement_test.go`).
- Key interfaces: `storage.GasBankStore`, `WithdrawalResolver`.

## Oracle Service

- File: `internal/app/services/oracle/service.go`
- Responsibilities: manage data sources, queue requests, and respond to external
  resolver callbacks.
- Example: `ExampleService_CreateRequest` submits a new request and prints the
  initial status.
- Tests: source deduplication and update flow, request lifecycle, dispatcher and
  resolver integration.
- Key interfaces: `storage.OracleStore`, `RequestResolver`.

## Price Feed Service

- File: `internal/app/services/pricefeed/service.go`
- Responsibilities: manage feed definitions, record snapshots, and periodically
  refresh active feeds through the refresher.
- Example: `ExampleService_CreateFeed` creates an active BTC/USD feed.
- Tests: feed lifecycle, refresher behaviour, HTTP fetcher, and the new
  inactive-feed guard (`TestRefresherSkipsInactiveFeeds`).
- Key interfaces: `storage.PriceFeedStore`, `Fetcher`.

## Secrets Service

- File: `internal/app/services/secrets/service.go`
- Responsibilities: store encrypted secrets, resolve plaintext for execution,
  rotate values, and enforce naming rules.
- Example: `ExampleService_Create` stores a secret and resolves it immediately.
- Tests: create/get, update/list/delete, resolver behaviour, custom cipher
  (AES-GCM) usage, and validation errors.
- Key interfaces: `storage.SecretStore`, `Cipher`, `Resolver`.

## Triggers Service

- File: `internal/app/services/triggers/service.go`
- Responsibilities: validate and register triggers that connect events or
  schedules to functions.
- Example: `ExampleService_Register` creates an event trigger for a function.
- Tests: registration, enable/disable, webhook validation, multiple trigger
  types.
- Key interfaces: `storage.TriggerStore`.

## Random Service

- File: `internal/app/services/random/service.go`
- Responsibilities: generate cryptographically secure random bytes and provide
  helper encoding.
- Example: `ExampleService_Generate` generates four random bytes and prints the
  raw/encoded lengths.
- Tests: happy path generation and validation of bounds (`TestServiceGenerate`,
  `TestServiceGenerateInvalidLength`).

## Integration Points

- Storage interfaces are defined in `internal/app/storage/interfaces.go`. The
  in-memory implementation (`internal/app/storage/memory`) is used for tests and
  examples; the PostgreSQL implementation lives under
  `internal/app/storage/postgres`.
- The HTTP API adapter in `internal/app/httpapi` wires the services together and
  exposes REST endpoints. See `docs/runtime_quickstart.md` for request samples.

## Testing the Service Layer

- Run the entire suite: `go test ./...`
- Run a focused subset (for example, functions):  
  `go test ./internal/app/services/functions -v`
- Examples double as documentation; `go test` verifies their output automatically.

## Extending the Layer

1. Identify the service (or create a new package) under `internal/app/services`.
2. Add or update the storage interface in `internal/app/storage/interfaces.go`.
3. Implement the logic, keeping side effects behind interfaces.
4. Provide unit tests and, when useful, an example function in the `*_test.go`.
5. Update this guide and any API docs when the surface changes.

For end-to-end wiring details, review `internal/app/runtime` and the
architecture notes in `docs/architecture_overview.md`.
