# Accounts Service

Account registry and metadata management service for the R3E Network service layer.

## Overview

The Accounts service provides core account lifecycle management, serving as the authoritative source for account identity and metadata within the platform. It manages account creation, metadata updates, and workspace wallet associations.

**Package ID:** `com.r3e.services.accounts`
**Version:** 1.0.0
**Layer:** Service
**Domain:** accounts

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP API Layer                         │
│  GET/POST /workspace-wallets, GET /workspace-wallets/{id}   │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                   Service Layer                             │
│  - Account CRUD operations                                  │
│  - Metadata management                                      │
│  - Workspace wallet management                              │
│  - Account validation & ownership checks                    │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                  Store Interface                            │
│  - PostgresStore (production)                               │
│  - MemoryStore (testing)                                    │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### Service

The `Service` struct is the main component that orchestrates account operations:

- **ServiceEngine**: Embedded framework providing validation, logging, metrics, and manifest management
- **Store**: Persistence layer abstraction for account and wallet data
- **Base**: Common CRUD utilities from the framework core

### Store Interface

Defines persistence operations for accounts and workspace wallets:

```go
type Store interface {
    // Account operations
    CreateAccount(ctx context.Context, acct Account) (Account, error)
    UpdateAccount(ctx context.Context, acct Account) (Account, error)
    GetAccount(ctx context.Context, id string) (Account, error)
    ListAccounts(ctx context.Context) ([]Account, error)
    DeleteAccount(ctx context.Context, id string) error

    // Workspace wallet operations
    CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error)
    GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error)
    ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error)
    FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, wallet string) (WorkspaceWallet, error)
}
```

**Implementations:**
- `PostgresStore`: Production-ready PostgreSQL backend
- `MemoryStore`: In-memory implementation for testing

## Domain Types

### Account

Represents a logical tenant or owner of resources within the service layer.

```go
type Account struct {
    ID        string                // Auto-generated unique identifier
    Owner     string                // Owner identifier (required)
    Metadata  map[string]string     // Flexible key-value metadata
    CreatedAt time.Time             // Creation timestamp
    UpdatedAt time.Time             // Last update timestamp
}
```

**Metadata Usage:**
- `tenant`: Multi-tenancy identifier
- `tier`: Service tier (e.g., "pro", "enterprise")
- `region`: Geographic region
- Custom application-specific fields

### WorkspaceWallet

Captures a blockchain wallet registered to a workspace/account.

```go
type WorkspaceWallet struct {
    ID            string      // Auto-generated unique identifier
    WorkspaceID   string      // Associated account/workspace ID
    WalletAddress string      // Ethereum-style wallet address (0x-prefixed, 42 chars)
    Label         string      // Human-readable label
    Status        string      // Wallet status
    CreatedAt     time.Time   // Creation timestamp
    UpdatedAt     time.Time   // Last update timestamp
}
```

**Wallet Address Validation:**
- Must start with `0x`
- Must be exactly 42 characters (0x + 40 hex digits)
- Automatically normalized to lowercase

## Service Methods

### Account Operations

#### Create
```go
func (s *Service) Create(ctx context.Context, owner string, metadata map[string]string) (Account, error)
```
Creates a new account with optional metadata. The `owner` field is required.

**Observability:**
- Logs account creation with account ID and owner
- Increments `accounts_created_total` counter
- Emits creation event

#### Get
```go
func (s *Service) Get(ctx context.Context, id string) (Account, error)
```
Retrieves an account by ID. Validates account existence before retrieval.

#### UpdateMetadata
```go
func (s *Service) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) (Account, error)
```
Replaces the entire metadata map for an account. Validates account existence and ownership.

**Observability:**
- Logs metadata updates
- Increments `accounts_metadata_updated_total` counter
- Emits update event

#### List
```go
func (s *Service) List(ctx context.Context) ([]Account, error)
```
Returns all accounts in the system.

#### Delete
```go
func (s *Service) Delete(ctx context.Context, id string) error
```
Removes an account by ID. Automatically trims whitespace from the ID.

**Observability:**
- Logs account deletion
- Increments `accounts_deleted_total` counter
- Emits deletion event

### Workspace Wallet Operations

