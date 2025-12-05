# datastreams Service Smart Contract Documentation

## Overview

The datastreams service smart contract provides on-chain integration for the datastreams service on the Neo N3 blockchain.

## Contract Information

- **Contract Name**: DatastreamsService
- **Language**: C# (Neo N3)
- **Network**: Neo N3 MainNet / TestNet

## Contract Methods

### 1. ProcessRequest

Submit a request to the datastreams service.

**Method Signature**:
```csharp
public static void ProcessRequest(
    ByteString requestId,
    UInt160 requester,
    ByteString payload
)
```

**Parameters**:
- `requestId` (ByteString): Unique request identifier
- `requester` (UInt160): Requester address
- `payload` (ByteString): Request payload

**Events Emitted**:
```csharp
[DisplayName("RequestCreated")]
public static event Action<ByteString, UInt160> OnRequestCreated;
```

### 2. GetRequest

Retrieve request details.

**Method Signature**:
```csharp
public static Map<string, object> GetRequest(ByteString requestId)
```

## Integration Guide

### Implementing Callback Contract

```csharp
public class MyContract : SmartContract
{
    private static readonly UInt160 DatastreamsService = 
        "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    public static void SubmitRequest()
    {
        var payload = StdLib.Serialize(new RequestData {
            // Request parameters
        });

        Contract.Call(
            DatastreamsService,
            "processRequest",
            CallFlags.All,
            new object[] { payload }
        );
    }

    public static void OnResponseReceived(
        ByteString requestId,
        bool success,
        ByteString data
    )
    {
        // Handle response
    }
}
```

## Gas Costs

- ProcessRequest: ~0.5 GAS
- GetRequest: ~0.01 GAS (read-only)

## References

- [API Documentation](./API.md)
- [Usage Examples](./EXAMPLES.md)
- [Neo N3 Documentation](https://docs.neo.org)
