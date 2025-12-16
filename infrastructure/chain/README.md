# Chain Module

The `infrastructure/chain` module is the **single source of truth** for Neo N3
chain communication in this repository.

It centralizes:

- Neo JSON-RPC client + endpoint pooling/failover
- contract invocation helpers (invoke + wait)
- transaction construction + signing helpers
- event monitoring/listening and typed event parsing
- TEE callback/fulfillment transaction helpers (Gateway callbacks, datafeeds, automation)

## Key Components

### RPC Client (`client.go`, `rpcpool.go`)

Create a Neo N3 client:

```go
client, err := chain.NewClient(chain.Config{
    RPCURL:    "https://testnet1.neo.coz.io:443",
    NetworkID: 894710606,
})
```

### Contract Addresses (`contracts_common.go`)

Contract hashes are typically provided via env vars. The helper supports
**new names + legacy fallbacks**:

- `CONTRACT_DATAFEEDS_HASH` (fallback: `CONTRACT_NEOFEEDS_HASH`)
- `CONTRACT_AUTOMATION_HASH` (fallback: `CONTRACT_NEOFLOW_HASH`)
- `CONTRACT_CONFIDENTIAL_HASH` (fallback: `CONTRACT_NEOCOMPUTE_HASH`)
- `CONTRACT_ORACLE_HASH` (fallback: `CONTRACT_NEOORACLE_HASH`)

```go
contracts := chain.ContractAddressesFromEnv()
```

### Event Listener (`listener_core.go`)

The listener polls Neo RPC for application logs and emits typed events for the
Service Layer contracts. Contract hashes are normalized (strip `0x`, lowercase)
before filtering, to avoid silent mismatches between configs and RPC output.

```go
listener := chain.NewEventListener(&chain.ListenerConfig{
    Client:     client,
    Contracts:  contracts,
    StartBlock: 0, // or a saved cursor
})
go listener.Start(ctx)
```

### TEE Fulfiller (`contracts_fulfiller.go`)

`TEEFulfiller` builds, signs, and broadcasts “service layer write” transactions
back to the Gateway or service contracts.

Common operations:

- `FulfillRequest` / `FailRequest` (Gateway callback pattern)
- `UpdatePrice` / `UpdatePrices` (datafeeds push pattern)
- `ExecuteTrigger` (automation pattern)
- `SetTEEMasterKey` (anchor master pubkey + attestation hash to the Gateway)

```go
fulfiller, err := chain.NewTEEFulfiller(client, contracts.Gateway, teePrivKeyHex)
if err != nil {
    // handle
}

txHash, err := fulfiller.FulfillRequest(ctx, requestID, resultBytes)
_ = txHash
```

## Responsibility Rules

- Services should **not** talk to Neo RPC directly.
- Contract bindings and typed event parsing live in `infrastructure/chain`
  (`contracts_*.go`, `listener_events_*.go`) so services don't duplicate chain I/O.

## Testing

```bash
go test ./infrastructure/chain/... -v
```
