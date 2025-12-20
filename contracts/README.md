# Neo N3 Smart Contracts

This directory contains the **Neo N3 MiniApp Platform** contracts. The platform
contracts enforce the blueprint constraints:

- **Payments/settlement:** GAS only
- **Governance:** NEO only

All sensitive invocations are expected to flow through the SDK intent + Edge
gateway + TEE services, with final enforcement at the contract layer.

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────┐
│                       Gateway + SDK                            │
│         (Supabase Edge + Host SDK intent flow)                  │
└────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│                    TEE Service Layer (EGo)                     │
│   datafeed / compute / automation / tx-proxy (attested TLS)     │
└────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│                    Platform Contracts (C#)                     │
│   PaymentHub · Governance · PriceFeed · RandomnessLog          │
│   AppRegistry · AutomationAnchor                               │
└────────────────────────────────────────────────────────────────┘
```

## Platform Contracts

| Contract             | Source                                 | Description                                        |
| -------------------- | -------------------------------------- | -------------------------------------------------- |
| **PaymentHub**       | `PaymentHub/PaymentHub.cs`             | GAS-only payments with configurable revenue splits |
| **Governance**       | `Governance/Governance.cs`             | NEO staking and proposal voting                    |
| **PriceFeed**        | `PriceFeed/PriceFeed.cs`               | Oracle price data with TEE attestation             |
| **RandomnessLog**    | `RandomnessLog/RandomnessLog.cs`       | Verifiable randomness with TEE attestation         |
| **AppRegistry**      | `AppRegistry/AppRegistry.cs`           | MiniApp manifest and status registry               |
| **AutomationAnchor** | `AutomationAnchor/AutomationAnchor.cs` | Task scheduling with nonce-based anti-replay       |

## Custom MiniApp Contracts

The platform **does not ship** on-chain MiniApp contracts. Built-in MiniApps
use the shared platform contracts (`PaymentHub`, `RandomnessLog`, `PriceFeed`)
and the SDK intent flow. Developers are free to deploy additional contracts, but
must:

- enforce **GAS-only** settlement or **NEO-only** governance where applicable
- register any on-chain dependencies in `manifest.contracts_needed`
- use `tx-proxy` allowlists for any enclave-origin writes

## Common Contract Features

All platform contracts include:

| Feature          | Method                      | Description                   |
| ---------------- | --------------------------- | ----------------------------- |
| Admin Transfer   | `SetAdmin(newAdmin)`        | Transfer admin to new address |
| Contract Upgrade | `Update(nefFile, manifest)` | Upgrade contract code         |
| Admin Validation | `ValidateAdmin()`           | Internal admin check          |

## Build

### Prerequisites

Install the Neo C# compiler:

```bash
dotnet tool install -g Neo.Compiler.CSharp
```

### Build All Platform Contracts

```bash
cd contracts
./build.sh
```

### Build Outputs

- `contracts/build/*.nef` - Contract bytecode
- `contracts/build/*.manifest.json` - Contract manifest

## Deploy & Initialize

### Local Development (Neo Express)

```bash
make -C deploy setup
make -C deploy run-neoexpress
make -C deploy deploy
make -C deploy init
```

### Testnet Deployment (Simulation + CLI)

```bash
# Requires: export NEO_TESTNET_WIF=...
go run ./cmd/deploy-testnet/main.go --check
go run ./cmd/deploy-testnet/main.go --estimate
```

The deployer writes simulated results to `deploy/config/testnet_contracts.json`.
For actual deployment, use `neo-go contract deploy` with the compiled `.nef` and
`.manifest.json` files, then call:

- `PriceFeed.SetUpdater(teeSigner)`
- `RandomnessLog.SetUpdater(teeSigner)`
- `AutomationAnchor.SetUpdater(teeSigner)`

## Security Considerations

1. **Admin Keys**: Store admin private keys securely, preferably in TEE.
2. **Signer Setup**: Always set TEE/Automation signers before going live.
3. **Upgradability**: Use `Update()` carefully; it requires admin witness.
4. **Randomness**: Never derive randomness from chain data; always use TEE output.
5. **Price Feeds**: Validate source freshness and enforce monotonic round IDs.
