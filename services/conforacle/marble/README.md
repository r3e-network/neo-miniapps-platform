# NeoOracle Marble Service

TEE-secured HTTP oracle proxy running inside MarbleRun enclave.

## Overview

The NeoOracle Marble service implements secure external data fetching:
1. User contracts request external data via Gateway
2. TEE fetches data from external URLs within secure enclave
3. Supports secret injection for authenticated API calls
4. Returns data with TEE attestation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                      │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐      ┌─────────────┐        │
│  │   Handler   │    │ URL Allow-  │      │  Secrets    │        │
│  │  (REST API) │───>│   list      │      │  Client     │        │
│  └─────────────┘    └──────┬──────┘      └──────┬──────┘        │
│         │                  │                    │               │
│         │                  │                    │               │
│  ┌──────▼──────────────────▼────────────────────▼──────┐        │
│  │                  HTTP Client                        │        │
│  │           (with optional secret auth)               │        │
│  └─────────────────────────┬───────────────────────────┘        │
└────────────────────────────┼────────────────────────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  External APIs  │
                    │ (api.coingecko.com│
                    │   etc.)         │
                    └─────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization, secrets provider |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `config.go` | URL allowlist configuration |
| `types.go` | Request/response types |

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    secretProvider secrets.Provider
    httpClient   *http.Client
    maxBodyBytes int64
    allowlist    URLAllowlist
}
```

### Secrets Provider

The oracle can inject user-owned secrets into outbound requests (e.g., API keys).
Secrets are fetched via the shared `infrastructure/secrets.Provider` interface and
enforced by per-secret access policies.

```go
type Provider interface {
    GetSecret(ctx context.Context, userID, name string) (string, error)
}
```

### URLAllowlist

Controls which external URLs can be fetched:

```go
type URLAllowlist struct {
    Prefixes []string
}

func (a URLAllowlist) Allows(url string) bool
```

## Security Features

### URL Allowlist

Only configured URL prefixes are allowed:

```go
allowlist := URLAllowlist{
    Prefixes: []string{
        "https://api.coingecko.com/",
        "https://api.binance.com/",
    },
}
```

### Secret Injection

Secrets can be automatically injected into request headers:

```json
{
    "url": "https://api.binance.com/api/v3/account",
    "secret_name": "binance_api_key",
    "secret_as_key": "X-MBX-APIKEY"
}
```

### Response Size Limit

Maximum response body size (default 2MB) to prevent memory exhaustion.

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/query` | POST | Fetch external data |

## Request/Response Types

### QueryInput

```go
type QueryInput struct {
    URL         string            `json:"url"`
    Method      string            `json:"method,omitempty"`        // default: GET
    Headers     map[string]string `json:"headers,omitempty"`
    SecretName  string            `json:"secret_name,omitempty"`   // optional: secret for auth
    SecretAsKey string            `json:"secret_as_key,omitempty"` // header key (default: Authorization)
    Body        string            `json:"body,omitempty"`          // body for POST/PUT
}
```

### QueryResponse

```go
type QueryResponse struct {
    StatusCode int               `json:"status_code"`
    Headers    map[string]string `json:"headers"`
    Body       string            `json:"body"`
}
```

## Configuration

```go
type Config struct {
    Marble            *marble.Marble
    SecretProvider    secrets.Provider
    MaxBodyBytes      int64        // optional: default 2MB
    URLAllowlist      URLAllowlist // optional: URL restrictions
    Timeout           time.Duration
}
```

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `ServiceID` | `neooracle` | Service identifier |
| `ServiceName` | `NeoOracle Service` | Display name |
| `Version` | `1.0.0` | Service version |
| `DefaultMaxBytes` | `2MB` | Default response limit |
| `DefaultTimeout` | `20s` | HTTP client timeout |

## Usage Examples

### Basic GET Request

```json
POST /query
{
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd"
}
```

### Authenticated Request with Secret

```json
POST /query
{
    "url": "https://api.private.com/data",
    "method": "GET",
    "secret_name": "private_api_key",
    "secret_as_key": "X-API-Key"
}
```

### POST Request with Body

```json
POST /query
{
    "url": "https://api.thegraph.com/subgraphs/name/neo-project/neo",
    "method": "POST",
    "headers": {
        "Content-Type": "application/json"
    },
    "body": "{\"query\": \"{ users { id } }\"}"
}
```

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `infrastructure/marble` | MarbleRun TEE utilities |
| `infrastructure/httputil` | HTTP response helpers |
| `infrastructure/service` | Base service framework |

### External Packages

| Package | Purpose |
|---------|---------|
| `github.com/gorilla/mux` | HTTP router |
| `github.com/google/uuid` | Request ID generation |

## Related Documentation

- [NeoOracle Service Overview](../README.md)
- [Smart Contract](../contract/README.md)
