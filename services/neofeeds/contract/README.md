# NeoFeeds Smart Contract

Neo N3 smart contract implementing push-based price feed oracle functionality.

## Overview

The `NeoFeedsService` contract stores and serves aggregated price data pushed by TEE (Trusted Execution Environment) accounts. It implements **Pattern 2: Push/Auto-Update** where the TEE proactively pushes price updates rather than responding to on-chain requests.

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | DataFeedsService |
| **Author** | R3E Network |
| **Version** | 1.0.0 |
| **Namespace** | ServiceLayer.DataFeeds |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                   NeoFeedsService Contract                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │      Admin      │  │      TEE        │  │     Queries     │ │
│  ├─────────────────┤  ├─────────────────┤  ├─────────────────┤ │
│  │ SetAdmin        │  │ RegisterTEE     │  │ GetLatestPrice  │ │
│  │ SetPaused       │  │ UpdatePrice     │  │ GetPrice        │ │
│  │ RegisterFeed    │  │ UpdatePrices    │  │ GetTimestamp    │ │
│  │ DeactivateFeed  │  │                 │  │ IsPriceFresh    │ │
│  └─────────────────┘  └─────────────────┘  │ GetFeedConfig   │ │
│                                            └─────────────────┘ │
│                                                                 │
│  Storage Prefixes:                                              │
│  ├── 0x01 ADMIN        ├── 0x20 PRICE                          │
│  ├── 0x02 PAUSED       ├── 0x30 FEED_CONFIG                    │
│  ├── 0x10 TEE_ACCOUNT  └── 0x40 NONCE                          │
│  └── 0x11 TEE_PUBKEY                                           │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `NeoFeedsService.cs` | Main contract class and deployment |
| `NeoFeedsService.Admin.cs` | Administrative methods |
| `NeoFeedsService.TEE.cs` | TEE account management |
| `NeoFeedsService.Feed.cs` | Feed registration and configuration |
| `NeoFeedsService.Updates.cs` | Price update methods |
| `NeoFeedsService.Queries.cs` | Read-only query methods |
| `NeoFeedsService.Types.cs` | Data structures |

## Events

### PriceUpdated

Emitted when a price is updated.

```csharp
event Action<string, BigInteger, BigInteger, ulong> OnPriceUpdated;
// Parameters: feedId, price, decimals, timestamp
```

### FeedRegistered

Emitted when a new feed is registered.

```csharp
event Action<string, string, BigInteger> OnFeedRegistered;
// Parameters: feedId, description, decimals
```

### FeedDeactivated

Emitted when a feed is deactivated.

```csharp
event Action<string> OnFeedDeactivated;
// Parameters: feedId
```

### TEERegistered

Emitted when a TEE account is registered.

```csharp
event Action<UInt160, ECPoint> OnTEERegistered;
// Parameters: account, publicKey
```

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `MAX_STALENESS` | 3,600,000 ms (1 hour) | Maximum age for valid prices |
| `MIN_UPDATE_INTERVAL` | 10,000 ms (10 seconds) | Minimum time between updates |

## Methods

### Query Methods (Public)

#### GetLatestPrice

Returns full price data for a feed.

```csharp
public static PriceData GetLatestPrice(string feedId)
```

**Returns**: `PriceData` struct or null if not found.

#### GetLatestPriceWithCheck

Returns price data with staleness validation.

```csharp
public static PriceData GetLatestPriceWithCheck(string feedId, ulong maxAge)
```

**Throws**: Exception if price is unavailable or too stale.

#### GetPrice

Returns raw price value (for simple integrations).

```csharp
public static BigInteger GetPrice(string feedId)
```

#### GetPriceTimestamp

Returns the timestamp of the latest update.

```csharp
public static ulong GetPriceTimestamp(string feedId)
```

#### IsPriceFresh

Checks if price is within staleness threshold.

```csharp
public static bool IsPriceFresh(string feedId)
```

#### GetFeedConfig

Returns feed configuration.

```csharp
public static FeedConfig GetFeedConfig(string feedId)
```

### TEE Methods (TEE Account Only)

#### UpdatePrice

Updates price for a single feed.

```csharp
public static void UpdatePrice(
    string feedId,
    BigInteger price,
    ulong timestamp,
    BigInteger nonce,
    byte[] signature
)
```

**Validation**:
- Contract must not be paused
- Caller must be registered TEE account
- Feed must exist and be active
- Price must be positive
- Timestamp must not be in future
- Timestamp must not be older than MAX_STALENESS
- Nonce must not have been used before
- Signature must be valid from registered TEE public key

#### UpdatePrices

Batch update multiple prices (more gas efficient).

