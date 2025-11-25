# Secrets Service Quickstart

Securely store and manage encrypted secrets for function execution.

## Overview

The Secrets service provides:
- **AES-GCM Encryption**: Secrets encrypted at rest
- **Function Integration**: Automatic secret injection during execution
- **ACL Enforcement**: Per-account isolation
- **Audit Trail**: Track secret access patterns

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
```

**Server Configuration**:
```bash
# Required for encryption (16, 24, or 32 bytes)
export SECRET_ENCRYPTION_KEY=your-32-byte-encryption-key-here
```

## Quick Start

### 1. Create Account

```bash
ACCOUNT_ID=$(curl -s -X POST $API/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"owner":"secure-app"}' | jq -r .id)
```

### 2. Store Secrets

```bash
# Store API key
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "apiKey", "value": "sk-123456789abcdef"}'

# Store database credentials
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "dbPassword", "value": "super-secret-password"}'

# Store JSON configuration
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "serviceConfig",
    "value": "{\"endpoint\":\"https://api.example.com\",\"timeout\":30}"
  }'
```

### 3. List Secrets (Names Only)

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/secrets | jq
```

**Response** (values are hidden):
```json
[
  {"name": "apiKey", "created_at": "2025-01-15T10:00:00Z"},
  {"name": "dbPassword", "created_at": "2025-01-15T10:01:00Z"},
  {"name": "serviceConfig", "created_at": "2025-01-15T10:02:00Z"}
]
```

### 4. Use Secrets in Functions

```bash
# Create function that uses secrets
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-caller",
    "runtime": "js",
    "source": "(params, secrets) => { return { key: secrets.apiKey.substring(0,5) + \"...\" }; }",
    "secrets": ["apiKey", "serviceConfig"]
  }' | jq -r .ID)

# Execute function - secrets are automatically injected
curl -s -X POST $API/accounts/$ACCOUNT_ID/functions/$FUNC_ID/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

## API Reference

### Create Secret

```http
POST /accounts/{account}/secrets
```

```json
{
  "name": "mySecret",
  "value": "secret-value"
}
```

### Get Secret

```http
GET /accounts/{account}/secrets/{name}
```

**Note**: Returns the actual value. Use with caution.

### Update Secret

```http
PUT /accounts/{account}/secrets/{name}
```

```json
{
  "value": "new-secret-value"
}
```

### Delete Secret

```http
DELETE /accounts/{account}/secrets/{name}
```

### List Secrets

```http
GET /accounts/{account}/secrets
```

Returns names and metadata only (not values).

## CLI Usage

```bash
# Create secret
slctl secrets create --account $ACCOUNT_ID \
  --name apiKey \
  --value "sk-123456789"

# List secrets
slctl secrets list --account $ACCOUNT_ID

# Get secret (careful - shows value)
slctl secrets get --account $ACCOUNT_ID --name apiKey

# Delete secret
slctl secrets delete --account $ACCOUNT_ID --name apiKey
```

## Function Integration

### JavaScript Function

```javascript
export default function(params, secrets) {
  // Access secrets directly
  const apiKey = secrets.apiKey;
  const config = JSON.parse(secrets.serviceConfig);

  // Make authenticated API call
  const response = Devpack.http.request({
    url: config.endpoint + "/data",
    method: "GET",
    headers: {
      "Authorization": `Bearer ${apiKey}`
    }
  });

  return Devpack.respond.success({
    data: response.body
  });
}
```

### Declare Required Secrets

When creating a function, specify which secrets it needs:

```json
{
  "name": "my-function",
  "runtime": "js",
  "source": "...",
  "secrets": ["apiKey", "dbPassword", "serviceConfig"]
}
```

Only declared secrets are injected at execution time.

## Security Best Practices

### 1. Key Rotation

```bash
# Update existing secret
curl -s -X PUT $API/accounts/$ACCOUNT_ID/secrets/apiKey \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"value": "new-api-key-value"}'
```

### 2. Minimal Exposure

- Only declare secrets a function actually needs
- Use short-lived tokens when possible
- Rotate secrets regularly

### 3. Naming Conventions

```bash
# Environment-prefixed
prod_apiKey
staging_apiKey
dev_apiKey

# Service-prefixed
stripe_apiKey
twilio_authToken
aws_secretKey
```

### 4. Encryption Key Management

```bash
# Generate secure key
openssl rand -base64 32

# Or hex format
openssl rand -hex 32

# Set in environment
export SECRET_ENCRYPTION_KEY=$(openssl rand -base64 32)
```

## Common Patterns

### Multi-Environment Setup

```bash
# Production secrets
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "prod_apiKey", "value": "sk-prod-xxx"}'

# Staging secrets
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "staging_apiKey", "value": "sk-staging-xxx"}'
```

### Database Credentials

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database",
    "value": "{\"host\":\"db.example.com\",\"port\":5432,\"user\":\"app\",\"password\":\"secret\"}"
  }'
```

### OAuth Tokens

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "oauth",
    "value": "{\"client_id\":\"xxx\",\"client_secret\":\"yyy\",\"refresh_token\":\"zzz\"}"
  }'
```

## Error Handling

| HTTP Status | Error | Resolution |
|-------------|-------|------------|
| 400 | "name is required" | Provide secret name |
| 400 | "value is required" | Provide secret value |
| 404 | "secret not found" | Check secret name exists |
| 409 | "secret already exists" | Use PUT to update |
| 500 | "encryption error" | Check SECRET_ENCRYPTION_KEY |

## Related Documentation

- [Functions Service](../service-catalog.md#2-functions-service)
- [Security Hardening](../security-hardening.md)
- [Service Catalog](../service-catalog.md)
