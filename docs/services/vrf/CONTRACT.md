# VRF Service Smart Contract Documentation

## Overview

The VRF Service smart contract (`VRFService`) provides on-chain integration for verifiable random number generation on the Neo N3 blockchain. It manages randomness requests, verifies proofs, and delivers results to requesting contracts.

## Contract Information

- **Contract Name**: VRFService (RandomnessHub)
- **Language**: C# (Neo N3)
- **Compiler**: neo-devpack-dotnet
- **Network**: Neo N3 MainNet / TestNet

## Contract Methods

### 1. RequestRandomness

Request verifiable random number generation.

**Method Signature**:
```csharp
public static ByteString RequestRandomness(
    ByteString seed,
    UInt160 callbackContract,
    string callbackMethod,
    BigInteger numWords
)
```

**Parameters**:
- `seed` (ByteString): User-provided seed for randomness
- `callbackContract` (UInt160): Contract to receive the result
- `callbackMethod` (string): Method name to invoke with result
- `numWords` (BigInteger): Number of random words (1-10)

**Returns**: Request ID (ByteString)

**Events Emitted**:
```csharp
[DisplayName("RandomnessRequested")]
public static event Action<ByteString, UInt160, ByteString, BigInteger> OnRandomnessRequested;
// Parameters: requestId, requester, seed, numWords
```

**Example**:
```csharp
ByteString requestId = VRFService.RequestRandomness(
    "user_seed_12345",
    Runtime.ExecutingScriptHash,
    "onRandomnessReceived",
    1
);
```

---

### 2. GetRequest

Retrieve VRF request details.

**Method Signature**:
```csharp
public static Map<string, object> GetRequest(ByteString requestId)
```

**Parameters**:
- `requestId` (ByteString): The request identifier

**Returns**: Map containing request details
```csharp
{
    "id": ByteString,
    "requester": UInt160,
    "seed": ByteString,
    "status": "fulfilled",
    "randomness": ByteString,
    "proof": ByteString,
    "created_at": 1701234567,
    "fulfilled_at": 1701234568
}
```

---

### 3. VerifyProof

Verify VRF proof on-chain.

**Method Signature**:
```csharp
public static bool VerifyProof(
    ByteString seed,
    ByteString randomness,
    ByteString proof,
    ByteString publicKey
)
```

**Parameters**:
- `seed` (ByteString): Original seed
- `randomness` (ByteString): Generated randomness
- `proof` (ByteString): VRF proof
- `publicKey` (ByteString): VRF public key

**Returns**: `true` if proof is valid

**Example**:
```csharp
bool valid = VRFService.VerifyProof(
    seed,
    randomness,
    proof,
    publicKey
);
```

---

### 4. FulfillRandomness

Fulfill a randomness request (service only).

**Method Signature**:
```csharp
public static bool FulfillRandomness(
    ByteString requestId,
    ByteString randomness,
    ByteString proof
)
```

**Parameters**:
- `requestId` (ByteString): The request identifier
- `randomness` (ByteString): Generated randomness
- `proof` (ByteString): VRF proof

**Returns**: `true` if fulfilled successfully

**Events Emitted**:
```csharp
[DisplayName("RandomnessFulfilled")]
public static event Action<ByteString, ByteString, ByteString> OnRandomnessFulfilled;
// Parameters: requestId, randomness, proof
```

**Access Control**: Only registered VRF service can call this method

---

### 5. GetPublicKey

Get VRF service public key.

**Method Signature**:
```csharp
public static ByteString GetPublicKey()
```

**Returns**: VRF public key (ByteString)

---

### 6. RegisterService

Register VRF service provider.

**Method Signature**:
```csharp
public static bool RegisterService(
    UInt160 serviceAddress,
    ByteString publicKey,
    ByteString attestationReport
)
```

**Parameters**:
- `serviceAddress` (UInt160): Service address
- `publicKey` (ByteString): VRF public key
- `attestationReport` (ByteString): TEE attestation report

**Returns**: `true` if registered successfully

**Events Emitted**:
```csharp
[DisplayName("ServiceRegistered")]
public static event Action<UInt160, ByteString> OnServiceRegistered;
// Parameters: serviceAddress, publicKey
```

**Access Control**: Admin only

