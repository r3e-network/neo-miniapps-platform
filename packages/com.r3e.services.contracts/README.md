# Contracts Service

Smart contract deployment, invocation, and lifecycle management service for the R3E Service Layer platform.

## Overview

The Contracts Service provides comprehensive smart contract management across multiple blockchain networks. It follows an Android OS-inspired architecture where the system provides core infrastructure contracts (engine contracts), services can register their own contracts (service contracts), and users can deploy custom contracts through the SDK.

**Package ID**: `com.r3e.services.contracts`
**Service Name**: `contracts`
**Domain**: `contracts`

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Contracts Service                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Contract   │  │   Template   │  │   Binding    │         │
│  │  Management  │  │  Management  │  │  Management  │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
│         │                  │                  │                 │
│         └──────────────────┴──────────────────┘                 │
│                            │                                    │
│         ┌──────────────────┴──────────────────┐                │
│         │                                      │                │
│  ┌──────▼───────┐                    ┌────────▼────────┐       │
│  │   Deployer   │                    │     Invoker     │       │
│  │  (External)  │                    │   (External)    │       │
│  └──────┬───────┘                    └────────┬────────┘       │
│         │                                      │                │
└─────────┼──────────────────────────────────────┼────────────────┘
          │                                      │
          ▼                                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Blockchain Networks                          │
│  Neo N3 │ Neo X │ Ethereum │ Polygon │ Arbitrum │ Optimism     │
└─────────────────────────────────────────────────────────────────┘
```

## Key Components

### Service (`service.go`)

Core service implementation that orchestrates contract operations:

- **Contract Lifecycle**: Create, update, retrieve, and list contracts
- **Deployment Management**: Deploy contracts to blockchain networks
- **Invocation Handling**: Execute contract methods with gas management
- **Template Operations**: Manage reusable contract templates
- **Service Bindings**: Link services to on-chain contracts
- **Network Configuration**: Multi-chain network management

### Store Interface (`store.go`)

Persistence layer abstraction for all contract-related data:

```go
type Store interface {
    // Contract operations
    CreateContract(ctx context.Context, c Contract) (Contract, error)
    UpdateContract(ctx context.Context, c Contract) (Contract, error)
    GetContract(ctx context.Context, id string) (Contract, error)
    GetContractByAddress(ctx context.Context, network Network, address string) (Contract, error)
    ListContracts(ctx context.Context, accountID string) ([]Contract, error)
    ListContractsByService(ctx context.Context, serviceID string) ([]Contract, error)
    ListContractsByNetwork(ctx context.Context, network Network) ([]Contract, error)

    // Template operations
    CreateTemplate(ctx context.Context, t Template) (Template, error)
    UpdateTemplate(ctx context.Context, t Template) (Template, error)
    GetTemplate(ctx context.Context, id string) (Template, error)
    ListTemplates(ctx context.Context, category TemplateCategory) ([]Template, error)
    ListTemplatesByService(ctx context.Context, serviceID string) ([]Template, error)

    // Deployment operations
    CreateDeployment(ctx context.Context, d Deployment) (Deployment, error)
    UpdateDeployment(ctx context.Context, d Deployment) (Deployment, error)
    GetDeployment(ctx context.Context, id string) (Deployment, error)
    ListDeployments(ctx context.Context, contractID string, limit int) ([]Deployment, error)

    // Invocation operations
    CreateInvocation(ctx context.Context, inv Invocation) (Invocation, error)
    UpdateInvocation(ctx context.Context, inv Invocation) (Invocation, error)
    GetInvocation(ctx context.Context, id string) (Invocation, error)
    ListInvocations(ctx context.Context, contractID string, limit int) ([]Invocation, error)
    ListAccountInvocations(ctx context.Context, accountID string, limit int) ([]Invocation, error)

    // Service binding operations
    CreateServiceBinding(ctx context.Context, b ServiceContractBinding) (ServiceContractBinding, error)
    GetServiceBinding(ctx context.Context, id string) (ServiceContractBinding, error)
    ListServiceBindings(ctx context.Context, serviceID string) ([]ServiceContractBinding, error)
    ListAccountBindings(ctx context.Context, accountID string) ([]ServiceContractBinding, error)

    // Network config operations
    GetNetworkConfig(ctx context.Context, network Network) (NetworkConfig, error)
    ListNetworkConfigs(ctx context.Context) ([]NetworkConfig, error)
    SaveNetworkConfig(ctx context.Context, cfg NetworkConfig) (NetworkConfig, error)
}
```

### Deployer Interface

External blockchain deployment handler:

```go
type Deployer interface {
    Deploy(ctx context.Context, deployment Deployment) (Deployment, error)
}
```

### Invoker Interface

External blockchain invocation handler:

```go
type Invoker interface {
    Invoke(ctx context.Context, invocation Invocation) (Invocation, error)
}
```

## Domain Types

### Contract Types

- **Engine Contracts**: Core Service Layer infrastructure (AccountManager, GasBank, OracleHub, etc.)
- **Service Contracts**: Per-service contracts registered by individual services
- **User Contracts**: Custom contracts deployed by users through SDK

### Contract

Represents a deployed or deployable smart contract:

```go
type Contract struct {
    ID           string            // Unique contract identifier
    AccountID    string            // Owner account ID
    ServiceID    string            // Associated service (optional)
    Name         string            // Contract name
    Symbol       string            // Contract symbol (optional)
    Description  string            // Human-readable description
    Type         ContractType      // engine | service | user
    Network      Network           // Target blockchain network
    Address      string            // On-chain address (after deployment)
    CodeHash     string            // Bytecode hash
    ConfigHash   string            // Configuration hash
    Version      string            // Contract version
    ABI          string            // Contract ABI (JSON)
    Bytecode     string            // Compiled bytecode
    SourceHash   string            // Source code hash
    Status       ContractStatus    // Lifecycle status
    Capabilities []string          // Supported capabilities
    DependsOn    []string          // Contract dependencies
    Metadata     map[string]string // Additional metadata
    Tags         []string          // Searchable tags
    DeployedAt   time.Time         // Deployment timestamp
    CreatedAt    time.Time         // Creation timestamp
    UpdatedAt    time.Time         // Last update timestamp
}
```

### Contract Status Lifecycle

```
draft → deploying → active → paused → upgrading → deprecated → revoked
                      ↓
                   active
