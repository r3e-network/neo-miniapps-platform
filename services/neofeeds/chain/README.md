# NeoFeeds Chain Integration

Neo N3 blockchain integration for the NeoFeeds price feed service.

## Overview

This package provides Go bindings for interacting with the `NeoFeedsService` smart contract on Neo N3. It enables:
- Reading price data from on-chain storage
- Parsing contract events (PriceUpdated, FeedRegistered, etc.)
- Invoking contract methods through the TEE

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    NeoFeeds Chain Package                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────┐         ┌─────────────────┐           │
│  │ NeoFeedsContract│         │  Event Parsers  │           │
│  ├─────────────────┤         ├─────────────────┤           │
│  │ GetLatestPrice  │         │ PriceUpdated    │           │
│  │ GetPrice        │         │ FeedRegistered  │           │
│  │ GetTimestamp    │         │ FeedDeactivated │           │
│  │ IsPriceFresh    │         │ TEERegistered   │           │
│  │ GetFeedConfig   │         └─────────────────┘           │
│  │ IsTEEAccount    │                                       │
│  └────────┬────────┘                                       │
│           │                                                 │
└───────────┼─────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/chain                            │
│  (Client, ContractParam, InvokeResult, TEEFulfiller)        │
└─────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Neo N3 Network                            │
│              (NeoFeedsService Contract)                      │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `contract.go` | Contract method invocations |
| `events.go` | Event parsing utilities |

## Contract Interface

### NeoFeedsContract

The main contract wrapper for interacting with NeoFeedsService:

```go
type NeoFeedsContract struct {
    client       *chain.Client
    contractHash string
    wallet       *chain.Wallet
}
```

### Methods

#### GetLatestPrice

Retrieves the latest price data for a feed including all metadata.

```go
func (d *NeoFeedsContract) GetLatestPrice(ctx context.Context, feedID string) (*chain.PriceData, error)
```

**Returns**: `PriceData` struct containing:
- `FeedId` - Feed identifier (e.g., "BTC/USD")
- `Price` - Price value as big.Int
- `Decimals` - Decimal precision
- `Timestamp` - Update timestamp
- `UpdatedBy` - TEE account that pushed the update

#### GetPrice

Returns only the raw price value for simple integrations.

```go
func (d *NeoFeedsContract) GetPrice(ctx context.Context, feedID string) (*big.Int, error)
```

#### GetPriceTimestamp

Returns the timestamp of the latest price update.

```go
func (d *NeoFeedsContract) GetPriceTimestamp(ctx context.Context, feedID string) (uint64, error)
```

#### IsPriceFresh

Checks if the price is within the staleness threshold (1 hour).

```go
func (d *NeoFeedsContract) IsPriceFresh(ctx context.Context, feedID string) (bool, error)
```

#### GetFeedConfig

Returns configuration for a price feed.

```go
func (d *NeoFeedsContract) GetFeedConfig(ctx context.Context, feedID string) (*chain.ContractFeedConfig, error)
```

#### IsTEEAccount

Checks if an account is registered as a TEE account.

```go
func (d *NeoFeedsContract) IsTEEAccount(ctx context.Context, account string) (bool, error)
```

## Event Parsers

### PriceUpdated Event

Emitted when a price is updated on-chain.

```go
type NeoFeedsPriceUpdatedEvent struct {
    FeedID    string
    Price     uint64
    Decimals  uint64
    Timestamp uint64
}

func ParseNeoFeedsPriceUpdatedEvent(event *chain.ContractEvent) (*NeoFeedsPriceUpdatedEvent, error)
```

### FeedRegistered Event

Emitted when a new price feed is registered.

```go
type NeoFeedsFeedRegisteredEvent struct {
    FeedID      string
    Description string
    Decimals    uint64
}

func ParseNeoFeedsFeedRegisteredEvent(event *chain.ContractEvent) (*NeoFeedsFeedRegisteredEvent, error)
```

### FeedDeactivated Event

Emitted when a price feed is deactivated.

```go
type NeoFeedsFeedDeactivatedEvent struct {
    FeedID string
}

func ParseNeoFeedsFeedDeactivatedEvent(event *chain.ContractEvent) (*NeoFeedsFeedDeactivatedEvent, error)
```

## Usage Examples

### Creating Contract Instance

```go
import (
    "github.com/R3E-Network/service_layer/internal/chain"
    neofeedschain "github.com/R3E-Network/service_layer/services/neofeeds/chain"
)

client, err := chain.NewClient(rpcURL)
if err != nil {
    return err
}

contract := neofeedschain.NewNeoFeedsContract(client, contractHash, wallet)
```

### Reading Price Data

```go
ctx := context.Background()

// Get full price data
priceData, err := contract.GetLatestPrice(ctx, "BTC/USD")
if err != nil {
    return fmt.Errorf("get price: %w", err)
}

fmt.Printf("BTC/USD: %s (decimals: %d, time: %d)\n",
    priceData.Price.String(),
    priceData.Decimals,
    priceData.Timestamp)

// Check freshness before using
fresh, err := contract.IsPriceFresh(ctx, "BTC/USD")
if err != nil || !fresh {
    return fmt.Errorf("price is stale")
}
```

### Parsing Events from Block

```go
import neofeedschain "github.com/R3E-Network/service_layer/services/neofeeds/chain"

func handleEvent(event *chain.ContractEvent) error {
    switch event.EventName {
    case "PriceUpdated":
        parsed, err := neofeedschain.ParseNeoFeedsPriceUpdatedEvent(event)
        if err != nil {
            return err
        }
        fmt.Printf("Price updated: %s = %d\n", parsed.FeedID, parsed.Price)

    case "FeedRegistered":
        parsed, err := neofeedschain.ParseNeoFeedsFeedRegisteredEvent(event)
        if err != nil {
            return err
        }
        fmt.Printf("Feed registered: %s (%s)\n", parsed.FeedID, parsed.Description)
    }
    return nil
}
```

## Dependencies

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/chain` | Core blockchain client and types |

The package relies on these types from `internal/chain`:
- `Client` - Neo N3 RPC client
- `Wallet` - Transaction signing
- `ContractParam` - Contract invocation parameters
- `ContractEvent` - Parsed event data
- `PriceData` - Structured price response
- `ContractFeedConfig` - Feed configuration

## Data Flow

### Push Pattern (NeoFeeds uses this)

```
TEE Service                    NeoFeeds Contract
    │                                │
    │  UpdatePrice(feedId,           │
    │    price, timestamp,           │
    │    nonce, signature)           │
    │──────────────────────────────>│
    │                                │
    │                        Verify TEE signature
    │                        Store price data
    │                        Emit PriceUpdated
    │                                │
    │       Success                  │
    │<──────────────────────────────│
```

### Read Pattern (User Contracts)

```
User Contract                  NeoFeeds Contract
    │                                │
    │  GetLatestPrice(feedId)        │
    │──────────────────────────────>│
    │                                │
    │                        Load from storage
    │                                │
    │       PriceData                │
    │<──────────────────────────────│
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Smart Contract](../contract/README.md)
- [Internal Chain Package](../../../internal/chain/README.md)
