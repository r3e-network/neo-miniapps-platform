# Service Template

This package provides a template for creating new services on the ServiceOS platform.

## Quick Start

1. Copy this entire `template` directory to create your new service:
   ```bash
   cp -r services/template services/myservice
   ```

2. Update the package name and constants in `service.go`:
   ```go
   const (
       ServiceID   = "myservice"
       ServiceName = "My Service"
       Version     = "1.0.0"
   )
   ```

3. Update the manifest with your required capabilities:
   ```go
   func Manifest() *os.Manifest {
       return &os.Manifest{
           ServiceID:   ServiceID,
           RequiredCapabilities: []os.Capability{
               os.CapStorage,
               // Add your required capabilities
           },
           // ...
       }
   }
   ```

4. Implement your business logic in `service.go`

5. Update domain types in `domain.go`

6. Implement storage operations in `store.go`

7. Implement TEE-protected operations in `enclave.go`

## Architecture

```
services/myservice/
├── service.go    # Main service implementation
├── domain.go     # Domain types and DTOs
├── store.go      # Data persistence
├── enclave.go    # TEE-protected operations
└── README.md     # Documentation
```

## Available Capabilities

| Capability | Description |
|------------|-------------|
| `CapSecrets` | Access to secret storage |
| `CapSecretsWrite` | Write secrets |
| `CapNetwork` | Outbound network requests |
| `CapKeys` | Key derivation |
| `CapKeysSign` | Sign with keys |
| `CapCompute` | Confidential computation |
| `CapStorage` | Persistent storage |
| `CapEvents` | Event bus access |
| `CapAttestation` | Remote attestation |
| `CapNeo` | Neo N3 operations |
| `CapNeoSign` | Sign Neo transactions |
| `CapDatabase` | Database access |
| `CapDatabaseWrite` | Write to database |
| `CapConfig` | Configuration management |
| `CapMetrics` | Metrics collection |
| `CapScheduler` | Task scheduling |
| `CapCache` | Caching |
| `CapChain` | Generic blockchain operations |
| `CapAuth` | Authentication/authorization |
| `CapQueue` | Message queue |
| `CapLock` | Distributed locking |

## Example Usage

### Using Secrets (TEE-protected)

```go
// Secrets are accessed via callback - they never leave the enclave
err := s.OS().Secrets().Use(ctx, "api_key", func(secret []byte) error {
    // Use secret here - it's only available inside this callback
    return doSomethingWithSecret(secret)
})
```

### Using Network with Secret Auth

```go
resp, err := s.OS().Network().FetchWithSecret(ctx, os.HTTPRequest{
    Method: "GET",
    URL:    "https://api.example.com/data",
}, "api_token", os.AuthBearer)
```

### Using Neo N3 Blockchain

```go
// Create wallet (private key stays in enclave)
handle, err := s.OS().Neo().CreateWallet(ctx, "wallet/main")

// Get address (safe to expose)
address, err := s.OS().Neo().GetAddress(ctx, handle)

// Sign transaction (signing happens inside enclave)
signature, err := s.OS().Neo().SignTransaction(ctx, handle, txHash)
```

### Using Cache

```go
// Get or set pattern
data, err := s.OS().Cache().GetOrSet(ctx, "key", func() (any, error) {
    return fetchExpensiveData()
}, 5*time.Minute)
```

### Using Scheduler

```go
// Schedule periodic task
taskID, err := s.OS().Scheduler().ScheduleInterval(ctx, 1*time.Hour, &os.ScheduledTask{
    Name:    "cleanup",
    Handler: "cleanup",
})

// Schedule cron task
taskID, err := s.OS().Scheduler().ScheduleCron(ctx, "0 0 * * *", &os.ScheduledTask{
    Name:    "daily_report",
    Handler: "daily_report",
})
```

### Using Distributed Lock

```go
lock, err := s.OS().Lock().Acquire(ctx, "resource_key", 30*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx)

// Do work with exclusive access
```

## Security Model

1. **TEE Trust Root**: All sensitive operations happen inside the TEE enclave
2. **Capability-Based Access**: Services declare required capabilities in manifest
3. **Secret Isolation**: Secrets are accessed via callbacks, never exposed
4. **Key Protection**: Private keys never leave the enclave
5. **Namespace Isolation**: Each service has its own namespace for secrets/storage