```csharp
public static void UpdatePrices(
    string[] feedIds,
    BigInteger[] prices,
    ulong[] timestamps,
    BigInteger nonce,
    byte[] signature
)
```

**Constraints**:
- Maximum 10 feeds per batch
- Single signature covers all prices

### Admin Methods (Admin Only)

#### RegisterFeed

Register a new price feed.

```csharp
public static void RegisterFeed(string feedId, string description, BigInteger decimals)
```

#### DeactivateFeed

Deactivate an existing feed.

```csharp
public static void DeactivateFeed(string feedId)
```

#### RegisterTEEAccount

Register a TEE account with its public key.

```csharp
public static void RegisterTEEAccount(UInt160 account, ECPoint publicKey)
```

#### SetAdmin

Transfer admin rights.

```csharp
public static void SetAdmin(UInt160 newAdmin)
```

#### SetPaused

Pause/unpause the contract.

```csharp
public static void SetPaused(bool paused)
```

## Data Types

### PriceData

```csharp
public class PriceData
{
    public string FeedId;
    public BigInteger Price;
    public BigInteger Decimals;
    public ulong Timestamp;
    public UInt160 UpdatedBy;
}
```

### FeedConfig

```csharp
public class FeedConfig
{
    public string FeedId;
    public string Description;
    public BigInteger Decimals;
    public bool Active;
}
```

## Storage Layout

| Prefix | Key Format | Value |
|--------|------------|-------|
| `0x01` | `[PREFIX_ADMIN]` | Admin account (UInt160) |
| `0x02` | `[PREFIX_PAUSED]` | Paused flag (bool) |
| `0x10` | `[PREFIX_TEE_ACCOUNT][account]` | TEE registration (bool) |
| `0x11` | `[PREFIX_TEE_PUBKEY][account]` | TEE public key (ECPoint) |
| `0x20` | `[PREFIX_PRICE][feedId]` | Price data (serialized) |
| `0x30` | `[PREFIX_FEED_CONFIG][feedId]` | Feed config (serialized) |
| `0x40` | `[PREFIX_NONCE][nonce]` | Nonce used flag (int) |

## Security Model

### Access Control

```
                 ┌─────────────┐
                 │    Admin    │
                 └──────┬──────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
  RegisterFeed    RegisterTEE      SetPaused
  DeactivateFeed  SetAdmin

                 ┌─────────────┐
                 │ TEE Account │
                 └──────┬──────┘
                        │
        ┌───────────────┴───────────────┐
        ▼                               ▼
   UpdatePrice                    UpdatePrices

                 ┌─────────────┐
                 │   Anyone    │
                 └──────┬──────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
  GetLatestPrice    GetPrice      IsPriceFresh
```

### Signature Verification

TEE updates require ECDSA signature verification:

```csharp
byte[] message = feedId + price + timestamp + nonce;
bool valid = CryptoLib.VerifyWithECDsa(
    message,
    teePubKey,
    signature,
    NamedCurve.secp256r1
);
```

### Replay Protection

- Each update requires a unique nonce
- Used nonces are stored permanently
- Prevents replay of old price updates

## Default Feeds

Registered on deployment:

| Feed ID | Description | Decimals |
|---------|-------------|----------|
| BTC/USD | Bitcoin to US Dollar | 8 |
| ETH/USD | Ethereum to US Dollar | 8 |
| NEO/USD | Neo to US Dollar | 8 |
| GAS/USD | Gas to US Dollar | 8 |
| NEO/GAS | Neo to Gas | 8 |

## Integration Guide

### Reading Prices (User Contracts)

```csharp
// Simple read
BigInteger btcPrice = NeoFeedsService.GetPrice("BTC/USD");

// Read with staleness check (max 5 minutes)
PriceData data = NeoFeedsService.GetLatestPriceWithCheck("BTC/USD", 300000);

// Check freshness before use
if (NeoFeedsService.IsPriceFresh("BTC/USD")) {
    BigInteger price = NeoFeedsService.GetPrice("BTC/USD");
    // Use price
}
```

### Price Interpretation

Prices are stored as integers with a decimal exponent:

```
Actual Price = StoredPrice / 10^Decimals

Example: BTC/USD
  StoredPrice = 10500000000000 (105000 * 10^8)
  Decimals = 8
  Actual Price = 105000.00000000 USD
```

## Build Instructions

```bash
# Navigate to contract directory
cd services/neofeeds/contract

# Build with Neo compiler
dotnet build

# Generate manifest
neo-sdk compile NeoFeedsService.cs
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Chain Integration](../chain/README.md)
- [Service Overview](../README.md)
