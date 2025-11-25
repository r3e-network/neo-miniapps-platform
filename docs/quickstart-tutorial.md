# Service Layer Quickstart Tutorial

Complete end-to-end tutorial: from zero to running your first oracle-powered function in 15 minutes.

## Prerequisites

```bash
# Go 1.24+
go version

# Docker & Docker Compose (optional, for full stack)
docker --version
docker compose version

# jq for JSON parsing (optional)
jq --version
```

## Part 1: Local Development (5 minutes)

### Step 1: Clone and Build

```bash
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer

# Build binaries
make build

# Verify
./bin/appserver --help
./bin/slctl --help
```

### Step 2: Start the Server (In-Memory)

```bash
# Set authentication token
export API_TOKENS=dev-token

# Start server (in-memory, no database required)
./bin/appserver

# Server is now running at http://localhost:8080
```

### Step 3: Verify It Works

```bash
# In another terminal
export TOKEN=dev-token

# Check health
curl -s http://localhost:8080/readyz
# Output: {"status":"ok",...}

# Check system status
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/system/status | jq '.modules_meta'
# Output: {"total":X,"started":X,...}
```

---

## Part 2: Create Your First Account (2 minutes)

### Using curl

```bash
export TOKEN=dev-token
export TENANT=tutorial-tenant

# Create account
ACCOUNT_ID=$(curl -s -X POST http://localhost:8080/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"owner":"tutorial-user","metadata":{"env":"dev"}}' | jq -r '.id')

echo "Account ID: $ACCOUNT_ID"

# Verify account exists
curl -s -H "Authorization: Bearer $TOKEN" -H "X-Tenant-ID: $TENANT" \
  http://localhost:8080/accounts/$ACCOUNT_ID | jq
```

### Using CLI

```bash
# Create account
ACCOUNT_ID=$(./bin/slctl accounts create \
  --token $TOKEN \
  --tenant $TENANT \
  --owner "tutorial-user" \
  --metadata '{"env":"dev"}' | jq -r '.id')

# List accounts
./bin/slctl accounts list --token $TOKEN --tenant $TENANT
```

---

## Part 3: Store a Secret (1 minute)

Secrets are encrypted at rest and injected into functions automatically.

```bash
# Create a secret
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"apiKey","value":"my-super-secret-key"}'

# List secrets (values are hidden)
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/secrets | jq
```

---

## Part 4: Create and Execute a Function (3 minutes)

### Create a Simple Function

```bash
# Function that echoes input and uses secrets
FUNC_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello-world",
    "runtime": "js",
    "source": "(params, secrets) => ({ message: `Hello, ${params.name}!`, hasSecret: !!secrets.apiKey })",
    "secrets": ["apiKey"]
  }' | jq -r '.ID')

echo "Function ID: $FUNC_ID"
```

### Execute the Function

```bash
# Execute with parameters
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/functions/$FUNC_ID/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}' | jq

# Output:
# {
#   "result": {
#     "message": "Hello, World!",
#     "hasSecret": true
#   }
# }
```

### List Executions

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/functions/$FUNC_ID/executions | jq
```

---

## Part 5: Oracle Data Source (3 minutes)

Create an oracle that fetches external data.

### Create Data Source

```bash
# Create oracle data source
SRC_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/oracle/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "crypto-prices",
    "url": "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
    "method": "GET",
    "headers": {"Accept": "application/json"}
  }' | jq -r '.ID')

echo "Source ID: $SRC_ID"
```

### Submit Oracle Request

```bash
# Submit request
REQ_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/oracle/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"data_source_id\": \"$SRC_ID\", \"payload\": \"{}\"}" | jq -r '.ID')

echo "Request ID: $REQ_ID"

# Check request status
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/oracle/requests/$REQ_ID | jq
```

---

## Part 6: Price Feed with Deviation Publishing (2 minutes)

Create a price feed that publishes on deviation threshold.

```bash
# Create price feed: NEO/USD with 1% deviation threshold
FEED_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "base_asset": "NEO",
    "quote_asset": "USD",
    "deviation_percent": 1.0,
    "update_interval": "@every 5m",
    "heartbeat_interval": "@every 1h"
  }' | jq -r '.ID')

