# Oracle Service

HTTP oracle proxy service for the Neo Service Layer.

## Overview

The Oracle service provides a secure HTTP proxy for smart contracts to fetch external data. Requests are processed within the TEE enclave, ensuring data integrity and authenticity.

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
       │ Signed Response    │                    │
       │<───────────────────│                    │
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/fetch` | POST | Fetch external data |
| `/request/{id}` | GET | Get request status |

## Request/Response Types

### Fetch Data

```json
POST /fetch
{
    "url": "https://api.example.com/data",
    "method": "GET",
    "headers": {
        "Accept": "application/json"
    },
    "json_path": "$.data.value"
}
```

### Fetch Response

```json
{
    "request_id": "uuid",
    "url": "https://api.example.com/data",
    "result": "extracted-value",
    "timestamp": 1733616000,
    "signature": "0x...",
    "public_key": "0x..."
}
```

## Supported Features

| Feature | Description |
|---------|-------------|
| HTTP Methods | GET, POST |
| JSON Path | Extract specific values from JSON |
| Headers | Custom request headers |
| Timeout | Configurable request timeout |

## Security

- All requests made from within TEE enclave
- Responses signed with TEE key
- URL whitelist support
- Rate limiting per user

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ORACLE_TIMEOUT` | Request timeout (default: 30s) |
| `ORACLE_MAX_SIZE` | Max response size (default: 1MB) |

## Testing

```bash
go test ./services/oracle/... -v -cover
```

Current test coverage: **58.6%**

## Version

- Service ID: `oracle`
- Version: `1.0.0`
