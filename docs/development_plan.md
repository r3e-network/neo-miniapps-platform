Development Plan
================

This roadmap breaks the refactor into discrete, verifiable milestones. Treat
each step as a mini project—merge only when the tasks for that step are green.

Step 0 – Clean Baseline
-----------------------

- Create a new branch from `origin/master` (or reclone the repository).
- Preserve any experimental work with `git stash -u`.
- Ensure `go test ./...` passes before starting the refactor.

Step 1 – Core Scaffolding *(Completed)*
-------------------------

- Introduce the `internal/app` hierarchy described in the architecture doc.
- Implement `system.Manager`, `storage/interfaces.go`, and in-memory adapters.
- Add account/function/trigger services with unit tests and in-memory stores.
- Create the `Application` facade (`internal/app/application.go`).
- Add a smoke HTTP API (`internal/app/httpapi`) and new `cmd/appserver` entry
  point.
- CI gate: `go test ./...`.

Step 2 – Persistence Layer *(In Progress)*
--------------------------

- Port migrations into `internal/platform/database` (or equivalent).
- Implement Postgres adapters in `internal/app/storage/postgres`.
- Add integration tests using the adapters (dockerized Postgres or test
  containers).
- Wire adapters through config (`platform/config`). *(CLI now accepts DSN/config to provision Postgres-backed stores.)*
- Update documentation with connection settings and migration process. *(See `docs/operations/postgres_setup.md`.)*

Step 3 – Domain Ports
---------------------

Iterate per module (gas bank, price feed, oracle, TEE, automation):

1. Define/verify domain contracts in `internal/app/domain`.
2. Implement storage adapter(s).
3. Implement service logic in `internal/app/services/<module>`.
4. Add tests (unit + integration with in-memory store).
5. Expose HTTP endpoints.
6. Register service with the manager.

Only after a module is complete should the old behaviour be removed.

Step 4 – Runtime Orchestration
------------------------------

- Build the execution runtime (work queues, concurrency limits).
- Implement trigger scheduler (cron/event-based, blockchain events).
- Integrate with domain services to schedule and execute functions.
- Add metrics (queue depth, execution latency) and detailed logging.

Step 5 – Observability & Ops
----------------------------

- Standardize logging format (structured JSON) and propagate request IDs.
- Instrument metrics for all critical paths.
- Add health/readiness endpoints and wiring for Kubernetes.
- Document deployment guidelines (env vars, secrets, scaling knobs).

Step 6 – Cleanup & Legacy Removal
---------------------------------

- Remove unused packages once the new modules achieve feature parity.
- Update README and user documentation to point to the new entrypoints.
- Archive or delete old CI jobs and scripts.

Documentation Checklist
-----------------------

- Update `docs/architecture/README.md` with any structural changes.
- Maintain module ADRs (Architecture Decision Records) under `docs/adr/` for
  significant choices.
- Provide runbooks for operations under `docs/runbooks/`.

Review cadence: each milestone should produce a merge request reviewed by at
least one other developer. Do not proceed to the next milestone until review
comments are addressed and tests pass.
