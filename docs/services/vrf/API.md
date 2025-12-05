# VRF Service API Documentation

## Base URL

```
http://localhost:8080/api
```

## Authentication

All API requests require authentication via JWT token or API key.

```http
Authorization: Bearer <token>
```

## Endpoints

### 1. Request Random Number

Generate verifiable random number.

**Endpoint**: `POST /vrf/request`

**Request Body**:
```json
{
  "seed": "user_provided_seed_12345",
  "callback": "0x1234567890abcdef"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "request_id": "req-1701234567890",
    "status": "pending",
    "service": "vrf",
    "message": "VRF request submitted"
  }
}
```

**Status Codes**:
- `200 OK`: Request accepted
- `400 Bad Request`: Invalid seed
- `503 Service Unavailable`: VRF service not available

---

### 2. Generate Randomness (Direct)

Directly generate randomness with proof.

**Endpoint**: `POST /vrf/generate`

**Request Body**:
```json
{
  "account_id": "user123",
  "seed": "0x1234567890abcdef",
  "block_hash": "0xabcdef1234567890",
  "block_number": 1000000
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "randomness": "0x9876543210fedcba...",
    "proof": "0xabcdef123456...",
    "input": "0x1234567890abcdef",
    "request_id": "vrf-1701234567890"
  }
}
```

**Status Codes**:
- `200 OK`: Generation successful
- `400 Bad Request`: Invalid parameters
- `500 Internal Server Error`: Generation failed

---

### 3. Verify Randomness

Verify VRF output and proof.

**Endpoint**: `POST /vrf/verify`

**Request Body**:
```json
{
  "randomness": "0x9876543210fedcba...",
  "proof": "0xabcdef123456...",
  "input": "0x1234567890abcdef"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "valid": true,
    "public_key": "0x04..."
  }
}
```

**Status Codes**:
- `200 OK`: Verification complete
- `400 Bad Request`: Invalid proof format

---

### 4. Get Public Key

Retrieve VRF public key for verification.

**Endpoint**: `GET /vrf/public-key`

**Response**:
```json
{
  "success": true,
  "data": {
    "public_key": "0x04...",
    "algorithm": "ECVRF-P256-SHA256",
    "format": "uncompressed"
  }
}
```

**Status Codes**:
- `200 OK`: Success

---

### 5. Get Request Status

Check VRF request status.

**Endpoint**: `GET /vrf/requests/{requestId}`

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "vrf-1701234567890",
    "account_id": "user123",
    "status": "fulfilled",
    "seed": "0x1234567890abcdef",
    "randomness": "0x9876543210fedcba...",
    "proof": "0xabcdef123456...",
    "callback_address": "0x1234567890abcdef",
    "created_at": "2025-12-04T10:30:00Z",
    "fulfilled_at": "2025-12-04T10:30:01Z"
  }
}
```

**Status Codes**:
- `200 OK`: Request found
- `404 Not Found`: Request does not exist

---

### 6. List Requests

List VRF requests for an account.

**Endpoint**: `GET /vrf/requests`

**Query Parameters**:
- `account_id` (string): Filter by account
- `status` (string): Filter by status (pending, fulfilled, failed)
- `limit` (integer): Maximum results (default: 50)
- `offset` (integer): Pagination offset (default: 0)

**Response**:
```json
{
  "success": true,
  "data": {
    "requests": [
      {
        "id": "vrf-1701234567890",
        "account_id": "user123",
        "status": "fulfilled",
        "created_at": "2025-12-04T10:30:00Z"
      }
    ],
    "total": 1
  }
}
```

**Status Codes**:
- `200 OK`: Success

---

### 7. Get Statistics

Retrieve VRF service statistics.

**Endpoint**: `GET /vrf/stats`

**Response**:
```json
{
  "success": true,
  "data": {
    "total_requests": 1234,
    "fulfilled_requests": 1200,
    "pending_requests": 10,
    "failed_requests": 24,
    "generated_at": "2025-12-04T10:30:00Z"
  }
}
```

**Status Codes**:
- `200 OK`: Success

---

## Data Types

### GenerateRandomnessRequest

```typescript
interface GenerateRandomnessRequest {
  account_id: string;       // Account identifier
  seed: string;             // Input seed (hex)
  block_hash?: string;      // Optional block hash
  block_number?: number;    // Optional block number
  callback_address?: string; // Callback contract address
}
```

### VRFOutput

```typescript
interface VRFOutput {
  randomness: string;       // 32-byte random value (hex)
  proof: string;            // VRF proof (hex)
  input: string;            // Original seed (hex)
}
```

### VRFRequest

```typescript
interface VRFRequest {
  id: string;               // Unique request ID
  account_id: string;       // Requester account
  status: RequestStatus;    // Request status
  seed: string;             // Input seed (hex)
  block_hash?: string;      // Block hash (hex)
  block_number?: number;    // Block number
  randomness?: string;      // Generated randomness (hex)
  proof?: string;           // VRF proof (hex)
  callback_address?: string; // Callback address
  created_at: string;       // ISO 8601 timestamp
  fulfilled_at?: string;    // ISO 8601 timestamp
}
```

### RequestStatus

```typescript
enum RequestStatus {
  Pending = "pending",
  Fulfilled = "fulfilled",
  Failed = "failed"
}
```

## Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

### Common Error Codes

- `400 Bad Request`: Invalid request parameters
- `401 Unauthorized`: Authentication required or failed
- `404 Not Found`: Resource not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error
- `503 Service Unavailable`: Service not available

## Rate Limiting

API requests are rate-limited:

- **Default**: 100 requests per minute per API key
- **Burst**: Up to 200 requests in short bursts

## SDK Examples

### JavaScript/TypeScript

```typescript
import { VRFClient } from '@r3e/service-layer-sdk';

const client = new VRFClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

// Generate randomness
const output = await client.generateRandomness({
  account_id: 'user123',
  seed: '0x1234567890abcdef'
});

console.log('Randomness:', output.randomness);
console.log('Proof:', output.proof);

// Verify randomness
const valid = await client.verifyRandomness(output);
console.log('Valid:', valid);
```

### Go

```go
import "github.com/R3E-Network/service_layer/sdk/go/client"

client := client.NewVRFClient("http://localhost:8080/api", "your-api-key")

// Generate randomness
output, err := client.GenerateRandomness(ctx, &client.GenerateRandomnessRequest{
    AccountID: "user123",
    Seed:      []byte{0x12, 0x34, 0x56, 0x78},
})

// Verify randomness
valid, err := client.VerifyRandomness(ctx, output)
```

## Testing

### Health Check

```bash
curl http://localhost:8080/api/health
```

### Generate Random Number

```bash
curl -X POST http://localhost:8080/api/vrf/generate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{
    "account_id": "user123",
    "seed": "0x1234567890abcdef"
  }'
```

### Verify Randomness

```bash
curl -X POST http://localhost:8080/api/vrf/verify \
  -H "Content-Type: application/json" \
  -d '{
    "randomness": "0x9876543210fedcba...",
    "proof": "0xabcdef123456...",
    "input": "0x1234567890abcdef"
  }'
```
