# NeoFeeds Marble Service

TEE-secured price feed aggregation service running inside MarbleRun enclave.

## Overview

The NeoFeeds Marble service is the core TEE component that:
1. Fetches prices from multiple external sources (Chainlink, Binance)
2. Aggregates prices using weighted median calculation
3. Signs aggregated data with TEE-attested keys
4. Pushes updates to the NeoFeedsService smart contract on Neo N3

This service implements the **Push/Auto-Update Pattern** - the TEE proactively pushes price updates on-chain rather than responding to on-chain requests.

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                     │
│                                                                │
│  ┌─────────────┐    ┌─────────────┐     ┌─────────────┐        │
│  │ Chainlink   │    │  Service    │     │  Fulfiller  │        │
│  │   Client    │───>│    Core     │────>│   (Push)    │        │
│  └─────────────┘    └─────────────┘     └──────┬──────┘        │
│                            │                   │               │
│  ┌─────────────┐           │                   │               │
│  │   Binance   │───────────┘                   │               │
│  │   Client    │                               │               │
│  └─────────────┘                               │               │
└────────────────────────────────────────────────┼───────────────┘
                                                 │
                                    ┌────────────▼────────────┐
                                    │  NeoFeedsService.cs     │
                                    │    (Neo N3 Contract)    │
                                    └─────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization and configuration |
| `core.go` | Price fetching and aggregation logic |
| `chainlink.go` | Chainlink Arbitrum price feed client |
| `fulfiller.go` | On-chain price push implementation |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `config.go` | YAML/JSON configuration loading |
| `types.go` | Data structures and request/response types |

Lifecycle is handled by the shared `commonservice.BaseService` (start/stop hooks, workers, standard routes).

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    httpClient      *http.Client
    signingKey      []byte
    chainlinkClient *ChainlinkClient
    config          *NeoFeedsConfig
    sources         map[string]*SourceConfig
    chainClient     *chain.Client
    teeFulfiller    *chain.TEEFulfiller
    neoFeedsHash    string
    updateInterval  time.Duration
    enableChainPush bool
}
```

### Configuration

The service supports three configuration methods:
1. **Direct Config**: Pass `FeedsConfig` struct programmatically
2. **Config File**: Load from YAML/JSON file via `ConfigFile` path
3. **Default Config**: Built-in defaults for standard feeds

```go
type Config struct {
    Marble          *marble.Marble
    DB              database.RepositoryInterface
    ConfigFile      string           // Optional YAML/JSON path
    FeedsConfig     *NeoFeedsConfig  // Optional direct config
    ArbitrumRPC     string           // Chainlink RPC URL
    ChainClient     *chain.Client
    TEEFulfiller    *chain.TEEFulfiller
    NeoFeedsHash    string           // Contract hash
    UpdateInterval  time.Duration    // Push interval
    EnableChainPush bool             // Enable auto-push
}
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check (from BaseService) |
| `/info` | GET | Service info with statistics |
| `/price/{pair}` | GET | Get single price (e.g., `/price/BTC/USD`) |
| `/prices` | GET | Get all prices |
| `/feeds` | GET | List available price feeds |
| `/sources` | GET | List configured data sources |
| `/config` | GET | Get full configuration |

## Data Flow

### Price Aggregation Flow

1. **Fetch Phase**
   - Query Chainlink price feeds on Arbitrum
   - Query Binance API for spot prices
   - Collect responses with timestamps

2. **Aggregation Phase**
   - Filter stale prices (> 5 minutes old)
   - Apply source weights (Chainlink: 3, Binance: 1)
   - Calculate weighted median

3. **Signing Phase**
   - Serialize price data
   - Sign with TEE-attested ECDSA key
   - Include signature in response

4. **Push Phase** (if enabled)
   - Build UpdatePrice transaction
   - Sign with TEE fulfiller account
   - Submit to Neo N3 network

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/chain` | Neo N3 blockchain interaction |
| `internal/marble` | MarbleRun TEE utilities |
| `internal/database` | Repository interface (optional) |
| `services/common/service` | Base service implementation |

### External Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/ethereum/go-ethereum` | Chainlink ABI decoding |
| `github.com/gorilla/mux` | HTTP routing |

## Configuration Example

```yaml
# neofeeds.yaml
update_interval: 60s
feeds:
  - id: "BTC/USD"
    pair: "BTCUSDT"
    base: "btc"
    quote: "usd"
    decimals: 8
    enabled: true
    sources: ["binance", "chainlink"]
  - id: "ETH/USD"
    pair: "ETHUSDT"
    base: "eth"
    quote: "usd"
    decimals: 8
    enabled: true
    sources: ["binance", "chainlink"]

sources:
  - id: "binance"
    name: "Binance"
    type: "http"
    url: "https://api.binance.com/api/v3/ticker/price?symbol={pair}"
    json_path: "price"
    weight: 1
  - id: "chainlink"
    name: "Chainlink"
    type: "chainlink"
    weight: 3
```

## Required Secrets

| Secret Name | Description |
|-------------|-------------|
| `NEOFEEDS_SIGNING_KEY` | ECDSA private key for signing prices |

Secrets are retrieved from the Marble enclave:
```go
if key, ok := cfg.Marble.Secret("NEOFEEDS_SIGNING_KEY"); ok {
    s.signingKey = key
}
```

## Background Workers

### Chain Push Loop

When `EnableChainPush` is true, the service runs a background worker:

```go
func (s *Service) runChainPushLoop(ctx context.Context) error {
    ticker := time.NewTicker(s.updateInterval)
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            s.pushAllPrices(ctx)
        }
    }
}
```

## Testing

```bash
# Run unit tests
go test ./services/neofeeds/marble/... -v

# Run with coverage
go test ./services/neofeeds/marble/... -v -cover

# Run specific test
go test ./services/neofeeds/marble/... -run TestGetPrice -v
```

## Related Documentation

- [NeoFeeds Service Overview](../README.md)
- [Chain Integration](../chain/README.md)
- [Smart Contract](../contract/README.md)
- [Common Service Base](../../common/service/README.md)
