# NeoFeeds Service

Price oracle service for the Neo Service Layer.

## Overview

The NeoFeeds service provides aggregated price feeds from multiple sources with TEE attestation. It fetches prices from Chainlink (Arbitrum) and Binance, aggregates them using median calculation, and signs the results with the TEE key. When configured, it anchors updates to the platform `PriceFeed` contract on Neo N3 via `txproxy` (allowlisted sign+broadcast) using the `≥0.1%` publish policy.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Price Sources│     │ NeoFeeds Svc │     │ PriceFeed    │
│ (Chainlink,  │     │ (TEE)        │     │ (Neo N3)     │
│  Binance)    │     │              │     │ Contract     │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Fetch Prices       │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Price Data         │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Aggregate & Sign   │
       │                    │                    │
       │                    │ update()           │
       │                    │───────────────────>│
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service configuration |
| `/price/{pair}` | GET | Get single price |
| `/prices` | GET | Get all prices |
| `/feeds` | GET | List available feeds |
| `/sources` | GET | List data sources |
| `/config` | GET | Get full configuration |

## Supported Feeds

| Feed ID | Base | Quote | Decimals |
|---------|------|-------|----------|
| BTC-USD | BTC | USD | 8 |
| ETH-USD | ETH | USD | 8 |
| NEO-USD | NEO | USD | 8 |
| GAS-USD | GAS | USD | 8 |
| NEO-GAS | NEO | GAS | 8 |

## Data Sources

| Source | Type | Priority | Weight |
|--------|------|----------|--------|
| Chainlink (Arbitrum) | On-chain | 1 (Primary) | 3 |
| Binance | HTTP API | 2 (Fallback) | 1 |

## Request/Response Types

### Get Price

```json
GET /price/BTC-USD

{
    "feed_id": "BTC-USD",
    "price": 10500000000000,
    "decimals": 8,
    "timestamp": 1733616000,
    "signature": "0x...",
    "public_key": "0x...",
    "sources": ["chainlink", "binance"]
}
```

### Get All Prices

```json
GET /prices

[
    {"feed_id": "BTC-USD", "price": 10500000000000, "decimals": 8, "...": "..."},
    {"feed_id": "ETH-USD", "price": 380000000000, "decimals": 8, "...": "..."}
]
```

## Configuration

### YAML Configuration

```yaml
update_interval: 5s
publish_policy:
  threshold_bps: 10
  hysteresis_bps: 8
  min_interval: 3s
  max_per_minute: 30
feeds:
  - id: "BTC-USD"
    pair: "BTCUSDT"
    base: "btc"
    quote: "usd"
    decimals: 8
    enabled: true
    sources: ["binance"]
sources:
  - id: "binance"
    name: "Binance"
    url: "https://api.binance.com/api/v3/ticker/price?symbol={pair}"
    json_path: "price"
    weight: 1
```

### Required Secrets

| Secret | Description |
|--------|-------------|
| `NEOFEEDS_SIGNING_KEY` | ECDSA key for signing prices |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ARBITRUM_RPC` | Arbitrum RPC URL for Chainlink |

## Aggregation Algorithm

1. Fetch prices from all enabled sources
2. Filter out stale prices (> 5 minutes old)
3. Calculate weighted median
4. Sign result with TEE key
5. Cache for configured interval

## Testing

```bash
go test ./services/datafeed/... -v -cover
```

Current test coverage: **61.5%**

## Version

- Service ID: `neofeeds`
- Version: `3.0.0`