echo "Feed ID: $FEED_ID"

# Submit price observations
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"price": 12.34, "source": "binance"}'

curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"price": 12.50, "source": "coinbase"}'

# Check aggregated rounds
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID | jq
```

---

## Part 7: Schedule Automation (1 minute)

Schedule the function to run periodically.

```bash
# Create scheduled job (every 5 minutes)
JOB_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"function_id\": \"$FUNC_ID\",
    \"schedule\": \"*/5 * * * *\",
    \"payload\": {\"name\": \"Scheduler\"},
    \"enabled\": true
  }" | jq -r '.ID')

echo "Job ID: $JOB_ID"

# List jobs
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/automation/jobs | jq
```

---

## Part 8: Using the Engine Bus (1 minute)

Publish events and data across services using the engine bus.

```bash
# Publish event to all EventEngines
curl -s -X POST http://localhost:8080/system/events \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"event": "tutorial.completed", "payload": {"user": "tutorial-user"}}'

# Push data to all DataEngines
curl -s -X POST http://localhost:8080/system/data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"topic": "metrics/tutorial", "payload": {"step": "completed"}}'

# CLI equivalents
./bin/slctl bus events --token $TOKEN --event "tutorial.completed" --payload '{"user":"cli-user"}'
./bin/slctl bus data --token $TOKEN --topic "metrics/cli" --payload '{"step":"done"}'
```

---

## Part 9: Full Stack with Docker (2 minutes)

For production-like environment with PostgreSQL persistence:

```bash
# Start full stack (Postgres + API + Dashboard)
make run

# Wait for services to start...
docker compose ps

# Access points:
# - API: http://localhost:8080
# - Dashboard: http://localhost:8081
# - Marketing site: http://localhost:8082
```

### Dashboard Quick Access

Open the dashboard with pre-filled credentials:
```
http://localhost:8081/?api=http://localhost:8080&token=dev-token&tenant=tutorial-tenant
```

---

## Summary: What You Built

In this tutorial, you:

1. **Started the Service Layer** in-memory for quick development
2. **Created an Account** with tenant isolation
3. **Stored a Secret** with encryption at rest
4. **Created and Executed a Function** with secret injection
5. **Set up an Oracle** for external data fetching
6. **Created a Price Feed** with deviation-based publishing
7. **Scheduled Automation** for periodic execution
8. **Used the Engine Bus** for cross-service communication
9. **Deployed Full Stack** with Docker

---

## Next Steps

| Goal | Documentation |
|------|---------------|
| Learn all 17 services | [Service Catalog](service-catalog.md) |
| Build custom services | [Developer Guide](developer-guide.md) |
| Understand architecture | [Architecture Layers](architecture-layers.md) |
| Production deployment | [Security Hardening](security-hardening.md) |
| Operations | [Operations Runbook](ops-runbook.md) |

---

## Troubleshooting

### Common Issues

**401 Unauthorized**
```bash
# Ensure token is set
export TOKEN=dev-token
# Ensure header is correct
curl -H "Authorization: Bearer $TOKEN" ...
```

**403 Forbidden (Tenant)**
```bash
# Ensure tenant header matches account
curl -H "X-Tenant-ID: $TENANT" ...
```

**Connection Refused**
```bash
# Check server is running
curl http://localhost:8080/readyz
# Check logs
docker compose logs appserver
```

### Get Help

```bash
# CLI help
./bin/slctl --help
./bin/slctl accounts --help

# API documentation
curl -s http://localhost:8080/system/status | jq
```

---

## CLI Quick Reference

```bash
# Account management
slctl accounts list|create|get|delete

# Functions
slctl functions list|create|execute --account <id>

# Oracle
slctl oracle sources|requests list|create --account <id>

# Price feeds
slctl pricefeeds list|create|snapshots --account <id>

# Automation
slctl automation jobs list|create --account <id>

# System
slctl status
slctl services list
slctl bus events|data|compute
```
