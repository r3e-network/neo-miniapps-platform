# Confidential Service

Confidential computing service for the Neo Service Layer.

## Overview

The Confidential service provides secure data processing within the TEE enclave. Data is encrypted at rest and in transit, with computation performed inside the SGX enclave and results attested.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Confidential │     │   Enclave    │
│              │     │ Service      │     │   (SGX)      │
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
- **Sealed Computation**: Processing inside SGX enclave
- **Result Attestation**: Results signed with TEE key
- **Zero Knowledge**: Service cannot see plaintext data

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/compute` | POST | Submit computation request |
| `/result/{id}` | GET | Get computation result |
| `/attestation` | GET | Get enclave attestation |

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
        "report": "base64-encoded-sgx-report",
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
2. **Key Exchange**: ECDH key exchange with enclave
3. **Sealed Processing**: Decryption only inside enclave
4. **Attestation**: Results include SGX attestation

## Testing

```bash
go test ./services/confidential/... -v -cover
```

Current test coverage: **65.3%**

## Version

- Service ID: `confidential`
- Version: `1.0.0`
- Status: **Beta**
