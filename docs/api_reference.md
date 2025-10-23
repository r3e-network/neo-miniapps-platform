# HTTP API Reference

The Neo N3 service layer exposes a REST API behind `/accounts/...`, guarded by a
static bearer token (see `API_TOKENS` or `-api-tokens`). All requests must send:

- `Authorization: Bearer <token>`
- `Content-Type: application/json` for POST/PATCH bodies

Errors use the form:

```json
{"error": "human readable message"}
```

The tables below summarise every endpoint implemented in
`internal/app/httpapi/handler.go`. Examples assume a base URL of
`https://service-layer.example.com`.

## System Endpoints

| Method | Path        | Description                |
|--------|-------------|----------------------------|
| GET    | `/healthz`  | Returns `{"status":"ok"}`   |
| GET    | `/metrics`  | Prometheus metrics scrape   |

No authentication is required for these endpoints.

## Accounts

| Method | Path        | Description                          |
|--------|-------------|--------------------------------------|
| POST   | `/accounts` | Create account `{owner, metadata}`   |
| GET    | `/accounts` | List all accounts                    |
| GET    | `/accounts/{accountID}` | Fetch an account         |
| DELETE | `/accounts/{accountID}` | Delete an account        |

### Create Account

```http
POST /accounts
Authorization: Bearer …
Content-Type: application/json

{
  "owner": "alice",
  "metadata": {"tier": "pro"}
}
```

Success returns `201 Created` with the new account record.

## Functions

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/functions` | Create a function definition |
| GET | `/accounts/{id}/functions` | List functions on the account |
| POST | `/accounts/{id}/functions/{functionID}/execute` | Execute a function immediately |
| GET | `/accounts/{id}/functions/executions/{executionID}` | Retrieve an execution by ID |
| GET | `/accounts/{id}/functions/{functionID}/executions` | List executions (optional `?limit=`) |
| GET | `/accounts/{id}/functions/{functionID}/executions/{executionID}` | Fetch a scoped execution |

The request body for creation mirrors the `function.Definition` fields:
`{name, description, source, secrets}`.

## Automation

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/automation/jobs` | Create a job |
| GET | `/accounts/{id}/automation/jobs` | List jobs |
| GET | `/accounts/{id}/automation/jobs/{jobID}` | Fetch job |
| PATCH | `/accounts/{id}/automation/jobs/{jobID}` | Update mutable fields (`name`, `schedule`, `description`, `enabled`, `next_run`) |

## Triggers

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/triggers` | Register trigger `{function_id,type,rule,config}` |
| GET | `/accounts/{id}/triggers` | List triggers |

## Gas Bank (requires service configured)

| Method | Path | Description |
| --- | --- | --- |
| GET | `/accounts/{id}/gasbank` | List gas accounts |
| POST | `/accounts/{id}/gasbank` | Ensure account `{wallet_address}` |
| GET | `/accounts/{id}/gasbank?gas_account_id=` | Fetch specific gas account |
| POST | `/accounts/{id}/gasbank/deposit` | Deposit `{gas_account_id, amount, tx_id, from_address, to_address}` |
| POST | `/accounts/{id}/gasbank/withdraw` | Withdraw `{gas_account_id, amount, to_address}` |
| GET | `/accounts/{id}/gasbank/transactions?gas_account_id=` | List transactions |

Responses include the updated `Account` and `Transaction` records.

## Price Feeds (requires service configured)

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/pricefeeds` | Create feed `{base_asset, quote_asset, update_interval, heartbeat_interval, deviation_percent}` |
| GET | `/accounts/{id}/pricefeeds` | List feeds |
| GET | `/accounts/{id}/pricefeeds/{feedID}` | Fetch feed |
| PATCH | `/accounts/{id}/pricefeeds/{feedID}` | Update intervals/deviation or toggle `active` |
| GET | `/accounts/{id}/pricefeeds/{feedID}/snapshots` | List snapshots |
| POST | `/accounts/{id}/pricefeeds/{feedID}/snapshots` | Record snapshot `{price, source, collected_at}` |

`collected_at` accepts RFC3339 timestamps.

## Oracle (requires service configured)

### Data Sources

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/oracle/sources` | Create data source |
| GET | `/accounts/{id}/oracle/sources` | List sources |
| GET | `/accounts/{id}/oracle/sources/{sourceID}` | Fetch source |
| PATCH | `/accounts/{id}/oracle/sources/{sourceID}` | Update mutable fields or enable/disable |

### Requests

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/oracle/requests` | Queue request `{data_source_id, payload}` |
| GET | `/accounts/{id}/oracle/requests` | List requests |
| GET | `/accounts/{id}/oracle/requests/{requestID}` | Fetch request |
| PATCH | `/accounts/{id}/oracle/requests/{requestID}` | Update status (`running`, `succeeded`, `failed`) |

Status transitions enforce required fields (`result` for success, optional
`error` for failure).

## Secrets (requires service configured)

| Method | Path | Description |
| --- | --- | --- |
| GET | `/accounts/{id}/secrets` | List metadata |
| POST | `/accounts/{id}/secrets` | Create `{name,value}` |
| GET | `/accounts/{id}/secrets/{name}` | Retrieve secret (plaintext) |
| PUT | `/accounts/{id}/secrets/{name}` | Update value |
| DELETE | `/accounts/{id}/secrets/{name}` | Delete secret |

Secret lookups return `id`, `version`, timestamps, and decrypted `value`.

## Random Service

| Method | Path | Description |
| --- | --- | --- |
| POST | `/accounts/{id}/random` | Generate randomness `{length}` (1–1024 bytes) |

Response:

```json
{
  "value": "base64-encoded-bytes",
  "created_at": "2025-01-01T12:00:00Z"
}
```

## Authentication

Set the bearer token via environment variable or flag:

```bash
# single token
API_TOKENS="supersecret" go run ./cmd/appserver

# multiple tokens (comma separated)
go run ./cmd/appserver -api-tokens token1,token2
```

Clients must send the token verbatim. Missing or invalid credentials yield
`401 Unauthorized`.

## Pagination & Filtering

Only function executions support pagination today via the optional `limit`
query parameter. Additional filtering can be added by extending the handler,
but is currently out of scope.

## Content Types

- Requests: `application/json`
- Responses: `application/json`

Binary payloads (e.g., oracle request bodies) should be base64 or JSON encoded
by the caller.

## Versioning

Endpoints are stable within the current major version. Backwards-incompatible
changes will be announced in release notes.

