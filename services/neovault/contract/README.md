# NeoVault Smart Contract

Neo N3 smart contract for off-chain privacy mixing with on-chain dispute resolution.

## Overview

The `NeoVaultService` contract implements a **minimal on-chain footprint** for privacy mixing:
- Service registration and bond management
- Dispute submission by users
- Dispute resolution by TEE
- Refund claims if TEE fails

**Important**: Normal mixing operations happen **entirely off-chain**. The contract is only used during disputes.

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | NeoVaultService |
| **Author** | R3E Network |
| **Version** | 5.0.0 |
| **Namespace** | ServiceLayer.Mixer |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    NeoVaultService Contract                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │      Admin      │  │     Service     │  │     Dispute     │ │
│  ├─────────────────┤  ├─────────────────┤  ├─────────────────┤ │
│  │ SetAdmin        │  │ RegisterService │  │ SubmitDispute   │ │
│  │ SetPaused       │  │ DepositBond     │  │ ResolveDispute  │ │
│  │ Update          │  │ WithdrawBond    │  │ ClaimRefund     │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
│                                                                 │
│  Storage Prefixes:                                              │
│  ├── 0x01 ADMIN        ├── 0x20 DISPUTE                        │
│  ├── 0x02 PAUSED       ├── 0x21 RESOLVED                       │
│  ├── 0x10 SERVICE      └── 0x30 NONCE                          │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `NeoVaultService.cs` | Main contract class and deployment |
| `NeoVaultService.Admin.cs` | Administrative methods |
| `NeoVaultService.Service.cs` | Service registration and bond |
| `NeoVaultService.Dispute.cs` | Dispute resolution logic |
| `NeoVaultService.Queries.cs` | Read-only query methods |
| `NeoVaultService.Types.cs` | Data structures |

## Events

### ServiceRegistered

Emitted when a mixing service registers.

```csharp
event Action<byte[], ECPoint> OnServiceRegistered;
// Parameters: serviceId, teePubKey
```

### BondDeposited

Emitted when a service deposits bond.

```csharp
event Action<byte[], BigInteger, BigInteger> OnBondDeposited;
// Parameters: serviceId, amount, totalBond
```

### DisputeSubmitted

Emitted when a user submits a dispute.

```csharp
event Action<byte[], UInt160, BigInteger, ulong> OnDisputeSubmitted;
// Parameters: requestHash, user, amount, deadline
```

### DisputeResolved

Emitted when TEE resolves a dispute.

```csharp
event Action<byte[], byte[], byte[]> OnDisputeResolved;
// Parameters: requestHash, serviceId, completionProof
```

### DisputeRefunded

Emitted when user is refunded after deadline.

```csharp
event Action<byte[], UInt160, BigInteger> OnDisputeRefunded;
// Parameters: requestHash, user, amount
```

### BondSlashed

Emitted when service bond is slashed.

```csharp
event Action<byte[], BigInteger, BigInteger> OnBondSlashed;
// Parameters: serviceId, slashedAmount, remainingBond
```

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `MIN_BOND` | 10 GAS | Minimum service bond |
| `DISPUTE_DEADLINE` | 7 days | Time for TEE to respond |

### Dispute Status

| Status | Value | Description |
|--------|-------|-------------|
| `DISPUTE_PENDING` | 0 | User submitted, waiting for TEE |
| `DISPUTE_RESOLVED` | 1 | TEE submitted completion proof |
| `DISPUTE_REFUNDED` | 2 | TEE failed, user refunded |

## Methods

### Admin Methods

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

### Service Methods

#### RegisterService

Register a mixing service with TEE public key.

```csharp
public static void RegisterService(byte[] serviceId, ECPoint teePubKey)
```

#### DepositBond

Deposit bond for service operation.

```csharp
public static void DepositBond(byte[] serviceId, BigInteger amount)
```

#### WithdrawBond

Withdraw excess bond (must maintain MIN_BOND).

```csharp
public static void WithdrawBond(byte[] serviceId, BigInteger amount)
```

### Dispute Methods

#### SubmitDispute

User submits dispute for incomplete mix.

```csharp
public static void SubmitDispute(
    byte[] requestHash,
    byte[] teeSignature,
    BigInteger amount
)
```

**Requirements**:
- Valid TEE signature on request hash
- No existing dispute for this request
- Contract not paused

**Effect**:
- Creates dispute record with 7-day deadline
- Emits `DisputeSubmitted` event

