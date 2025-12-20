# MiniApp Platform Contract Deployment

This directory contains deployment scripts and configuration for the **Neo MiniApp Platform** contracts.

## Quick Start (Neo Express)

```bash
# 1. Setup environment (create wallets, build contracts)
make setup

# 2. Start Neo Express in another terminal
make run-neoexpress

# 3. Deploy all contracts
make deploy

# 4. Initialize contracts (register TEE, set fees, etc.)
make init

# 5. Run Go integration tests (neo-express)
make test-go
```

Or run everything at once:

```bash
make all
```

## Directory Structure

```
deploy/
├── Makefile                    # Main entry point
├── config/
│   ├── default.neo-express     # Neo Express config (auto-generated)
│   ├── deployed_contracts.json # Deployed contract addresses
│   └── testnet.json           # TestNet configuration
├── scripts/
│   ├── setup_neoexpress.sh    # Setup Neo Express environment
│   ├── deploy_all.sh          # Deploy contracts
│   ├── initialize.py          # Initialize deployed contracts
│   └── (tests live under ./test/contract)
└── wallets/
    ├── owner.json             # Admin wallet
    ├── tee.json               # TEE wallet
    └── user.json              # Test user wallet
```

## Dependencies

Install required tools:

```bash
# Neo Express (local blockchain)
dotnet tool install -g Neo.Express

# Neo Contract Compiler
dotnet tool install -g Neo.Compiler.CSharp

# Python 3 (for scripts)
# Usually pre-installed on Linux

# Check all dependencies
make check-deps
```

## Deployment Targets

### Neo Express (Local Development)

```bash
# Full setup and deployment
make all NETWORK=neoexpress

# Or step by step:
make setup
make build
make deploy
make init
```

### TestNet

1. Set environment variables:

```bash
export TESTNET_OWNER_WALLET=/path/to/owner.json
export TESTNET_OWNER_ADDRESS=<neo-n3-owner-address>
export TESTNET_TEE_WALLET=/path/to/tee.json
export TESTNET_TEE_ADDRESS=<neo-n3-tee-address>
export TESTNET_TEE_PUBKEY=<33-byte-compressed-pubkey-hex>
```

2. Ensure wallets have GAS for deployment (~50 GAS recommended)

3. Deploy:

```bash
make build
make deploy NETWORK=testnet
make init NETWORK=testnet
```

## Contract Initialization

After deployment, contracts are initialized with:

1. **Updater Configuration (TEE signer)**
   - `PriceFeed.setUpdater(tee)`
   - `RandomnessLog.setUpdater(tee)`
   - `AutomationAnchor.setUpdater(tee)`

In production, the updater should be the enclave-managed signer (GlobalSigner / TxProxy).

## Testing

### Run Go Integration Tests

```bash
make test-go
```

## neo-fairy-test Integration

This deployment system is designed to work with [neo-fairy-test](https://github.com/r3e-network/neo-fairy-test), a Foundry-style testing framework for Neo N3.

Key features used:

- **VirtualDeploy**: Deploy contracts in isolated test sessions
- **Session Snapshots**: Revert state between tests
- **Cheatcodes**: `Prank` (impersonate), `Deal` (set balance), `Warp` (set time)

This repo’s primary validation path is the Go integration tests under `test/contract`.
For ad-hoc Fairy deployments, use `go run ./cmd/deploy-fairy/main.go`.

## Deployed Contracts

After deployment, contract addresses are saved to `config/deployed_contracts.json`.

### Neo N3 Testnet (Live)

| Contract         | Address                                      | Description               |
| ---------------- | -------------------------------------------- | ------------------------- |
| PaymentHub       | `0x56730a72af8363c8da7ae61ed930accb30aefe87` | GAS payments & settlement |
| Governance       | `0x75b653ebe873d706757bab6f1166323d701da445` | NEO voting & governance   |
| PriceFeed        | `0x1d1b28a968d45bce20ef9344f605856ac5b93418` | Price oracle anchoring    |
| RandomnessLog    | `0xef79193a3a7d8c27655a7c111d4df6f8cec1d253` | Randomness anchoring      |
| AppRegistry      | `0x3e23e14355209e6f48cd24174143535ae3f211a1` | MiniApp registration      |
| AutomationAnchor | `0xde6bd82083eb52c214b50877d6a7b4e45fd806fd` | Task execution logs       |

**Network:** Neo N3 Testnet
**RPC:** `https://testnet1.neo.coz.io:443`
**Network Magic:** `894710606`

### Local Development (Neo Express)

```json
{
  "PaymentHub": "0x1234...",
  "Governance": "0xdef0...",
  "PriceFeed": "0xaaaa...",
  "RandomnessLog": "0xbbbb...",
  "AppRegistry": "0xcccc...",
  "AutomationAnchor": "0xdddd..."
}
```

## Troubleshooting

### "neoxp not found"

```bash
dotnet tool install -g Neo.Express
export PATH="$PATH:$HOME/.dotnet/tools"
```

### "nccs not found"

```bash
dotnet tool install -g Neo.Compiler.CSharp
```

### Contract deployment fails

- Ensure Neo Express is running: `make run-neoexpress`
- Check wallet has sufficient GAS
- Verify contract compiled successfully in `contracts/build/`

### Initialization fails

- Ensure contracts are deployed first
- Check `deployed_contracts.json` has valid contract hashes
