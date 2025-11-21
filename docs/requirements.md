# Neo N3 Service Layer Specification

## Purpose & Scope
- Deliver a multi-tenant orchestration runtime for Neo N3 that exposes automation, oracle, price feed, randomness, gas bank, wallet, and confidential-compute utilities behind a consistent HTTP API, CLI, and SDK.
- Preserve compatibility with existing Neo services while introducing Chainlink parity (CRE, CCIP, Data Feeds, Data Streams, DataLink, DTA, VRF, Confidential Compute) in the same Go backend.
- Provide a lightweight developer experience that can run entirely in-memory for experimentation or attach to PostgreSQL for persistence, without requiring Redis or external schedulers. Blockchain node management, billing, and external TEE hosts remain out of scope for this repository.

## Documentation Governance
- This specification is the single source of truth for product behaviour, APIs, storage, and operations. Update it before changing code, Helm charts, or SDKs.
- Use [`docs/README.md`](README.md) as the entry point for related references (Devpack examples, dashboard notes, infrastructure docs).
- Every feature/change proposal must link to the relevant sections below when discussed in issues/PRs so reviewers can trace intent to implementation.
- Deprecations and removals should be staged through this document first, with migration notes captured alongside the affected services.
- Service descriptors expose a single `platform` layer so every capability carries the same priority; update descriptors + docs alongside CLI/dashboard expectations.
- During reviews, step through the [Service Layer Review Checklist](review-checklist.md) to keep documentation, CLI, and dashboard coverage aligned.

## System Overview
- `cmd/appserver` is the single Go binary that wires all services (`internal/app/services/*`), HTTP handlers (`internal/app/httpapi`), and storage adapters (`internal/app/storage/{memory,postgres}`).
- `cmd/slctl` is the CLI wrapper over the HTTP API. It honours `SERVICE_LAYER_ADDR` and `SERVICE_LAYER_TOKEN` for pointing at different environments.
- `apps/dashboard` hosts the React + Vite front-end surface that consumes the same HTTP API for operator workflows. Additional surfaces must be documented here before landing in the repo.
- `sdk/devpack` plus `examples/functions/devpack` provide the TypeScript toolchain used to author functions locally. Devpack helpers let functions queue automation, oracle, price feed, gas bank, and trigger actions that execute after the JavaScript runtime completes.
- Persistence defaults to in-memory stores. Supplying a PostgreSQL DSN (via `-dsn`, `DATABASE_URL`, or config files under `configs/`) switches all services to the Postgres adapters and enables migrations embedded in `internal/platform/migrations` (0001–0017).
- Optional TEE execution paths use the Goja JavaScript runtime today and can be swapped with Azure Confidential Computing-backed executors when runners are provisioned (see Confidential Compute sections for expectations).

## Functional Requirements
### Service Catalogue
#### Accounts & Authentication
- Manage account/workspace records, metadata, and lifecycle through `/accounts` endpoints and enforce per-account ownership across dependent services.
- Enforce HTTP authentication via static bearer tokens configured through `API_TOKENS` or the `-api-tokens` flag. All endpoints except `/healthz` and `/system/version` require a valid `Authorization: Bearer <token>` header.
- Provide workspace-level capability descriptors via `/system/descriptors` for dashboards/CLI discovery.

#### Workspace Wallets
- Register signer wallets, threshold policies, and association to accounts. Wallet metadata is exposed under `/accounts/{id}/workspace-wallets` and used by downstream services (CCIP, VRF, Data Feeds, DTA) to gate actions.

#### Secrets Vault
- Store, rotate, and resolve AES-GCM encrypted secrets per account. Plaintext material must never be persisted; secret resolution only happens in-memory during function execution.
- Require `SECRET_ENCRYPTION_KEY` (raw/base64/hex) whenever PostgreSQL is enabled. Reject operations if encryption is misconfigured.

#### Functions Runtime
- Provide CRUD, execution, and execution-history endpoints for JavaScript functions compiled via Goja.
- Allow immediate invocation (`/functions/{id}/execute`) and integrate with automation, oracle, price feed, gas bank, trigger, and randomness services through Devpack action queues.
- Record execution metadata (inputs, outputs, action results, errors). Store deterministic randomness signatures when `RANDOM_SIGNING_KEY` is configured.
- Support pluggable executors, including the TEE executor path, with per-execution logging and timeout enforcement.

