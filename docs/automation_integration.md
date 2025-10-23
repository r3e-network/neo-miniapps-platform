# Contract Automation Integration Guide

> **Legacy notice:** The automation flows described here refer to the original
> Service Layer implementation focused on the refactored runtime.
> API surface under `internal/app/httpapi`.

This guide explains how to use the Contract Automation service to integrate with Neo N3 smart contracts.

## Overview

The Contract Automation service allows you to:

1. Set up triggers to execute JavaScript functions based on various events
2. Automate Neo N3 smart contract interactions
3. Respond to Neo N3 blockchain events

## Trigger Types

The service supports three types of triggers:

### 1. Cron Triggers

Time-based triggers using cron syntax to execute functions on a schedule.

```json
{
  "trigger_type": "cron",
  "trigger_config": {
    "schedule": "0 0 * * *",  // Daily at midnight
    "timezone": "UTC"
  }
}
```

### 2. Price Triggers

Triggers that execute functions when token prices cross certain thresholds.

```json
{
  "trigger_type": "price",
  "trigger_config": {
    "asset_pair": "NEO/USD",
    "condition": "above",
    "threshold": 50.0,
    "duration": 300  // Must be above threshold for 5 minutes
  }
}
```

### 3. Blockchain Triggers

Triggers that execute functions in response to Neo N3 blockchain events.

```json
{
  "trigger_type": "blockchain",
  "trigger_config": {
    "contract_hash": "0x1234567890abcdef1234567890abcdef12345678",
    "event_name": "Transfer"
  }
}
```

## Neo N3 Smart Contract Integration

### Step 1: Deploy Your Smart Contract

Deploy your Neo N3 smart contract with events that you want to monitor.

