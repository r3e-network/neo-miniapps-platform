# Oracle Service Usage Examples

## Table of Contents

1. [Basic HTTP Requests](#basic-http-requests)
2. [Price Feed Integration](#price-feed-integration)
3. [Weather Data Oracle](#weather-data-oracle)
4. [Authenticated API Requests](#authenticated-api-requests)
5. [JSON Path Extraction](#json-path-extraction)
6. [Smart Contract Integration](#smart-contract-integration)
7. [Data Feed Management](#data-feed-management)
8. [Supabase Integration](#supabase-integration)

---

## Basic HTTP Requests

### Simple GET Request

```bash
curl -X POST http://localhost:8080/api/oracle/request \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
    "method": "GET"
  }'
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

### POST Request with Body

```javascript
const axios = require('axios');

const response = await axios.post('http://localhost:8080/api/oracle/request', {
  url: 'https://api.example.com/data',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    query: 'bitcoin price',
    currency: 'USD'
  })
}, {
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN'
  }
});

console.log('Request ID:', response.data.data.request_id);
```

---

## Price Feed Integration

### Fetch Bitcoin Price

```typescript
import { OracleClient } from '@r3e/service-layer-sdk';

const client = new OracleClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

async function getBitcoinPrice() {
  const response = await client.fetch({
    url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
    method: 'GET'
  });

  const data = JSON.parse(response.body);
  console.log('BTC Price:', data.bitcoin.usd);
  console.log('TEE Signature:', response.signature);

  return data.bitcoin.usd;
}

getBitcoinPrice();
```

### Create Recurring Price Feed

```javascript
const feed = await client.createFeed({
  name: 'BTC/USD Price Feed',
  description: 'Bitcoin price from CoinGecko, updated every 5 minutes',
  url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
  method: 'GET',
  interval: '5m',
  headers: {
    'Accept': 'application/json'
  }
});

console.log('Feed created:', feed.id);
```

### Monitor Price Feed

```javascript
// List all feeds
const feeds = await client.listFeeds();

// Get specific feed
const btcFeed = await client.getFeed('feed-btc-usd');
console.log('Last price:', btcFeed.last_fetched);
console.log('Fetch count:', btcFeed.fetch_count);

// Execute feed manually
const result = await client.executeFeed('feed-btc-usd');
console.log('Current price:', JSON.parse(result.body));
```

---

## Weather Data Oracle

### Fetch Weather Information

```python
import requests
import json

def get_weather(city):
    response = requests.post(
        'http://localhost:8080/api/oracle/request',
        headers={
            'Authorization': 'Bearer YOUR_TOKEN',
            'Content-Type': 'application/json'
        },
        json={
            'url': f'https://api.openweathermap.org/data/2.5/weather?q={city}&appid=YOUR_API_KEY',
            'method': 'GET',
            'secret_name': 'openweather_api_key',
            'auth_type': 'apikey'
        }
    )

    return response.json()

# Request weather data
result = get_weather('London')
print(f"Request ID: {result['data']['request_id']}")
```

### Weather Feed with Callback

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/R3E-Network/service_layer/sdk/go/client"
)

func main() {
    client := client.NewOracleClient("http://localhost:8080/api", "your-api-key")

    // Create weather feed
    feed := &client.DataFeed{
        Name:        "London Weather",
        Description: "Weather data for London, updated hourly",
        URL:         "https://api.openweathermap.org/data/2.5/weather?q=London",
        Method:      "GET",
        SecretName:  "openweather_api_key",
        AuthType:    "apikey",
        Interval:    "1h",
    }

    created, err := client.CreateFeed(context.Background(), feed)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Weather feed created: %s\n", created.ID)
}
```

---

## Authenticated API Requests

### Bearer Token Authentication

```javascript
// Store API key as secret first
await secretsClient.storeSecret({
  key: 'my_api_token',
  value: 'sk-1234567890abcdef'
});

// Use secret in oracle request
const response = await oracleClient.fetch({
  url: 'https://api.example.com/protected/data',
  method: 'GET',
  secret_name: 'my_api_token',
  auth_type: 'bearer'
});
```

### API Key in Header

```typescript
interface APIKeyConfig {
  secret_name: string;
  auth_type: 'apikey';
  headers: {
    'X-API-Key': string; // Will be populated from secret
  };
}

const config: APIKeyConfig = {
  secret_name: 'exchange_api_key',
  auth_type: 'apikey',
  headers: {
    'X-API-Key': '${secret}' // Placeholder, replaced by service
  }
};

const response = await oracleClient.fetch({
  url: 'https://api.exchange.com/v1/ticker',
  method: 'GET',
  ...config
});
```

### Basic Authentication

```bash
# Store credentials
curl -X POST http://localhost:8080/api/secrets \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "key": "basic_auth_creds",
    "value": "username:password"
  }'

# Use in oracle request
curl -X POST http://localhost:8080/api/oracle/request \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "url": "https://api.example.com/data",
    "method": "GET",
    "secret_name": "basic_auth_creds",
    "auth_type": "basic"
  }'
```

---

## JSON Path Extraction

### Extract Specific Fields

```javascript
// Full response
const fullResponse = await client.fetch({
  url: 'https://api.coingecko.com/api/v3/coins/bitcoin',
  method: 'GET'
});
// Returns entire JSON object (large)

// Extract only price using JSON path
const priceOnly = await client.fetch({
  url: 'https://api.coingecko.com/api/v3/coins/bitcoin',
  method: 'GET',
  json_path: '$.market_data.current_price.usd'
});
// Returns: 42000.50
```

### Complex JSON Path Examples

```typescript
// Extract nested array element
const path1 = '$.data.prices[0].value';

// Extract multiple fields
const path2 = '$.data[*].price';

// Conditional extraction
const path3 = '$.data[?(@.symbol=="BTC")].price';

// Example usage
const response = await client.fetch({
  url: 'https://api.example.com/markets',
  method: 'GET',
  json_path: '$.data[?(@.symbol=="BTC")].price'
});

console.log('BTC Price:', response.body);
```

---

## Smart Contract Integration

### Neo N3 Contract Example

```csharp
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

public class PriceConsumer : SmartContract
{
    // Oracle contract address
    private static readonly UInt160 OracleContract = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    // Gateway contract address
    private static readonly UInt160 GatewayContract = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    public static void RequestBTCPrice()
    {
        // Prepare oracle request payload
        var requestData = new OracleRequestData
        {
            Url = "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
            Method = "GET",
            Headers = "{}",
            Body = "",
            JsonPath = "$.bitcoin.usd"
        };

        var payload = StdLib.Serialize(requestData);

        // Submit request via Gateway
        var requestId = (ByteString)Contract.Call(
            GatewayContract,
            "request",
            CallFlags.All,
            new object[] { "oracle", payload, Runtime.ExecutingScriptHash, "onPriceReceived" }
        );

        Runtime.Log($"Oracle request submitted: {requestId}");
    }

    // Callback method - receives oracle response
    public static void OnPriceReceived(ByteString requestId, bool success, ByteString data, BigInteger statusCode)
    {
        // Verify caller is Gateway
        if (Runtime.CallingScriptHash != GatewayContract)
        {
            throw new Exception("Unauthorized callback");
        }

        if (!success)
        {
            Runtime.Log($"Oracle request failed: {statusCode}");
            return;
        }

        // Parse price data
        BigInteger price = (BigInteger)StdLib.Deserialize(data);

        // Store price
        Storage.Put(Storage.CurrentContext, "btc_price", price);
        Storage.Put(Storage.CurrentContext, "btc_price_updated", Runtime.Time);

        Runtime.Log($"BTC Price updated: {price}");

        // Trigger price-dependent logic
        CheckPriceThreshold(price);
    }

    private static void CheckPriceThreshold(BigInteger price)
    {
        BigInteger threshold = 50000_00000000; // $50,000

        if (price > threshold)
        {
            Runtime.Log("Price threshold exceeded!");
            // Execute threshold logic
        }
    }

    public static BigInteger GetBTCPrice()
    {
        return (BigInteger)Storage.Get(Storage.CurrentContext, "btc_price");
    }
}

public class OracleRequestData
{
    public string Url;
    public string Method;
    public ByteString Headers;
    public ByteString Body;
    public string JsonPath;
}
```

### Monitoring Contract Events

```javascript
const { rpc } = require('@cityofzion/neon-js');

const client = new rpc.RPCClient('https://testnet.neo.org:443');

// Subscribe to OracleRequest events
async function monitorOracleRequests() {
  const blockHeight = await client.getBlockCount();

  // Get application logs for recent blocks
  for (let i = blockHeight - 10; i < blockHeight; i++) {
    const block = await client.getBlock(i);

    for (const tx of block.tx) {
      const log = await client.getApplicationLog(tx.hash);

      log.executions[0].notifications.forEach(notification => {
        if (notification.eventname === 'OracleRequest') {
          const [requestId, requester, url, method] = notification.state.value;
          console.log('New Oracle Request:', {
            requestId: requestId.value,
            requester: requester.value,
            url: url.value,
            method: method.value
          });
        }
      });
    }
  }
}

monitorOracleRequests();
```

---

## Data Feed Management

### Create Multiple Feeds

```typescript
const feeds = [
  {
    name: 'BTC/USD',
    url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
    interval: '5m'
  },
  {
    name: 'ETH/USD',
    url: 'https://api.coingecko.com/api/v3/simple/price?ids=ethereum&vs_currencies=usd',
    interval: '5m'
  },
  {
    name: 'NEO/USD',
    url: 'https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd',
    interval: '5m'
  }
];

for (const feedConfig of feeds) {
  const feed = await client.createFeed({
    name: feedConfig.name,
    url: feedConfig.url,
    method: 'GET',
    interval: feedConfig.interval
  });

  console.log(`Created feed: ${feed.name} (${feed.id})`);
}
```

### Update Feed Configuration

```javascript
// Pause a feed
await client.updateFeed('feed-btc-usd', {
  active: false
});

// Change update interval
await client.updateFeed('feed-btc-usd', {
  interval: '10m'
});

// Update URL
await client.updateFeed('feed-btc-usd', {
  url: 'https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT'
});
```

### Feed Statistics Dashboard

```typescript
async function displayFeedStats() {
  const feeds = await client.listFeeds();

  console.log('=== Oracle Feed Statistics ===\n');

  for (const feed of feeds.feeds) {
    const successRate = ((feed.fetch_count - feed.error_count) / feed.fetch_count * 100).toFixed(2);

    console.log(`Feed: ${feed.name}`);
    console.log(`  Status: ${feed.active ? 'Active' : 'Inactive'}`);
    console.log(`  Total Fetches: ${feed.fetch_count}`);
    console.log(`  Errors: ${feed.error_count}`);
    console.log(`  Success Rate: ${successRate}%`);
    console.log(`  Last Fetched: ${feed.last_fetched}`);
    console.log('');
  }
}

displayFeedStats();
```

---

## Supabase Integration

### Configure Supabase Publishing

```javascript
// Store Supabase configuration in sealed storage
const config = {
  project_url: 'https://your-project.supabase.co',
  api_key_secret: 'supabase_api_key',
  table: 'oracle_prices',
  allowed_hosts: ['*.supabase.co'],
  default_headers: {
    'Content-Type': 'application/json',
    'Prefer': 'return=representation'
  }
};

// Configuration is loaded automatically by service from sealed storage
```

### Publish Price Data

```typescript
interface PriceData {
  symbol: string;
  price: number;
  volume?: number;
  timestamp: string;
  source: string;
}

async function publishPrice(data: PriceData) {
  const response = await fetch('http://localhost:8080/api/oracle/supabase/publish', {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer YOUR_TOKEN',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(data)
  });

  return response.json();
}

// Publish BTC price
await publishPrice({
  symbol: 'BTC/USD',
  price: 42000.50,
  volume: 1234567890,
  timestamp: new Date().toISOString(),
  source: 'coingecko'
});
```

### Query Published Data

```sql
-- Query recent prices from Supabase
SELECT * FROM oracle_prices
WHERE symbol = 'BTC/USD'
ORDER BY timestamp DESC
LIMIT 100;

-- Calculate average price
SELECT
  symbol,
  AVG(price) as avg_price,
  MIN(price) as min_price,
  MAX(price) as max_price
FROM oracle_prices
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY symbol;
```

---

## Error Handling

### Retry Logic

```typescript
async function fetchWithRetry(url: string, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await client.fetch({
        url,
        method: 'GET'
      });

      if (response.status_code === 200) {
        return response;
      }

      console.log(`Attempt ${i + 1} failed with status ${response.status_code}`);
    } catch (error) {
      console.error(`Attempt ${i + 1} error:`, error);

      if (i === maxRetries - 1) {
        throw error;
      }

      // Exponential backoff
      await new Promise(resolve => setTimeout(resolve, Math.pow(2, i) * 1000));
    }
  }
}
```

### Timeout Handling

```javascript
async function fetchWithTimeout(url, timeoutMs = 10000) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch('http://localhost:8080/api/oracle/request', {
      method: 'POST',
      headers: {
        'Authorization': 'Bearer YOUR_TOKEN',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ url, method: 'GET' }),
      signal: controller.signal
    });

    return await response.json();
  } catch (error) {
    if (error.name === 'AbortError') {
      console.error('Request timeout');
    }
    throw error;
  } finally {
    clearTimeout(timeout);
  }
}
```

---

## Testing

### Unit Test Example

```typescript
import { describe, it, expect } from '@jest/globals';
import { OracleClient } from '@r3e/service-layer-sdk';

describe('Oracle Service', () => {
  const client = new OracleClient({
    baseURL: 'http://localhost:8080/api',
    apiKey: 'test-api-key'
  });

  it('should fetch Bitcoin price', async () => {
    const response = await client.fetch({
      url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
      method: 'GET'
    });

    expect(response.status_code).toBe(200);
    expect(response.body).toBeDefined();
    expect(response.signature).toBeDefined();

    const data = JSON.parse(response.body);
    expect(data.bitcoin.usd).toBeGreaterThan(0);
  });

  it('should create and execute feed', async () => {
    const feed = await client.createFeed({
      name: 'Test Feed',
      url: 'https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd',
      method: 'GET',
      interval: '5m'
    });

    expect(feed.id).toBeDefined();
    expect(feed.active).toBe(true);

    const result = await client.executeFeed(feed.id);
    expect(result.status_code).toBe(200);
  });
});
```

---

## Performance Optimization

### Batch Requests

```javascript
async function fetchMultiplePrices(symbols) {
  const requests = symbols.map(symbol =>
    client.fetch({
      url: `https://api.coingecko.com/api/v3/simple/price?ids=${symbol}&vs_currencies=usd`,
      method: 'GET'
    })
  );

  const responses = await Promise.all(requests);

  return responses.map((response, index) => ({
    symbol: symbols[index],
    price: JSON.parse(response.body)[symbols[index]].usd,
    signature: response.signature
  }));
}

const prices = await fetchMultiplePrices(['bitcoin', 'ethereum', 'neo']);
console.log(prices);
```

### Caching Strategy

```typescript
class CachedOracleClient {
  private cache = new Map<string, { data: any, timestamp: number }>();
  private cacheTTL = 60000; // 1 minute

  async fetch(url: string) {
    const cached = this.cache.get(url);

    if (cached && Date.now() - cached.timestamp < this.cacheTTL) {
      console.log('Cache hit:', url);
      return cached.data;
    }

    console.log('Cache miss:', url);
    const response = await client.fetch({ url, method: 'GET' });

    this.cache.set(url, {
      data: response,
      timestamp: Date.now()
    });

    return response;
  }
}
```

---

## Additional Resources

- [Oracle Service README](./README.md)
- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
- [Neo N3 Documentation](https://docs.neo.org)
