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
    MarbleType: "neocompute",
})
```

### Service (`service.go`)

Low-level service base (identity + router + dependency holders).

Most services should embed `infrastructure/service.BaseService`, which wraps
`*marble.Service` and adds:
- Lifecycle hooks (`Start/Stop`) with safe shutdown handling
- Background worker registration helpers
- Standard endpoints (`/health`, `/ready`, `/info`)

```go
base := commonservice.NewBase(&commonservice.BaseConfig{
    ID:      "myservice",
    Name:    "My Service",
    Version: "1.0.0",
    Marble:  cfg.Marble,
    DB:      cfg.DB,
})

base.RegisterStandardRoutes()
```

## Secret Management

Secrets are injected by MarbleRun Coordinator and accessed via the Marble instance:

```go
// Get secret (returns false if not found)
secret, ok := m.Secret("COMPUTE_MASTER_KEY")
if !ok {
    return errors.New("COMPUTE_MASTER_KEY not configured")
}
defer crypto.ZeroBytes(secret) // Always zero secrets after use
```

### Available Secrets

| Secret | Service | Description |
|--------|---------|-------------|
| `NEOFEEDS_SIGNING_KEY` | Datafeed | ECDSA private key for signing prices |
| `COMPUTE_MASTER_KEY` | Confidential Compute | Master key for encryption/signing (>= 32 bytes) |
| `POOL_MASTER_KEY` | AccountPool | HD derivation master key (>= 32 bytes) |
| `POOL_MASTER_KEY_HASH` | AccountPool | SHA-256 hash of derived master pubkey (32 bytes; required in enclave mode) |
| `POOL_MASTER_ATTESTATION_HASH` | AccountPool | Optional attestation/bundle hash (for on-chain anchoring) |
| `GLOBALSIGNER_MASTER_SEED` | GlobalSigner | 32-byte master seed for deterministic key derivation |
| `SECRETS_MASTER_KEY` | Gateway | AES-256 encryption key for user secrets (32 bytes) |
| `OAUTH_TOKENS_MASTER_KEY` | Gateway | OAuth token at-rest encryption key (32 bytes) |
| `JWT_SECRET` | Gateway | JWT signing key (>= 32 bytes recommended) |

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
// Service lifecycle and workers are provided by `infrastructure/service.BaseService`.

// Get service info
id := svc.ID()
name := svc.Name()
version := svc.Version()

// Get HTTP router
router := svc.Router()

// Get database repository
db := svc.DB()
```

## Testing

For testing without actual MarbleRun:

```go
m, _ := marble.New(marble.Config{MarbleType: "test"})

// Set test secrets
m.SetTestSecret("MY_SECRET", []byte("test-value"))
```

```bash
go test ./infrastructure/marble/... -v
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
