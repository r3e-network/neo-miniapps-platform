# Neo N3 Service Layer

[![Build Status](https://github.com/R3E-Network/service_layer/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/R3E-Network/service_layer/actions/workflows/ci-cd.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/R3E-Network/service_layer)](https://goreportcard.com/report/github.com/R3E-Network/service_layer)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

The Service Layer provides a lightweight orchestration runtime for Neo N3. It
wraps account management, function execution, automation, and gas-bank utilities
behind a simple HTTP API. The runtime can run entirely in-memory for local
experimentation, or wire itself to PostgreSQL when a DSN is supplied.

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
```

To use PostgreSQL, supply a DSN via flag or environment variable. Migrations are
embedded and executed automatically when `-migrate` is left enabled.

```bash
go run ./cmd/appserver \
  -config configs/examples/appserver.json \
  -dsn "postgres://user:pass@localhost:5432/service_layer?sslmode=disable"
```

(You may also omit `-config` entirely and only pass `-dsn`.)

See [Runtime Quick Start](docs/runtime_quickstart.md) for additional startup
options and the current HTTP API surface. The [Function Devpack guide](docs/function_devpack.md)
covers authoring and orchestrating functions with the new helpers.

Check `examples/functions/devpack` for a TypeScript project that uses the SDK to
ensure gas accounts and submit oracle requests.

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
- `PRICEFEED_FETCH_URL`, `ORACLE_RESOLVER_URL`, and `GASBANK_RESOLVER_URL` point
  to the external services responsible for price data, oracle results, and
  withdrawal settlement. Optional `*_KEY` environment variables attach bearer
  tokens when calling those endpoints.
- `configs/config.yaml` and `configs/examples/appserver.json` provide
  overrideable samples for the refactored runtime.

## Project Layout

```
cmd/
  appserver/           - runtime entry point
configs/               - sample configuration files
internal/app/          - services, storage adapters, HTTP API
internal/config/       - configuration structs & helpers
internal/platform/     - database helpers and migrations
internal/version/      - build/version metadata
pkg/                   - shared utility packages (logger, errors, etc.)
```

## Documentation

- [Service Layer Guide](docs/service_layer_guide.md) – responsibilities,
  examples, and tests for every service.
- [API Reference](docs/api_reference.md) – exhaustive REST endpoint
  documentation with request/response samples.
- [Runtime Quick Start](docs/runtime_quickstart.md) – launch options and HTTP
  API walkthrough.
- [Deployment Playbook](docs/deployment_playbook.md) – configuration and
  operational guidance for dev/staging/production.
- [Architecture Overview](docs/architecture_overview.md) – high-level component
  map and design considerations.

## Development

- Run **all** tests: `go test ./...`
