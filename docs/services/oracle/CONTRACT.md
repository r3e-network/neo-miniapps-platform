# Oracle Service Smart Contract Documentation

## Overview

The Oracle Service smart contract (`OracleHub`) provides on-chain integration for external data requests on the Neo N3 blockchain. It manages oracle requests, handles callbacks, and maintains service registration.

## Contract Information

- **Contract Name**: OracleHub
- **Language**: C# (Neo N3)
- **Compiler**: neo-devpack-dotnet
- **Network**: Neo N3 MainNet / TestNet

## Contract Methods

### 1. RequestData

Submit a new oracle data request.

**Method Signature**:
```csharp
public static string RequestData(
    string url,
    string method,
    ByteString headers,
    ByteString body,
    string jsonPath,
    UInt160 callbackContract,
    string callbackMethod
)
```

**Parameters**:
- `url` (string): Target URL to fetch data from
- `method` (string): HTTP method (GET, POST, PUT, DELETE)
- `headers` (ByteString): JSON-encoded headers
- `body` (ByteString): Request body (for POST/PUT)
- `jsonPath` (string): Optional JSON path for data extraction
- `callbackContract` (UInt160): Contract to receive the result
- `callbackMethod` (string): Method name to invoke with result

**Returns**: Request ID (string)

**Events Emitted**:
```csharp
[DisplayName("OracleRequestCreated")]
public static event Action<string, UInt160, string, string> OnRequestCreated;
// Parameters: requestId, requester, url, callbackContract
```

**Example**:
```csharp
string requestId = OracleHub.RequestData(
    "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
    "GET",
    "{}",
    "",
    "$.bitcoin.usd",
    Runtime.ExecutingScriptHash,
    "onPriceReceived"
);
```

---

### 2. GetRequest

Retrieve oracle request details.

**Method Signature**:
```csharp
public static Map<string, object> GetRequest(string requestId)
```

**Parameters**:
- `requestId` (string): The request identifier

**Returns**: Map containing request details
```csharp
{
    "id": "req-1234567890",
    "requester": UInt160,
    "url": "https://api.example.com/data",
    "method": "GET",
    "status": "pending",
    "created_at": 1701234567,
    "callback_contract": UInt160,
    "callback_method": "onDataReceived"
}
```

---

### 3. CancelRequest

Cancel a pending oracle request (only by requester).

**Method Signature**:
```csharp
public static bool CancelRequest(string requestId)
```

**Parameters**:
- `requestId` (string): The request identifier

**Returns**: `true` if cancelled successfully

**Events Emitted**:
```csharp
[DisplayName("OracleRequestCancelled")]
public static event Action<string, UInt160> OnRequestCancelled;
// Parameters: requestId, requester
```

---

### 4. FulfillRequest

Fulfill an oracle request with data (service only).

**Method Signature**:
```csharp
public static bool FulfillRequest(
    string requestId,
    ByteString data,
    ByteString signature
)
```

**Parameters**:
- `requestId` (string): The request identifier
- `data` (ByteString): The fetched data
- `signature` (ByteString): TEE signature for verification

**Returns**: `true` if fulfilled successfully

**Events Emitted**:
```csharp
[DisplayName("OracleRequestFulfilled")]
public static event Action<string, ByteString, UInt160> OnRequestFulfilled;
// Parameters: requestId, data, serviceAddress
```

**Access Control**: Only registered oracle service can call this method

---

### 5. RegisterService

Register an oracle service provider.

**Method Signature**:
```csharp
public static bool RegisterService(
    UInt160 serviceAddress,
    ByteString publicKey,
    ByteString attestationReport
)
```

**Parameters**:
- `serviceAddress` (UInt160): Service contract/wallet address
- `publicKey` (ByteString): TEE public key for signature verification
- `attestationReport` (ByteString): Remote attestation report

**Returns**: `true` if registered successfully

**Events Emitted**:
```csharp
[DisplayName("ServiceRegistered")]
public static event Action<UInt160, ByteString> OnServiceRegistered;
// Parameters: serviceAddress, publicKey
```

**Access Control**: Requires admin signature or governance approval

---

### 6. GetServiceInfo

Get information about a registered service.

**Method Signature**:
```csharp
public static Map<string, object> GetServiceInfo(UInt160 serviceAddress)
```

**Parameters**:
- `serviceAddress` (UInt160): Service address

**Returns**: Map containing service details
```csharp
{
    "address": UInt160,
    "public_key": ByteString,
    "active": true,
    "request_count": 1234,
    "success_count": 1200,
    "registered_at": 1701234567
}
```

---

### 7. SetFee

Set the oracle request fee (admin only).

**Method Signature**:
```csharp
public static bool SetFee(BigInteger feeAmount)
```

**Parameters**:
- `feeAmount` (BigInteger): Fee in GAS (8 decimals)

**Returns**: `true` if set successfully

**Access Control**: Admin only

---

### 8. GetFee

Get the current oracle request fee.

**Method Signature**:
```csharp
public static BigInteger GetFee()
```

**Returns**: Fee amount in GAS (8 decimals)

---

## Contract Events

### OracleRequestCreated

Emitted when a new oracle request is created.

```csharp
[DisplayName("OracleRequestCreated")]
public static event Action<string, UInt160, string, string> OnRequestCreated;
```

**Parameters**:
- `requestId` (string): Unique request identifier
- `requester` (UInt160): Address that created the request
- `url` (string): Target URL
- `callbackContract` (string): Callback contract address (hex)

---

### OracleRequestFulfilled

Emitted when an oracle request is fulfilled.

```csharp
[DisplayName("OracleRequestFulfilled")]
public static event Action<string, ByteString, UInt160> OnRequestFulfilled;
```

