# NeoRand Smart Contract

Neo N3 smart contract implementing Verifiable Random Function (VRF) service.

## Overview

The `VRFService` contract provides:
- On-chain randomness requests from user contracts
- VRF proof storage and verification
- Callback mechanism to deliver randomness

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | VRFService |
| **Author** | R3E Network |
| **Version** | 1.0.0 |
| **Namespace** | ServiceLayer.VRF |

## Architecture

```
User Contract                  VRF Contract                    TEE
    │                              │                            │
    │ requestRandomness            │                            │
    │   (seed, numWords)           │                            │
    │─────────────────────────────>│                            │
    │                              │                            │
    │                        Emit VRFRequest                    │
    │                              │────────────────────────────>│
    │                              │                            │
    │                              │        Generate VRF        │
    │                              │                            │
    │                              │  fulfillRandomness         │
    │                              │   (requestId, words, proof)│
    │                              │<────────────────────────────│
    │                              │                            │
    │  callback(randomWords)       │                            │
    │<─────────────────────────────│                            │
```

## File Structure

| File | Purpose |
|------|---------|
| `NeoRandService.cs` | Main contract class |
| `NeoRandService.Methods.cs` | Request and fulfill methods |
| `NeoRandService.Queries.cs` | Read-only query methods |
| `NeoRandService.Storage.cs` | Storage management |
| `NeoRandService.Types.cs` | Data structures |

## Events

### VRFRequest

Emitted when randomness is requested.

```csharp
event Action<BigInteger, UInt160, byte[], BigInteger> OnVRFRequest;
// Parameters: requestId, userContract, seed, numWords
```

### VRFFulfilled

Emitted when randomness is fulfilled.

```csharp
event Action<BigInteger, byte[], byte[]> OnVRFFulfilled;
// Parameters: requestId, randomWords, proof
```

## Methods

### Public Methods

#### requestRandomness

Called by user contracts to request random numbers.

```csharp
public static BigInteger requestRandomness(byte[] seed, BigInteger numWords)
```

**Parameters**:
- `seed`: User-provided seed (for uniqueness)
- `numWords`: Number of random words requested (1-10)

**Returns**: Request ID for tracking.

### TEE Methods

#### fulfillRandomness

Called by TEE to deliver randomness with proof.

```csharp
public static void fulfillRandomness(
    BigInteger requestId,
    byte[] randomWords,
    byte[] proof
)
```

### Query Methods

#### getRequest

Returns request information.

```csharp
public static VRFRequest getRequest(BigInteger requestId)
```

#### getRandomness

Returns randomness for a fulfilled request.

```csharp
public static byte[] getRandomness(BigInteger requestId)
```

#### getProof

Returns VRF proof for verification.

```csharp
public static byte[] getProof(BigInteger requestId)
```

#### verifyProof

Verifies a VRF proof on-chain.

```csharp
public static bool verifyProof(byte[] seed, byte[] randomWords, byte[] proof)
```

## Data Types

### VRFRequest

```csharp
public class VRFRequest
{
    public BigInteger RequestId;
    public UInt160 UserContract;
    public byte[] Seed;
    public BigInteger NumWords;
    public byte[] RandomWords;  // Set when fulfilled
    public byte[] Proof;        // Set when fulfilled
    public bool Fulfilled;
    public ulong Timestamp;
}
```

## Integration Guide

### Requesting Randomness (User Contract)

```csharp
public class MyContract : SmartContract
{
    public static void RequestRandom()
    {
        byte[] seed = Runtime.GetRandom().ToByteArray();
        BigInteger requestId = (BigInteger)Contract.Call(
            VRFServiceHash,
            "requestRandomness",
            CallFlags.All,
            seed,
            1  // numWords
        );
        Storage.Put(Storage.CurrentContext, "pendingRequest", requestId);
    }

    // Callback from VRF service
    public static void fulfillRandomness(BigInteger requestId, byte[][] randomWords)
    {
        // Verify caller is VRF service
        if (Runtime.CallingScriptHash != VRFServiceHash) return;

        // Use random words
        BigInteger randomValue = new BigInteger(randomWords[0]);
        // ... use randomValue for game logic, lottery, etc.
    }
}
```

## Security

- VRF proof is cryptographically verifiable
- Randomness is deterministic given seed (no manipulation)
- Only registered TEE can fulfill requests
- User contracts must implement callback interface

## Build Instructions

```bash
cd services/vrf/contract
dotnet build
neo-sdk compile NeoRandService.cs
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Chain Integration](../../../infrastructure/chain/README.md)
- [Service Overview](../README.md)
