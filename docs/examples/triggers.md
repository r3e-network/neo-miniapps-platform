# Triggers Service Quickstart

Route events and webhooks to functions for event-driven architectures.

## Overview

The Triggers service provides:
- **Webhook Triggers**: HTTP endpoint routing to functions
- **Event Triggers**: Internal event subscription
- **Filter Rules**: Conditional execution based on payload
- **Retry Logic**: Automatic retry on failure

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
export ACCOUNT_ID=<your-account-id>
```

## Quick Start

### 1. Create a Function

```bash
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "event-handler",
    "runtime": "js",
    "source": "(params) => ({ received: params, timestamp: new Date().toISOString() })"
  }' | jq -r .ID)

echo "Function ID: $FUNC_ID"
```

### 2. Create Webhook Trigger

```bash
TRIGGER_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-webhook",
    "type": "webhook",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "path": "/hooks/price-update",
      "methods": ["POST"],
      "secret": "webhook-secret-123"
    },
    "enabled": true
  }' | jq -r .ID)

echo "Trigger ID: $TRIGGER_ID"
```

### 3. Create Event Trigger

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "transfer-listener",
    "type": "event",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "event_type": "neo.transfer",
      "filter": {
        "asset": "NEO",
        "min_amount": 100
      }
    },
    "enabled": true
  }'
```

### 4. Test Webhook

```bash
# Simulate external webhook call
curl -s -X POST $API/hooks/price-update \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: webhook-secret-123" \
  -d '{"price": 12.34, "source": "external"}'
```

## Trigger Types

### Webhook Trigger

Exposes an HTTP endpoint that invokes a function.

```json
{
  "name": "payment-webhook",
  "type": "webhook",
  "function_id": "func-uuid",
  "config": {
    "path": "/hooks/payment",
    "methods": ["POST"],
    "secret": "shared-secret",
    "headers_to_pass": ["X-Request-ID"],
    "response_timeout_ms": 30000
  },
  "enabled": true
}
```

**Config Options**:

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | URL path for webhook |
| `methods` | array | Allowed HTTP methods |
| `secret` | string | Shared secret for validation |
| `headers_to_pass` | array | Headers to forward to function |
| `response_timeout_ms` | int | Max execution time |

### Event Trigger

Subscribes to internal events and invokes function.

```json
{
  "name": "block-listener",
  "type": "event",
  "function_id": "func-uuid",
  "config": {
    "event_type": "neo.block",
    "filter": {
      "min_tx_count": 1
    },
    "batch_size": 1,
    "batch_timeout_ms": 1000
  },
  "enabled": true
}
```

**Event Types**:

| Event | Description |
|-------|-------------|
| `neo.block` | New block produced |
| `neo.transfer` | Asset transfer |
| `neo.contract` | Contract invocation |
| `pricefeed.update` | Price feed updated |
| `oracle.fulfilled` | Oracle request completed |
| `gasbank.deposit` | Gas bank deposit |
| `gasbank.withdraw` | Gas bank withdrawal |

### Cron Trigger

Scheduled function execution (alias for automation).

```json
{
  "name": "hourly-sync",
  "type": "cron",
  "function_id": "func-uuid",
  "config": {
    "schedule": "@hourly",
    "timezone": "UTC",
    "payload": {"action": "sync"}
  },
  "enabled": true
}
```

## API Reference

### Create Trigger

```http
POST /accounts/{account}/triggers
```

```json
{
  "name": "trigger-name",
  "type": "webhook|event|cron",
  "function_id": "func-uuid",
  "config": {...},
  "enabled": true,
  "metadata": {"env": "prod"}
}
```

### Update Trigger

```http
PATCH /accounts/{account}/triggers/{id}
```

```json
{
  "config": {"path": "/new-path"},
  "enabled": false
}
```

### List Triggers

```http
GET /accounts/{account}/triggers?type=webhook&enabled=true
```

### Delete Trigger

```http
DELETE /accounts/{account}/triggers/{id}
```

## CLI Usage

```bash
# List triggers
slctl triggers list --account $ACCOUNT_ID

# Create webhook trigger
slctl triggers create --account $ACCOUNT_ID \
  --name "my-webhook" \
  --type webhook \
  --function $FUNC_ID \
  --path "/hooks/my-webhook"

# Create event trigger
slctl triggers create --account $ACCOUNT_ID \
  --name "transfer-listener" \
  --type event \
  --function $FUNC_ID \
  --event-type "neo.transfer"

# Disable trigger
slctl triggers update --account $ACCOUNT_ID \
  --trigger $TRIGGER_ID \
  --enabled false

# Delete trigger
slctl triggers delete --account $ACCOUNT_ID --trigger $TRIGGER_ID
```

