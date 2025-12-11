# NeoFlow Smart Contract

Neo N3 smart contract for trigger-based task automation.

## Overview

The `NeoFlowService` contract manages:
- Trigger registration and configuration
- Condition monitoring
- Callback execution

## Contract Identity

| Property | Value |
|----------|-------|
| **Display Name** | NeoFlowService |
| **Author** | R3E Network |
| **Version** | 1.0.0 |

## Trigger Types

| Type | ID | Description |
|------|-----|-------------|
| Time | 1 | Cron-based time triggers |
| Price | 2 | Price threshold triggers |
| Event | 3 | On-chain event triggers |
| Threshold | 4 | Balance/value thresholds |

## Events

### TriggerRegistered

```csharp
event Action<BigInteger, UInt160, byte> OnTriggerRegistered;
// Parameters: triggerId, owner, triggerType
```

### TriggerExecuted

```csharp
event Action<BigInteger, bool, byte[]> OnTriggerExecuted;
// Parameters: triggerId, success, result
```

## Methods

### registerTrigger

Register a new trigger.

```csharp
public static BigInteger registerTrigger(
    byte triggerType,
    byte[] condition,
    UInt160 callbackContract,
    string callbackMethod
)
```

### executeTrigger

Execute a trigger (TEE only).

```csharp
public static void executeTrigger(BigInteger triggerId, byte[] proof)
```

### enableTrigger / disableTrigger

Toggle trigger state.

```csharp
public static void enableTrigger(BigInteger triggerId)
public static void disableTrigger(BigInteger triggerId)
```

## Integration Guide

```csharp
// Register a time-based trigger
BigInteger triggerId = NeoFlowService.registerTrigger(
    1,                    // Time type
    cronExpression,       // "0 9 * * *"
    myContractHash,       // Callback contract
    "onSchedule"          // Callback method
);
```

## Related Documentation

- [Marble Service](../marble/README.md)
- [Chain Integration](../chain/README.md)
