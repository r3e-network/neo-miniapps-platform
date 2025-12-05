# Oracle Service API Documentation

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

### 1. Submit Oracle Request

Submit a data fetch request to the oracle service.

**Endpoint**: `POST /oracle/request`

**Request Body**:
```json
{
  "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
  "method": "GET",
  "headers": {
    "Accept": "application/json"
  },
  "body": "",
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
    "service": "oracle",
    "message": "Oracle request submitted"
  }
}
```

**Status Codes**:
- `200 OK`: Request accepted
- `400 Bad Request`: Invalid request body
- `503 Service Unavailable`: Oracle service not available

---

### 2. Fetch Data (Direct)

Directly fetch data from an external source with TEE protection.

**Endpoint**: `POST /oracle/fetch`

**Request Body**:
```json
{
  "url": "https://api.example.com/data",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer token123"
  },
  "secret_name": "api_key_example",
  "auth_type": "bearer"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "status_code": 200,
    "headers": {
      "content-type": "application/json"
    },
    "body": "{\"price\": 42000}",
    "signature": "0x1234abcd...",
    "timestamp": "2025-12-04T10:30:00Z"
  }
}
```

**Status Codes**:
- `200 OK`: Fetch successful
- `400 Bad Request`: Invalid parameters
- `401 Unauthorized`: Authentication failed
- `500 Internal Server Error`: Fetch failed

---

### 3. Create Data Feed

Create a new scheduled data feed.

**Endpoint**: `POST /oracle/feeds`

**Request Body**:
```json
{
  "name": "BTC Price Feed",
  "description": "Bitcoin price from CoinGecko",
  "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
  "method": "GET",
  "headers": {
    "Accept": "application/json"
  },
  "secret_name": "",
  "auth_type": "none",
  "schedule": "*/5 * * * *",
  "interval": "5m"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "feed-btc-usd",
    "name": "BTC Price Feed",
    "status": "active",
    "created_at": "2025-12-04T10:30:00Z"
  }
}
```

**Status Codes**:
- `201 Created`: Feed created successfully
- `400 Bad Request`: Invalid feed configuration
- `409 Conflict`: Feed with same name exists

---

### 4. Get Data Feed

Retrieve a specific data feed configuration.

