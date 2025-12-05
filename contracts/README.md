# Service Layer Smart Contracts

Neo N3 smart contracts for the Service Layer platform.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Contracts                           │
│  (ExampleConsumer, DApps, etc.)                            │
└─────────────────────┬───────────────────────────────────────┘
                      │ Request()
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              ServiceLayerGateway                            │
│  • Central entry point for all requests                     │
│  • Gas management (deposit/withdraw)                        │
│  • Request routing and validation                           │
│  • Callback delivery                                        │
└─────────────────────┬───────────────────────────────────────┘
                      │ RegisterService()
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  Service Contracts                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  Oracle  │ │   VRF    │ │DataFeeds │ │Automation│       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│  │ Secrets  │ │ GasBank  │ │  Mixer   │                    │
│  └──────────┘ └──────────┘ └──────────┘                    │
└─────────────────────────────────────────────────────────────┘
                      ▲
                      │ Callback()
┌─────────────────────┴───────────────────────────────────────┐
│              Service Layer (TEE)                            │
│  • Monitors ServiceRequest events                           │
│  • Processes requests in TEE                                │
│  • Delivers callbacks with TEE signature                    │
└─────────────────────────────────────────────────────────────┘
```

## Contracts

| Contract | Description |
|----------|-------------|
| ServiceLayerGateway | Central gateway for all service requests |
| OracleService | External data oracle service |
| VRFService | Verifiable random function service |
| DataFeedsService | Price and data feeds service |
| AutomationService | Scheduled task automation |
| SecretsService | Secure secrets management |
| GasBankService | Gas sponsorship service |

## Quick Start

### Prerequisites

```bash
# Install .NET SDK 9.0+
# https://dotnet.microsoft.com/download

# Install Neo tools
dotnet tool install -g Neo.Express
dotnet tool install -g Neo.Compiler.CSharp

# Verify installation
neoxp --version
nccs --version
```

### Build Contracts

```bash
# Build all contracts
./bin/contract-cli build -all

# Build specific contract
./bin/contract-cli build -contract ServiceLayerGateway
```

### Deploy to Neo-Express (Local Development)

```bash
# Start neo-express
./bin/contract-cli express -action start

# Deploy all contracts
./bin/contract-cli deploy -network neo-express -init

# Check status
./bin/contract-cli status
```

### Deploy to TestNet

```bash
# Configure wallet in config/testnet-wallet.json
./bin/contract-cli deploy -network testnet -init
```

### Deploy to MainNet

```bash
# Configure wallet in config/mainnet-wallet.json
./bin/contract-cli deploy -network mainnet -init
```

## Testing

### Run Full Integration Tests

```bash
# This will:
# 1. Start neo-express
# 2. Build all contracts
# 3. Deploy to neo-express
# 4. Run integration tests
./scripts/run_contract_tests.sh full
```

### Run Individual Test Steps

```bash
# Build only
./scripts/run_contract_tests.sh build

# Deploy only
./scripts/run_contract_tests.sh deploy

# Run tests only
./scripts/run_contract_tests.sh test

# Run unit tests
./scripts/run_contract_tests.sh unit
```

### Using Makefile

```bash
cd contracts

# Build all
make build

# Deploy to neo-express
make deploy-express

# Run integration tests
make test-integration

# Full development cycle
make dev
```

## Contract CLI Reference

```bash
# Show help
./bin/contract-cli help

# Build commands
./bin/contract-cli build -all
./bin/contract-cli build -contract <name>

# Deploy commands
./bin/contract-cli deploy -network <neo-express|testnet|mainnet> [-contract <name>] [-init]

# Initialize commands
./bin/contract-cli init -network <name> [-gateway <hash>] [-service-layer <addr>]

# Test commands
./bin/contract-cli test -integration
./bin/contract-cli test -unit

# Neo-express management
./bin/contract-cli express -action <start|stop|reset|checkpoint>

# Status
./bin/contract-cli status
```

## Request Flow

1. **User deposits GAS** to Gateway contract
2. **User contract calls** `Gateway.Request(serviceId, callback, payload, gasLimit)`
3. **Gateway emits** `ServiceRequest` event
4. **Service Layer (TEE)** monitors events and processes request
5. **Service Layer calls** `Gateway.Callback(requestId, success, result, signature)`
6. **Gateway calls** user's callback method with result
7. **Unused GAS** is refunded to user's balance

## Example: Oracle Request

```csharp
// In your contract
public static void RequestPrice(string symbol)
{
    var gateway = (UInt160)"0x..."; // Gateway contract hash
    var payload = StdLib.Serialize(new { url = "https://api.example.com/price", symbol = symbol });

    Contract.Call(gateway, "request", CallFlags.All, new object[] {
        "oracle",           // serviceId
        Runtime.ExecutingScriptHash, // callback contract
        "onPriceReceived",  // callback method
        payload,            // request data
        1_00000000          // gas limit (1 GAS)
    });
}

public static void OnPriceReceived(ByteString requestId, ByteString result)
{
    // Handle the oracle response
    var price = StdLib.Deserialize(result);
    // ...
}
```

## Security

- All service contracts inherit from `ServiceLayerBase` with access control
- Only admin can configure gateway and service layer addresses
- Only gateway can call service-specific methods
- Only service layer (TEE) can deliver callbacks
- Contracts can be paused by admin in emergencies

## Development

### Project Structure

```
contracts/
├── Common/
│   └── ServiceLayerBase.cs      # Base class for all contracts
├── ServiceLayerGateway/
│   ├── ServiceLayerGateway.cs   # Gateway contract
│   └── ServiceLayerGateway.csproj
├── OracleService/
│   ├── OracleService.cs
│   └── OracleService.csproj
├── VRFService/
│   ├── VRFService.cs
│   └── VRFService.csproj
├── DataFeedsService/
├── AutomationService/
├── SecretsService/
├── GasBankService/
├── Examples/
│   └── ExampleConsumer.cs       # Example consumer contract
├── build/                       # Compiled contracts (.nef, .manifest.json)
├── Makefile
└── README.md
```

### Adding a New Service

1. Create new directory: `contracts/NewService/`
2. Create contract file inheriting from `ServiceLayerBase`
3. Create `.csproj` file
4. Add to `contracts` list in `tools/contract-cli/main.go`
5. Build and deploy

## License

MIT License - R3E Network