**Parameters**:
- `requestId` (string): Request identifier
- `data` (ByteString): Fetched data
- `serviceAddress` (UInt160): Service that fulfilled the request

---

### OracleRequestCancelled

Emitted when a request is cancelled.

```csharp
[DisplayName("OracleRequestCancelled")]
public static event Action<string, UInt160> OnRequestCancelled;
```

**Parameters**:
- `requestId` (string): Request identifier
- `requester` (UInt160): Address that cancelled the request

---

### ServiceRegistered

Emitted when a new oracle service is registered.

```csharp
[DisplayName("ServiceRegistered")]
public static event Action<UInt160, ByteString> OnServiceRegistered;
```

**Parameters**:
- `serviceAddress` (UInt160): Service address
- `publicKey` (ByteString): TEE public key

---

## Storage Schema

### Request Storage

Key: `request:{requestId}`

Value:
```json
{
  "id": "req-1234567890",
  "requester": "0x1234567890abcdef",
  "url": "https://api.example.com/data",
  "method": "GET",
  "headers": "{}",
  "body": "",
  "json_path": "$.data.price",
  "status": "pending",
  "callback_contract": "0xabcdef1234567890",
  "callback_method": "onDataReceived",
  "created_at": 1701234567,
  "fulfilled_at": 0,
  "result": null
}
```

### Service Storage

Key: `service:{serviceAddress}`

Value:
```json
{
  "address": "0x1234567890abcdef",
  "public_key": "0x04...",
  "active": true,
  "request_count": 1234,
  "success_count": 1200,
  "registered_at": 1701234567,
  "attestation_report": "0x..."
}
```

---

## Integration Guide

### 1. Implementing Callback Contract

Your contract must implement a callback method to receive oracle results:

```csharp
public class MyContract : SmartContract
{
    // Store the OracleHub contract address
    private static readonly UInt160 OracleHubAddress = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    public static void RequestPrice()
    {
        // Request BTC price
        string requestId = Contract.Call(
            OracleHubAddress,
            "requestData",
            CallFlags.All,
            new object[] {
                "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd",
                "GET",
                "{}",
                "",
                "$.bitcoin.usd",
                Runtime.ExecutingScriptHash,
                "onPriceReceived"
            }
        ) as string;

        Runtime.Log($"Oracle request created: {requestId}");
    }

    // Callback method - must be public
    public static void OnPriceReceived(string requestId, ByteString data, ByteString signature)
    {
        // Verify caller is OracleHub
        if (Runtime.CallingScriptHash != OracleHubAddress)
        {
            throw new Exception("Unauthorized callback");
        }

        // Parse the price data
        string dataStr = (string)StdLib.JsonDeserialize(data);
        BigInteger price = (BigInteger)dataStr;

        Runtime.Log($"BTC Price: {price}");

        // Store or use the price data
        Storage.Put(Storage.CurrentContext, "btc_price", price);
    }
}
```

### 2. Listening to Events

Monitor oracle events using Neo RPC:

```javascript
const neo3 = require('@cityofzion/neon-js');

const client = new neo3.rpc.RPCClient('https://testnet.neo.org:443');

// Subscribe to OracleRequestCreated events
client.getApplicationLog(txHash).then(log => {
  log.executions[0].notifications.forEach(notification => {
    if (notification.eventname === 'OracleRequestCreated') {
      const [requestId, requester, url, callback] = notification.state.value;
      console.log('New oracle request:', requestId);
    }
  });
});
```

### 3. Paying Request Fees

Oracle requests require a fee payment:

```csharp
// Get current fee
BigInteger fee = Contract.Call(OracleHubAddress, "getFee", CallFlags.ReadOnly) as BigInteger;

// Transfer GAS to OracleHub
UInt160 gasToken = "0xd2a4cff31913016155e38e474a2c06d08be276cf".ToScriptHash();
Contract.Call(gasToken, "transfer", CallFlags.All,
    new object[] { Runtime.ExecutingScriptHash, OracleHubAddress, fee, null });

// Then submit request
string requestId = Contract.Call(OracleHubAddress, "requestData", ...);
```

---

## Security Considerations

### Signature Verification

The contract verifies TEE signatures on all fulfilled requests:

```csharp
// Pseudo-code for signature verification
bool isValid = CryptoLib.VerifyWithECDsa(
    Hash256(requestId + data),
    publicKey,
    signature,
    NamedCurve.secp256r1
);
```

### Access Control

- Only the requester can cancel their requests
- Only registered services can fulfill requests
- Only admin can register services and set fees

### Reentrancy Protection

The contract uses checks-effects-interactions pattern to prevent reentrancy attacks.

---

## Deployment

### Compile Contract

```bash
cd contracts/OracleService
dotnet build
```

### Deploy to TestNet

```bash
neo-express contract deploy OracleService.nef
```

### Verify Deployment

```bash
neo-express contract get OracleService
```

---

## Testing

### Unit Tests

```bash
cd contracts/OracleService
dotnet test
```

### Integration Tests

See `test/contracts/oracle_test.go` for integration test examples.

---

## Gas Costs

Estimated GAS costs for contract operations:

- `RequestData`: ~0.5 GAS + request fee
- `GetRequest`: ~0.01 GAS (read-only)
- `CancelRequest`: ~0.2 GAS
- `FulfillRequest`: ~0.3 GAS (service pays)
- `RegisterService`: ~1.0 GAS (one-time)

---

## References

- [Neo N3 Documentation](https://docs.neo.org/docs/n3/develop/write/basics)
- [Oracle Service API](./API.md)
- [Usage Examples](./EXAMPLES.md)
