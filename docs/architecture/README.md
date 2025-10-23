Service Layer Architecture
==========================

This document codifies the target architecture for the service layer so that
development can proceed module by module and layer by layer. Treat it as the
source of truth when adding or refactoring services.

Guiding Principles
------------------

- **Separation of concerns** – configuration, domain logic, persistence,
  transport, and orchestration live in distinct packages.
- **Dependency injection via interfaces** – every service depends on contracts
  declared in `internal/app/storage` or domain packages, not concrete
  implementations.
- **Deterministic lifecycle** – the `system.Manager` starts/stops services in a
  predictable order. Each module must implement `Start`/`Stop`.
- **Testability first** – in-memory adapters exist for every contract, enabling
  unit/integration tests without external infrastructure.

Layered Layout
--------------

```
internal/
  app/
    system/        # Lifecycle manager, service interfaces
    storage/       # Contracts + adapters (memory, postgres…)
    domain/        # Domain models (accounts, functions, triggers, ...)
    services/      # Business logic per domain
    runtime/       # Execution engines, schedulers
    httpapi/       # HTTP handlers and router wiring
  platform/
    config/        # Config loading & validation
    logging/       # Structured logging helpers
    metrics/       # Prometheus registry + instrumentation

cmd/
  appserver/       # Entry point that wires everything together
```

Each new module should slot into this structure. Keep experimental features in
clearly scoped subpackages until they mature.

Service Lifecycle
-----------------

1. All modules implement `system.Service` (`Name()`, `Start(ctx)`, `Stop(ctx)`).
2. The entrypoint constructs an `Application`, registers services via
   `application.Attach(...)`, and finally calls `Start`.
3. Shutdown propagates via `Stop(ctx)`; services must block until child
   goroutines exit or the timeout elapses.

Domain Services
---------------

| Domain      | Responsibility                                                     |
|-------------|---------------------------------------------------------------------|
| Accounts    | Tenant metadata, ownership, quotas                                  |
| Functions   | Function definitions, secret references, versioning, orchestration  |
| Triggers    | Event scheduling, dependency validation                             |
| Automation  | Cron/interval scheduling of functions                               |
| Gas Bank    | Account balances, deposits, withdrawals                             |
| Price Feed  | Market data feeds & historical snapshots                            |
| Oracle      | External data ingestion & contract callbacks                        |
| Runtime     | Execution engines, job queues, schedulers                           |
| Integrations| Blockchain, price feeds, oracle adapters                            |

Each domain service owns validation and orchestration logic but defers to
storage interfaces for persistence.

Storage Contracts
-----------------

Storage interfaces live in `internal/app/storage`. For each domain, define a
contract (e.g., `AccountStore`, `FunctionStore`, `TriggerStore`,
`GasBankStore`). Implementations include:

- `memory` – default in-memory adapter used for tests and local development.
- `postgres` – real persistence layer (to be implemented per domain). Start by
  porting the contract, migrations, and repository logic from the existing code
  base.

Runtime & Scheduling
--------------------

- `runtime.Executor` – executes function definitions (JS, WASM, external tasks).
- `runtime.Scheduler` – evaluates triggers and enqueues executions.
- Runtime components should expose metrics (queue depth, execution success) and
  logs.

Transport Layer
---------------

- HTTP API routes live under `internal/app/httpapi`.
- Handlers depend only on application services, not storage.
- Middleware (auth, logging, metrics) is composed centrally.

Observability
-------------

- Structured logging with request IDs.
- Prometheus metrics exported by each service via instrumentation helpers.
- Health probes: readiness (dependencies up), liveness (manager running), and
  module-specific diagnostics.

Migration Strategy
------------------

1. **Baseline** – merge the new `internal/app` scaffolding and ensure tests pass.
2. **Persistence** – create `storage/postgres` adapters and align migrations.
3. **Service Ports** – keep services isolated under `internal/app/services/<module>`
   using shared contracts.
4. **Transport** – extend the `httpapi` endpoints as new capabilities land.
5. **Cleanup** – remove superseded code paths quickly so the runtime stays lean.

Document any deviations here before implementing them.

Additional References
---------------------

- [Persistence Architecture](persistence.md)
- [Accounts Module](modules/accounts.md)
- [Functions Module](modules/functions.md)
- [Gas Bank Module](modules/gasbank.md)
- [Automation Module](modules/automation.md)
- [Triggers Module](modules/triggers.md)
- [Price Feed Module](modules/pricefeed.md)
- [Oracle Module](modules/oracle.md)