#### Automation & Triggers
- Support creation and management of automation jobs (time-based, cron-like, event-driven) under `/automation/jobs` and maintain job run history.
- Register triggers that connect events/webhooks to function executions. Validate payload schemas, enable/disable triggers, and ensure cross-service ownership checks.
- Provide schedulers and dispatchers that retry failures, emit metrics, and can be paused for maintenance.

#### Oracle Adapter
- Manage oracle data sources, queue requests, and accept resolver callbacks/webhook updates.
- Support multiple sources per feed with configurable aggregation (e.g., threshold/median); requests may specify alternate source IDs and aggregators should median/quorum the results.
- Track request lifecycle states (pending, running, succeeded, failed) with persisted attempts, TTL/backoff/DLQ metadata, and idempotent updates. Expired/exhausted requests must dead-letter and surface clearly to operators.
- Provide operator tooling to list failed/expired requests and manually retry them (e.g., via CLI/HTTP PATCH).
- Allow per-source authentication headers, outbound host allowlisting, and per-account/source rate limits. Runners/resolvers must authenticate when marking requests running/complete via shared runner tokens.
- Attach schema/versioning to request payloads and constraints on result size; expose latency/success metrics and SLA windows.
- Optional signed result/attestation output with chain/job/spec identifiers for downstream consumers.

#### Price Feed Service
- Create/edit price feed definitions, store historical snapshots, and run periodic refreshers that fetch external data via `PRICEFEED_FETCH_URL` (with optional API keys).
- Emit deviation-based triggers and record submission metadata. Guard against updates when feeds are inactive.

#### Gas Bank
- Manage gas accounts, deposits, withdrawals, and settlement polling. Integrate with external withdrawal resolvers defined by `GASBANK_RESOLVER_URL` and optional bearer tokens.
- Expose retry cadence and polling interval tuning via `GASBANK_POLL_INTERVAL` and `GASBANK_MAX_ATTEMPTS` (or `runtime.gasbank.poll_interval` / `runtime.gasbank.max_attempts` in config files).
- Support scheduled withdrawals via a future `schedule_at` timestamp; cron expressions are not yet supported and must be rejected.
- Enforce transactional safety (rollbacks on resolver failures), provide transaction history listings, and expose balances via the HTTP API.

#### Randomness Service
- Generate cryptographically secure random bytes per account and optionally sign responses when deterministic replay is required.
- Enforce bounds on the requested length (default 32 bytes, max 1024) and provide multiple encodings in responses.
- Expose `/accounts/{id}/random` for generation requests and `/accounts/{id}/random/requests` for retrieving recent history.

#### Observability & System Services
- Expose `/metrics` for Prometheus scraping and `/healthz` for readiness probes. `/metrics` requires authentication; `/healthz` and `/system/version` remain unauthenticated for orchestrators/discovery.
- Emit structured logs annotated with account/service identifiers. Provide hooks for tracing/metrics per service (see `internal/app/metrics`).
- Enforce pagination limits via `limit` parameters to protect the API, returning cursors/tokens where applicable.

#### CRE Orchestrator
- Manage playbooks, runs, and executors for cross-runtime automation.
- Provide CRUD endpoints, execution tracking, and executor registration/heartbeat flows with per-account scoping.

#### CCIP
- Register cross-chain lanes, attach workspace wallets for signing, and manage message dispatch.
- Track status transitions (queued, sending, dispatched, failed) and surface retry metadata to operators.

#### Data Feeds
- Manage the feed registry, signer sets, decimals, and update submission metadata.
- Enforce wallet-gated permissions for submissions and store historic rounds + signatures for auditing. Submissions must validate cryptographic signatures against the configured signer set, enforce minimum signer thresholds, and aggregate (e.g., median) multiple submissions per round.
- Apply price/decimals validation, heartbeat/deviation enforcement, and replay protection per signer/round. Expose metrics for stale/under-signed rounds, signer participation, submission latency, and deviations.

#### Data Streams
- Configure high-frequency ingestion, frame publication, SLAs, and retention enforcement parameters.
- Expose frame listings and latency metrics per stream for troubleshooting.