---

### 7. SetFee

Set randomness request fee (admin only).

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

Get current randomness request fee.

**Method Signature**:
```csharp
public static BigInteger GetFee()
```

**Returns**: Fee amount in GAS (8 decimals)

---

## Contract Events

### RandomnessRequested

Emitted when a new randomness request is created.

```csharp
[DisplayName("RandomnessRequested")]
public static event Action<ByteString, UInt160, ByteString, BigInteger> OnRandomnessRequested;
```

**Parameters**:
- `requestId` (ByteString): Unique request identifier
- `requester` (UInt160): Address that created the request
- `seed` (ByteString): Input seed
- `numWords` (BigInteger): Number of random words requested

---

### RandomnessFulfilled

Emitted when a randomness request is fulfilled.

```csharp
[DisplayName("RandomnessFulfilled")]
public static event Action<ByteString, ByteString, ByteString> OnRandomnessFulfilled;
```

**Parameters**:
- `requestId` (ByteString): Request identifier
- `randomness` (ByteString): Generated randomness
- `proof` (ByteString): VRF proof

---

### ServiceRegistered

Emitted when VRF service is registered.

```csharp
[DisplayName("ServiceRegistered")]
public static event Action<UInt160, ByteString> OnServiceRegistered;
```

**Parameters**:
- `serviceAddress` (UInt160): Service address
- `publicKey` (ByteString): VRF public key

---

## Storage Schema

### Request Storage

Key: `request:{requestId}`

Value:
```json
{
  "id": "0x1234...",
  "requester": "0xabcd...",
  "seed": "0x5678...",
  "status": "fulfilled",
  "randomness": "0x9876...",
  "proof": "0xfedc...",
  "num_words": 1,
  "created_at": 1701234567,
  "fulfilled_at": 1701234568
}
```

### Service Storage

Key: `service:{serviceAddress}`

Value:
```json
{
  "address": "0x1234...",
  "public_key": "0x04...",
  "active": true,
  "request_count": 1234,
  "registered_at": 1701234567
}
```

---

## Integration Guide

### 1. Implementing Callback Contract

Your contract must implement a callback method to receive randomness:

```csharp
public class LotteryContract : SmartContract
{
    private static readonly UInt160 VRFServiceAddress = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    public static void DrawWinner()
    {
        // Generate seed from block hash + timestamp
        ByteString seed = Runtime.GetScriptContainer().Hash + Runtime.Time;

        // Request randomness
        ByteString requestId = Contract.Call(
            VRFServiceAddress,
            "requestRandomness",
            CallFlags.All,
            new object[] {
                seed,
                Runtime.ExecutingScriptHash,
                "onRandomnessReceived",
                1
            }
        ) as ByteString;

        Runtime.Log($"Randomness requested: {requestId}");
    }

    // Callback method - must be public
    public static void OnRandomnessReceived(ByteString requestId, ByteString randomness, ByteString proof)
    {
        // Verify caller is VRF service
        if (Runtime.CallingScriptHash != VRFServiceAddress)
        {
            throw new Exception("Unauthorized callback");
        }

        // Convert randomness to winner index
        BigInteger random = (BigInteger)randomness;
        BigInteger participantCount = GetParticipantCount();
        BigInteger winnerIndex = random % participantCount;

        // Select winner
        UInt160 winner = GetParticipant(winnerIndex);
        Storage.Put(Storage.CurrentContext, "winner", winner);

        Runtime.Log($"Winner selected: {winner}");

        // Distribute prize
        DistributePrize(winner);
    }

    private static BigInteger GetParticipantCount()
    {
        return (BigInteger)Storage.Get(Storage.CurrentContext, "participant_count");
    }

    private static UInt160 GetParticipant(BigInteger index)
    {
        return (UInt160)Storage.Get(Storage.CurrentContext, "participant_" + index);
    }

    private static void DistributePrize(UInt160 winner)
    {
        // Prize distribution logic
    }
}
```

### 2. Verifying Randomness On-Chain