## Use Cases

### Payment Webhook

```bash
# Create payment handler function
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "payment-handler",
    "runtime": "js",
    "source": "(params, secrets) => { if (params.status === \"completed\") { /* process payment */ } return { processed: true }; }",
    "secrets": ["stripeKey"]
  }' | jq -r .ID)

# Create webhook trigger
curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "stripe-webhook",
    "type": "webhook",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "path": "/hooks/stripe",
      "methods": ["POST"],
      "secret": "whsec_..."
    },
    "enabled": true
  }'
```

### Block Monitor

```bash
# Create block analyzer function
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "block-analyzer",
    "runtime": "js",
    "source": "(params) => { const txCount = params.block.transactions.length; return { height: params.block.index, txCount }; }"
  }' | jq -r .ID)

# Create event trigger
curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "block-monitor",
    "type": "event",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "event_type": "neo.block"
    },
    "enabled": true
  }'
```

### Price Alert

```bash
# Create price alert function
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-alert",
    "runtime": "js",
    "source": "(params, secrets) => { if (params.price > 15) { Devpack.http.request({ url: secrets.webhookUrl, method: \"POST\", body: JSON.stringify({ alert: \"Price high\", price: params.price }) }); } return { alerted: params.price > 15 }; }",
    "secrets": ["webhookUrl"]
  }' | jq -r .ID)

# Create price update trigger
curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "neo-price-alert",
    "type": "event",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "event_type": "pricefeed.update",
      "filter": {
        "pair": "NEO/USD"
      }
    },
    "enabled": true
  }'
```

### Multi-Tenant Webhook

```bash
# Create tenant-aware function
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tenant-handler",
    "runtime": "js",
    "source": "(params) => { const tenant = params.headers[\"X-Tenant-ID\"]; return { tenant, data: params.body }; }"
  }' | jq -r .ID)

# Create webhook with tenant header passthrough
curl -s -X POST $API/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tenant-webhook",
    "type": "webhook",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "path": "/hooks/tenant",
      "methods": ["POST"],
      "headers_to_pass": ["X-Tenant-ID", "X-Request-ID"]
    },
    "enabled": true
  }'
```

## Webhook Security

### Secret Validation

```javascript
// Function validates webhook signature
export default function(params, secrets) {
  const signature = params.headers['X-Webhook-Signature'];
  const payload = JSON.stringify(params.body);

  // HMAC validation
  const crypto = require('crypto');
  const expected = crypto
    .createHmac('sha256', secrets.webhookSecret)
    .update(payload)
    .digest('hex');

  if (signature !== `sha256=${expected}`) {
    return Devpack.respond.error('Invalid signature');
  }

  // Process valid webhook
  return Devpack.respond.success({ validated: true });
}
```

### IP Allowlisting

```json
{
  "name": "secure-webhook",
  "type": "webhook",
  "config": {
    "path": "/hooks/secure",
    "allowed_ips": ["203.0.113.0/24", "198.51.100.0/24"],
    "secret": "..."
  }
}
```

## Filter Expressions

### Event Filters

```json
{
  "event_type": "neo.transfer",
  "filter": {
    "asset": "NEO",
    "min_amount": 100,
    "to_address": "NeoAddress..."
  }
}
```

### Conditional Execution

```json
{
  "event_type": "pricefeed.update",
  "filter": {
    "pair": "NEO/USD",
    "deviation_percent_gte": 5.0
  }
}
```

## Error Handling

| HTTP Status | Error | Resolution |
|-------------|-------|------------|
| 400 | "path is required" | Provide webhook path |
| 400 | "function_id required" | Specify target function |
| 404 | "trigger not found" | Check trigger ID |
| 409 | "path already registered" | Use different path |
| 500 | "function execution failed" | Check function logs |

## Best Practices

1. **Use Secrets**: Never hardcode webhook secrets in config
2. **Validate Early**: Check signatures before processing
3. **Idempotent Functions**: Design for potential duplicate deliveries
4. **Set Timeouts**: Configure appropriate response timeouts
5. **Monitor Failures**: Track failed trigger executions
6. **Graceful Degradation**: Handle partial failures in event batches

## Related Documentation

- [Functions Service](../service-catalog.md#2-functions-service)
- [Automation Service](automation.md)
- [Engine Bus](bus.md)
- [Service Catalog](../service-catalog.md)
