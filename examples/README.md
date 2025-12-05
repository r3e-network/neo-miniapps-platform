# Neo Service Layer Examples

Code examples demonstrating how to use the Neo Service Layer services.

## Prerequisites

```bash
# Set environment variables
export SUPABASE_URL=http://localhost:54321
export SUPABASE_ANON_KEY=your-anon-key

# For TypeScript examples
npm install @supabase/supabase-js typescript ts-node
```

## Go Examples

### Oracle Service

Fetch external data with TEE protection:

```bash
cd go
go run oracle_example.go
```

### VRF Service

Generate verifiable random numbers:

```bash
cd go
go run vrf_example.go
```

## TypeScript Examples

### Oracle Service

```bash
cd typescript
npx ts-node oracle_example.ts
```

## Quick Start Code

### Go - Submit Service Request

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
)

func main() {
    request := map[string]interface{}{
        "service_type": "oracle",
        "operation":    "fetch",
        "payload": map[string]string{
            "url": "https://api.example.com/data",
        },
    }

    body, _ := json.Marshal(request)
    req, _ := http.NewRequest("POST",
        "http://localhost:54321/rest/v1/service_requests",
        bytes.NewReader(body))

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("apikey", "your-key")
    req.Header.Set("Authorization", "Bearer your-key")
    req.Header.Set("Prefer", "return=representation")

    resp, _ := http.DefaultClient.Do(req)
    defer resp.Body.Close()
    // Handle response...
}
```

### TypeScript - Submit Service Request

```typescript
import { createClient } from '@supabase/supabase-js';

const supabase = createClient(
  'http://localhost:54321',
  'your-anon-key'
);

async function submitRequest() {
  const { data, error } = await supabase
    .from('service_requests')
    .insert({
      service_type: 'oracle',
      operation: 'fetch',
      payload: { url: 'https://api.example.com/data' }
    })
    .select()
    .single();

  console.log('Request ID:', data.id);
}
```

### Real-time Updates

```typescript
// Subscribe to request updates
supabase
  .channel('my-request')
  .on('postgres_changes', {
    event: 'UPDATE',
    schema: 'public',
    table: 'service_requests',
    filter: `id=eq.${requestId}`
  }, (payload) => {
    console.log('Status:', payload.new.status);
    if (payload.new.status === 'completed') {
      console.log('Result:', payload.new.result);
    }
  })
  .subscribe();
```

## Service Operations

### Oracle

| Operation | Description | Payload |
|-----------|-------------|---------|
| `fetch` | Fetch URL data | `{url, method?, headers?, body?, json_path?}` |
| `callback` | Fetch with contract callback | `{url, callback_hash}` |

### VRF

| Operation | Description | Payload |
|-----------|-------------|---------|
| `random` | Generate random | `{seed, num_values?}` |
| `verify` | Verify proof | `{randomness, proof, seed}` |

### Secrets

| Operation | Description | Payload |
|-----------|-------------|---------|
| `store` | Store secret | `{key, value}` |
| `get` | Get secret | `{key}` |
| `delete` | Delete secret | `{key}` |
| `list` | List secrets | `{}` |

### GasBank

| Operation | Description | Payload |
|-----------|-------------|---------|
| `balance` | Check balance | `{account}` |
| `deposit` | Deposit GAS | `{account, amount}` |
| `withdraw` | Withdraw GAS | `{account, amount}` |

## Response Format

All service requests follow this response format:

```json
{
  "id": "uuid",
  "service_type": "oracle",
  "operation": "fetch",
  "status": "completed",
  "result": {
    "data": "...",
    "tee_signature": "..."
  },
  "created_at": "2025-12-04T00:00:00Z",
  "completed_at": "2025-12-04T00:00:01Z"
}
```

## Error Handling

```typescript
const result = await waitForCompletion(requestId);

if (result.status === 'failed') {
  console.error('Error:', result.error_message);
  // Handle error...
} else {
  console.log('Success:', result.result);
}
```

## More Examples

- [Service Documentation](../docs/services/README.md)
- [API Reference](../docs/README.md)
