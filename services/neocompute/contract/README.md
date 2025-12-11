# NeoCompute Smart Contract

Neo N3 smart contract for confidential computing requests.

## Overview

The `NeoComputeService` (ConfidentialService) contract manages on-chain confidential computation:
- Receives encrypted input from user contracts
- Emits events for TEE processing
- Stores encrypted results with commitments for verification

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | ConfidentialService |
| **Author** | R3E Network |
| **Version** | 2.0.0 |

## Request Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│User Contract │     │   Gateway    │     │ Confidential │     │     TEE      │
│              │     │              │     │   Service    │     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │                    │
       │ RequestService     │                    │                    │
       │ ("confidential",   │                    │                    │
       │  encryptedPayload) │                    │                    │
       │───────────────────>│                    │                    │
       │                    │                    │                    │
       │                    │ OnRequest          │                    │
       │                    │───────────────────>│                    │
       │                    │                    │                    │
       │                    │                    │ ConfidentialRequest│
       │                    │                    │ Event              │
       │                    │                    │───────────────────>│
       │                    │                    │                    │
       │                    │                    │                    │ Decrypt,
       │                    │                    │                    │ Compute,
       │                    │                    │                    │ Encrypt
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
| `NeoComputeService.cs` | Main contract, events |
| `NeoComputeService.Methods.cs` | Request handling, TEE key management |
| `NeoComputeService.Types.cs` | Data structures |

## Events

### ConfidentialRequest

Emitted when a new confidential request is created.

```csharp
[DisplayName("ConfidentialRequest")]
public static event Action<BigInteger, UInt160, string, byte[], byte[]> OnConfidentialRequest;
// Parameters: requestId, userContract, computationType, encryptedInput, inputCommitment
```

### ConfidentialFulfilled

Emitted when computation is complete.

```csharp
[DisplayName("ConfidentialFulfilled")]
public static event Action<BigInteger, byte[], byte[]> OnConfidentialFulfilled;
// Parameters: requestId, encryptedOutput, outputCommitment
```

## Methods

### SetTEEPublicKey

Set the TEE public key for input encryption.

```csharp
public static void SetTEEPublicKey(ECPoint pubKey)
```

**Access**: Gateway only

### GetTEEPublicKey

Get the current TEE public key.

```csharp
public static ECPoint GetTEEPublicKey()
```

### OnRequest

Called by ServiceLayerGateway when a user contract requests confidential computation.

```csharp
public static void OnRequest(BigInteger requestId, UInt160 userContract, byte[] payload)
```

**Access**: Gateway only

**Behavior**:
1. Parses confidential request payload
2. Validates encrypted input is provided
3. Computes input commitment (SHA256)
4. Stores request and commitment
5. Emits `ConfidentialRequest` event

### OnFulfill

Called by ServiceLayerGateway when TEE completes computation.

```csharp
public static void OnFulfill(BigInteger requestId, byte[] result)
```

**Access**: Gateway only

**Behavior**:
1. Parses result with encrypted output and commitment
2. Stores result for future reference
3. Cleans up pending request
4. Emits `ConfidentialFulfilled` event

### GetResult

Query stored result for a completed request.

```csharp
public static byte[] GetResult(BigInteger requestId)
```

### GetInputCommitment

Query the input commitment for verification.

```csharp
public static byte[] GetInputCommitment(BigInteger requestId)
```

### VerifyInputCommitment

Verify that encrypted input matches the stored commitment.

```csharp
public static bool VerifyInputCommitment(BigInteger requestId, byte[] encryptedInput)
```

## Data Types

### ConfidentialRequestPayload

Request payload from user contract:

```csharp
public class ConfidentialRequestPayload
{
    public string ComputationType;   // aggregate, compare, auction, vote
    public byte[] EncryptedInput;    // Input encrypted with TEE public key
    public bool OutputPublic;        // Public or encrypted output
    public byte[] UserPublicKey;     // For encrypted output delivery
}
```

### ConfidentialStoredRequest

Stored request in contract storage:

```csharp
public class ConfidentialStoredRequest
{
    public string ComputationType;
    public byte[] EncryptedInput;
    public byte[] InputCommitment;
    public bool OutputPublic;
    public UInt160 UserContract;
}
```

### ConfidentialResultPayload

Result from TEE:

```csharp
public class ConfidentialResultPayload
{
    public byte[] EncryptedOutput;    // Encrypted or public output
    public byte[] OutputCommitment;   // Hash for verification
    public byte[] Proof;              // Optional ZK proof
}
```

## Storage Prefixes

| Prefix | Value | Purpose |
|--------|-------|---------|
| `PREFIX_TEE_PUBKEY` | `0x02` | TEE public key |
| `PREFIX_REQUEST` | `0x10` | Pending requests |
| `PREFIX_RESULT` | `0x20` | Completed results |
| `PREFIX_COMMITMENT` | `0x30` | Input commitments |

## Computation Types

| Type | Description |
|------|-------------|
| `aggregate` | Aggregate private values (sum, avg, max) |
| `compare` | Compare encrypted values |
| `auction` | Sealed-bid auction processing |
| `vote` | Confidential voting tallying |

## Use Cases

### Private Auction

```csharp
// Encrypt bid with TEE public key
ECPoint teePubKey = ConfidentialService.GetTEEPublicKey();
byte[] encryptedBid = Encrypt(teePubKey, myBidAmount);

byte[] payload = StdLib.Serialize(new ConfidentialRequestPayload {
    ComputationType = "auction",
    EncryptedInput = encryptedBid,
    OutputPublic = true  // Winner announced publicly
});

Gateway.RequestService("confidential", payload, "onAuctionResult");
```

### Confidential Voting

```csharp
byte[] payload = StdLib.Serialize(new ConfidentialRequestPayload {
    ComputationType = "vote",
    EncryptedInput = encryptedVote,
    OutputPublic = true,  // Final tally is public
});

Gateway.RequestService("confidential", payload, "onVoteResult");
```

## Security Properties

### Input Privacy

- Inputs encrypted with TEE public key
- Only TEE can decrypt
- Input commitments stored for verification

### Output Verification

- Output commitment allows result verification
- Optional ZK proofs for computation correctness

### Commitment Scheme

Input and output commitments enable:
- Verify inputs weren't modified
- Verify outputs match computation
- Audit trail without revealing data

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