```csharp
public static bool VerifyRandomnessOnChain(
    ByteString seed,
    ByteString randomness,
    ByteString proof
)
{
    // Get VRF public key
    ByteString publicKey = Contract.Call(
        VRFServiceAddress,
        "getPublicKey",
        CallFlags.ReadOnly
    ) as ByteString;

    // Verify proof
    bool valid = Contract.Call(
        VRFServiceAddress,
        "verifyProof",
        CallFlags.ReadOnly,
        new object[] { seed, randomness, proof, publicKey }
    ) as bool;

    return valid;
}
```

### 3. Paying Request Fees

```csharp
// Get current fee
BigInteger fee = Contract.Call(VRFServiceAddress, "getFee", CallFlags.ReadOnly) as BigInteger;

// Transfer GAS to VRF service
UInt160 gasToken = "0xd2a4cff31913016155e38e474a2c06d08be276cf".ToScriptHash();
Contract.Call(gasToken, "transfer", CallFlags.All,
    new object[] { Runtime.ExecutingScriptHash, VRFServiceAddress, fee, null });

// Then request randomness
ByteString requestId = Contract.Call(VRFServiceAddress, "requestRandomness", ...);
```

---

## Security Considerations

### VRF Proof Verification

The contract verifies VRF proofs using ECVRF algorithm:

```csharp
// Pseudo-code for VRF verification
bool isValid = VerifyVRFProof(
    publicKey,
    seed,
    randomness,
    proof
);
```

### Access Control

- Only requester can view their request details
- Only registered service can fulfill requests
- Only admin can register services and set fees

### Seed Requirements

- Seeds should be unpredictable (use block hash + timestamp)
- Avoid user-controlled seeds for security-critical applications
- Combine multiple entropy sources when possible

---

## Example Use Cases

### 1. NFT Trait Assignment

```csharp
public static void MintNFT(UInt160 owner)
{
    // Request randomness for trait assignment
    ByteString seed = Runtime.GetScriptContainer().Hash + owner;
    ByteString requestId = VRFService.RequestRandomness(
        seed,
        Runtime.ExecutingScriptHash,
        "onTraitsGenerated",
        5 // 5 random words for different traits
    );

    // Store pending mint
    Storage.Put(Storage.CurrentContext, "pending_mint_" + requestId, owner);
}

public static void OnTraitsGenerated(ByteString requestId, ByteString randomness, ByteString proof)
{
    UInt160 owner = (UInt160)Storage.Get(Storage.CurrentContext, "pending_mint_" + requestId);

    // Use randomness to assign traits
    BigInteger[] traits = ParseRandomWords(randomness, 5);

    // Mint NFT with traits
    MintWithTraits(owner, traits);
}
```

### 2. Random Validator Selection

```csharp
public static void SelectValidators(BigInteger count)
{
    ByteString seed = Runtime.GetScriptContainer().Hash + Runtime.Time;

    ByteString requestId = VRFService.RequestRandomness(
        seed,
        Runtime.ExecutingScriptHash,
        "onValidatorsSelected",
        count
    );
}

public static void OnValidatorsSelected(ByteString requestId, ByteString randomness, ByteString proof)
{
    BigInteger[] randomValues = ParseRandomWords(randomness, GetValidatorCount());

    // Select validators based on random values
    UInt160[] validators = SelectFromPool(randomValues);

    // Update validator set
    UpdateValidators(validators);
}
```

---

## Gas Costs

Estimated GAS costs for contract operations:

- `RequestRandomness`: ~0.5 GAS + request fee
- `GetRequest`: ~0.01 GAS (read-only)
- `VerifyProof`: ~0.1 GAS (read-only)
- `FulfillRandomness`: ~0.3 GAS (service pays)
- `RegisterService`: ~1.0 GAS (one-time)

---

## Testing

### Unit Tests

```bash
cd contracts/VRFService
dotnet test
```

### Integration Tests

See `test/contracts/vrf_test.go` for integration test examples.

---

## Deployment

### Compile Contract

```bash
cd contracts/VRFService
dotnet build
```

### Deploy to TestNet

```bash
neo-express contract deploy VRFService.nef
```

---

## References

- [VRF Service API](./API.md)
- [Usage Examples](./EXAMPLES.md)
- [VRF Specification (RFC 9381)](https://datatracker.ietf.org/doc/html/rfc9381)
- [Neo N3 Documentation](https://docs.neo.org/docs/n3/develop/write/basics)
