# Neo N3 Smart Contracts

This directory contains the Neo N3 smart contracts used by the Service Layer.

The current contract set is designed around a single entry-point contract (`ServiceLayerGateway`) that:

- routes user requests to service contracts,
- validates TEE signatures for fulfillments/failures,
- manages fees and replay protection.

## Contracts (Current)

| Contract | Display Name | Source | Notes |
|---|---|---|---|
| Gateway | `ServiceLayerGateway` | `contracts/gateway/ServiceLayerGateway.cs` | Main entry point for service requests + callbacks |
| Data Feeds | `DataFeedsService` | `services/datafeed/contract/NeoFeedsService*.cs` | Push/auto-update pattern |
| Automation | `NeoFlowService` | `services/automation/contract/NeoFlowService*.cs` | Trigger registration + execution |
| Conf Compute | `ConfidentialService` | `services/confcompute/contract/NeoComputeService*.cs` | Request/response pattern |
| Conf Oracle | `OracleService` | `services/conforacle/contract/NeoOracleService*.cs` | Request/response pattern |

Example consumer contracts:

- `contracts/examples/ExampleConsumer.cs`
- `contracts/examples/DeFiPriceConsumer.cs`

## Patterns

 - **Request/Response** (Oracle, Confidential): user contract calls `ServiceLayerGateway.RequestService(...)` → service contract emits event → enclave processes → `ServiceLayerGateway.FulfillRequest(...)` → user callback.
- **Push / Auto-Update** (DataFeeds): enclave periodically updates on-chain feed state.
- **Triggers** (NeoFlow): user registers triggers → enclave evaluates conditions → executes callbacks.

## Build

Prereq: install the Neo C# compiler:

```bash
dotnet tool install -g Neo.Compiler.CSharp
```

Build all contracts:

```bash
cd contracts
./build.sh
```

Build outputs:

- `contracts/build/*.nef`
- `contracts/build/*.manifest.json`

## Deploy & Initialize

Local (Neo Express) deployment helpers live under `deploy/`:

```bash
make -C deploy setup
make -C deploy run-neoexpress
make -C deploy deploy
make -C deploy init
```

For details and testnet notes, see `deploy/README.md`.
