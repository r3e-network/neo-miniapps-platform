# NeoCompute Marble Service

TEE-secured confidential computing service running inside MarbleRun enclave.

## Overview

The NeoCompute Marble service implements secure JavaScript execution:
1. Users submit JavaScript code with optional encrypted inputs
2. TEE executes code within secure enclave using goja runtime
3. Secrets can be injected from user's NeoStore vault
4. Results are signed for verifiable TEE origin

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                      │
│                                                                 │
│    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│    │   Handler   │    │  goja JS    │    │  Secrets    │        │
│    │  (REST API) │───>│  Runtime    │<──>│  (Injected) │        │
│    └─────────────┘    └──────┬──────┘    └─────────────┘        │
│           │                  │                                  │
│    ┌──────▼──────┐    ┌──────▼──────┐    ┌─────────────┐        │
│    │ Job Manager │    │  Crypto     │    │   Output    │        │
│    │  (TTL/GC)   │    │  Utilities  │    │  Signing    │        │
│    └─────────────┘    └─────────────┘    └─────────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization, job management |
| `lifecycle.go` | Service lifecycle (Start/Stop) |
| `core.go` | Script execution, goja runtime |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `types.go` | Request/response types |

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    masterKey       []byte
    signingKey      []byte        // Derived key for HMAC signing
    jobs            sync.Map      // map[jobID]jobEntry
    resultTTL       time.Duration
    cleanupInterval time.Duration
}
```

### goja JavaScript Runtime

Secure JavaScript execution with:
- `input` - User-provided input data
- `secrets` - Injected secrets from NeoStore
- `console.log` - Debug logging (with limits)
- `crypto.sha256()` - Hash function
- `crypto.randomBytes()` - Secure random generation

## Security Features

### Resource Limits

| Constant | Value | Description |
|----------|-------|-------------|
| `MaxScriptSize` | 100KB | Max script size |
| `MaxInputSize` | 1MB | Max input data |
| `MaxOutputSize` | 1MB | Max output data |
| `MaxSecretRefs` | 10 | Max secrets per execution |
| `MaxLogEntries` | 100 | Max console.log calls |
| `MaxLogEntrySize` | 4KB | Max log entry size |
| `MaxConcurrentJobs` | 5 | Max parallel jobs per user |
| `DefaultTimeout` | 30s | Execution timeout |

### Output Protection

Results include cryptographic attestation:

```go
type ExecuteResponse struct {
    // ... basic fields ...
    EncryptedOutput string `json:"encrypted_output,omitempty"` // AES-GCM encrypted
    OutputHash      string `json:"output_hash,omitempty"`      // SHA256 of output
    Signature       string `json:"signature,omitempty"`        // HMAC-SHA256 signature
}
```

### Job Isolation

- Jobs stored per-user with TTL expiration
- Automatic cleanup of expired results
- User can only access their own jobs

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status + statistics |
| `/execute` | POST | Execute JavaScript |
| `/jobs` | GET | List user's jobs |
| `/jobs/{id}` | GET | Get job result |

## Request/Response Types

### ExecuteRequest

```go
type ExecuteRequest struct {
    Script     string                 `json:"script"`
    EntryPoint string                 `json:"entry_point,omitempty"` // default: "main"
    Input      map[string]interface{} `json:"input,omitempty"`
    SecretRefs []string               `json:"secret_refs,omitempty"` // secret names
    Timeout    int                    `json:"timeout,omitempty"`
}
```

### ExecuteResponse

```go
type ExecuteResponse struct {
    JobID           string                 `json:"job_id"`
    Status          string                 `json:"status"`
    Output          map[string]interface{} `json:"output,omitempty"`
    Logs            []string               `json:"logs,omitempty"`
    Error           string                 `json:"error,omitempty"`
    GasUsed         int64                  `json:"gas_used"`
    StartedAt       time.Time              `json:"started_at"`
    Duration        string                 `json:"duration,omitempty"`
    EncryptedOutput string                 `json:"encrypted_output,omitempty"`
    OutputHash      string                 `json:"output_hash,omitempty"`
    Signature       string                 `json:"signature,omitempty"`
}
```

## Configuration

```go
type Config struct {
    Marble          *marble.Marble
    DB              database.RepositoryInterface
    ResultTTL       time.Duration // default: 24h
    CleanupInterval time.Duration // default: 1min
}
```

### Required Secrets

| Secret | Description |
|--------|-------------|
| `COMPUTE_MASTER_KEY` | Master key for encryption/signing |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `NEOCOMPUTE_RESULT_TTL` | Custom result retention time |

## Usage Examples

### Basic Script Execution

```json
POST /execute
{
    "script": "function main() { return { result: input.a + input.b }; }",
    "input": { "a": 5, "b": 3 }
}
```

### Using Secrets

```json
POST /execute
{
    "script": "function main() { return { apiKey: secrets.api_key }; }",
    "secret_refs": ["api_key"]
}
```

### Using Crypto Utilities

```json
POST /execute
{
    "script": "function main() { return { hash: crypto.sha256(input.data), random: crypto.randomBytes(16) }; }",
    "input": { "data": "hello world" }
}
```

## Job Status Values

| Status | Description |
|--------|-------------|
| `running` | Execution in progress |
| `completed` | Successfully finished |
| `failed` | Execution failed |

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/crypto` | AES-GCM, HMAC, hashing |
| `internal/marble` | MarbleRun TEE utilities |
| `internal/database` | Secrets access |
| `services/common/service` | Base service framework |

### External Packages

| Package | Purpose |
|---------|---------|
| `github.com/dop251/goja` | JavaScript runtime |
| `github.com/gorilla/mux` | HTTP router |
| `github.com/google/uuid` | Job ID generation |
| `golang.org/x/crypto/hkdf` | Key derivation |

## Related Documentation

- [NeoCompute Service Overview](../README.md)
- [Smart Contract](../contract/README.md)
