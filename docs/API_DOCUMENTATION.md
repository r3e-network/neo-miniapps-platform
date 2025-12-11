# Service Layer API Documentation

## Overview

The Neo Service Layer provides a comprehensive set of TEE-protected services for the Neo N3 blockchain. All services are protected by MarbleRun/EGo TEE and coordinated through MarbleRun.

## Base URL

```
Production: https://api.service-layer.neo.org
Staging: https://staging-api.service-layer.neo.org
Development: http://localhost:8080
```

## Authentication

All API requests require JWT authentication via the Gateway service.

### Headers

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

## Services

### 1. Gateway Service

The API Gateway handles authentication, rate limiting, and request routing.

#### Health Check

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-12-10T10:00:00Z",
  "version": "1.0.0"
}
```

#### Metrics

```http
GET /metrics
```

Returns Prometheus metrics for monitoring.

---

### 2. Oracle Service

Provides external data fetching with TEE attestation.

#### Get Price Data

```http
POST /neooracle/price
```

**Request:**
```json
{
  "symbol": "NEO/USD",
  "sources": ["binance", "coinbase"]
}
```

**Response:**
```json
{
  "symbol": "NEO/USD",
  "price": 15.42,
  "timestamp": "2025-12-10T10:00:00Z",
  "sources": [
    {
      "name": "binance",
      "price": 15.41,
      "timestamp": "2025-12-10T10:00:00Z"
    },
    {
      "name": "coinbase",
      "price": 15.43,
      "timestamp": "2025-12-10T10:00:00Z"
    }
  ],
  "attestation": "base64_encoded_attestation"
}
```

#### Submit Data to Blockchain

```http
POST /neooracle/submit
```

**Request:**
```json
{
  "contract_hash": "0x1234567890abcdef",
  "method": "updatePrice",
  "params": {
    "symbol": "NEO/USD",
    "price": 1542,
    "decimals": 2
  }
}
```

**Response:**
```json
{
  "tx_hash": "0xabcdef1234567890",
  "status": "pending",
  "submitted_at": "2025-12-10T10:00:00Z"
}
```

---

### 3. VRF Service

Verifiable Random Function service for provably fair randomness.

#### Generate Random Number

```http
POST /neorand/generate
```

**Request:**
```json
{
  "seed": "user_provided_seed",
  "min": 1,
  "max": 100
}
```

**Response:**
```json
{
  "random_number": 42,
  "proof": "base64_encoded_proof",
  "public_key": "base64_encoded_public_key",
  "attestation": "base64_encoded_attestation"
}
```

#### Verify Random Number

```http
POST /neorand/verify
```

**Request:**
```json
{
  "random_number": 42,
  "proof": "base64_encoded_proof",
  "public_key": "base64_encoded_public_key",
  "seed": "user_provided_seed"
}
```

**Response:**
```json
{
  "valid": true,
  "verified_at": "2025-12-10T10:00:00Z"
}
```

---

### 4. NeoVault Service

Privacy-preserving token mixing service.

#### Create Mix Request

```http
POST /neovault/mix
```

**Request:**
```json
{
  "token_type": "GAS",
  "amount": 100,
  "deposit_address": "NSourceAddress123",
  "withdrawal_address": "NDestAddress456",
  "delay_hours": 24
}
```

**Response:**
```json
{
  "mix_id": "mix_123456",
  "deposit_address": "NNeoVaultDepositAddr",
  "amount": 100,
  "service_fee": 0.5,
  "network_fee": 0.1,
  "total_required": 100.6,
  "status": "pending_deposit",
  "created_at": "2025-12-10T10:00:00Z"
}
```

#### Get Mix Status

```http
GET /neovault/status/{mix_id}
```

**Response:**
```json
{
  "mix_id": "mix_123456",
  "status": "completed",
  "deposit_confirmed": true,
  "withdrawal_tx": "0xabcdef1234567890",
  "completed_at": "2025-12-11T10:00:00Z"
}
```

---

### 5. Account Pool Service

Manages a pool of funded accounts for service operations.

#### Request Accounts

```http
POST /accountpool/request
```

**Request:**
```json
{
  "service_id": "neovault",
  "count": 5,
  "purpose": "mixing_operation"
}
```

**Response:**
```json
{
  "accounts": [
    {
      "id": "acc_001",
      "address": "NAccountAddr001",
      "balance": 1000000,
      "locked_by": "neovault",
      "locked_at": "2025-12-10T10:00:00Z"
    }
  ],
  "lock_id": "lock_123456"
}
```

#### Release Accounts

```http
POST /accountpool/release
```

**Request:**
```json
{
  "service_id": "neovault",
  "account_ids": ["acc_001", "acc_002"]
}
```

**Response:**
```json
{
  "released_count": 2,
  "released_at": "2025-12-10T10:00:00Z"
}
```

#### Update Account Balance

```http
POST /accountpool/balance
```

**Request:**
```json
{
  "service_id": "neovault",
  "account_id": "acc_001",
  "delta": -50000,
  "absolute": null
}
```

**Response:**
```json
{
  "account_id": "acc_001",
  "old_balance": 1000000,
  "new_balance": 950000,
  "updated_at": "2025-12-10T10:00:00Z"
}
```

---

### 6. NeoFlow Service

Automated job execution with TEE protection.

#### Create Job

```http
POST /neoflow/jobs
```

**Request:**
```json
{
  "name": "daily_price_update",
  "schedule": "0 0 * * *",
  "action": {
    "type": "oracle_submit",
    "params": {
      "symbol": "NEO/USD",
      "contract": "0x1234567890abcdef"
    }
  },
  "enabled": true
}
```

**Response:**
```json
{
  "job_id": "job_123456",
  "name": "daily_price_update",
  "schedule": "0 0 * * *",
  "next_run": "2025-12-11T00:00:00Z",
  "created_at": "2025-12-10T10:00:00Z"
}
```

#### List Jobs

```http
GET /neoflow/jobs
```

**Response:**
```json
{
  "jobs": [
    {
      "job_id": "job_123456",
      "name": "daily_price_update",
      "schedule": "0 0 * * *",
      "enabled": true,
      "last_run": "2025-12-10T00:00:00Z",
      "next_run": "2025-12-11T00:00:00Z",
      "status": "active"
    }
  ],
  "total": 1
}
```

#### Get Job Status

```http
GET /neoflow/jobs/{job_id}
```

**Response:**
```json
{
  "job_id": "job_123456",
  "name": "daily_price_update",
  "schedule": "0 0 * * *",
  "enabled": true,
  "last_run": {
    "timestamp": "2025-12-10T00:00:00Z",
    "status": "success",
    "duration_ms": 1234
  },
  "next_run": "2025-12-11T00:00:00Z",
  "execution_history": [
    {
      "timestamp": "2025-12-10T00:00:00Z",
      "status": "success",
      "duration_ms": 1234
    }
  ]
}
```

---

### 7. NeoCompute Service

Secure data encryption and signing within TEE.

#### Encrypt Data

```http
POST /neocompute/encrypt
```

**Request:**
```json
{
  "data": "sensitive_data_to_encrypt",
  "key_id": "user_key_001"
}
```

**Response:**
```json
{
  "encrypted_data": "base64_encoded_encrypted_data",
  "key_id": "user_key_001",
  "nonce": "base64_encoded_nonce",
  "attestation": "base64_encoded_attestation"
}
```

#### Decrypt Data

```http
POST /neocompute/decrypt
```

**Request:**
```json
{
  "encrypted_data": "base64_encoded_encrypted_data",
  "key_id": "user_key_001",
  "nonce": "base64_encoded_nonce"
}
```

**Response:**
```json
{
  "data": "sensitive_data_to_encrypt",
  "decrypted_at": "2025-12-10T10:00:00Z"
}
```

#### Sign Data

```http
POST /neocompute/sign
```

**Request:**
```json
{
  "data": "data_to_sign",
  "key_id": "signing_key_001"
}
```

**Response:**
```json
{
  "signature": "base64_encoded_signature",
  "public_key": "base64_encoded_public_key",
  "attestation": "base64_encoded_attestation"
}
```

---

## Error Responses

All services return consistent error responses:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "field": "Additional context"
    }
  },
  "timestamp": "2025-12-10T10:00:00Z"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Invalid or missing authentication token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `INVALID_REQUEST` | 400 | Invalid request parameters |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |

---

## Rate Limiting

Rate limits are enforced per API key:

- **Default**: 100 requests per minute
- **Burst**: 200 requests per minute
- **Daily**: 10,000 requests per day

Rate limit headers are included in all responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1702209600
```