#### CreateWorkspaceWallet
```go
func (s *Service) CreateWorkspaceWallet(ctx context.Context, wallet WorkspaceWallet) (WorkspaceWallet, error)
```
Creates a new workspace wallet. Validates workspace existence and normalizes wallet address.

**Observability:**
- Logs wallet creation
- Increments `accounts_workspace_wallets_created_total` counter

#### GetWorkspaceWallet
```go
func (s *Service) GetWorkspaceWallet(ctx context.Context, id string) (WorkspaceWallet, error)
```
Retrieves a workspace wallet by ID.

#### ListWorkspaceWallets
```go
func (s *Service) ListWorkspaceWallets(ctx context.Context, workspaceID string) ([]WorkspaceWallet, error)
```
Lists all wallets associated with a workspace. Validates workspace existence.

#### FindWorkspaceWalletByAddress
```go
func (s *Service) FindWorkspaceWalletByAddress(ctx context.Context, workspaceID, walletAddr string) (WorkspaceWallet, error)
```
Finds a wallet by address within a specific workspace.

### Engine API Methods

These methods implement the `accountAPI` interface for core engine integration:

#### CreateAccount
```go
func (s *Service) CreateAccount(ctx context.Context, owner string, metadata map[string]string) (string, error)
```
Engine-compatible account creation that returns only the account ID.

#### ListAccounts
```go
func (s *Service) ListAccounts(ctx context.Context) ([]any, error)
```
Engine-compatible account listing that returns accounts as `[]any`.

### Account Checker Interface

Implements `AccountChecker` for cross-service account validation:

#### AccountExists
```go
func (s *Service) AccountExists(ctx context.Context, accountID string) error
```
Returns `nil` if the account exists, or an error if it does not.

#### AccountTenant
```go
func (s *Service) AccountTenant(ctx context.Context, accountID string) string
```
Returns the tenant identifier from account metadata, or empty string if none.

## HTTP API Endpoints

The service exposes HTTP endpoints through declarative method naming:

### GET /workspace-wallets
**Handler:** `HTTPGetWorkspaceWallets`
**Description:** List all wallets for the authenticated account
**Authentication:** Required (uses `req.AccountID`)

**Response:**
```json
[
  {
    "id": "wallet-123",
    "workspace_id": "account-456",
    "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb",
    "label": "Primary Wallet",
    "status": "active",
    "created_at": "2025-12-01T10:00:00Z",
    "updated_at": "2025-12-01T10:00:00Z"
  }
]
```

### POST /workspace-wallets
**Handler:** `HTTPPostWorkspaceWallets`
**Description:** Create a new workspace wallet
**Authentication:** Required (uses `req.AccountID`)

**Request Body:**
```json
{
  "wallet_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0beb",
  "label": "Primary Wallet",
  "status": "active"
}
```

**Validation:**
- `wallet_address` must be valid (0x-prefixed, 42 characters, hexadecimal)
- Address is automatically normalized to lowercase

**Response:**
```json
{
  "id": "wallet-123",
  "workspace_id": "account-456",
  "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb",
  "label": "Primary Wallet",
  "status": "active",
  "created_at": "2025-12-01T10:00:00Z",
  "updated_at": "2025-12-01T10:00:00Z"
}
```

### GET /workspace-wallets/{id}
**Handler:** `HTTPGetWorkspaceWalletsById`
**Description:** Get a specific workspace wallet by ID
**Authentication:** Required (validates ownership)

**Path Parameters:**
- `id`: Wallet identifier

**Response:**
```json
{
  "id": "wallet-123",
  "workspace_id": "account-456",
  "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb",
  "label": "Primary Wallet",
  "status": "active",
  "created_at": "2025-12-01T10:00:00Z",
  "updated_at": "2025-12-01T10:00:00Z"
}
```

**Error Response:**
Returns error if the wallet does not belong to the authenticated account.

## Configuration

### Service Configuration

```go
framework.ServiceConfig{
    Name:         "accounts",
    Description:  "Account registry and metadata",
    DependsOn:    []string{"store"},
    RequiresAPIs: []engine.APISurface{
        engine.APISurfaceStore,
        engine.APISurfaceAccount,
    },
    Capabilities: []string{"accounts"},
}
```

### Resource Limits (from manifest.yaml)

- **Max Storage:** 100 MB
- **Max Concurrent Requests:** 1,000
- **Max Requests/Second:** 5,000
- **Max Events/Second:** 1,000