**Endpoint**: `GET /oracle/feeds/{feedId}`

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "feed-btc-usd",
    "name": "BTC Price Feed",
    "description": "Bitcoin price from CoinGecko",
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
    "method": "GET",
    "headers": {
      "Accept": "application/json"
    },
    "schedule": "*/5 * * * *",
    "interval": "5m",
    "active": true,
    "last_fetched": "2025-12-04T10:25:00Z",
    "fetch_count": 1234,
    "error_count": 5,
    "created_at": "2025-12-04T10:00:00Z",
    "updated_at": "2025-12-04T10:25:00Z"
  }
}
```

**Status Codes**:
- `200 OK`: Feed found
- `404 Not Found`: Feed does not exist

---

### 5. List Data Feeds

List all configured data feeds.

**Endpoint**: `GET /oracle/feeds`

**Query Parameters**:
- `active` (boolean): Filter by active status
- `limit` (integer): Maximum number of results (default: 50)
- `offset` (integer): Pagination offset (default: 0)

**Response**:
```json
{
  "success": true,
  "data": {
    "feeds": [
      {
        "id": "feed-btc-usd",
        "name": "BTC Price Feed",
        "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
        "active": true,
        "last_fetched": "2025-12-04T10:25:00Z"
      },
      {
        "id": "feed-eth-usd",
        "name": "ETH Price Feed",
        "url": "https://api.coingecko.com/api/v3/simple/price?ids=ethereum&vs_currencies=usd",
        "active": true,
        "last_fetched": "2025-12-04T10:24:00Z"
      }
    ],
    "total": 2
  }
}
```

**Status Codes**:
- `200 OK`: Success

---

### 6. Update Data Feed

Update an existing data feed configuration.

**Endpoint**: `PUT /oracle/feeds/{feedId}`

**Request Body**:
```json
{
  "name": "BTC Price Feed (Updated)",
  "description": "Updated description",
  "active": true,
  "interval": "10m"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "feed-btc-usd",
    "name": "BTC Price Feed (Updated)",
    "updated_at": "2025-12-04T10:30:00Z"
  }
}
```

**Status Codes**:
- `200 OK`: Feed updated
- `400 Bad Request`: Invalid update data
- `404 Not Found`: Feed does not exist

---

### 7. Delete Data Feed

Delete a data feed.

**Endpoint**: `DELETE /oracle/feeds/{feedId}`

**Response**:
```json
{
  "success": true,
  "data": {
    "message": "Feed deleted successfully"
  }
}
```

**Status Codes**:
- `200 OK`: Feed deleted
- `404 Not Found`: Feed does not exist

---

### 8. Execute Feed

Manually trigger a data feed execution.

**Endpoint**: `POST /oracle/feeds/{feedId}/execute`

**Response**:
```json
{
  "success": true,
  "data": {
    "status_code": 200,
    "body": "{\"bitcoin\":{\"usd\":42000}}",
    "signature": "0x1234abcd...",
    "timestamp": "2025-12-04T10:30:00Z"
  }
}
```

**Status Codes**:
- `200 OK`: Execution successful
- `404 Not Found`: Feed does not exist
- `500 Internal Server Error`: Execution failed

---

### 9. Get Request Status

Check the status of a submitted oracle request.

**Endpoint**: `GET /oracle/requests/{requestId}`

**Response**:
```json
{
  "success": true,
  "data": {
    "request_id": "req-1701234567890",
    "status": "completed",
    "result": {
      "status_code": 200,
      "body": "{\"price\": 42000}",
      "signature": "0x1234abcd..."
    },
    "created_at": "2025-12-04T10:30:00Z",
    "completed_at": "2025-12-04T10:30:05Z"
  }
}
```

**Status Codes**:
- `200 OK`: Request found
- `404 Not Found`: Request does not exist

---

### 10. Publish to Supabase

Publish price data to configured Supabase instance.

**Endpoint**: `POST /oracle/supabase/publish`

**Request Body**:
```json
{
  "symbol": "BTC/USD",
  "price": 42000.50,
  "volume": 1234567890,
  "timestamp": "2025-12-04T10:30:00Z",
  "source": "coingecko"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "message": "Data published to Supabase",
    "table": "price_data"
  }
}
```

**Status Codes**:
- `200 OK`: Published successfully
- `400 Bad Request`: Invalid data
- `503 Service Unavailable`: Supabase not configured

---

## Data Types

### FetchRequest

```typescript
interface FetchRequest {
  url: string;              // Target URL
  method: string;           // HTTP method (GET, POST, PUT, DELETE)
  headers?: Record<string, string>;  // Custom headers
  body?: string;            // Request body (for POST/PUT)
  secret_name?: string;     // Secret reference for auth
  auth_type?: AuthType;     // Authentication type
}
```

### FetchResponse

```typescript
interface FetchResponse {
  status_code: number;      // HTTP status code
  headers?: Record<string, string>;  // Response headers
  body: string;             // Response body
  signature: string;        // TEE signature (hex)
  timestamp: string;        // ISO 8601 timestamp
}
```

### DataFeed

```typescript
interface DataFeed {
  id: string;               // Unique identifier
  name: string;             // Feed name
  description?: string;     // Description
  url: string;              // API endpoint
  method: string;           // HTTP method
  headers?: Record<string, string>;  // Custom headers
  secret_name?: string;     // Secret reference
  auth_type?: AuthType;     // Authentication type
  schedule?: string;        // Cron expression
  interval?: string;        // Polling interval (e.g., "5m")
  active: boolean;          // Active status
  last_fetched?: string;    // Last fetch timestamp
  fetch_count: number;      // Total fetch count
  error_count: number;      // Error count
  created_at: string;       // Creation timestamp
  updated_at: string;       // Last update timestamp
}
```

### AuthType

```typescript
enum AuthType {
  None = "none",
  Bearer = "bearer",
  APIKey = "apikey",
  Basic = "basic"
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
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource already exists
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error
- `503 Service Unavailable`: Service not available

## Rate Limiting

API requests are rate-limited to prevent abuse:

- **Default**: 100 requests per minute per API key
- **Burst**: Up to 200 requests in short bursts
- **Headers**: Rate limit info in response headers
  - `X-RateLimit-Limit`: Maximum requests per window
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Unix timestamp when limit resets

## Webhooks

The oracle service can send webhook notifications for feed updates:

### Webhook Payload

```json
{
  "event": "feed.updated",
  "feed_id": "feed-btc-usd",
  "timestamp": "2025-12-04T10:30:00Z",
  "data": {
    "status_code": 200,
    "body": "{\"bitcoin\":{\"usd\":42000}}",
    "signature": "0x1234abcd..."
  }
}
```

### Webhook Events

- `feed.updated`: Feed successfully fetched new data
- `feed.error`: Feed fetch failed
- `request.completed`: Oracle request completed
- `request.failed`: Oracle request failed

## SDK Examples

### JavaScript/TypeScript

```typescript
import { OracleClient } from '@r3e/service-layer-sdk';

const client = new OracleClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

// Submit oracle request
const request = await client.submitRequest({
  url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
  method: 'GET'
});

// Create data feed
const feed = await client.createFeed({
  name: 'BTC Price Feed',
  url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
  method: 'GET',
  interval: '5m'
});
```

### Go

```go
import "github.com/R3E-Network/service_layer/sdk/go/client"

client := client.NewOracleClient("http://localhost:8080/api", "your-api-key")

// Submit oracle request
req := &client.FetchRequest{
    URL:    "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
    Method: "GET",
}
resp, err := client.SubmitRequest(ctx, req)
```

## Testing

### Health Check

```bash
curl http://localhost:8080/api/health
```

### Submit Test Request

```bash
curl -X POST http://localhost:8080/api/oracle/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-token" \
  -d '{
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
    "method": "GET"
  }'
```