---

## Attestation

All critical operations include TEE attestation data that can be verified independently:

```json
{
  "attestation": {
    "quote": "base64_encoded_attestation_quote",
    "manifest_signature": "base64_encoded_signature",
    "timestamp": "2025-12-10T10:00:00Z"
  }
}
```

To verify attestation:

1. Extract the MarbleRun quote
2. Verify the quote with Intel Attestation Service (IAS) or DCAP
3. Verify the manifest signature matches the expected MarbleRun manifest
4. Check the timestamp is recent

---

## SDK Support

Official SDKs are available for:

- **Go**: `github.com/R3E-Network/service-layer-sdk-go`
- **TypeScript**: `@r3e-network/service-layer-sdk`
- **Python**: `service-layer-sdk` (coming soon)

Example usage (TypeScript):

```typescript
import { ServiceLayerClient } from '@r3e-network/service-layer-sdk';

const client = new ServiceLayerClient({
  baseUrl: 'https://api.service-layer.neo.org',
  apiKey: 'your_api_key'
});

// Generate random number
const result = await client.vrf.generate({
  seed: 'my_seed',
  min: 1,
  max: 100
});

console.log('Random number:', result.random_number);
console.log('Proof:', result.proof);
```

---

## Webhooks

Services can send webhook notifications for asynchronous operations:

### Webhook Configuration

```http
POST /webhooks
```

**Request:**
```json
{
  "url": "https://your-app.com/webhook",
  "events": ["mix.completed", "job.executed"],
  "secret": "webhook_secret_for_verification"
}
```

### Webhook Payload

```json
{
  "event": "mix.completed",
  "timestamp": "2025-12-10T10:00:00Z",
  "data": {
    "mix_id": "mix_123456",
    "status": "completed",
    "withdrawal_tx": "0xabcdef1234567890"
  },
  "signature": "hmac_sha256_signature"
}
```

---

## Support

For API support and questions:

- **Documentation**: https://docs.service-layer.neo.org
- **GitHub Issues**: https://github.com/R3E-Network/service_layer/issues
- **Discord**: https://discord.gg/neo
- **Email**: support@r3e-network.org
