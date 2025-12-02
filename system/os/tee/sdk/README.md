# Enclave SDK

The Enclave SDK provides a unified interface for TEE (Trusted Execution Environment) operations within SGX enclaves. It enables Go and JavaScript scripts to securely interact with the enclave environment.

## Overview

The SDK provides the following capabilities:

- **Secrets Management**: Add, update, delete, and retrieve secrets with AES-GCM encryption
- **Key Management**: Generate, import, and manage ECDSA cryptographic keys
- **Permission Verification**: Role-based access control for enclave operations
- **Transaction Signing**: Sign transactions and messages with enclave-protected keys
- **Secure HTTP**: Make HTTPS requests from within the enclave
- **TEE Attestation**: Generate and verify remote attestation reports

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Service Layer                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ VRF Service │  │ Secrets Svc │  │ Oracle Svc  │  ...    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Service Enclave Package                 │   │
│  │  (packages/com.r3e.services.xxx/enclave/enclave.go) │   │
│  └──────────────────────┬──────────────────────────────┘   │
│                         ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  Enclave SDK                         │   │
│  │           (system/enclave/sdk/sdk.go)               │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │ SecretsManager │ KeyManager │ Signer │ HTTP │ Attest│   │
│  └──────────────────────┬──────────────────────────────┘   │
│                         ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Runtime Bridge                          │   │
│  │        (system/enclave/sdk/runtime_bridge.go)       │   │
│  └──────────────────────┬──────────────────────────────┘   │
│                         ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           TEE Script Engine (goja/V8)               │   │
│  │              (system/tee/script_engine.go)          │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Go Integration

```go
import (
    "context"
    "github.com/R3E-Network/service_layer/system/enclave/sdk"
)

// Create SDK configuration
cfg := &sdk.Config{
    ServiceID: "my-service",
    RequestID: "req-123",
    CallerID:  "user-456",
    Metadata: map[string]string{
        "account_id": "acc-789",
    },
}

// Initialize SDK with seal key
sealKey := make([]byte, 32) // Derive from TEE sealing
enclaveSDK := sdk.New(cfg)

// Use secrets manager
ctx := context.Background()
resp, err := enclaveSDK.Secrets().Add(ctx, &sdk.AddSecretRequest{
    Name:  "api_key",
    Value: []byte("secret-value"),
    Type:  sdk.SecretTypeAPIKey,
})

// Generate signing key
keyResp, err := enclaveSDK.Keys().GenerateKey(ctx, &sdk.GenerateKeyRequest{
    Type:  sdk.KeyTypeECDSA,
    Curve: sdk.KeyCurveP256,
})

// Sign data
signResp, err := enclaveSDK.Signer().Sign(ctx, &sdk.SignRequest{
    KeyID: keyResp.KeyID,
    Data:  []byte("data to sign"),
})
```

### JavaScript Integration (via Runtime Bridge)

```javascript
// Secrets API
const apiKey = enclave.secrets.get("api_key");
enclave.secrets.set("new_secret", "value");
enclave.secrets.delete("old_secret");

// Crypto API
const key = enclave.crypto.generateKey("ecdsa");
const signature = enclave.crypto.sign(key.keyId, "data");
const valid = enclave.crypto.verify(key.publicKey, "data", signature);

// HTTP API
const response = enclave.http.get("https://api.example.com/data");
const postResp = enclave.http.post("https://api.example.com/submit", JSON.stringify({data: "value"}));

// Attestation API
const report = enclave.attestation.generateReport("user-data");
const info = enclave.attestation.getEnclaveInfo();

// Context
console.log(enclave.context.serviceId);
console.log(enclave.context.requestId);
console.log(enclave.context.accountId);
```

## SDK Components

### SecretsManager

Manages encrypted secrets within the enclave.

```go
type SecretsManager interface {
    Add(ctx context.Context, req *AddSecretRequest) (*AddSecretResponse, error)
    Update(ctx context.Context, req *UpdateSecretRequest) (*UpdateSecretResponse, error)
    Delete(ctx context.Context, req *DeleteSecretRequest) error
    Get(ctx context.Context, secretID string) (*Secret, error)
    Find(ctx context.Context, req *FindSecretRequest) (*FindSecretResponse, error)
    List(ctx context.Context, req *ListSecretsRequest) (*ListSecretsResponse, error)
    Exists(ctx context.Context, secretID string) (bool, error)
}
```

**Secret Types:**
- `SecretTypeGeneric` - Generic secret data
- `SecretTypeAPIKey` - API keys
- `SecretTypePrivateKey` - Private keys
- `SecretTypeCertificate` - Certificates
- `SecretTypePassword` - Passwords

### KeyManager

Manages cryptographic keys with enclave sealing.

```go
type KeyManager interface {
    GenerateKey(ctx context.Context, req *GenerateKeyRequest) (*GenerateKeyResponse, error)
    ImportKey(ctx context.Context, req *ImportKeyRequest) (*ImportKeyResponse, error)
    ExportPublicKey(ctx context.Context, keyID string) ([]byte, error)
    DeleteKey(ctx context.Context, keyID string) error
    ListKeys(ctx context.Context) ([]string, error)
    DeriveKey(ctx context.Context, req *DeriveKeyRequest) (*DeriveKeyResponse, error)
}
```

