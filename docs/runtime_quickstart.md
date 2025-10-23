# Runtime Quick Start

The refactored runtime focuses on a small, composable HTTP API that can run with
in-memory storage or PostgreSQL. This guide walks through the common startup
paths and the endpoints exposed by `cmd/appserver`.

## Prerequisites

- Go 1.22+
- PostgreSQL 14+ (optional)

No Redis or external TEE configuration is required for the new runtime.

## Launching the API Server

### In-Memory Mode

```bash
go run ./cmd/appserver
```

With no DSN configured, the server wires the in-memory storage adapter. Data is
kept for the lifetime of the process.

### Using PostgreSQL

```bash
go run ./cmd/appserver \
  -dsn "postgres://user:pass@localhost:5432/service_layer?sslmode=disable" \
  -migrate
```

Migrations are embedded and run automatically when `-migrate` (default `true`) is
left enabled. You can also pass the DSN via environment variable or config file:

```bash
DATABASE_URL=postgres://user:pass@localhost:5432/service_layer?sslmode=disable \
  go run ./cmd/appserver -config configs/examples/appserver.json
```

Copy `.env.example` to `.env` to keep environment variables alongside the
codebase. The Docker Compose workflow reads from `.env`, and the server will
automatically pick up `DATABASE_URL`, `API_TOKENS`, `SECRET_ENCRYPTION_KEY`,
and `LOG_LEVEL` when they are present.

When connecting production integrations, specify `PRICEFEED_FETCH_URL`, 
`ORACLE_RESOLVER_URL`, and `GASBANK_RESOLVER_URL` (plus optional `*_KEY` tokens) so
the refreshers and resolvers can reach external services.

See [`integration_endpoints.md`](integration_endpoints.md) for response schemas expected by these services.

### Flags

- `-addr` &mdash; overrides the listen address (default derives from config or
  falls back to `:8080`).
- `-dsn` &mdash; overrides the database DSN. When empty, the runtime stays
  in-memory.
- `-config` &mdash; optional path to JSON/YAML config file.
- `-migrate` &mdash; run migrations on startup (ignored in in-memory mode).

## HTTP API Surface

All endpoints live under `/` and operate on application accounts and their
associated services.

### Accounts

- `GET /accounts` &mdash; list accounts.
- `POST /accounts` &mdash; create a new account.
- `GET /accounts/{accountID}` &mdash; fetch account details.
- `DELETE /accounts/{accountID}` &mdash; remove an account.

### Functions

- `POST /accounts/{accountID}/functions` &mdash; create a function definition.
- `GET /accounts/{accountID}/functions` &mdash; list functions for an account.
- `POST /accounts/{accountID}/functions/{functionID}/execute` &mdash; run a
  function immediately.
- `GET /accounts/{accountID}/functions/{functionID}/executions` &mdash; list recent
  execution history for a function.
- `GET /accounts/{accountID}/functions/{functionID}/executions/{executionID}`
  &mdash; fetch details for a specific execution.
- `GET /accounts/{accountID}/functions/executions/{executionID}` &mdash; retrieve an
  execution by identifier for the account.

### Secrets

- `POST /accounts/{accountID}/secrets` &mdash; create a secret for the account.
- `GET /accounts/{accountID}/secrets` &mdash; list secret metadata.
- `GET /accounts/{accountID}/secrets/{name}` &mdash; retrieve a secret value.
- `PUT /accounts/{accountID}/secrets/{name}` &mdash; rotate a secret value.
- `DELETE /accounts/{accountID}/secrets/{name}` &mdash; remove a secret.

### Random

- `POST /accounts/{accountID}/random` &mdash; generate cryptographically secure
  random bytes (defaults to 32 bytes; accepts `length` in the payload up to 1024).

### Gas Bank

- `GET /accounts/{accountID}/gasbank` — list gas bank accounts for the owner.
- `POST /accounts/{accountID}/gasbank` — ensure an account exists (optionally set wallet address).
- `POST /accounts/{accountID}/gasbank/deposit` — record a deposit.
- `POST /accounts/{accountID}/gasbank/withdraw` — request a withdrawal.
- `GET /accounts/{accountID}/gasbank/transactions` — list transactions for a gas account.

### Observability

- `GET /metrics` — scrape Prometheus metrics (HTTP volumes, durations, and function execution stats). Requires a valid API token.
- `GET /healthz` — liveness/readiness probe (no authentication required).
- Structured service logs annotate resource identifiers (accounts, functions, jobs, gas accounts) and automation metrics track job success/failure counts plus durations.

### Triggers

- `POST /accounts/{accountID}/triggers` — register a trigger for a function.
- `GET /accounts/{accountID}/triggers` — list triggers for the account.
- *(More trigger management endpoints will appear as features land.)*

### Automation

- `GET /accounts/{accountID}/automation/jobs` — list automation jobs.
- `POST /accounts/{accountID}/automation/jobs` — create a job.
- `GET /accounts/{accountID}/automation/jobs/{jobID}` — fetch job details.
- `PATCH /accounts/{accountID}/automation/jobs/{jobID}` — update metadata or toggle enabled status.



### Oracle

- `POST /accounts/{accountID}/oracle/sources` — register a data source.
- `PATCH /accounts/{accountID}/oracle/sources/{sourceID}` — update metadata or toggle enabled state.
- `POST /accounts/{accountID}/oracle/requests` — create a request against a source.
- `GET /accounts/{accountID}/oracle/requests` — list requests for the account.
- `PATCH /accounts/{accountID}/oracle/requests/{requestID}` — update request status (running/succeeded/failed).

### Price Feeds

- `POST /accounts/{accountID}/pricefeeds` — create a price feed.
- `GET /accounts/{accountID}/pricefeeds/{feedID}` — fetch details.
- `PATCH /accounts/{accountID}/pricefeeds/{feedID}` — update feed metadata or toggle active state.
- `POST /accounts/{accountID}/pricefeeds/{feedID}/snapshots` — record a snapshot.
- `GET /accounts/{accountID}/pricefeeds/{feedID}/snapshots` — list snapshots.


Refer to the handler implementation in
`internal/app/httpapi/handler.go` for the current set of verbs and payloads.

When retrieving execution history (`GET /accounts/{accountID}/functions/{functionID}/executions`)
each record now includes an `actions` array whenever the function queued Devpack
operations. Each entry details the action `type`, final `status` (succeeded or
failed), and any `result` or `error` data returned by the underlying service.
Failed actions also mark the entire execution as failed so they’re easy to spot
in monitoring.

> Encryption: if `SECRET_ENCRYPTION_KEY` is set in the environment, secrets are
> encrypted at rest using AES-GCM. Supported formats are raw 16/24/32 byte keys,
> base64 (44 chars) or 64-char hex strings.
