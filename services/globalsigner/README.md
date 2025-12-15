# GlobalSigner Service

TEE-protected master key management service with automatic 30-day rotation.

## Overview

GlobalSigner is the **central signing authority** for the service layer. It manages master keys within a TEE (Trusted Execution Environment) and provides:

- **Key Generation**: Deterministic P-256 key derivation from master seed
- **Key Rotation**: Automatic 30-day rotation with 7-day overlap
- **Domain-Separated Signing**: Signing-as-a-service for child services
- **Attestation**: SGX quote generation binding public key to enclave

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    GlobalSigner (TEE)                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Master Seed │→ │ Key Manager │→ │ Active Key (v2025-01)│ │
│  │ (Marble)    │  │             │  │ Overlap (v2024-12)  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                          │                                  │
│                          ▼                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Domain-Separated Signing                │   │
│  │  neovault:proof  │  neoaccounts:pool  │  neoflow:*  │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │   TxSubmitter   │
                    │ set_tee_master  │
                    └─────────────────┘
```

## Configuration

```yaml
globalsigner:
  rotation:
    rotation_period: "720h" # 30 days
    overlap_period: "168h" # 7 days
    auto_rotate: true
    require_on_chain_anchor: true
```

## API Endpoints

| Endpoint       | Method | Description                   |
| -------------- | ------ | ----------------------------- |
| `/health`      | GET    | Health check                  |
| `/ready`       | GET    | Readiness check               |
| `/info`        | GET    | Service info and statistics   |
| `/status`      | GET    | Detailed status with key info |
| `/rotate`      | POST   | Trigger key rotation          |
| `/sign`        | POST   | Domain-separated signing      |
| `/derive`      | POST   | Deterministic key derivation  |
| `/attestation` | GET    | Get current key attestation   |
| `/keys`        | GET    | List all key versions         |

## Key Lifecycle

```
┌─────────┐     ┌────────┐     ┌─────────────┐     ┌─────────┐
│ pending │ ──→ │ active │ ──→ │ overlapping │ ──→ │ revoked │
└─────────┘     └────────┘     └─────────────┘     └─────────┘
     │               │                │
     │               │                │
     ▼               ▼                ▼
  On-chain       30 days          7 days
   anchor        rotation         overlap
```

### States

- **pending**: Key created, awaiting on-chain anchor confirmation
- **active**: Currently active for signing (only one at a time)
- **overlapping**: Previous key, still valid during overlap period
- **revoked**: No longer valid for any operations

## Signing API

### Domain-Separated Signing

```bash
curl -X POST http://localhost:8080/sign \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "neovault",
    "data": "0x1234567890abcdef"
  }'
```

Response:

```json
{
  "signature": "0x...",
  "key_version": "v2025-01",
  "pubkey_hex": "0x..."
}
```

### Key Derivation

```bash
curl -X POST http://localhost:8080/derive \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "neoaccounts",
    "path": "pool/0"
  }'
```

Response:

```json
{
  "pubkey_hex": "0x...",
  "key_version": "v2025-01"
}
```

## Attestation

GlobalSigner generates SGX attestations binding the master public key to the enclave:

```json
{
  "key_version": "v2025-01",
  "pubkey_hex": "0x...",
  "pubkey_hash": "sha256(pubkey)",
  "quote": "base64(sgx_quote)",
  "mrenclave": "base64(...)",
  "mrsigner": "base64(...)",
  "prod_id": 1,
  "isvsvn": 1,
  "timestamp": "2025-01-15T00:00:00Z",
  "simulated": false
}
```

The `pubkey_hash` is bound to the SGX report data, allowing verifiers to confirm the key was generated inside the attested enclave.

## Security Model

1. **Master Seed**: Injected via MarbleRun manifest, never leaves TEE
2. **Key Derivation**: HKDF with version-specific info, deterministic
3. **Domain Separation**: Each service gets isolated signing domain
4. **Overlap Period**: Allows graceful key transition without service disruption
5. **Attestation**: Cryptographic proof of key-enclave binding

## Metrics

- `signatures_issued`: Total signatures generated
- `rotations_count`: Number of key rotations performed
- `active_version`: Currently active key version
- `key_versions`: List of all loaded key versions

## Integration

### NeoVault Integration

NeoVault calls GlobalSigner for proof signing instead of managing its own keys:

```go
resp, err := globalSignerClient.Sign(ctx, &SignRequest{
    Domain: "neovault",
    Data:   hex.EncodeToString(proofData),
})
```

### TxSubmitter Integration

GlobalSigner submits `set_tee_master_key` transactions through TxSubmitter:

```go
txResp, err := txSubmitter.Submit(ctx, "globalsigner", &TxRequest{
    TxType:     "set_tee_master_key",
    MethodName: "setTEEMasterKey",
    Params:     params,
})
```
