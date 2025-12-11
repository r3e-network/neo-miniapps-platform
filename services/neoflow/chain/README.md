# NeoFlow Chain Integration

Neo N3 blockchain integration for the NeoFlow automation service.

## Overview

This package provides Go bindings for interacting with the `NeoFlowService` smart contract on Neo N3.

## File Structure

| File | Purpose |
|------|---------|
| `contract.go` | Contract method invocations |
| `events.go` | Event parsing utilities |

## Contract Interface

### NeoFlowContract

```go
type NeoFlowContract struct {
    client       *chain.Client
    contractHash string
    wallet       *chain.Wallet
}
```

### Methods

- `GetTrigger(ctx, triggerID)` - Get trigger details
- `GetActiveTriggers(ctx)` - List active triggers
- `ExecuteTrigger(ctx, triggerID)` - Execute trigger callback

## Event Parsers

### TriggerExecuted

Emitted when a trigger is executed.

```go
type TriggerExecutedEvent struct {
    TriggerID uint64
    Success   bool
    Result    []byte
    Timestamp uint64
}
```

### TriggerRegistered

Emitted when a new trigger is registered.

```go
type TriggerRegisteredEvent struct {
    TriggerID   uint64
    Owner       string
    TriggerType uint8
}
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Smart Contract](../contract/README.md)
