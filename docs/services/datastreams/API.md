# datastreams Service API Documentation

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

### 1. Service Status

Get service status and health information.

**Endpoint**: `GET /services/datastreams/status`

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "datastreams",
    "version": "1.0.0",
    "state": "running",
    "healthy": true
  }
}
```

### 2. Submit Request

Submit a request to the datastreams service.

**Endpoint**: `POST /datastreams/request`

**Request Body**:
```json
{
  "account_id": "user123",
  "parameters": {}
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "request_id": "req-1701234567890",
    "status": "pending"
  }
}
```

## Error Responses

```json
{
  "success": false,
  "error": "Error message"
}
```

## Rate Limiting

- Default: 100 requests per minute per API key
- Burst: Up to 200 requests in short bursts

## SDK Examples

### JavaScript/TypeScript

```typescript
import { DatastreamsClient } from '@r3e/service-layer-sdk';

const client = new DatastreamsClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

const result = await client.submitRequest({
  account_id: 'user123',
  parameters: {}
});
```

### Go

```go
import "github.com/R3E-Network/service_layer/sdk/go/client"

client := client.NewDatastreamsClient("http://localhost:8080/api", "your-api-key")
result, err := client.SubmitRequest(ctx, &client.Request{
    AccountID: "user123",
})
```