#### DataLink
- Define provider channels, delivery attempts, payload schemas, and retry policies for off-chain data movement.
- Surface per-delivery logs/status plus signer information required for attestations.

#### DTA
- Orchestrate subscription/redemption workflows with approval requirements and wallet checks.
- Track product catalogues, orders, and settlement instructions with immutable audit history.

#### VRF
- Register randomness keys bound to workspace wallets and submit/retrieve randomness requests with attested proofs.
- Store attestation material, consumer metadata, and fulfillment status for each request.

#### Confidential Compute
- Track enclaves, sealed keys, attestation material, and coordinate runner assignments for trusted execution.
- Enforce compliance controls for key upload/rotation and expose enclave health for operators.

### Tooling & Developer Experience
- Maintain parity between CLI commands, HTTP endpoints, and SDK helpers so every capability is scriptable.
- Publish runnable examples (`go test ./...`) for each service to serve as living documentation.
- Provide dashboard modules for operator visibility, backed by the same API routes.

## API & Interface Requirements
### HTTP API
- All routes live under `/accounts/{accountID}/...` (plus `/accounts`, `/metrics`, `/healthz`, `/system`). Payloads are JSON with standard HTTP status codes.
- Enforce bearer-token auth, input validation, and consistent error envelopes (message + code + field errors when applicable).
- List endpoints accept `limit`/`cursor` parameters with enforced bounds and deterministic ordering.
- Long-running operations (automation runs, CRE executions) expose status resources that clients can poll. Webhook callbacks validate shared secrets when configured.

### CLI (`cmd/slctl`)
- Covers every documented service. Current surface:
  - `slctl accounts list|get|create|delete`.
  - `slctl functions list|get|create|delete` plus execution helpers.
  - `slctl automation jobs ...` and `slctl secrets ...` for operator workflows.
  - `slctl gasbank ...` for balances/transactions.
- `slctl oracle sources|requests ...` to manage adapters; `oracle requests list --status <state> --limit <n> [--cursor|--all]` for DLQ triage and `oracle requests retry` to requeue failures.
  - `slctl pricefeeds list|create|get|snapshots`.
  - `slctl cre playbooks|executors|runs --account <id> [--limit n]` for Chainlink Reliability Engine inventory and activity.
  - `slctl ccip lanes|messages --account <id> [--limit n]` for cross-chain lane/message introspection.
  - `slctl vrf keys|requests --account <id> [--limit n]` for randomness key inventory and request history.
  - `slctl datalink channels|deliveries --account <id> [--limit n]` for data movement channel/delivery auditing.
  - `slctl dta products|orders --account <id> [--limit n]` for subscription/redemption inventory review.
  - `slctl datastreams streams|frames --account <id> [--limit n]` for stream configuration/frame inspection.
  - `slctl confcompute enclaves --account <id> [--limit n]` for confidential-compute inventory.
  - `slctl workspace-wallets list --account <id> [--limit n]` for signer/inventory auditing.
  - `slctl random generate` (trigger a draw) and `slctl random list` (recent `/random/requests` history).
  - `slctl services list` for descriptor discovery.
- Reads configuration from flags/environment variables and prints machine-readable tables/JSON for scripting. Extend CLI coverage alongside new HTTP endpoints so every catalogued service eventually ships with parity commands.

### TypeScript Devpack & SDK
- Provide helpers for declaring functions, managing secrets, enqueuing Devpack actions (automation jobs, oracle requests, price feed updates, gas bank operations), and validating inputs locally.
- Include scaffolding/templates under `examples/functions/devpack` to bootstrap projects.

### Dashboard & External Touchpoints
- The React + Vite dashboard authenticates via the same API tokens and surfaces account/service management flows.
- External integrations: Stripe/Resend for billing/support communications, Logtail/Axiom for log shipping, Neo N3 RPC nodes and other data providers for service execution.

