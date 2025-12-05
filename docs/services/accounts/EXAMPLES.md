# accounts Service Usage Examples

## Table of Contents

1. [Basic Usage](#basic-usage)
2. [Smart Contract Integration](#smart-contract-integration)
3. [Advanced Examples](#advanced-examples)

---

## Basic Usage

### Simple Request

```bash
curl -X POST http://localhost:8080/api/accounts/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "account_id": "user123",
    "parameters": {}
  }'
```

### JavaScript/TypeScript

```typescript
import { AccountsClient } from '@r3e/service-layer-sdk';

const client = new AccountsClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

async function submitRequest() {
  const result = await client.submitRequest({
    account_id: 'user123',
    parameters: {}
  });

  console.log('Request ID:', result.request_id);
  console.log('Status:', result.status);

  return result;
}

submitRequest();
```

### Go

```go
package main

import (
    "context"
    "fmt"

    "github.com/R3E-Network/service_layer/sdk/go/client"
)

func main() {
    client := client.NewAccountsClient(
        "http://localhost:8080/api",
        "your-api-key"
    )

    result, err := client.SubmitRequest(context.Background(), &client.Request{
        AccountID: "user123",
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Request ID: %s\n", result.RequestID)
}
```

---

## Smart Contract Integration

### Neo N3 Contract Example

```csharp
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

public class MyContract : SmartContract
{
    private static readonly UInt160 AccountsService = 
        "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    public static void UseService()
    {
        var payload = StdLib.Serialize(new RequestData {
            // Request parameters
        });

        var requestId = (ByteString)Contract.Call(
            AccountsService,
            "processRequest",
            CallFlags.All,
            new object[] { payload, Runtime.ExecutingScriptHash, "onResponse" }
        );

        Runtime.Log($"Request submitted: {requestId}");
    }

    public static void OnResponse(
        ByteString requestId,
        bool success,
        ByteString data
    )
    {
        if (!success) {
            Runtime.Log("Request failed");
            return;
        }

        // Process response data
        var result = StdLib.Deserialize(data);
        Storage.Put(Storage.CurrentContext, "result", result);
    }
}
```

---

## Advanced Examples

### Error Handling

```typescript
async function submitWithRetry(maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const result = await client.submitRequest({
        account_id: 'user123',
        parameters: {}
      });

      return result;
    } catch (error) {
      console.error(`Attempt ${i + 1} failed:`, error);

      if (i === maxRetries - 1) {
        throw error;
      }

      await new Promise(resolve => 
        setTimeout(resolve, Math.pow(2, i) * 1000)
      );
    }
  }
}
```

### Testing

```typescript
import { describe, it, expect } from '@jest/globals';

describe('accounts Service', () => {
  it('should submit request successfully', async () => {
    const result = await client.submitRequest({
      account_id: 'test_user',
      parameters: {}
    });

    expect(result.request_id).toBeDefined();
    expect(result.status).toBe('pending');
  });
});
```

---

## Additional Resources

- [accounts Service README](./README.md)
- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
