# Neo N3 Service Layer

![Service Layer logo](docs/assets/service-layer-logo.svg)

[![Build Status](https://github.com/R3E-Network/service_layer/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/R3E-Network/service_layer/actions/workflows/ci-cd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/R3E-Network/service_layer)](https://goreportcard.com/report/github.com/R3E-Network/service_layer)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

The Service Layer provides a lightweight orchestration runtime for Neo N3. It
wraps account management, function execution, automation, and gas-bank utilities
behind a simple HTTP API. The runtime can run entirely in-memory for local
experimentation, or wire itself to PostgreSQL when a DSN is supplied.

## Requirements

- Go 1.24+ (matching the toolchain declared in `go.mod`)
- Node.js 20+ / npm 10+ for building the dashboard and Devpack examples
- PostgreSQL 14+ when persistence is required (in-memory stores remain the default)

## Current Capabilities

- Account registry with pluggable storage (memory by default, PostgreSQL when
  configured)
- Function catalogue and executor with trigger, automation, oracle, price-feed,
  data-feed, data-stream, DataLink, randomness, and gas-bank integrations
- Devpack runtime with declarative action queueing and SDKs for authoring
  functions locally (TypeScript, plus matching helpers in Go/Rust/Python)
- Secret vault with optional encryption and runtime resolution for function
  execution
- Cryptographically secure random number generation per account
- Workspace wallets gate data feeds and DataLink channels; signer sets are enforced per account.
- Modular service manager that wires the domain services together
- HTTP API located in `internal/app/httpapi`, exposing the new surface under
  `/accounts/...`
- Auditing: in-memory by default, with optional JSONL persistence via `AUDIT_LOG_PATH` and an admin-only `/admin/audit` endpoint (dashboard viewer included).

## Quick Start

Local full-stack (API + Postgres + dashboard + marketing site):

```bash
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer
make run  # copies .env.example if missing, then docker compose up -d --build
```

Once up:
- API: http://localhost:8080 (auth: `Authorization: Bearer dev-token` or JWT via `/auth/login` using admin/changeme)
- Dashboard: http://localhost:8081 (prefills when opened as `http://localhost:8081/?baseUrl=http://localhost:8080&token=dev-token`)
- Public site: http://localhost:8082

CLI or manual server (in-memory):

```bash
export API_TOKENS=dev-token   # or set AUTH_USERS for JWT: admin:changeme:admin
go run ./cmd/appserver
go run ./cmd/slctl --token "$API_TOKENS" accounts list
```

To force PostgreSQL without docker compose, supply a DSN via flag or env. Migrations
are embedded and executed automatically when `-migrate` is left enabled:

```bash
go run ./cmd/appserver -dsn "postgres://user:pass@localhost:5432/service_layer?sslmode=disable"
```

Auditing (optional):
- Set `AUDIT_LOG_PATH=/var/log/service-layer-audit.jsonl` to persist audit events (JSONL) in addition to the in-memory buffer.
- View recent audit entries via `GET /admin/audit?limit=200` (admin JWT required) or the dashboard Admin panel. Token-only auth is not admin.

Examples for Devpack usage live under `examples/functions/devpack` (JS + TS samples for price feeds, randomness, gasbank/oracle orchestration). API examples for all services are in `docs/examples/services.md`. Polyglot SDKs mirroring the Devpack surface live under `sdk/go`, `sdk/rust`, and `sdk/python`.

Check `examples/functions/devpack` for a TypeScript project that uses the SDK to
ensure gas accounts and submit oracle requests.

## Operator Interfaces

- **CLI (`cmd/slctl`)** — wraps the HTTP API for scripting. Honours `SERVICE_LAYER_ADDR`
  and `SERVICE_LAYER_TOKEN` like the server. Use it to create accounts, register functions,
  request randomness (`slctl random generate --account <id> --length 64`) or inspect recent draws (`slctl random list --account <id>`), and inspect automation/oracle history from a terminal.
- **Dashboard (`apps/dashboard`)** — React + Vite SPA for day-to-day operations. See
  `apps/dashboard/README.md` for Docker/local instructions. Configure API and Prometheus
  endpoints in the UI once the server is running.

### CLI Quick Reference
- `slctl accounts list|get|create|delete` — manage account records.
- `slctl functions list|get|create|delete` (+ execution helpers) — deploy and inspect functions.
- `slctl automation jobs ...` / `slctl secrets ...` — administer schedulers and secret vault entries.
- `slctl gasbank ...` — view balances and transfer history.
- `slctl oracle sources|requests ...` — configure HTTP adapters and inspect inflight work.
- `slctl datastreams ...` — list/create streams or publish frames.
- `slctl datalink ...` — list/create channels and queue deliveries.
- `slctl datafeeds ...` — manage data feed definitions and submit/list rounds (with per-feed aggregation).
- `slctl pricefeeds list|create|get|snapshots` — define asset pairs and monitor submissions.
- `slctl jam ...` — upload preimages and submit/list packages/reports.
- `slctl random generate --account <id> --length <n>` — request deterministic bytes.
- `slctl random list --account <id> [--limit n]` — fetch recent `/random/requests` history.
- `slctl cre playbooks|executors|runs --account <id>` — inspect Chainlink Reliability Engine assets and activity.
- `slctl ccip lanes|messages --account <id>` — list cross-chain lanes and recent CCIP messages.
- `slctl vrf keys|requests --account <id>` — inspect VRF key inventory and recent randomness requests.
- `slctl datalink channels|deliveries --account <id>` — inspect data movement channels and recent delivery attempts.
- `slctl dta products|orders --account <id>` — inspect DTA product catalogues and order history.
- `slctl datastreams streams|frames --account <id>` — inspect high-frequency streams and recent frames.
- `slctl confcompute enclaves --account <id>` — inspect confidential-compute enclave inventory.
- `slctl jam status|packages|reports|receipt|receipts` — inspect JAM status, packages/reports, and accumulator receipts/roots.
- `slctl workspace-wallets list --account <id>` — inspect registered signing wallets.
- `slctl services list` — dump `/system/descriptors` for feature discovery.
- `slctl status` — fetch `/system/status` to inspect server health, version, and services.
- `slctl version` — print CLI build info and query `/system/version` on the server.
- `slctl gasbank summary --account <id>` — view balances, pending withdrawals, and recent gas bank activity.
- See `docs/gasbank-workflows.md` for a full ensure → deposit → scheduled/multi-sig withdraw walkthrough (CLI + HTTP) plus settlement retry/DLQ commands embraced by both Devpack and the dashboard.

### Docker

```bash
docker compose up --build
```

The compose file launches PostgreSQL, the appserver (port 8080), and the
dashboard (port 8081). Defaults include `API_TOKENS=dev-token` and a sample
`SECRET_ENCRYPTION_KEY` for local use. If `DATABASE_URL` is left empty (either
in the environment or `.env`) and the config DSN is cleared, the runtime falls
back to the in-memory stores; by default compose supplies a Postgres DSN so
everything is persisted. The compose stack waits for Postgres health
(`pg_isready`) before starting the appserver.
Compose will read a `.env` file automatically if present; copy `.env.example`
to `.env` when you want to override the defaults.
- Any endpoint also accepts a query token for convenience, e.g.
  `http://localhost:8080/system/status?token=dev-token`.

Once running:
- API: `http://localhost:8080` (use `Authorization: Bearer <jwt>`; obtain via `/auth/login` or wallet login)
- Dashboard: `http://localhost:8081` (configure API URL/token in settings)
- Public site: `http://localhost:8082` (marketing/docs entry)
- Login (JWT): `POST /auth/login` with configured `AUTH_USERS` and `AUTH_JWT_SECRET`; all endpoints also accept `?token=<API_TOKEN>`.
- **Production:** override `API_TOKENS`, `AUTH_USERS`, and `AUTH_JWT_SECRET` (the repo defaults are for local compose only).

## Configuration Notes

- `DATABASE_URL` (env) or `-dsn` (flag) control persistence. When omitted, the
  runtime keeps everything in memory.
- `auth.tokens` (config), `API_TOKENS`/`API_TOKEN` (env), or `-api-tokens` (flag)
  configure bearer tokens for HTTP authentication. All protected requests must
  present `Authorization: Bearer <token>`; `/healthz` and `/system/version` stay
  public. When no tokens are configured, protected endpoints return 401 and the
  server logs a warning. Always set tokens for any deployment.
- Oracle dispatcher settings honour the runtime config or `ORACLE_*` env vars:
  `ORACLE_TTL_SECONDS`, `ORACLE_MAX_ATTEMPTS`, `ORACLE_BACKOFF`, and
  `ORACLE_DLQ_ENABLED` control retry/backoff/expiry. `ORACLE_RUNNER_TOKENS`
  (or `runtime.oracle.runner_tokens`) require runner callbacks to include
  `X-Oracle-Runner-Token: <token>` alongside normal API authentication. When
  unset, callbacks only require API tokens. Set multiple runner tokens with
  `ORACLE_RUNNER_TOKENS=tok1,tok2` (`,` or `;` separators).
- Gas bank settlement requires `GASBANK_RESOLVER_URL` (+ optional
  `GASBANK_RESOLVER_KEY`). Tuning knobs include `GASBANK_POLL_INTERVAL`
  (duration string, default 15s) and `GASBANK_MAX_ATTEMPTS` (default 5) for
  retry/DLQ behaviour (or `runtime.gasbank.poll_interval` / `runtime.gasbank.max_attempts`
  in `configs/config.yaml`).
- `security.secret_encryption_key` (config) or `SECRET_ENCRYPTION_KEY` (env)
  provide the AES key for secret storage. A key is required when using
  persistent stores.
- `SECRET_ENCRYPTION_KEY` enables AES-GCM encryption for stored secrets (16/24/32
  byte raw, base64, or hex keys are supported). It is required when using
  PostgreSQL.
- `PRICEFEED_FETCH_URL` and `GASBANK_RESOLVER_URL` point
  to the external services responsible for price data and
  withdrawal settlement. Optional `*_KEY` environment variables attach bearer
  tokens when calling those endpoints.
- Data feed aggregation supports `median` (default), `mean`, `min`, and `max`.
  Set a global default via `runtime.datafeeds.aggregation` / `DATAFEEDS_AGGREGATION`,
  and override per feed by supplying `aggregation` in the data feed create/update
  payloads.
- Oracle data sources are configured per-feed via the HTTP API; no global
  resolver URL is required.
- `RANDOM_SIGNING_KEY` (base64 or hex encoded ed25519 private key) enables
  deterministic signatures for the randomness API. When omitted, a fresh key is
  generated on startup and returned with each response.
- The `runtime` block in `configs/config.yaml` mirrors the legacy environment
  variables for TEE mode selection, random signing keys, price feed fetchers,
  gas bank settlement resolvers, and the CRE HTTP runner toggle. Populate this
  section (or set the corresponding env vars) to drive the new builder-based
  application wiring. CLI flags continue to take precedence where applicable.
- `configs/config.yaml` and `configs/examples/appserver.json` provide
  overrideable samples for the refactored runtime (see `configs/README.md` for details).

## Project Layout

```
cmd/
  appserver/           - runtime entry point
apps/
  dashboard/           - React + Vite operator surface (only maintained front-end)
configs/               - sample configuration files
docs/                  - specification + documentation index
examples/              - runnable Devpack samples
internal/app/          - services, storage adapters, HTTP API
internal/config/       - configuration structs & helpers
internal/platform/     - database helpers and migrations
internal/version/      - build/version metadata
pkg/                   - shared utility packages (logger, errors, etc.)
sdk/devpack/           - TypeScript SDK consumed by function authors
sdk/go/devpack/        - Go helpers mirroring the Devpack action surface
sdk/rust/devpack/      - Rust helpers mirroring the Devpack action surface
sdk/python/devpack/    - Python helpers mirroring the Devpack action surface
scripts/               - automation helpers (see scripts/README.md)
```

## Documentation

All project documentation lives under `docs/`. Start with [`docs/README.md`](docs/README.md)
for navigation and context, then update the [`Neo Service Layer Specification`](docs/requirements.md)
whenever behaviour, APIs, or operations change. The retired LaTeX/PDF spec has been
fully removed—keep the Markdown specification as the single source of truth. Run
the [Service Layer Review Checklist](docs/review-checklist.md) before merging to
confirm the documentation, CLI, and dashboard remain in lockstep. Service
descriptors all advertise the same `platform` layer so every capability is
treated with equal priority.

### Tutorials
- Data Feeds: `docs/examples/datafeeds.md`
- DataLink: `docs/examples/datalink.md`
- JAM: `docs/examples/jam.md`

## Development

- Quick targets:
  - `make build` / `make test` (build outputs land in `./bin`)
  - `make run` brings up Postgres + appserver + dashboard via docker compose (detached) and prints port info.
  - `make run-local` runs the appserver binary directly; export `DATABASE_URL` to point at Postgres.
  - `make build-dashboard` builds the React UI (Node 20+, npm).
  - `make docker` builds the appserver + dashboard images.
  - `make docker-compose` (or `make docker-compose-run`) brings up Postgres + appserver + dashboard with sensible defaults.
- Run **all** tests: `go test ./...`
- Go modules are vendored for offline Docker builds; run `go mod vendor` after
  updating dependencies.
