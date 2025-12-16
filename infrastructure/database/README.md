# Database Module

The `database` module provides Supabase (PostgreSQL) integration for the Service Layer.

## Overview

This module handles **shared** database operations including:

- Supabase REST API client
- Core data models (User, APIKey, Session, etc.)
- Repository pattern for data access
- OAuth and session management
- GasBank operations

**Note:** Service-specific database operations have been moved to each service's own `supabase/` subdirectory. See [Service-Specific Repositories](#service-specific-repositories) below.

## Architecture

```
infrastructure/database/     # Shared database infrastructure
├── supabase_client.go       # Supabase HTTP client
├── supabase_repository.go   # Base repository implementation
├── repository_interface.go  # Interface definitions
├── supabase_models.go       # Shared model definitions
├── errors.go                # Error definitions
├── supabase_*.go            # Shared operations (wallets, apikeys, sessions, oauth, gasbank, secrets)
└── mock_*.go                # Test mocks

services/*/supabase/         # Service-specific database operations (when needed)
├── repository.go            # Service-specific repository interface
└── models.go                # Service-specific data models

infrastructure/*/supabase/   # Infrastructure-backed repos (shared capabilities)
├── accountpool/supabase/    # Account pool persistence
├── globalsigner/supabase/   # Global signer persistence
└── secrets/supabase/        # User secrets persistence + policy metadata
```

## Components

### Supabase Client (`supabase_client.go`)

Main client for Supabase REST API communication.

```go
client, err := database.NewClient(database.Config{
    URL:        "https://your-project.supabase.co",
    ServiceKey: "your-service-key",
})
```

### Repository (`supabase_repository.go`)

High-level data access layer implementing the repository pattern.

```go
repo := database.NewRepository(client)

// Create a user
user, err := repo.CreateUser(ctx, &database.User{
    Address: "NAddr123...",
})

// Get user by address
user, err := repo.GetUserByAddress(ctx, "NAddr123...")
```

## Data Models (`supabase_models.go`)

### Core Models (Shared)

| Model | Description |
|-------|-------------|
| `User` | User account information |
| `UserWallet` | User wallet addresses |
| `APIKey` | API key for authentication |
| `UserSession` | User session data |
| `Secret` | Encrypted secrets |
| `GasBankAccount` | Gas balance tracking |
| `GasBankTransaction` | Gas transaction history |
| `DepositRequest` | Deposit request tracking |
| `ServiceRequest` | Service request tracking |
| `PriceFeed` | Price feed data |
| `OAuthProvider` | OAuth provider data |

## Service-Specific Repositories

Service-specific database operations have been moved to each service's own `supabase/` package following the **Interface Segregation Principle (ISP)**.

### NeoFlow Service

```go
import neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"

neoflowRepo := neoflowsupabase.NewRepository(baseRepo)

err := neoflowRepo.CreateTrigger(ctx, &neoflowsupabase.Trigger{...})
triggers, err := neoflowRepo.GetTriggers(ctx, userID)
err := neoflowRepo.CreateExecution(ctx, &neoflowsupabase.Execution{...})
```

### NeoAccounts (AccountPool) Service

```go
import neoaccountssupabase "github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"

poolRepo := neoaccountssupabase.NewRepository(baseRepo)

err := poolRepo.Create(ctx, &neoaccountssupabase.Account{...})
accounts, err := poolRepo.ListAvailable(ctx, 10)
err := poolRepo.Update(ctx, account)
```

### Secrets (User Secrets + Policies)

```go
import secretssupabase "github.com/R3E-Network/service_layer/infrastructure/secrets/supabase"

	secretsRepo := secretssupabase.NewRepository(baseRepo)

	err := secretsRepo.CreateSecret(ctx, &secretssupabase.Secret{...})
	secrets, err := secretsRepo.GetSecrets(ctx, userID)
	err := secretsRepo.SetAllowedServices(ctx, userID, secretName, []string{"neocompute", "neoflow"})
	```

## Shared Operations

### GasBank (`supabase_gasbank.go`)

```go
// Get or create account
account, err := repo.GetOrCreateGasBankAccount(ctx, userID)

// Update balance
err := repo.UpdateGasBankBalance(ctx, userID, newBalance, reserved)

// Create transaction
err := repo.CreateGasBankTransaction(ctx, &database.GasBankTransaction{...})
```

### Authentication

#### OAuth (`supabase_oauth.go`)

```go
// Link OAuth provider
err := repo.LinkOAuthProvider(ctx, &database.OAuthProvider{...})

// Get linked providers
providers, err := repo.GetOAuthProviders(ctx, userID)
```

#### Sessions (`supabase_sessions.go`)

```go
// Create session
session, err := repo.CreateSession(ctx, &database.UserSession{...})

// Validate session
session, err := repo.GetSessionByToken(ctx, tokenHash)

// Revoke session
err := repo.DeleteSession(ctx, sessionID)
```

#### API Keys (`supabase_apikeys.go`)

```go
// Create API key
err := repo.CreateAPIKey(ctx, &database.APIKey{...})

// Validate API key
apiKey, err := repo.GetAPIKeyByHash(ctx, keyHash)

// List user's API keys
keys, err := repo.GetAPIKeys(ctx, userID)
```

### Wallets (`supabase_wallets.go`)

```go
// Add wallet
err := repo.CreateWallet(ctx, &database.UserWallet{...})

// Get user wallets
wallets, err := repo.GetWallets(ctx, userID)

// Set primary wallet
err := repo.SetPrimaryWallet(ctx, userID, walletID)
```

## Testing

```bash
go test ./infrastructure/database/... -v
```

### Mock Repository

```go
// Create mock for testing
mockRepo := database.NewMockRepository()

// Inject errors for testing error paths
mockRepo.ErrorOnNextCall = errors.New("database error")

// Reset mock state
mockRepo.Reset()
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SUPABASE_URL` | Supabase project URL |
| `SUPABASE_SERVICE_KEY` | Supabase service role key |

## Migration Guide

If you have code using the old service-specific methods from `database.RepositoryInterface`, migrate to the new service-specific packages:

| Old (Deprecated) | New |
|------------------|-----|
| `repo.CreateVRFRequest()` | `vrfRepo.Create()` |
| `repo.GetVRFRequest()` | `vrfRepo.GetByRequestID()` |
| `repo.CreateNeoFlowTrigger()` | `neoflowRepo.CreateTrigger()` |
| `repo.GetNeoFlowTriggers()` | `neoflowRepo.GetTriggers()` |
| `repo.CreatePoolAccount()` | `poolRepo.Create()` |
| `repo.GetPoolAccount()` | `poolRepo.GetByID()` |
| `repo.GetSecretPolicies()` | `secretsRepo.GetAllowedServices()` |
| `repo.SetSecretPolicies()` | `secretsRepo.SetAllowedServices()` |