**Key Types:**
- `KeyTypeECDSA` - ECDSA signing keys
- `KeyTypeEd25519` - Ed25519 signing keys
- `KeyTypeAES` - AES encryption keys

**Curves:**
- `KeyCurveP256` - NIST P-256
- `KeyCurveP384` - NIST P-384
- `KeyCurveSecp256k1` - Bitcoin/Ethereum curve

### TransactionSigner

Signs transactions and messages.

```go
type TransactionSigner interface {
    Sign(ctx context.Context, req *SignRequest) (*SignResponse, error)
    SignTransaction(ctx context.Context, req *SignTransactionRequest) (*SignTransactionResponse, error)
    SignMessage(ctx context.Context, req *SignMessageRequest) (*SignMessageResponse, error)
    Verify(ctx context.Context, req *VerifyRequest) (bool, error)
    GetSigningKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error)
}
```

### SecureHTTPClient

Makes secure HTTP requests from within the enclave.

```go
type SecureHTTPClient interface {
    Get(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error)
    Post(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error)
    Put(ctx context.Context, url string, body []byte, opts ...HTTPOption) (*HTTPResponse, error)
    Delete(ctx context.Context, url string, opts ...HTTPOption) (*HTTPResponse, error)
    Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
    SetTLSConfig(config *tls.Config)
    AddTrustedCert(cert []byte) error
}
```

**HTTP Options:**
```go
sdk.WithHeader("Authorization", "Bearer token")
sdk.WithTimeout(30 * time.Second)
sdk.WithBasicAuth("user", "pass")
sdk.WithBearerToken("token")
sdk.WithAPIKey("X-API-Key", "key")
```

### AttestationProvider

Generates and verifies TEE attestation reports.

```go
type AttestationProvider interface {
    GenerateReport(ctx context.Context, userData []byte) (*AttestationReport, error)
    VerifyReport(ctx context.Context, report *AttestationReport) (bool, error)
    GetEnclaveInfo(ctx context.Context) (*EnclaveInfo, error)
    GetQuote(ctx context.Context, reportData []byte) ([]byte, error)
}
```

## Service Integration Pattern

Each service enclave follows this integration pattern:

```go
// packages/com.r3e.services.xxx/enclave/enclave.go

package enclave

import (
    "context"
    "github.com/R3E-Network/service_layer/system/enclave/sdk"
)

type EnclaveXXX struct {
    // Service-specific fields
    // ...

    // SDK integration
    sdk         sdk.EnclaveSDK
    initialized bool
}

type XXXConfig struct {
    ServiceID string
    RequestID string
    CallerID  string
    AccountID string
    SealKey   []byte
}

// Constructor with SDK
func NewEnclaveXXXWithSDK(cfg *XXXConfig) (*EnclaveXXX, error) {
    sdkCfg := &sdk.Config{
        ServiceID: cfg.ServiceID,
        RequestID: cfg.RequestID,
        CallerID:  cfg.CallerID,
        Metadata: map[string]string{
            "account_id": cfg.AccountID,
            "service":    "xxx",
        },
    }

    enclaveSDK := sdk.New(sdkCfg)

    return &EnclaveXXX{
        sdk:         enclaveSDK,
        initialized: true,
    }, nil
}

// Initialize with existing SDK
func (e *EnclaveXXX) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) {
    e.sdk = enclaveSDK
    e.initialized = true
}

// Use SDK for attestation
func (e *EnclaveXXX) GenerateAttestationReport(userData []byte) ([]byte, error) {
    if e.sdk != nil && e.initialized {
        ctx := context.Background()
        report, err := e.sdk.Attestation().GenerateReport(ctx, userData)
        if err == nil {
            return report.ReportData, nil
        }
    }
    // Fallback to local implementation
    // ...
}

// Accessors
func (e *EnclaveXXX) SDK() sdk.EnclaveSDK { return e.sdk }
func (e *EnclaveXXX) IsInitialized() bool { return e.initialized }
```

## Security Considerations

1. **Seal Key**: The seal key should be derived from the TEE's hardware sealing mechanism (MRENCLAVE/MRSIGNER)
2. **Key Storage**: All private keys are encrypted with AES-GCM using the seal key
3. **Memory Protection**: Sensitive data is cleared from memory after use
4. **TLS**: HTTP client enforces TLS 1.2+ for all connections
5. **Attestation**: Remote attestation proves code integrity to external verifiers

## File Structure

```
system/enclave/sdk/
├── sdk.go              # Core interfaces and types
├── secrets_impl.go     # SecretsManager implementation
├── keys_impl.go        # KeyManager implementation
├── signer_impl.go      # TransactionSigner implementation
├── http_impl.go        # SecureHTTPClient implementation
├── attestation_impl.go # AttestationProvider implementation
├── permissions_impl.go # PermissionManager implementation
├── runtime_bridge.go   # Bridge to TEE script engine
└── README.md           # This documentation

system/tee/
├── sdk_adapter.go      # Adapter for goja JavaScript runtime
├── script_engine.go    # JavaScript execution engine
└── ...
```

## Building

```bash
# Build SDK
go build ./system/enclave/sdk/...

# Build TEE integration
go build ./system/tee/...

# Build all packages
go build ./packages/...
```

## Testing

```bash
# Run SDK tests
go test ./system/enclave/sdk/...

# Run integration tests
go test ./system/tee/...
```
