# Service Layer API Documentation

## Overview

The Neo Service Layer provides a comprehensive set of TEE-protected services for the Neo N3 blockchain. All services are protected by MarbleRun/EGo TEE and coordinated through MarbleRun.

## Base URL

```
Production:  https://api.service-layer.neo.org
Staging:     https://staging-api.service-layer.neo.org
Development: http://localhost:8080
```

Gateway API prefix:

```
/api/v1
```

The examples below use the Gateway and include the `/api/v1` prefix unless noted
(e.g., `/health`, `/metrics`, `/attestation`).

## Authentication

The Gateway supports three authentication mechanisms:

- **HTTP-only session cookie** (recommended for browsers): enabled when `OAUTH_COOKIE_MODE=true`. The gateway sets `sl_auth_token` and the browser sends it automatically with `credentials: "include"`.
- **Bearer JWT** (good for CLI/server integrations): `Authorization: Bearer <jwt_token>`.
- **API keys** (service-to-service / automation): `X-API-Key: <api_key>`.

### Headers

```
Authorization: Bearer <jwt_token>
X-API-Key: <api_key>
Content-Type: application/json
```

**Important:** `X-User-ID`, `X-User-Role`, `X-Service-ID`, and `X-Service-Token` are internal identity headers. Public clients should not send them. The gateway strips these headers from inbound requests and forwards only the derived identity to internal services.

### Wallet Auth (Neo N3)

1. Request a nonce + message to sign:

```http
POST /api/v1/auth/nonce
```

```json
{
    "address": "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
}
```

2. Sign the returned `message` using your wallet, then login (or register):

```http
POST /api/v1/auth/login
POST /api/v1/auth/register
```

```json
{
    "address": "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
    "publicKey": "hex_or_base64_public_key",
    "signature": "hex_or_base64_signature",
    "message": "original_message_from_nonce_endpoint",
    "nonce": "nonce_from_nonce_endpoint"
}
```

On success, the gateway returns a JWT and (when `OAUTH_COOKIE_MODE=true`) also sets an HTTP-only cookie for browser-based sessions.

## Services

### 1. Gateway Service

The API Gateway handles authentication, rate limiting, and request routing.

#### Health Check

```http
GET /health
```

**Response:**

```json
{
    "status": "healthy",
    "service": "gateway",
    "timestamp": "2025-12-10T10:00:00Z",
    "version": "1.0.0",
    "enclave": true
}
```

#### Metrics

```http
GET /metrics
```

Returns Prometheus metrics for monitoring.

Production note: avoid exposing `/metrics` publicly. Scrape it from inside the cluster (or protect it behind an internal-only ingress/service).

#### Secrets Management

The gateway exposes a user secrets API backed by Supabase. Secrets are encrypted with `SECRETS_MASTER_KEY` and protected by per-secret allowed-services policies.

```http
GET    /api/v1/secrets
POST   /api/v1/secrets
GET    /api/v1/secrets/{name}
DELETE /api/v1/secrets/{name}
GET    /api/v1/secrets/{name}/permissions
PUT    /api/v1/secrets/{name}/permissions
GET    /api/v1/secrets/audit
GET    /api/v1/secrets/{name}/audit
```

---

### 2. NeoOracle Service

Provides external data fetching inside a MarbleRun enclave.

#### Query External Data

```http
POST /api/v1/neooracle/query
```

**Request:**

```json
{
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
    "method": "GET",
    "headers": {
        "Accept": "application/json"
    }
}
```

**Response:**

```json
{
    "status_code": 200,
    "headers": {
        "Content-Type": "application/json; charset=utf-8"
    },
    "body": "{\"neo\":{\"usd\":15.42}}"
}
```

**Notes:**

- Backward-compatible alias: `POST /api/v1/neooracle/fetch` (same handler).
- Legacy gateway alias: `POST /api/v1/oracle/query`.
- In production/SGX (strict identity mode), only `https://` URLs are allowed.
- Access to external URLs is restricted by `ORACLE_HTTP_ALLOWLIST`.
- Optional secret injection fields: `secret_name` and `secret_as_key`.