## Data Management & Persistence
- PostgreSQL 14+ is the canonical store. Tables cover accounts, workspace wallets, secrets (encrypted values + metadata), functions (definitions, executions, action history), automation jobs/runs, triggers, oracle sources/requests, price feeds/snapshots, gas accounts/transactions, randomness history, and each service entity described in this catalogue (CRE, CCIP, VRF, Data Feeds, Data Streams, DataLink, DTA, Confidential Compute).
- All database mutations go through versioned migrations under `internal/platform/migrations`; the server auto-applies them when `-migrate` is enabled (default true) in Postgres mode.
- In-memory adapters (under `internal/app/storage/memory`) provide a dependency-free option for tests and local experimentation.
- Secrets are encrypted before persistence; other sensitive blobs (sealed keys, attestations) follow the same cipher utilities.
- When Postgres is enabled, startup must fail if the configured secret encryption key is missing or invalid to avoid persisting plaintext values.
- No external cache is required; any caching remains in-process. When future Redis integrations are added, they must remain optional and feature-flagged.

## Non-Functional Requirements
### Security
- Mandatory TLS termination (handled by the deployment environment) in front of the HTTP API.
- Bearer-token authentication, per-account authorization checks, and validation of cross-service ownership on every request.
- AES-GCM encryption for secrets and sealed materials; zero plaintext storage. Enforce secure key rotation and audit secret access through execution logs.
- Optional TEE executors must verify attestation quotes before accepting workloads.

### Reliability & Observability
- Provide `/healthz` for liveness/readiness, structured logs, and `/metrics` compatible with Prometheus.
- Emit per-service metrics (execution counts, latencies, success/failure) via the observation hooks defined in `internal/app/metrics`.
- Automation, oracle, CCIP, and delivery pipelines include retry policies with jitter and exponential backoff. Failed jobs must surface in metrics and execution history.

### Performance
- Aim for sub-100ms median latency for metadata/CRUD endpoints and keep function execution scheduling overhead minimal (<10ms beyond runtime execution) under nominal load.
- Price feed refreshers and gas bank settlement pollers must support configurable concurrency and respect external rate limits.

### Scalability
- Support horizontal scaling by running multiple `appserver` instances behind a load balancer; all shared state resides in Postgres.
- Enforce pagination and request limits to protect cluster resources. Each service should guard per-account quotas (feeds, streams, wallets, keys, etc.).

### Maintainability & Developer Experience
- Keep services isolated (`internal/app/services/<name>`), with interfaces documented in `internal/app/storage/interfaces.go` and examples in `*_test.go`.
- Require `go test ./...` to pass before merge, and update this specification whenever surfaces change so it remains the single source of truth.

### Compliance & Auditability
- Record execution/action history and include account IDs, timestamps, and operator identifiers in logs for traceability.
- Preserve attestations, approvals, and wallet mappings required for regulated services (DTA, VRF, Confidential Compute).

## Technical Constraints & Dependencies
- Go 1.24+ for the backend (matching `go.mod`), Node.js 20+ with React 18+/Vite 7+ for the dashboard SPA, and TypeScript 5+ for the Devpack SDK.
- PostgreSQL 14+ with UUID support and advisory locks. The runtime embeds migrations and does not rely on external migration tools at runtime.
- Optional Azure Confidential Computing or equivalent TEE infrastructure for hardware-backed execution. Until production-ready, the software Goja executor remains the default.
- External systems: Neo N3 RPC nodes, price/oracle data providers, Stripe/Resend (billing/support), Logtail/Axiom (log shipping), observability backends compatible with Prometheus/OpenTelemetry.

## Testing, Delivery & Operations
- Unit and integration tests (`go test ./...`) exercise every service, including HTTP handlers and Postgres adapters. Examples in `*_test.go` double as documentation and must remain deterministic.
- CI/CD runs via GitHub Actions (`.github/workflows/ci-cd.yml`), ensuring lint/test/build coverage on every change.
- Local development: `go run ./cmd/appserver` (in-memory) or `docker compose up --build` (Postgres). CLI interactions use `go run ./cmd/slctl` or the installed binary.
- Configuration: `DATABASE_URL`, `API_TOKENS`, `SECRET_ENCRYPTION_KEY`, `PRICEFEED_FETCH_URL`, `GASBANK_RESOLVER_URL`, `RANDOM_SIGNING_KEY`, and logging level env vars or config files under `configs/` (see `configs/README.md` for the samples).
- Deployment artifacts are containerized via `Dockerfile` and `docker-compose.yml`. Rolling updates must respect migrations (run once) and maintain backward compatibility of the HTTP API.
- Helper automation lives under `scripts/` (see `scripts/README.md`) and mirrors the workflows documented in this section.