```

- **draft**: Contract registered but not deployed
- **deploying**: Deployment in progress
- **active**: Contract deployed and operational
- **paused**: Contract temporarily disabled
- **upgrading**: Contract upgrade in progress
- **deprecated**: Contract marked for replacement
- **revoked**: Contract permanently disabled

### Template

Reusable contract template for rapid deployment:

```go
type Template struct {
    ID           string            // Template identifier
    ServiceID    string            // Owning service (optional)
    Name         string            // Template name
    Symbol       string            // Token symbol (if applicable)
    Description  string            // Template description
    Category     TemplateCategory  // Template category
    Networks     []Network         // Supported networks
    Version      string            // Template version
    ABI          string            // Contract ABI
    Bytecode     string            // Compiled bytecode
    SourceCode   string            // Source code (optional)
    SourceLang   string            // Source language (solidity, python, etc.)
    CodeHash     string            // Bytecode hash
    Audited      bool              // Audit status
    AuditReport  string            // Audit report URL
    Params       []TemplateParam   // Constructor parameters
    Capabilities []string          // Supported capabilities
    DependsOn    []string          // Template dependencies
    Metadata     map[string]string // Additional metadata
    Tags         []string          // Searchable tags
    Status       TemplateStatus    // Template status
    CreatedAt    time.Time         // Creation timestamp
    UpdatedAt    time.Time         // Last update timestamp
}
```

### Template Categories

- **engine**: Core engine contracts
- **token**: Token contracts (ERC20, NEP17, etc.)
- **oracle**: Oracle contracts
- **vrf**: Verifiable Random Function contracts
- **feed**: Data feed contracts
- **defi**: DeFi protocol contracts
- **vault**: Vault and custody contracts
- **stake**: Staking contracts
- **proxy**: Proxy and upgrade contracts
- **multisig**: Multi-signature contracts
- **governance**: Governance contracts
- **custom**: Custom user templates

### Deployment

Tracks contract deployment operations:

```go
type Deployment struct {
    ID              string            // Deployment identifier
    AccountID       string            // Account performing deployment
    ContractID      string            // Contract being deployed
    Network         Network           // Target network
    Bytecode        string            // Deployment bytecode
    ConstructorArgs map[string]any    // Constructor arguments
    GasLimit        int64             // Gas limit
    GasUsed         int64             // Actual gas used
    GasPrice        string            // Gas price
    TxHash          string            // Transaction hash
    Address         string            // Deployed contract address
    BlockNumber     int64             // Block number
    Status          DeploymentStatus  // pending | submitted | confirmed | failed
    Error           string            // Error message (if failed)
    Metadata        map[string]string // Additional metadata
    SubmittedAt     time.Time         // Submission timestamp
    ConfirmedAt     time.Time         // Confirmation timestamp
    CreatedAt       time.Time         // Creation timestamp
    UpdatedAt       time.Time         // Last update timestamp
}
```

### Invocation

Records contract method invocations:

```go
type Invocation struct {
    ID          string            // Invocation identifier
    AccountID   string            // Account performing invocation
    ContractID  string            // Target contract
    MethodName  string            // Method name
    Args        map[string]any    // Method arguments
    GasLimit    int64             // Gas limit
    GasUsed     int64             // Actual gas used
    GasPrice    string            // Gas price
    Value       string            // Native token value sent
    TxHash      string            // Transaction hash
    BlockNumber int64             // Block number
    BlockHash   string            // Block hash
    Status      InvocationStatus  // pending | submitted | confirmed | failed | reverted
    Result      any               // Method return value
    Error       string            // Error message (if failed)
    Logs        []EventLog        // Emitted events
    Metadata    map[string]string // Additional metadata
    SubmittedAt time.Time         // Submission timestamp
    ConfirmedAt time.Time         // Confirmation timestamp
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

### ServiceContractBinding

Links services to on-chain contracts:

```go
type ServiceContractBinding struct {
    ID         string            // Binding identifier
    ServiceID  string            // Service identifier
    AccountID  string            // Account identifier
    ContractID string            // Contract identifier
    Network    Network           // Network
    Role       string            // Service role (consumer, provider, etc.)
    Enabled    bool              // Binding status
    Metadata   map[string]string // Additional metadata
    CreatedAt  time.Time         // Creation timestamp
    UpdatedAt  time.Time         // Last update timestamp
}
```

### NetworkConfig

Network-specific configuration:

```go
type NetworkConfig struct {
    Network         Network           // Network identifier
    ChainID         int64             // Chain ID
    RPCEndpoint     string            // RPC endpoint URL
    WSEndpoint      string            // WebSocket endpoint URL
    ExplorerURL     string            // Block explorer URL
    NativeToken     string            // Native token symbol
    NativeDecimals  int               // Native token decimals
    BlockTime       int               // Average block time (seconds)
    Confirmations   int               // Required confirmations
    EngineContracts map[string]string // Engine contract addresses
    Metadata        map[string]string // Additional metadata
    Enabled         bool              // Network status
}
```

## Supported Networks

- **neo-n3**: Neo N3 mainnet
- **neo-x**: Neo X (EVM-compatible)
- **ethereum**: Ethereum mainnet
- **polygon**: Polygon (Matic)
- **arbitrum**: Arbitrum One
- **optimism**: Optimism
- **base**: Base (Coinbase L2)
- **avalanche**: Avalanche C-Chain
- **bsc**: Binance Smart Chain
- **testnet**: Neo N3 testnet
- **local-priv**: Local private network

## Service Operations

### Contract Management

```go
// Create a new contract
contract, err := service.CreateContract(ctx, Contract{
    AccountID:   "acc_123",
    Name:        "MyToken",
    Symbol:      "MTK",
    Type:        ContractTypeUser,
    Network:     NetworkNeoN3,
    Version:     "1.0.0",
    ABI:         abiJSON,
    Bytecode:    bytecodeHex,
    Status:      ContractStatusDraft,
})

// Update contract metadata
updated, err := service.UpdateContract(ctx, contract)

// Get contract by ID
contract, err := service.GetContract(ctx, accountID, contractID)

// Get contract by address
contract, err := service.GetContractByAddress(ctx, NetworkNeoN3, "0x1234...")

// List account contracts
contracts, err := service.ListContracts(ctx, accountID)

// List contracts by service
contracts, err := service.ListContractsByService(ctx, serviceID)

// List contracts by network
contracts, err := service.ListContractsByNetwork(ctx, NetworkNeoN3)

// List engine contracts
contracts, err := service.ListEngineContracts(ctx)
```

### Deployment Operations

```go
// Deploy a contract
deployment, err := service.Deploy(ctx, accountID, contractID,
    map[string]any{
        "initialSupply": 1000000,
        "owner": "0xabcd...",
    },
    5000000, // gas limit
    map[string]string{"env": "production"},
)

// Get deployment status
deployment, err := service.GetDeployment(ctx, accountID, deploymentID)

// List contract deployments
deployments, err := service.ListDeployments(ctx, accountID, contractID, 10)
```

### Invocation Operations

```go
// Invoke contract method
invocation, err := service.Invoke(ctx, accountID, contractID, "transfer",
    map[string]any{
        "to": "0x5678...",
        "amount": 100,
    },
    2000000, // gas limit
    "0",     // value
    map[string]string{"priority": "high"},
)

// Get invocation result
invocation, err := service.GetInvocation(ctx, accountID, invocationID)

// List contract invocations
invocations, err := service.ListInvocations(ctx, accountID, contractID, 20)

// List account invocations
invocations, err := service.ListAccountInvocations(ctx, accountID, 50)
```

### Template Operations

```go
// Create template
template, err := service.CreateTemplate(ctx, Template{
    Name:        "ERC20 Token",
    Category:    TemplateCategoryToken,
    Networks:    []Network{NetworkEthereum, NetworkPolygon},
    Version:     "1.0.0",
    ABI:         abiJSON,
    Bytecode:    bytecodeHex,
    Audited:     true,
    Status:      TemplateStatusActive,
})

// Deploy from template
contract, deployment, err := service.DeployFromTemplate(ctx,
    accountID, templateID, "MyToken",
    map[string]any{"initialSupply": 1000000},
    5000000,
    map[string]string{"env": "production"},
)

// List templates by category
templates, err := service.ListTemplates(ctx, TemplateCategoryToken)

// List engine templates
templates, err := service.ListEngineTemplates(ctx)
```

### Service Binding Operations

```go
// Create service binding
binding, err := service.CreateServiceBinding(ctx, ServiceContractBinding{
    ServiceID:  "oracle",
    AccountID:  accountID,
    ContractID: contractID,
    Network:    NetworkNeoN3,
    Role:       "provider",
})

// List service bindings
bindings, err := service.ListServiceBindings(ctx, serviceID)

// List account bindings
bindings, err := service.ListAccountBindings(ctx, accountID)
```

### Network Configuration

```go
// Get network configuration
config, err := service.GetNetworkConfig(ctx, NetworkNeoN3)

// List all network configurations
configs, err := service.ListNetworkConfigs(ctx)
```

## Configuration

### Service Dependencies

- **store**: Persistence layer
- **svc-accounts**: Account management service
- **svc-gasbank**: Gas management service

### Required API Surfaces

- `APISurfaceStore`: Data persistence
- `APISurfaceContracts`: Contract operations
- `APISurfaceGasBank`: Gas payment handling

### Capabilities

- `contracts`: General contract management
- `deploy`: Contract deployment
- `invoke`: Contract invocation

### Dependency Injection

```go
// Create service
service := contracts.New(accountChecker, store, logger)

// Inject deployer (required for actual deployments)
service.WithDeployer(myDeployer)

// Inject invoker (required for actual invocations)
service.WithInvoker(myInvoker)

// Configure retry policy
service.WithDispatcherRetry(core.RetryPolicy{
    MaxAttempts: 3,
    BackoffMs:   1000,
})

// Configure observability
service.WithTracer(tracer)
service.WithDispatcherHooks(hooks)
service.WithObservationHooks(obsHooks)
```

## Engine Contracts

Standard engine contract set:

- **Manager**: Core engine manager
- **AccountManager**: Account lifecycle management
- **ServiceRegistry**: Service registration and discovery
- **GasBank**: Gas payment and accounting
- **OracleHub**: Oracle data aggregation
- **RandomnessHub**: VRF coordination
- **DataFeedHub**: Data feed management
- **AutomationScheduler**: Automated task execution
- **SecretsVault**: Secure secret storage
- **JAMInbox**: Cross-chain messaging

## Testing

```bash
# Run all tests
go test ./packages/com.r3e.services.contracts/...

# Run with coverage
go test -cover ./packages/com.r3e.services.contracts/...

# Run specific test
go test -run TestServiceCreateContract ./packages/com.r3e.services.contracts/

# Run with race detection
go test -race ./packages/com.r3e.services.contracts/...
```

## Security Considerations

1. **Account Ownership**: All operations validate account ownership before execution
2. **Engine Contract Access**: Engine contracts are accessible to all accounts (read-only)
3. **Service Contract Isolation**: Service contracts are scoped to their owning service
4. **User Contract Privacy**: User contracts are private to the owning account
5. **Gas Management**: All deployments and invocations require gas limit specification
6. **Status Validation**: Only active contracts can be invoked
7. **Template Auditing**: Templates track audit status for security transparency

## Error Handling

The service returns structured errors for common failure scenarios:

- **Account Not Found**: Account validation failure
- **Contract Not Found**: Contract does not exist
- **Ownership Violation**: Account does not own the contract
- **Invalid Status**: Contract not in required status for operation
- **Missing Bytecode**: Deployment attempted without bytecode
- **Template Inactive**: Template not in active status
- **Network Unavailable**: Target network not configured or disabled

## Performance Considerations

- **List Operations**: Use limit parameters to control result set size (default: 50, max: 500)
- **Deployment Tracking**: Deployments are tracked asynchronously with status updates
- **Invocation Logging**: All invocations are logged with full transaction details
- **Template Caching**: Templates should be cached at the application layer
- **Network Configuration**: Network configs are read-frequently, write-rarely

## Integration Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.contracts"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    ctx := context.Background()
    log := logger.New()

    // Initialize service
    svc := contracts.New(accountChecker, store, log)
    svc.WithDeployer(blockchainDeployer)
    svc.WithInvoker(blockchainInvoker)

    // Create and deploy contract
    contract, err := svc.CreateContract(ctx, contracts.Contract{
        AccountID: "acc_123",
        Name:      "MyContract",
        Type:      contracts.ContractTypeUser,
        Network:   contracts.NetworkNeoN3,
        ABI:       abiJSON,
        Bytecode:  bytecodeHex,
    })
    if err != nil {
        log.Fatal(err)
    }

    deployment, err := svc.Deploy(ctx, "acc_123", contract.ID,
        map[string]any{"param1": "value1"},
        5000000,
        nil,
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Infof("Contract deployed: %s", deployment.Address)
}
```

## File Structure

```
/home/neo/git/service_layer/packages/com.r3e.services.contracts/
├── domain.go              # Domain types and constants
├── service.go             # Core service implementation
├── store.go               # Store interface definition
├── templates.go           # Template management operations
├── package.go             # Package registration
├── service_test.go        # Service tests
├── service_env_internal_test.go  # Environment tests
└── README.md              # This file
```

## License

Copyright (c) R3E Network. All rights reserved.
