# Database Module

The `database` module provides Supabase (PostgreSQL) integration for the Service Layer.

## Overview

This module handles all database operations including:

- Supabase REST API client
- Data models for all services
- Repository pattern for data access
- OAuth and session management

## Components

### Supabase Client (`supabase_client.go`)

Main client for Supabase REST API communication.

```go
client, err := database.NewSupabaseClient(database.SupabaseConfig{
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

### Core Models

| Model | Description |
|-------|-------------|
| `User` | User account information |
| `Wallet` | User wallet addresses |
| `APIKey` | API key for authentication |
| `Session` | User session data |

### Service-Specific Models

| Model | Service | Description |
|-------|---------|-------------|
| `VRFRequest` | VRF | Random number requests |
| `MixerRequestRecord` | Mixer | Mix request data |
| `AutomationTrigger` | Automation | Trigger definitions |
| `AutomationExecution` | Automation | Execution history |
| `GasBankAccount` | GasBank | Gas balance tracking |
| `Secret` | Secrets | Encrypted secrets |

## Service-Specific Repositories

### VRF (`supabase_vrf.go`)

```go
// Create VRF request
err := repo.CreateVRFRequest(ctx, &database.VRFRequest{...})

// Get pending requests
requests, err := repo.GetPendingVRFRequests(ctx)

// Update request status
err := repo.UpdateVRFRequestStatus(ctx, requestID, "fulfilled")
```

### Automation (`supabase_automation.go`)

```go
// Create trigger
err := repo.CreateAutomationTrigger(ctx, &database.AutomationTrigger{...})

// Get user triggers
triggers, err := repo.GetAutomationTriggers(ctx, userID)

// Log execution
err := repo.CreateAutomationExecution(ctx, &database.AutomationExecution{...})
```

### Mixer (`mixer.go`)

```go
// Create mix request
err := repo.CreateMixerRequest(ctx, &database.MixerRequestRecord{...})

// Get request by ID
request, err := repo.GetMixerRequest(ctx, requestID)

// List requests by status
requests, err := repo.ListMixerRequestsByStatus(ctx, "mixing")
```

### GasBank (`supabase_gasbank.go`)

```go
// Get or create account
account, err := repo.GetOrCreateGasBankAccount(ctx, userID)

// Update balance
err := repo.UpdateGasBankBalance(ctx, userID, newBalance)
```

### Account Pool (`accountpool.go`)

```go
// Get pool accounts
accounts, err := repo.GetPoolAccounts(ctx, serviceID)

// Lock account
err := repo.LockPoolAccount(ctx, accountID, serviceID)

// Release account
err := repo.ReleasePoolAccount(ctx, accountID)
```

## Authentication

### OAuth (`supabase_oauth.go`)

```go
// Get OAuth URL
url := repo.GetOAuthURL("google", redirectURL)

// Exchange code for session
session, err := repo.ExchangeOAuthCode(ctx, code)
```

### Sessions (`supabase_sessions.go`)

```go
// Create session
session, err := repo.CreateSession(ctx, userID)

// Validate session
valid, err := repo.ValidateSession(ctx, sessionToken)

// Revoke session
err := repo.RevokeSession(ctx, sessionToken)
```

### API Keys (`supabase_apikeys.go`)

```go
// Create API key
apiKey, err := repo.CreateAPIKey(ctx, userID, name)

// Validate API key
user, err := repo.ValidateAPIKey(ctx, apiKeyHash)

// List user's API keys
keys, err := repo.ListAPIKeys(ctx, userID)
```

## Testing

```bash
go test ./internal/database/... -v
```

Current test coverage: **17.1%**

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SUPABASE_URL` | Supabase project URL |
| `SUPABASE_SERVICE_KEY` | Supabase service role key |
