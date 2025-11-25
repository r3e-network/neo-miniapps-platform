# Price Feeds Quickstart

This guide covers the Price Feed service - a decentralized oracle aggregation system for asset price data with deviation-based publishing, heartbeat intervals, and multi-source observation aggregation.

## Overview

The Price Feed service provides:
- **Feed Management**: Define asset pairs (e.g., NEO/USD, BTC/ETH) with configurable update intervals
- **Deviation-Based Publishing**: Only publish new prices when they deviate beyond a threshold
- **Heartbeat Intervals**: Guarantee fresh data even without price changes
- **Multi-Source Aggregation**: Collect observations from multiple providers and compute median
- **Round-Based History**: Track price rounds with full observation lineage

## Prerequisites

- Service Layer running (e.g., `go run ./cmd/appserver`)
- API token exported as `SERVICE_LAYER_TOKEN`
- `slctl` available (`go run ./cmd/slctl ...`)

```bash
export TOKEN=dev-token
export TENANT=tenant-a  # omit only if your account is unscoped
export API=http://localhost:8080
```

## Quick Start

### 1. Create an Account

```bash
# Create account with tenant
ACCOUNT_ID=$(curl -s -X POST $API/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"owner":"price-oracle","metadata":{"tenant":"'"$TENANT"'"}}' | jq -r .id)

echo "Account ID: $ACCOUNT_ID"
```

### 2. Create a Price Feed

```bash
# Create NEO/USD price feed with:
# - 1% deviation threshold (only publish when price changes by 1%+)
# - 5 minute update interval
# - 1 hour heartbeat (force publish every hour even without deviation)
FEED_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/pricefeeds \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{
    "base_asset": "NEO",
    "quote_asset": "USD",
    "deviation_percent": 1.0,
    "update_interval": "@every 5m",
    "heartbeat_interval": "@every 1h"
  }' | jq -r .ID)

echo "Feed ID: $FEED_ID"
```

### 3. Submit Price Observations

```bash
# Submit a price observation (creates/updates a round)
curl -s -X POST $API/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{
    "price": 12.34,
    "source": "binance",
    "collected_at": "2025-01-15T10:30:00Z"
  }'
```

### 4. List Price Snapshots

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  "$API/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots" | jq
```

## API Reference

### Feed Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/accounts/{account}/pricefeeds` | List all price feeds |
| POST | `/accounts/{account}/pricefeeds` | Create a new price feed |
| GET | `/accounts/{account}/pricefeeds/{feed}` | Get feed details |
| PATCH | `/accounts/{account}/pricefeeds/{feed}` | Update feed settings |
| GET | `/accounts/{account}/pricefeeds/{feed}/snapshots` | List price snapshots |
| POST | `/accounts/{account}/pricefeeds/{feed}/snapshots` | Submit price observation |

### Create Feed Request

