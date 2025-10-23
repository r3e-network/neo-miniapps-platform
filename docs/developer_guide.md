# Service Layer Developer Guide

This guide walks through the refactored Service Layer runtime that ships under
`cmd/appserver`. The new architecture exposes a small HTTP surface rooted at
`/accounts/{accountID}/...`, backs services with in-memory storage by default,
and optionally wires PostgreSQL when a DSN is supplied.

## 1. Getting Started

```bash
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer

# In-memory mode
go run ./cmd/appserver

# With PostgreSQL (+ migrations)
DATABASE_URL=postgres://user:pass@localhost:5432/service_layer?sslmode=disable \
  go run ./cmd/appserver -migrate
```

Copy `.env.example` to `.env` if you prefer to manage environment variables
within the repository. Docker Compose automatically consumes the file.

## 2. Core Concepts

| Concept          | Description                                                                 |
|------------------|-----------------------------------------------------------------------------|
| Account          | Tenant boundary for secrets, functions, automation, oracle requests, etc.  |
| Function         | JavaScript snippet executed via the embedded Goja runtime.                  |
| Automation       | Cron-style jobs that invoke functions on a schedule.                        |
| Gas Bank         | Simple ledger tracking deposits/withdrawals for contract execution funds.   |
| Oracle           | Account-scoped data sources and one-off requests with dispatcher support.   |
| Price Feed       | Reference price definitions with snapshot storage.                          |

## 3. API Walkthrough

All requests are plain JSON. Authentication is not enforced yet; the focus is on
functionality parity with the migration plan.

### 3.1 Accounts

```bash
# Create
curl -sS -X POST http://localhost:8080/accounts \
  -d '{"owner":"alice"}' | jq

# List
curl -sS http://localhost:8080/accounts | jq

# Delete
curl -sS -X DELETE http://localhost:8080/accounts/{accountID}
```

### 3.2 Secrets

```bash
# Store a secret
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/secrets \
  -d '{"name":"apiKey","value":"top-secret"}' | jq

# List metadata
curl -sS http://localhost:8080/accounts/{accountID}/secrets | jq
```

### 3.3 Functions & Execution

```bash
# Create a function referencing the secret
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/functions \
  -d '{
        "name":"hello",
        "source":"(params, secrets) => ({secret: secrets.apiKey})",
        "secrets":["apiKey"]
      }' | jq

# Execute immediately
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/functions/{functionID}/execute \
  -d '{"input":"hi"}' | jq

# Inspect execution history
curl -sS \
  http://localhost:8080/accounts/{accountID}/functions/{functionID}/executions | jq
```

### 3.4 Gas Bank

```bash
# Ensure / create a gas account (optionally with wallet binding)
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/gasbank \
  -d '{"wallet_address":"WALLET-1"}' | jq

# Deposit gas
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/gasbank/deposit \
  -d '{"gas_account_id":"{gasAccountID}","amount":5.0,"tx_id":"tx1"}' | jq

# Request withdrawal
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/gasbank/withdraw \
  -d '{"gas_account_id":"{gasAccountID}","amount":1.5,"to_address":"ADDR"}' | jq
```

### 3.5 Automation

```bash
# Create a cron job that triggers a function every minute
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/automation/jobs \
  -d '{
        "function_id":"{functionID}",
        "name":"every-minute",
        "schedule":"@every 1m"
      }' | jq

# Disable the job
curl -sS -X PATCH \
  http://localhost:8080/accounts/{accountID}/automation/jobs/{jobID} \
  -d '{"enabled":false}' | jq
```

### 3.6 Oracle

```bash
# Register a data source
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/oracle/sources \
  -d '{"name":"prices","url":"https://api.example.com"}' | jq

# Issue a request
curl -sS -X POST \
  http://localhost:8080/accounts/{accountID}/oracle/requests \
  -d '{"data_source_id":"{sourceID}","payload":"{}"}' | jq
```

## 4. Observability

- `/metrics` exposes Prometheus counters/histograms for HTTP traffic and function
  runtimes.
- Structured service logs include identifiers such as `account_id`,
  `function_id`, `job_id`, and `gas_account_id`, simplifying trace correlation.

## 5. Configuration Checklist

| Control                | Source                    | Notes                                         |
|------------------------|---------------------------|-----------------------------------------------|
| `DATABASE_URL`         | env / `-dsn` flag         | Empty string keeps storage in-memory.         |
| `SECRET_ENCRYPTION_KEY`| env                       | Enables AES-GCM for secrets.                  |
| Logging options        | env (`LOG_LEVEL`) or YAML | Defaults to text output at info level.        |
| Metrics toggle         | `metrics.enabled`         | `/metrics` served from the main HTTP mux.     |

Sample configuration lives in `configs/config.yaml`. Adjust as needed and supply
via `-config` (JSON or YAML).

## 6. Next Steps

Refer to [`integration_endpoints.md`](integration_endpoints.md) for the expected HTTP payloads used by the price feed, oracle, and gas bank integrations.

- Explore `docs/runtime_quickstart.md` for the full endpoint matrix.
- Review `docs/automation_integration.md` for end-to-end automation workflows.
