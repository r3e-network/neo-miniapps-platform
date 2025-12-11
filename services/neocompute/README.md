# NeoCompute Service

NeoCompute computing service for the Neo Service Layer.

## Overview

The NeoCompute service provides secure data processing within the MarbleRun TEE. Data is encrypted at rest and in transit, with computation performed inside the MarbleRun TEE and results attested.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ NeoCompute   │     │ TEE          │
│              │     │ Service      │     │ (MarbleRun)  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Submit Data        │                    │
       │ (encrypted)        │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Decrypt & Process  │
       │                    │───────────────────>│
       │                    │                    │
       │                    │ Encrypted Result   │
       │                    │<───────────────────│
       │                    │                    │
       │ Result + Attestation                    │
       │<───────────────────│                    │
```

## Features

- **Data Encryption**: AES-256-GCM encryption for all data
- **Sealed Computation**: Processing inside MarbleRun TEE
- **Result Attestation**: Results signed with TEE key
- **Zero Knowledge**: Service cannot see plaintext data

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/compute` | POST | Submit computation request |
| `/result/{id}` | GET | Get computation result |
| `/attestation` | GET | Get TEE attestation |

## Request/Response Types

### Submit Computation

```json
POST /compute
{
    "encrypted_data": "base64-encoded-ciphertext",
    "computation_type": "aggregate",
    "parameters": {
        "operation": "sum"
    }
}
```

### Computation Response

```json
{
    "request_id": "uuid",
    "status": "completed",
    "encrypted_result": "base64-encoded-ciphertext",
    "attestation": {
        "report": "base64-encoded-attestation-report",
        "signature": "0x..."
    }
}
```

## Computation Types

| Type | Description |
|------|-------------|
| `aggregate` | Aggregate operations (sum, avg, min, max) |
| `transform` | Data transformation |
| `validate` | Data validation |
| `custom` | Custom WASM computation |

## Security Model

1. **Client-Side Encryption**: Data encrypted before sending
2. **Key Exchange**: ECDH key exchange with TEE
3. **Sealed Processing**: Decryption only inside TEE
4. **Attestation**: Results include MarbleRun attestation

## Testing

```bash
go test ./services/neocompute/... -v -cover
```

Current test coverage: **65.3%**

## Version

- Service ID: `neocompute`
- Version: `1.0.0`
- Status: **Beta**