---

### 3. NeoFeeds Service (Data Feeds)

Market data aggregation + signing inside the enclave.

#### Get Price

```http
GET /api/v1/neofeeds/price/{pair}
```

Example:

```http
GET /api/v1/neofeeds/price/BTC-USD
```

Notes:

- Canonical symbols use `BASE-QUOTE` (e.g. `BTC-USD`).
- Legacy `BASE/QUOTE` requests are accepted (URL-encode the slash, e.g. `BTC%2FUSD`) but are normalized to `BASE-QUOTE` in responses.

**Response:**

```json
{
    "feed_id": "BTC-USD",
    "pair": "BTC-USD",
    "price": 4500000000000,
    "decimals": 8,
    "timestamp": "2025-12-10T10:00:00Z",
    "sources": ["chainlink", "binance"],
    "signature": "base64...",
    "public_key": "base64..."
}
```

#### List Prices

```http
GET /api/v1/neofeeds/prices
```

#### List Feeds / Sources / Config

```http
GET /api/v1/neofeeds/feeds
GET /api/v1/neofeeds/sources
GET /api/v1/neofeeds/config
```

**Notes:**

- Legacy gateway alias: `GET /api/v1/datafeeds/*` (maps to `neofeeds`).

---

### 4. NeoAccounts (Account Pool) Service (Internal)

Manages a pool of funded accounts for service operations. This service is
intended for **internal service-to-service usage (mesh-only)** and is not
proxied by the public Gateway by default.

#### Request Accounts

```http
POST /request
```

**Request:**

```json
{
    "service_id": "neoflow",
    "count": 5,
    "purpose": "automation_job"
}
```

**Response:**

```json
{
    "accounts": [
        {
            "id": "acc_001",
            "address": "NAccountAddr001",
            "balances": {
                "GAS": { "amount": 1000000 }
            },
            "locked_by": "neoflow"
        }
    ],
    "lock_id": "lock_123456"
}
```

#### Release Accounts

```http
POST /release
```

**Request:**

```json
{
    "service_id": "neoflow",
    "account_ids": ["acc_001", "acc_002"]
}
```

**Response:**

```json
{
    "released_count": 2
}
```

#### Update Account Balance

```http
POST /balance
```

**Request:**

```json
{
    "service_id": "neoflow",
    "account_id": "acc_001",
    "token": "GAS",
    "delta": -50000,
    "absolute": null
}
```

**Response:**

```json
{
    "account_id": "acc_001",
    "old_balance": 1000000,
    "new_balance": 950000,
    "tx_count": 42
}
```

---

### 5. NeoFlow Service

Trigger-based automation with TEE protection.

#### Create Trigger

```http
POST /api/v1/neoflow/triggers
```

**Request:**

```json
{
    "name": "daily_price_update",
    "trigger_type": "cron",
    "schedule": "0 0 * * *",
    "action": {
        "type": "webhook",
        "url": "https://example.com/webhook",
        "method": "POST",
        "body": { "event": "daily_price_update" }
    }
}
```

**Response:**

```json
{
    "id": "trigger_123456",
    "name": "daily_price_update",
    "trigger_type": "cron",
    "schedule": "0 0 * * *",
    "enabled": true,
    "created_at": "2025-12-10T10:00:00Z"
}
```

#### List Triggers

```http
GET /api/v1/neoflow/triggers
```

**Response:**

```json
[
    {
        "id": "trigger_123456",
        "name": "daily_price_update",
        "trigger_type": "cron",
        "schedule": "0 0 * * *",
        "enabled": true,
        "created_at": "2025-12-10T10:00:00Z"
    }
]
```

#### List Trigger Executions

```http
GET /api/v1/neoflow/triggers/{id}/executions
```

**Response:**

```json
[]
```

---

### 6. NeoCompute Service

Secure script execution within TEE.

#### Execute Script

```http
POST /api/v1/neocompute/execute
```

**Request:**

```json
{
    "script": "function main() { return { ok: true } }",
    "entry_point": "main",
    "input": { "value": 1 },
    "secret_refs": ["my_api_key"],
    "timeout": 30
}
```