#### ResolveDispute

TEE resolves dispute with completion proof.

```csharp
public static void ResolveDispute(
    byte[] requestHash,
    byte[] outputsHash,
    byte[] proofSignature
)
```

**Requirements**:
- Caller must be registered TEE service
- Dispute must be pending
- Valid signature on outputs hash

**Effect**:
- Marks dispute as resolved
- Emits `DisputeResolved` event

#### ClaimRefund

User claims refund after deadline passes.

```csharp
public static void ClaimRefund(byte[] requestHash)
```

**Requirements**:
- Dispute must be pending
- Deadline must have passed
- Caller must be dispute creator

**Effect**:
- Marks dispute as refunded
- Slashes service bond
- Transfers refund to user
- Emits `DisputeRefunded` and `BondSlashed` events

### Query Methods

#### GetService

Returns service information.

```csharp
public static ServiceInfo GetService(byte[] serviceId)
```

#### GetDispute

Returns dispute information.

```csharp
public static DisputeInfo GetDispute(byte[] requestHash)
```

#### IsDisputeResolved

Checks if dispute is resolved.

```csharp
public static bool IsDisputeResolved(byte[] requestHash)
```

## Data Types

### ServiceInfo

```csharp
public class ServiceInfo
{
    public byte[] ServiceId;
    public ECPoint TeePubKey;
    public BigInteger Bond;
    public bool Active;
    public ulong RegisterAt;
}
```

### DisputeInfo

```csharp
public class DisputeInfo
{
    public byte[] RequestHash;
    public UInt160 User;
    public BigInteger Amount;
    public byte[] ServiceId;
    public ulong Deadline;
    public byte Status;  // 0=Pending, 1=Resolved, 2=Refunded
    public ulong SubmittedAt;
    public byte[] OutputsHash;    // Set when resolved
    public byte[] ProofSignature; // Set when resolved
}
```

## Storage Layout

| Prefix | Key Format | Value |
|--------|------------|-------|
| `0x01` | `[PREFIX_ADMIN]` | Admin account (UInt160) |
| `0x02` | `[PREFIX_PAUSED]` | Paused flag (bool) |
| `0x10` | `[PREFIX_SERVICE][serviceId]` | Service info (serialized) |
| `0x20` | `[PREFIX_DISPUTE][requestHash]` | Dispute info (serialized) |
| `0x21` | `[PREFIX_RESOLVED][requestHash]` | Resolution flag (bool) |
| `0x30` | `[PREFIX_NONCE][nonce]` | Nonce used flag (int) |

## Security Model

### Privacy-First Design

Normal operations have **ZERO on-chain footprint**:
- User requests mix via API (off-chain)
- User deposits to anonymous pool account
- Mixing happens off-chain
- Delivery happens off-chain
- User never connects to known service layer account

### Dispute Mechanism

Only dispute path touches on-chain:

```
User Perspective:
1. If mixing incomplete after deadline, submit dispute
2. Wait for TEE to resolve (7 days max)
3. If TEE fails, claim refund from slashed bond

TEE Perspective:
1. Monitor for DisputeSubmitted events
2. Submit completion proof if mixing was done
3. Or do nothing if honestly failed (bond slashed)
```

### Bond Economics

- Services must deposit minimum 10 GAS bond
- Bond is slashed on dispute failure
- Slashed amount compensates user
- Incentivizes honest service operation

## Integration Guide

### User: Submitting Dispute

```csharp
// User has RequestProof from off-chain request
// Mixing deadline passed without delivery

NeoVaultService.SubmitDispute(
    requestHash,     // From RequestProof
    teeSignature,    // From RequestProof
    amount           // Deposited amount
);
// Wait up to 7 days for resolution or claim refund
```

### TEE: Resolving Dispute

```csharp
// TEE monitors DisputeSubmitted events
// If mixing was completed, submit proof

NeoVaultService.ResolveDispute(
    requestHash,
    outputsHash,     // Hash of delivery transactions
    proofSignature   // TEE signature on outputsHash
);
```

### User: Claiming Refund

```csharp
// If 7 days passed without resolution
NeoVaultService.ClaimRefund(requestHash);
// User receives refund from slashed bond
```

## Build Instructions

```bash
# Navigate to contract directory
cd services/neovault/contract

# Build with Neo compiler
dotnet build

# Generate manifest
neo-sdk compile NeoVaultService.cs
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Chain Integration](../chain/README.md)
- [Service Overview](../README.md)
