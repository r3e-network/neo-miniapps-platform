# NeoStore Service

Encrypted secrets management service for the Neo Service Layer.

## Overview

The NeoStore service provides secure storage and retrieval of user secrets. All secrets are encrypted with AES-256-GCM using keys derived from the master key, ensuring that secrets are only accessible within the MarbleRun TEE.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ Secrets      │     │   Database   │
│              │     │ Service (TEE)│     │  (Encrypted) │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Store Secret       │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Encrypt & Store    │
       │                    │───────────────────>│
       │                    │                    │
       │ Success            │                    │
       │<───────────────────│                    │
       │                    │                    │
       │ Get Secret         │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Fetch & Decrypt    │
       │                    │<───────────────────│
       │                    │                    │
       │ Decrypted Secret   │                    │
       │<───────────────────│                    │
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/neostore` | GET | List user's secrets (metadata only) |
| `/neostore` | POST | Create/update secret |
| `/neostore/{name}` | GET | Get secret value |
| `/neostore/{name}` | DELETE | Delete secret |

## Request/Response Types

### Create Secret

```json
POST /neostore
{
    "name": "api_key",
    "value": "sk-1234567890abcdef",
    "description": "External API key"
}
```

### Create Response

```json
{
    "name": "api_key",
    "description": "External API key",
    "created_at": "2025-12-08T00:00:00Z",
    "updated_at": "2025-12-08T00:00:00Z"
}
```

### List Secrets

```json
GET /neostore

{
    "secrets": [
        {
            "name": "api_key",
            "description": "External API key",
            "created_at": "2025-12-08T00:00:00Z"
        },
        {
            "name": "webhook_secret",
            "description": "Webhook signing secret",
            "created_at": "2025-12-07T00:00:00Z"
        }
    ]
}
```

### Get Secret

```json
GET /neostore/api_key

{
    "name": "api_key",
    "value": "sk-1234567890abcdef",
    "description": "External API key",
    "created_at": "2025-12-08T00:00:00Z"
}
```

## Encryption

Secrets are encrypted using:
- **Algorithm**: AES-256-GCM
- **Key Derivation**: HKDF from master key + user ID + secret name
- **Nonce**: Random 12 bytes per encryption

```
encryption_key = HKDF(master_key, salt=user_id, info=secret_name)
ciphertext = AES-GCM-Encrypt(encryption_key, nonce, plaintext)
```

## Security Features

- Secrets encrypted at rest in database
- Decryption only inside MarbleRun TEE
- Per-secret derived keys (compromise of one doesn't affect others)
- Automatic key rotation support
- Audit logging of access

## Configuration

### Required Secrets

| Secret | Description |
|--------|-------------|
| `SECRETS_MASTER_KEY` | AES-256 master key (32 bytes) |

## Use Cases

- API keys for external services
- Webhook signing secrets
- Database credentials
- Private keys for signing

## Data Layer

The NeoStore service uses a service-specific Supabase repository for database operations.

### Package Structure

```
services/neostore/
├── supabase/
│   ├── repository.go    # Secrets-specific repository interface
│   └── models.go        # Secrets data models (Secret, AllowedService)
├── secrets.go           # Service implementation
└── README.md
```

### Repository Interface

```go
import secretssupabase "github.com/R3E-Network/service_layer/services/neostore/supabase"

// Create repository
secretsRepo := secretssupabase.NewRepository(baseRepo)

// Operations
err := secretsRepo.CreateSecret(ctx, &secretssupabase.Secret{...})
secrets, err := secretsRepo.GetSecrets(ctx, userID)
secret, err := secretsRepo.GetSecret(ctx, userID, name)
err := secretsRepo.DeleteSecret(ctx, userID, name)
err := secretsRepo.SetAllowedServices(ctx, userID, name, []string{"vrf", "neoflow"})
allowed, err := secretsRepo.GetAllowedServices(ctx, userID, name)
```

### Data Models

| Model | Description |
|-------|-------------|
| `Secret` | Encrypted secret with name, value, metadata |
| `AllowedService` | Service access permission for a secret |

## Testing

```bash
go test ./services/neostore/... -v -cover
```

Current test coverage: **31.4%**

## Version

- Service ID: `secrets`
- Version: `1.0.0`