### Permissions

- `system.api.storage` (required): For persisting account data
- `system.api.bus` (optional): For publishing account events
- `system.api.ledger` (optional): For on-chain account operations

## Dependencies

### Required
- **store**: Persistence layer module
- **com.r3e.platform.storage** (v1.0.0+): Storage platform package

### Framework Dependencies
- `github.com/R3E-Network/service_layer/pkg/logger`: Structured logging
- `github.com/R3E-Network/service_layer/system/framework`: Service framework
- `github.com/R3E-Network/service_layer/system/framework/core`: Core utilities

## Observability

### Metrics

The service emits the following metrics:

- `accounts_created_total{account_id}`: Total accounts created
- `accounts_metadata_updated_total{account_id}`: Total metadata updates
- `accounts_deleted_total{account_id}`: Total accounts deleted
- `accounts_workspace_wallets_created_total{workspace_id}`: Total workspace wallets created

### Logging

All operations emit structured logs with relevant context:

```go
s.Logger().WithField("account_id", id).
    WithField("owner", owner).
    Info("account created")
```

### Tracing

Operations use `StartObservation` for distributed tracing:

```go
attrs := map[string]string{"resource": "account", "owner": owner}
ctx, finish := s.StartObservation(ctx, attrs)
defer finish(err)
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./packages/com.r3e.services.accounts/

# Run with coverage
go test -cover ./packages/com.r3e.services.accounts/

# Run specific test
go test -run TestService_Create ./packages/com.r3e.services.accounts/
```

### Test Coverage

The service includes comprehensive tests for:

- Service initialization and configuration
- Account CRUD operations
- Metadata management
- Workspace wallet operations
- Engine API compatibility
- Input validation and error handling
- Whitespace handling in identifiers

### Example Usage

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    // Initialize service
    store := accounts.NewMemoryStore()
    log := logger.NewDefault("accounts")
    svc := accounts.New(nil, store, log)

    ctx := context.Background()

    // Create account
    acct, err := svc.Create(ctx, "alice", map[string]string{
        "tier": "pro",
        "region": "us-west",
    })
    if err != nil {
        panic(err)
    }

    // Update metadata
    updated, err := svc.UpdateMetadata(ctx, acct.ID, map[string]string{
        "tier": "enterprise",
        "region": "us-west",
    })

    // Create workspace wallet
    wallet, err := svc.CreateWorkspaceWallet(ctx, accounts.WorkspaceWallet{
        WorkspaceID:   acct.ID,
        WalletAddress: "0x742d35Cc6634C0532925a3b844Bc9e7595f0beb",
        Label:         "Primary Wallet",
        Status:        "active",
    })

    // List wallets
    wallets, err := svc.ListWorkspaceWallets(ctx, acct.ID)
}
```

## Error Handling

### Common Errors

- **Required Field Missing**: Returns `core.RequiredError("field_name")`
- **Account Not Found**: Returns error from store layer
- **Invalid Wallet Address**: Returns validation error with specific reason
- **Ownership Violation**: Returns `core.EnsureOwnership` error

### Validation Rules

1. **Account Creation:**
   - `owner` field is required (non-empty)

2. **Wallet Address:**
   - Must start with `0x`
   - Must be exactly 42 characters
   - Must contain only hexadecimal characters
   - Automatically normalized to lowercase

3. **Account Operations:**
   - Account ID must exist for Get, Update, Delete operations
   - Account ID is trimmed of whitespace in Delete operations

## Security Considerations

1. **Ownership Validation**: HTTP endpoints validate that wallets belong to the authenticated account
2. **Input Sanitization**: Wallet addresses are normalized and validated
3. **Account Isolation**: Services use `AccountChecker` interface to validate cross-service access
4. **Metadata Flexibility**: Metadata is stored as key-value pairs without schema enforcement

## Future Enhancements

Potential areas for extension:

- Account status management (active, suspended, deleted)
- Wallet verification and signature validation
- Account hierarchy and sub-accounts
- Audit trail for account changes
- Bulk operations for account management
- Account search and filtering capabilities

## Support

- **Documentation**: https://docs.r3e.network/services/accounts
- **Support**: support@r3e.network
- **Category**: Core
- **Tags**: accounts, authentication, identity

## License

MIT License - Copyright (c) R3E Network