Example contract with events (in C#):

```csharp
[DisplayName("MyContract")]
public class MyContract : SmartContract
{
    // Define event
    [DisplayName("Transfer")]
    public static event Action<UInt160, UInt160, BigInteger> OnTransfer;

    // Transfer method that emits the event
    public static bool Transfer(UInt160 from, UInt160 to, BigInteger amount)
    {
        // Implementation logic here
        
        // Emit event
        OnTransfer(from, to, amount);
        return true;
    }
}
```

### Step 2: Create a JavaScript Function

Create a function that will be executed when the trigger fires:

```bash
curl -X POST https://api.servicelayer.neo.org/v1/functions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "handleTransfer",
    "description": "Handle Transfer event from Neo N3 contract",
    "source_code": "function process(params) { \n  // Get event data from trigger \n  const contractHash = params.contract_hash; \n  const eventName = params.event_name; \n  // Process the event \n  console.log(`Processing ${eventName} event from ${contractHash}`); \n  return { processed: true }; \n}",
    "timeout": 30,
    "memory": 128
  }'
```

### Step 3: Create a Blockchain Trigger

Create a trigger that listens for events from your Neo N3 smart contract:

```bash
curl -X POST https://api.servicelayer.neo.org/v1/triggers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "MonitorTransfers",
    "description": "Monitor Transfer events from our contract",
    "function_id": 123,
    "trigger_type": "blockchain",
    "trigger_config": {
      "contract_hash": "0x1234567890abcdef1234567890abcdef12345678",
      "event_name": "Transfer"
    }
  }'
```

### Step 4: Verify Trigger Creation

Check that your trigger was created successfully:

```bash
curl -X GET https://api.servicelayer.neo.org/v1/triggers \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Step 5: Testing the Trigger

You can manually test the trigger:

```bash
curl -X POST https://api.servicelayer.neo.org/v1/triggers/123/execute \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## Common Neo N3 Integration Patterns

### 1. Daily Token Balance Updates

Create a cron trigger that executes a function daily to check token balances and update your contract.

```bash
# Create function
curl -X POST https://api.servicelayer.neo.org/v1/functions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "updateTokenBalances",
    "description": "Update token balances daily",
    "source_code": "function process(params) { \n  // Get NEO N3 wallet addresses \n  const addresses = ['address1', 'address2']; \n  const results = []; \n  // Check balances for each address \n  for (const address of addresses) { \n    // Logic to check balance \n    results.push({ address, balance: 100 }); \n  } \n  return { balances: results }; \n}",
    "timeout": 60,
    "memory": 128
  }'

# Create trigger
curl -X POST https://api.servicelayer.neo.org/v1/triggers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "DailyBalanceCheck",
    "description": "Check token balances every day at 1 AM",
    "function_id": 124,
    "trigger_type": "cron",
    "trigger_config": {
      "schedule": "0 1 * * *",
      "timezone": "UTC"
    }
  }'
```

### 2. Price-Based Contract Updates

Create a price trigger that executes a function when the NEO price changes significantly.

```bash
# Create function
curl -X POST https://api.servicelayer.neo.org/v1/functions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "updatePriceData",
    "description": "Update price data in contract when NEO price changes",
    "source_code": "function process(params) { \n  const assetPair = params.asset_pair; \n  const threshold = params.threshold; \n  console.log(`${assetPair} price crossed ${threshold}`); \n  // Logic to update contract with new price \n  return { updated: true }; \n}",
    "timeout": 30,
    "memory": 128
  }'

# Create trigger
curl -X POST https://api.servicelayer.neo.org/v1/triggers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "NEOPriceMonitor",
    "description": "Monitor NEO price and update when it goes above $50",
    "function_id": 125,
    "trigger_type": "price",
    "trigger_config": {
      "asset_pair": "NEO/USD",
      "condition": "above",
      "threshold": 50.0,
      "duration": 300
    }
  }'
```

### 3. Event-Based Smart Contract Interaction

Create a blockchain trigger that executes a function when a specific event occurs on the Neo N3 blockchain.

```bash
# Create function
curl -X POST https://api.servicelayer.neo.org/v1/functions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "processTransfer",
    "description": "Process Transfer events and update another contract",
    "source_code": "function process(params) { \n  const contractHash = params.contract_hash; \n  const eventName = params.event_name; \n  // Logic to react to the transfer event \n  // Could include calling another contract \n  return { processed: true }; \n}",
    "timeout": 30,
    "memory": 128
  }'

# Create trigger
curl -X POST https://api.servicelayer.neo.org/v1/triggers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "TransferMonitor",
    "description": "Monitor Transfer events from a specific contract",
    "function_id": 126,
    "trigger_type": "blockchain",
    "trigger_config": {
      "contract_hash": "0x1234567890abcdef1234567890abcdef12345678",
      "event_name": "Transfer"
    }
  }'
```

## Best Practices

### Error Handling

Always include proper error handling in your JavaScript functions to ensure they handle unexpected situations gracefully:

```javascript
function process(params) {
  try {
    // Your logic here
    return { success: true, data: result };
  } catch (error) {
    console.error(`Error processing event: ${error.message}`);
    return { success: false, error: error.message };
  }
}
```

### Idempotency

Design your functions to be idempotent, meaning they can be executed multiple times without causing unintended side effects:

```javascript
function process(params) {
  // Check if we've already processed this event
  const eventId = params.event_id;
  // Logic to check if event was already processed
  
  // If already processed, return early
  if (alreadyProcessed(eventId)) {
    return { already_processed: true };
  }
  
  // Process the event
  // ...
  
  // Mark as processed
  markAsProcessed(eventId);
  
  return { processed: true };
}
```

### Gas Management

Be mindful of gas costs when making Neo N3 blockchain transactions:

```javascript
function process(params) {
  // Check if there's enough gas before making a transaction
  const gasBalance = checkGasBalance();
  const estimatedGas = estimateGasForOperation();
  
  if (gasBalance < estimatedGas) {
    return { success: false, error: "Insufficient gas" };
  }
  
  // Proceed with transaction
  // ...
  
  return { success: true };
}
```

## Monitoring and Troubleshooting

### View Trigger History

Get the execution history for a trigger:

```bash
curl -X GET https://api.servicelayer.neo.org/v1/triggers/123/history \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Check Function Execution Logs

Get the logs for a specific function execution:

```bash
curl -X GET https://api.servicelayer.neo.org/v1/accounts/123/functions/executions/456 \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Disable a Trigger

If a trigger is causing issues, you can update its status to inactive:

```bash
curl -X PUT https://api.servicelayer.neo.org/v1/triggers/123 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "name": "MonitorTransfers",
    "description": "Monitor Transfer events from our contract",
    "function_id": 123,
    "trigger_type": "blockchain",
    "trigger_config": {
      "contract_hash": "0x1234567890abcdef1234567890abcdef12345678",
      "event_name": "Transfer"
    },
    "status": "inactive"
  }'
```
