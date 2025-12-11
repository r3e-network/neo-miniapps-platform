# NeoRand Chain Integration

Neo N3 blockchain integration for the NeoRand VRF service.

## Overview

This package provides Go bindings for interacting with the `VRFService` smart contract on Neo N3. It implements:
- VRF request event parsing
- Randomness and proof retrieval
- On-chain proof verification

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                   NeoRand Chain Package                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐              ┌─────────────────┐          │
│  │   VRFContract   │              │  Event Parsers  │          │
│  ├─────────────────┤              ├─────────────────┤          │
│  │ GetRandomness   │              │ VRFRequest      │          │
│  │ GetProof        │              │ VRFFulfilled    │          │
│  │ GetVRFPublicKey │              └─────────────────┘          │
│  │ VerifyProof     │                                           │
│  └────────┬────────┘                                           │
│           │                                                     │
└───────────┼─────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│                      internal/chain                              │
│    (Client, ContractParam, InvokeResult, EventListener)         │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure

| File | Purpose |
|------|---------|
| `contract.go` | Contract method invocations |
| `events.go` | Event parsing utilities |

## Contract Interface

### VRFContract

```go
type VRFContract struct {
    client       *chain.Client
    contractHash string
    wallet       *chain.Wallet
}
```

### Methods

#### GetRandomness

Returns the randomness for a fulfilled VRF request.

```go
func (v *VRFContract) GetRandomness(ctx context.Context, requestID *big.Int) ([]byte, error)
```

#### GetProof

Returns the VRF proof for a request.

```go
func (v *VRFContract) GetProof(ctx context.Context, requestID *big.Int) ([]byte, error)
```

#### GetVRFPublicKey

Returns the registered VRF public key.

```go
func (v *VRFContract) GetVRFPublicKey(ctx context.Context) ([]byte, error)
```

#### VerifyProof

Verifies a VRF proof on-chain.

```go
func (v *VRFContract) VerifyProof(ctx context.Context, seed, randomWords, proof []byte) (bool, error)
```

## Event Parsers

### VRFRequestEvent

Emitted when a user contract requests randomness.

```go
type VRFRequestEvent struct {
    RequestID    uint64
    UserContract string
    Seed         []byte
    NumWords     uint64
}
```

### VRFFulfilledEvent

Emitted when randomness is fulfilled.

```go
type VRFFulfilledEvent struct {
    RequestID   uint64
    RandomWords []byte
    Proof       []byte
}
```

## Module Registration

The package auto-registers with the chain registry:

```go
func init() {
    chain.RegisterServiceChain(&Module{})
    chain.RegisterEventParser("neorand", &RequestEventParser{})
    chain.RegisterEventParser("neorand", &FulfilledEventParser{})
}
```

## Usage Examples

### Listening for Requests

```go
events := eventListener.Subscribe("VRFRequest")
for event := range events {
    parsed, _ := vrfchain.ParseVRFRequestEvent(event)
    fmt.Printf("Request %d: seed=%x, numWords=%d\n",
        parsed.RequestID, parsed.Seed, parsed.NumWords)
}
```

### Verifying Proof

```go
valid, err := contract.VerifyProof(ctx, seed, randomWords, proof)
if err != nil {
    return err
}
if !valid {
    return fmt.Errorf("invalid VRF proof")
}
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Smart Contract](../contract/README.md)
- [Internal Chain Package](../../../internal/chain/README.md)