```json
{
  "base_asset": "NEO",
  "quote_asset": "USD",
  "deviation_percent": 1.0,
  "update_interval": "@every 5m",
  "heartbeat_interval": "@every 1h"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `base_asset` | string | Yes | Base asset symbol (e.g., "NEO", "BTC") |
| `quote_asset` | string | Yes | Quote asset symbol (e.g., "USD", "ETH") |
| `deviation_percent` | float | Yes | Minimum price change % to trigger publish |
| `update_interval` | string | No | Cron-style interval (default: "@every 1m") |
| `heartbeat_interval` | string | No | Max time between updates (default: "@every 10m") |

### Update Feed Request (PATCH)

```json
{
  "update_interval": "@every 10m",
  "heartbeat_interval": "@every 2h",
  "deviation_percent": 0.5,
  "active": false
}
```

All fields are optional. Only provided fields are updated.

### Submit Observation Request

```json
{
  "price": 12.34,
  "source": "binance",
  "collected_at": "2025-01-15T10:30:00Z"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `price` | float | Yes | Price value (must be positive) |
| `source` | string | No | Data source identifier (default: "manual") |
| `collected_at` | string | No | RFC3339 timestamp (default: now) |

### Response Models

#### Feed

```json
{
  "ID": "feed-uuid",
  "AccountID": "account-uuid",
  "BaseAsset": "NEO",
  "QuoteAsset": "USD",
  "Pair": "NEO/USD",
  "UpdateInterval": "@every 5m",
  "DeviationPercent": 1.0,
  "Heartbeat": "@every 1h",
  "Active": true,
  "CreatedAt": "2025-01-15T10:00:00Z",
  "UpdatedAt": "2025-01-15T10:00:00Z"
}
```

#### Snapshot

```json
{
  "ID": "snap-uuid",
  "FeedID": "feed-uuid",
  "Price": 12.34,
  "Source": "binance",
  "CollectedAt": "2025-01-15T10:30:00Z",
  "CreatedAt": "2025-01-15T10:30:01Z"
}
```

#### Round

```json
{
  "ID": "round-uuid",
  "FeedID": "feed-uuid",
  "RoundID": 1,
  "AggregatedPrice": 12.34,
  "ObservationCount": 3,
  "StartedAt": "2025-01-15T10:30:00Z",
  "ClosedAt": "2025-01-15T10:35:00Z",
  "Finalized": true,
  "CreatedAt": "2025-01-15T10:30:00Z"
}
```

## CLI Examples

```bash
# List all feeds for an account
slctl pricefeeds list --account $ACCOUNT_ID --token $TOKEN --tenant $TENANT

# Get feed details
slctl pricefeeds get --account $ACCOUNT_ID --feed $FEED_ID --token $TOKEN

# List snapshots
slctl pricefeeds snapshots --account $ACCOUNT_ID --feed $FEED_ID --token $TOKEN --limit 10
```

## Advanced Usage

### Multi-Provider Aggregation

The service supports collecting observations from multiple providers before finalizing a round. Configure minimum submissions via the service:

```go
// In Go code
svc := pricefeed.New(store, store, log)
svc.SetMinimumSubmissions(3) // Require 3 observations before finalizing
```

With this configuration:
1. First 2 observations are collected but round stays pending
2. Third observation triggers median calculation
3. Round finalizes if deviation threshold is met

### Deviation Gate Example

```bash
# Initial price: round 1 finalizes immediately
curl -s -X POST $API/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"price": 100.00, "source": "oracle-a"}'

# Small change (0.2%): deviation gate prevents publish
curl -s -X POST $API/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"price": 100.20, "source": "oracle-b"}'

# Large change (10%): exceeds 1% threshold, round finalizes
curl -s -X POST $API/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"price": 110.00, "source": "oracle-c"}'
```

### Heartbeat-Based Updates

Even without deviation, the heartbeat ensures regular updates:

```
Timeline (1% deviation, 1h heartbeat):
10:00 - Price 100.00, Round 1 published
10:15 - Price 100.50, deviation 0.5% < 1%, deferred
10:30 - Price 100.80, deviation 0.8% < 1%, deferred
11:00 - Heartbeat triggers, Round 2 published with 100.80
```

### External Price Fetcher

Configure an HTTP fetcher for automated price collection:

```bash
# Environment variables
export PRICEFEED_FETCH_URL=https://api.prices.example.com/v1/price
export PRICEFEED_FETCH_KEY=your-api-key
```

The fetcher expects responses in this format:
```json
{
  "price": 12.34,
  "source": "example-exchange"
}
```

Query parameters `?base=NEO&quote=USD` are appended automatically.

### Event-Driven Observations

Submit observations via the engine event bus:

```bash
curl -s -X POST $API/system/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "event": "observation",
    "payload": {
      "feed_id": "'"$FEED_ID"'",
      "price": 12.50,
      "source": "event-bus"
    }
  }'
```

## Devpack Integration

Use price feeds in serverless functions:

```typescript
// In a Devpack function
async function handler(params: any, secrets: any) {
  // Read latest price (via oracle action)
  const priceAction = Devpack.oracle.request({
    dataSourceId: params.sourceId,
    payload: JSON.stringify({ pair: "NEO/USD" })
  });

  // Submit to price feed via HTTP action
  const submitAction = Devpack.http.request({
    url: `${params.apiUrl}/accounts/${params.accountId}/pricefeeds/${params.feedId}/snapshots`,
    method: "POST",
    headers: {
      "Authorization": `Bearer ${secrets.apiToken}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      price: priceAction.result.price,
      source: "devpack-oracle"
    })
  });

  return Devpack.respond.success({ submitted: true });
}
```

## Health & Monitoring

Check service health:

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "$API/system/status" | jq '.modules[] | select(.name == "pricefeed")'
```

The pricefeed service reports:
- `status`: "running" | "stopped"
- `ready`: boolean
- `uptime_ms`: milliseconds since start
- `interfaces`: ["event"] (supports event publishing)

## Error Handling

| HTTP Status | Error | Resolution |
|-------------|-------|------------|
| 400 | "price must be positive" | Ensure price > 0 |
| 400 | "deviation_percent must be positive" | Set deviation > 0 |
| 400 | "price feed for pair X already exists" | Use existing feed or different pair |
| 403 | Feed ownership mismatch | Use correct account ID |
| 404 | Feed not found | Verify feed ID exists |
| 501 | "price feed service not configured" | Enable service in config |

## Best Practices

1. **Set Appropriate Deviation**: Balance between update frequency and gas costs
2. **Use Heartbeats**: Ensure stale data is refreshed even during stable periods
3. **Multiple Sources**: Configure minimum submissions for decentralization
4. **Monitor Rounds**: Track finalized vs pending rounds for data freshness
5. **Validate Sources**: Use trusted oracle providers for production feeds

## Related Documentation

- [Data Feeds Quickstart](datafeeds.md) - Chainlink-style signed data feeds
- [Oracle Service](services.md#oracle-http-adapter) - HTTP data adapters
- [Engine Bus](bus.md) - Event-driven architecture
