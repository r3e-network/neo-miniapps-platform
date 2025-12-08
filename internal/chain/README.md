# Chain Module

The `chain` module provides Neo N3 blockchain interaction capabilities for the Service Layer.

## Overview

This module handles all blockchain-related operations including:

- Neo N3 RPC client communication
- Smart contract interaction (invoke, deploy)
- Event listening and parsing
- Transaction fulfillment via TEE

## Components

### Client (`client.go`)

The main Neo N3 RPC client for blockchain interaction.

```go
client, err := chain.NewClient(chain.ClientConfig{
    RPCURL:     "https://mainnet1.neo.coz.io:443",
    NetworkID:  860833102, // MainNet
})
```

### Event Listener (`listener_core.go`)

Listens for on-chain events from Service Layer contracts.

```go
listener := chain.NewEventListener(client, contractHashes)
listener.OnVRFRequest(func(event *VRFRequestEvent) {
    // Handle VRF request
})
listener.Start(ctx)
```

### Contract Interfaces

| File | Contract | Purpose |
|------|----------|---------|
| `contracts_gateway.go` | ServiceLayerGateway | Main entry point for all services |
| `contracts_vrf.go` | VRFService | Verifiable random function |
| `contracts_mixer.go` | MixerService | Privacy mixing |
| `contracts_datafeeds.go` | DataFeedsService | Price oracle |
| `contracts_automation.go` | AutomationService | Task automation |

### TEE Fulfiller (`contracts_fulfiller.go`)

Executes on-chain callbacks from within the TEE enclave.

```go
fulfiller := chain.NewTEEFulfiller(client, privateKey)
txHash, err := fulfiller.FulfillVRF(ctx, requestID, randomWords, proof)
```

### Event Parsers (`contracts_parsers.go`)

Parses Neo N3 notification events into typed structures.

## Event Types

### VRF Events (`listener_events_vrf.go`)

- `RandomnessRequested` - New VRF request
- `RandomnessFulfilled` - VRF request completed

### Mixer Events (`listener_events_mixer.go`)

- `MixRequested` - New mix request
- `MixCompleted` - Mix completed
- `DisputeSubmitted` - Dispute filed

### DataFeeds Events (`listener_events_datafeeds.go`)

- `PriceUpdated` - Price feed updated
- `FeedConfigured` - Feed configuration changed

### Automation Events (`listener_events_automation.go`)

- `TriggerRegistered` - New trigger registered
- `TriggerExecuted` - Trigger executed
- `TriggerDisabled` - Trigger disabled

## Usage Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/internal/chain"
)

func main() {
    // Create client
    client, _ := chain.NewClient(chain.ClientConfig{
        RPCURL: "https://mainnet1.neo.coz.io:443",
    })

    // Create VRF contract interface
    vrfContract := chain.NewVRFContract(client, "0x1234...")

    // Get pending requests
    requests, _ := vrfContract.GetPendingRequests(context.Background())

    // Create fulfiller for TEE operations
    fulfiller := chain.NewTEEFulfiller(client, teePrivateKey)

    // Fulfill request
    for _, req := range requests {
        fulfiller.FulfillVRF(context.Background(), req.ID, randomWords, proof)
    }
}
```

## Testing

```bash
go test ./internal/chain/... -v
```

## Dependencies

- `github.com/nspcc-dev/neo-go` - Neo N3 Go SDK
