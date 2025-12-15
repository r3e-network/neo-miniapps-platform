# Marble SDK Wrapper

This package is a thin SDK that wraps MarbleRun primitives for services:
- Loads Coordinator-injected TLS certs/CA and builds an mTLS HTTP client for cross-marble traffic.
- Exposes injected secrets via `Marble.Secret/UseSecret`.
- Surfaces TEE identity (report, UUID, marble type) to services.

We keep this layer even though MarbleRun is used because it provides:
1. A stable Go API inside services (tests and simulation can stub it).
2. A single place to translate environment injection (`MARBLE_CERT`, `MARBLE_KEY`, `MARBLE_ROOT_CA`, `MARBLE_SECRETS`, `MARBLE_UUID`) into usable clients/configs.
3. Enforcement of the official cross-marble communication path (mTLS via `Marble.HTTPClient`), rather than ad-hoc HTTP clients.

## Components

### Marble (`marble.go`)

Core Marble type for TEE service configuration.

```go
m, err := marble.New(marble.Config{
    MarbleType: "neorand",
})
```

### Service (`service.go`)

Base service class that all TEE services embed.

```go
type MyService struct {
    *marble.Service
    // service-specific fields
}

func New(cfg Config) (*MyService, error) {
    base := marble.NewService(marble.ServiceConfig{
        ID:      "myservice",
        Name:    "My Service",
        Version: "1.0.0",
        Marble:  cfg.Marble,
        DB:      cfg.DB,
    })
    return &MyService{Service: base}, nil
}
```

### Worker (`worker.go`)

Background worker management for services.

### Config (`config.go`)

Configuration management for Marble services.

## Secret Management

Secrets are injected by MarbleRun Coordinator and accessed via the Marble instance:

```go
// Get secret (returns false if not found)
secret, ok := m.GetSecret("VRF_PRIVATE_KEY")
if !ok {
    return errors.New("VRF_PRIVATE_KEY not configured")
}
defer crypto.ZeroBytes(secret) // Always zero secrets after use
```

### Available Secrets

| Secret | Service | Description |
|--------|---------|-------------|
| `VRF_PRIVATE_KEY` | NeoRand | ECDSA P-256 private key |
| `NEOVAULT_MASTER_KEY` | NeoVault | HMAC signing key |
| `POOL_MASTER_KEY` | NeoAccounts | HD wallet master key |
| `NEOFEEDS_SIGNING_KEY` | NeoFeeds | Price signing key |
| `SECRETS_MASTER_KEY` | NeoStore | AES-256 encryption key |

## mTLS Communication

For secure inter-service communication within the MarbleRun mesh:

```go
// Get mTLS HTTP client
httpClient := m.HTTPClient()

// Make request to another marble
resp, err := httpClient.Get("https://neoaccounts:8085/info")
```

## External HTTP Calls

For outbound calls to non-mesh endpoints (Supabase, Neo RPC, third-party APIs), use:

```go
httpClient := m.ExternalHTTPClient()
resp, err := httpClient.Get("https://api.coingecko.com/api/v3/ping")
```

## Service Lifecycle

```go
// Start service
err := svc.Start(ctx)

// Stop service
err := svc.Stop()

// Get service info
id := svc.ID()
name := svc.Name()
version := svc.Version()

// Get HTTP router
router := svc.Router()

// Get database repository
db := svc.DB()
```

## Health Handler

Standard health endpoint handler:

```go
router.HandleFunc("/health", marble.HealthHandler(svc.Service)).Methods("GET")
```

Returns:
```json
{
    "status": "healthy",
    "service": "vrf",
    "version": "2.0.0",
    "enclave": true,
    "timestamp": "2025-12-08T00:00:00Z"
}
```

## Testing

For testing without actual MarbleRun:

```go
m, _ := marble.New(marble.Config{MarbleType: "test"})

// Set test secrets
m.SetTestSecret("MY_SECRET", []byte("test-value"))
```

```bash
go test ./internal/marble/... -v
```

Current test coverage: **43.8%**

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MARBLE_TYPE` | Service type identifier |
| `MARBLE_CERT` | TLS certificate (injected) |
| `MARBLE_KEY` | TLS private key (injected) |
| `MARBLE_ROOT_CA` | Root CA certificate (injected) |
| `MARBLE_SECRETS` | JSON-encoded secrets (injected) |
| `MARBLE_UUID` | Unique marble instance ID |
