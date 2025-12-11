# NeoOracle Smart Contract

Neo N3 smart contract for external data oracle requests.

## Overview

The `NeoOracleService` contract manages the on-chain lifecycle of oracle requests:
- Receives requests from ServiceLayerGateway
- Emits events for TEE monitoring
- Stores results for future reference

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | OracleService |
| **Author** | R3E Network |
| **Version** | 2.0.0 |

## Request Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│User Contract │     │   Gateway    │     │OracleService │     │     TEE      │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ RequestService     │                    │                    │
       │ ("oracle",payload) │                    │                    │
       │───────────────────>│                    │                    │
       │                    │                    │                    │
       │                    │ OnRequest          │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │                    │                    │ OracleRequest      │
       │                    │                    │ Event              │
       │                    │                    │───────────────────>│
       │                    │                    │                    │
       │                    │                    │                    │ Fetch URL
       │                    │                    │                    │───────────>
       │                    │                    │                    │
       │                    │ FulfillRequest     │                    │
       │                    │<───────────────────│────────────────────│
       │                    │                    │                    │
       │                    │ OnFulfill          │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │ callback(result)   │                    │                    │
       │<───────────────────│                    │                    │
```

## File Structure

| File | Purpose |
|------|---------|
| `NeoOracleService.cs` | Main contract, events |
| `NeoOracleService.Methods.cs` | Request handling methods |
| `NeoOracleService.Types.cs` | Data structures |

## Events

### OracleRequest

Emitted when a new oracle request is created. TEE monitors this event.

```csharp
[DisplayName("OracleRequest")]
public static event Action<BigInteger, UInt160, string, string, string, string> OnOracleRequest;
// Parameters: requestId, userContract, url, method, headers, jsonPath
```

### OracleFulfilled

Emitted when an oracle request is fulfilled.

```csharp
[DisplayName("OracleFulfilled")]
public static event Action<BigInteger, byte[]> OnOracleFulfilled;
// Parameters: requestId, result
```

## Methods

### OnRequest

Called by ServiceLayerGateway when a user contract requests oracle service.

```csharp
public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
```

**Access**: Gateway only (via `RequireGateway()`)

**Behavior**:
1. Parses request payload
2. Validates URL is provided
3. Stores request details
4. Emits `OracleRequest` event

### OnFulfill

Called by ServiceLayerGateway when TEE fulfills the request.

```csharp
public static void OnFulfill(BigInteger requestId, byte[] result)
```

**Access**: Gateway only

**Behavior**:
1. Stores result for future reference
2. Cleans up pending request
3. Emits `OracleFulfilled` event

### GetResult

Query stored result for a completed request.

```csharp
public static byte[] GetResult(BigInteger requestId)
```

### GetRequest

Query pending request details.

```csharp
public static OracleStoredRequest GetRequest(BigInteger requestId)
```

## Data Types

### OracleRequestPayload

Request payload from user contract:

```csharp
public class OracleRequestPayload
{
    public string Url;        // URL to fetch
    public string Method;     // HTTP method (GET, POST)
    public string Headers;    // JSON-encoded headers
    public string JsonPath;   // JSONPath to extract from response
    public string Body;       // Request body for POST
}
```

### OracleStoredRequest

Stored request in contract storage:

```csharp
public class OracleStoredRequest
{
    public string Url;
    public string Method;
    public string Headers;
    public string JsonPath;
    public UInt160 UserContract;
}
```

## Storage Prefixes

| Prefix | Value | Purpose |
|--------|-------|---------|
| `PREFIX_REQUEST` | `0x10` | Pending requests |
| `PREFIX_RESULT` | `0x20` | Completed results |

## Integration Guide

### User Contract Example

```csharp
// Request external data
byte[] payload = StdLib.Serialize(new OracleRequestPayload {
    Url = "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
    Method = "GET",
    JsonPath = "$.neo.usd"
});

Gateway.RequestService("oracle", payload, "onPriceReceived");

// Callback method
public static void onPriceReceived(BigInteger requestId, byte[] result)
{
    // Process the price data
    string priceJson = (string)StdLib.Deserialize((ByteString)result);
}
```

### Supported HTTP Methods

| Method | Description |
|--------|-------------|
| `GET` | Default, retrieve data |
| `POST` | Send data, retrieve response |

### JSONPath Support

Extract specific values from JSON responses:

| JSONPath | Description |
|----------|-------------|
| `$.data.value` | Extract nested value |
| `$.items[0].name` | First array element |
| `$.results[*].id` | All IDs from array |

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
