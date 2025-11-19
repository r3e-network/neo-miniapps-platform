# Neo N3 Service Layer

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
  and gas-bank integrations
- Devpack runtime with declarative action queueing and a TypeScript SDK for
  authoring functions locally
- Secret vault with optional encryption and runtime resolution for function
  execution
- Cryptographically secure random number generation per account
- Modular service manager that wires the domain services together
- HTTP API located in `internal/app/httpapi`, exposing the new surface under
  `/accounts/...`

## Quick Start

```bash
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer

# In-memory mode (no external dependencies)
go run ./cmd/appserver

# Interact with a running instance via CLI (defaults to http://localhost:8080)
go run ./cmd/slctl --token <api-token> accounts list
```

To use PostgreSQL, supply a DSN via flag or environment variable. Migrations are
embedded and executed automatically when `-migrate` is left enabled.

```bash
go run ./cmd/appserver \
  -config configs/examples/appserver.json \
  -dsn "postgres://user:pass@localhost:5432/service_layer?sslmode=disable"
```

(You may also omit `-config` entirely and only pass `-dsn`.)

Examples for Devpack usage live under `examples/functions/devpack`.

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
- `slctl pricefeeds list|create|get|snapshots` — define asset pairs and monitor submissions.
- `slctl random generate --account <id> --length <n>` — request deterministic bytes.
- `slctl random list --account <id> [--limit n]` — fetch recent `/random/requests` history.
- `slctl services list` — dump `/system/descriptors` for feature discovery.

### Docker

```bash
cp .env.example .env   # optional, customise DSN / encryption key
docker compose up --build
```

The compose file launches PostgreSQL and the appserver. If `DATABASE_URL` is
left empty (either in the environment or `.env`) the runtime falls back to the
in-memory stores.

## Configuration Notes

- `DATABASE_URL` (env) or `-dsn` (flag) control persistence. When omitted, the
  runtime keeps everything in memory.
- `API_TOKENS` (env) or `-api-tokens` (flag) configure bearer tokens for HTTP
  authentication. All requests must present `Authorization: Bearer <token>`.
- `SECRET_ENCRYPTION_KEY` enables AES-GCM encryption for stored secrets (16/24/32
  byte raw, base64, or hex keys are supported). It is required when using
  PostgreSQL.
- `PRICEFEED_FETCH_URL` and `GASBANK_RESOLVER_URL` point
  to the external services responsible for price data and
  withdrawal settlement. Optional `*_KEY` environment variables attach bearer
  tokens when calling those endpoints.
- Oracle data sources are configured per-feed via the HTTP API; no global
  resolver URL is required.
- `RANDOM_SIGNING_KEY` (base64 or hex encoded ed25519 private key) enables
  deterministic signatures for the randomness API. When omitted, a fresh key is
  generated on startup and returned with each response.
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
scripts/               - automation helpers (see scripts/README.md)
```

## Documentation

All project documentation lives under `docs/`. Start with [`docs/README.md`](docs/README.md)
for navigation and context, then update the [`Neo Service Layer Specification`](docs/requirements.md)
whenever behaviour, APIs, or operations change. The retired LaTeX/PDF spec has been
fully removed—keep the Markdown specification as the single source of truth. Run
the [Service Layer Review Checklist](docs/review-checklist.md) before merging to
confirm the documentation, CLI, and dashboard remain in lockstep.

## Development

- Run **all** tests: `go test ./...`