`secret_refs` entries are secret names owned by the user (managed via `POST /api/v1/secrets`).
The user must allow the `neocompute` service to read a secret via `PUT /api/v1/secrets/{name}/permissions`.

**Response:**

```json
{
    "job_id": "job_123456",
    "status": "completed",
    "output": { "ok": true },
    "logs": [],
    "gas_used": 1000,
    "started_at": "2025-12-10T10:00:00Z",
    "duration": "100ms"
}
```

**Randomness (recommended):**

Use `crypto.randomBytes(n)` inside the script to generate secure random bytes (hex string):

```json
{
  "script": "function main() { return { random_hex: crypto.randomBytes(32) } }",
  "entry_point": "main"
}
```

#### Get Job

```http
GET /api/v1/neocompute/jobs/{id}
```

#### List Jobs

```http
GET /api/v1/neocompute/jobs
```

---

## Error Responses

All services return consistent error responses:

```json
{
    "code": "SVC_5006",
    "message": "Rate limit exceeded",
    "details": {
        "limit": 100,
        "window": "1m0s"
    },
    "trace_id": "..."
}
```

### Common Error Codes

| Code            | HTTP Status | Description                             |
| --------------- | ----------- | --------------------------------------- |
| `HTTP_<status>` | varies      | Generic HTTP errors written by handlers |
| `AUTH_1001`     | 401         | Unauthorized                            |
| `AUTH_1002`     | 401         | Invalid token                           |
| `AUTH_1003`     | 401         | Token expired                           |
| `SVC_5006`      | 429         | Rate limit exceeded                     |

---

## Rate Limiting

When enabled (`RATE_LIMIT_ENABLED=true`), rate limits are enforced per user ID
(authenticated) or client IP (unauthenticated). The gateway returns `429` with a
`Retry-After` header.

Configure with:

- `RATE_LIMIT_REQUESTS` (budget, default 100)
- `RATE_LIMIT_WINDOW` (duration, default `1m`)
- `RATE_LIMIT_BURST` (optional burst budget; defaults to `RATE_LIMIT_REQUESTS`)

---

## Attestation

MarbleRun establishes enclave identity and service-to-service trust boundaries
via a signed manifest and mTLS between marbles.

To validate that the gateway is running inside an enclave (or in simulation),
use:

```http
GET /attestation
```

For SGX hardware deployments, ensure:

1. Enclave images are signed with stable keys and `SignerID`s match `manifests/manifest.json`.
2. Services communicate over MarbleRun-provisioned mTLS (verified chains).
3. Coordinator state is healthy (`marblerun status`).

---

## SDK Support

This repository includes a minimal TypeScript client used by the Vercel UI:
`frontend/src/api/client.ts`.

Example (TypeScript, API key):

```typescript
const baseUrl = "https://api.service-layer.neo.org/api/v1";
const apiKey = "your_api_key";

const resp = await fetch(`${baseUrl}/neocompute/execute`, {
    method: "POST",
    headers: {
        "Content-Type": "application/json",
        "X-API-Key": apiKey,
    },
    body: JSON.stringify({
        script: "function main() { return { random_hex: crypto.randomBytes(32) } }",
        entry_point: "main",
    }),
});

console.log(await resp.json());
```

---

## Webhooks

NeoFlow supports executing **webhook actions** as part of triggers.

**Security note:** In production/SGX (strict identity mode), webhook targets must be `https://` URLs. Internal service-to-service webhooks should use mesh DNS names and will be dispatched over HTTPS+mTLS when MarbleRun credentials are available. External webhooks are blocked from targeting private/loopback/link-local networks by default; override with `NEOFLOW_WEBHOOK_ALLOW_PRIVATE_NETWORKS=true` only if required.

Webhook URLs are configured inside a NeoFlow trigger’s `action` payload.

---

## Support

For API support and questions:

- **Documentation**: https://docs.service-layer.neo.org
- **GitHub Issues**: https://github.com/R3E-Network/service_layer/issues
- **Discord**: https://discord.gg/neo
- **Email**: support@r3e-network.org
