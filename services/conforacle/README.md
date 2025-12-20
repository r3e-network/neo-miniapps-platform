# NeoOracle Service

HTTP oracle proxy service for the Neo Service Layer.

## Overview

The NeoOracle service provides a secure HTTP proxy for fetching external data from within the MarbleRun TEE. It enforces an outbound URL allowlist and can inject user-owned secrets into outbound requests (for authenticated APIs).

This service is intended to be reached via the gateway (Supabase Edge) rather than directly.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ User Contract│     │ Oracle       │     │ External API │
│              │     │ Service (TEE)│     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Request Data       │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Fetch Data         │
       │                    │───────────────────>│
       │                    │                    │
       │                    │ Response           │
       │                    │<───────────────────│
       │                    │                    │
       │ Response (mTLS)    │                    │
       │<───────────────────│                    │
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/query` | POST | Fetch external data (primary) |
| `/fetch` | POST | Alias for `/query` (backward compatible) |

## Request/Response Types

### Query (Fetch Data)

```json
POST /query
{
    "url": "https://api.binance.com/api/v3/account",
    "headers": {
        "Accept": "application/json"
    },
    "method": "GET",
    "secret_name": "binance_api_key",
    "secret_as_key": "X-MBX-APIKEY"
}
```

### Query Response

```json
{
    "status_code": 200,
    "headers": {
        "Content-Type": "application/json"
    },
    "body": "{\"any\":\"string\"}"
}
```

## Supported Features

| Feature | Description |
|---------|-------------|
| HTTP Methods | GET/POST/PUT/etc via `method` |
| URL allowlist | Restrict outbound destinations (required in strict identity / SGX mode) |
| Secret injection | Inject a user secret into a header (`secret_name`, `secret_as_key`) |
| Response cap | Enforced max body size (default 2MB) |

## Security

- All outbound requests originate from within the MarbleRun TEE (attested identity via mTLS)
- Strict identity mode enforces HTTPS-only outbound URLs
- URL allowlist support via `ORACLE_HTTP_ALLOWLIST`

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ORACLE_HTTP_ALLOWLIST` | Comma-separated URL prefixes allowed for outbound fetches |
| `ORACLE_TIMEOUT` | Outbound request timeout (Go duration, e.g. `20s`) |
| `ORACLE_MAX_SIZE` | Max upstream response body size (bytes, or `KiB`/`MiB`/`GiB` suffix) |

## Testing

```bash
go test ./services/conforacle/... -v -cover
```

Current test coverage: **58.6%**

## Version

- Service ID: `neooracle`
- Version: `1.0.0`
