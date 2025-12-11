# Service Layer Contract Deployment

This directory contains deployment scripts and configuration for Service Layer smart contracts using the neo-fairy-test framework approach.

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

# 5. Run tests
make test
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
│   └── run_tests.py           # Run contract tests
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
export TESTNET_OWNER_ADDRESS=NOwnerAddressXXX
export TESTNET_TEE_WALLET=/path/to/tee.json
export TESTNET_TEE_ADDRESS=NTeeAddressXXX
export TESTNET_TEE_PUBKEY=03xxx
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

1. **Gateway Configuration**
   - Register TEE account and public key
   - Set service fees (Oracle: 0.1 GAS, VRF: 0.1 GAS, NeoVault: 0.5 GAS, etc.)
   - Register service contracts (VRF, NeoVault, NeoFeeds, NeoFlow)

2. **Service Contract Configuration**
   - Set Gateway address on each service contract

3. **Example Contract Configuration**
   - Set Gateway address
   - Set NeoFeeds address (for DeFiPriceConsumer)

## Testing

### Run All Tests
```bash
make test
```

### Run Specific Service Tests
```bash
make test-neorand
make test-neovault
make test-neofeeds
```

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

The `run_tests.py` script implements a compatible testing API.

## Deployed Contracts

After deployment, contract addresses are saved to `config/deployed_contracts.json`:

```json
{
  "ServiceLayerGateway": "0x1234...",
  "VRFService": "0x5678...",
  "NeoVaultService": "0x9abc...",
  "NeoFeedsService": "0xdef0...",
  "NeoFlowService": "0x1234...",
  "VRFLottery": "0x5678...",
  "NeoVaultClient": "0x9abc...",
  "DeFiPriceConsumer": "0xdef0..."
}
```

## Service Fees

Default fees (configurable in `initialize.py`):

| Service | Fee (GAS) |
|---------|-----------|
| Oracle | 0.1 |
| VRF | 0.1 |
| NeoVault | 0.5 |
| NeoFeeds | 0.05 |
| NeoFlow | 0.2 |
| NeoCompute | 1.0 |

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
