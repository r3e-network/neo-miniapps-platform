# Service Layer - Complete Service Catalog

This document provides a comprehensive reference for all 17 services in the Neo N3 Service Layer, organized by functional domain.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SERVICE LAYER ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    SERVICES LAYER (17 Applications)                  │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │   │
│  │  │Accounts │ │Functions│ │  Oracle │ │ Gasbank │ │   VRF   │       │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘       │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │   │
│  │  │DataFeeds│ │DataLink │ │Streams  │ │PriceFeed│ │  Random │       │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘       │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │   │
│  │  │Automation│ │Triggers │ │ Secrets │ │  CCIP   │ │   CRE   │       │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘       │   │
│  │  ┌─────────┐ ┌─────────┐                                            │   │
│  │  │Confident│ │   DTA   │                                            │   │
│  │  └─────────┘ └─────────┘                                            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                      ENGINE LAYER (OS Kernel)                        │   │
│  │     Registry │ Lifecycle │ Bus │ Health │ Recovery │ Metrics         │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     FRAMEWORK LAYER (SDK)                            │   │
│  │     ServiceBase │ BusClient │ Manifest │ Builder │ Testing           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     PLATFORM LAYER (Drivers)                         │   │
│  │       RPC │ Storage │ Cache │ Queue │ Crypto │ Migrations            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Service Categories

### Account & Identity Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **Accounts** | Account registry with tenant isolation | Create, List, Get, Delete, Metadata |
| **Secrets** | Encrypted secret vault with ACL | Create, Get, List, Delete, Encryption |

### Function Execution Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **Functions** | JavaScript function execution with Devpack SDK | Create, Execute, List Executions |
| **Automation** | Cron-style job scheduling | Create Job, List Jobs, Enable/Disable |
| **Triggers** | Event/webhook routing to functions | Create, List, Validate, Route |

### Oracle & Data Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **Oracle** | HTTP adapter for external data sources | Create Source, Submit Request, Retry |
| **DataFeeds** | Chainlink-style signed data feeds | Create Feed, Submit Update, Aggregation |
| **PriceFeed** | Deviation-based oracle aggregation | Create Feed, Submit Observation, Rounds |
| **DataStreams** | Real-time high-frequency data streams | Create Stream, Publish Frame, List |
| **DataLink** | Data delivery channels | Create Channel, Queue Delivery |

### Randomness Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **Random** | ED25519 signed random number generation | Generate, List History |
| **VRF** | Verifiable Random Function | Create Key, Submit Request |

### Financial Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **GasBank** | Service-owned gas accounts and settlement | Deposit, Withdraw, Balance, Settlement |

### Cross-Chain & Advanced Services

| Service | Description | Key Operations |
|---------|-------------|----------------|
| **CCIP** | Cross-Chain Interoperability Protocol | Create Lane, Send Message, List |
| **CRE** | Composable Run Engine playbooks | Create Playbook, Execute Run |
| **Confidential** | TEE enclaves and sealed keys | Create Enclave, Seal Key, Attest |
| **DTA** | Decentralized Trading Architecture | Create Product, Submit Order |

---

## 1. Accounts Service

**Purpose**: Account registry with pluggable storage and multi-tenant isolation.

**Location**: `internal/services/accounts/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts` | Create new account |
| GET | `/accounts` | List all accounts |
| GET | `/accounts/{id}` | Get account by ID |
| DELETE | `/accounts/{id}` | Delete account |

### Example Usage

```bash
# Create account with tenant
curl -s -X POST http://localhost:8080/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: tenant-a" \
  -H "Content-Type: application/json" \
  -d '{"owner":"alice","metadata":{"env":"prod"}}'

# List accounts
curl -s -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: tenant-a" \
  http://localhost:8080/accounts
```

### Data Model

```go
type Account struct {
    ID        string            `json:"id"`
    Owner     string            `json:"owner"`
    Metadata  map[string]string `json:"metadata"`
    Tenant    string            `json:"tenant"`
    CreatedAt time.Time         `json:"created_at"`
    UpdatedAt time.Time         `json:"updated_at"`
}
```

---

## 2. Functions Service

**Purpose**: JavaScript function execution with Devpack SDK integration.

**Location**: `internal/services/functions/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/functions` | Create function |
| GET | `/accounts/{account}/functions` | List functions |
| GET | `/accounts/{account}/functions/{id}` | Get function |
| DELETE | `/accounts/{account}/functions/{id}` | Delete function |
| POST | `/accounts/{account}/functions/{id}/execute` | Execute function |
| GET | `/accounts/{account}/functions/{id}/executions` | List executions |

### Example Usage

```bash
# Create function
FUNC_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-updater",
    "runtime": "js",
    "source": "(params, secrets) => ({ price: params.value * 1.1 })",
    "secrets": ["apiKey"]
  }' | jq -r .ID)

# Execute function
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/functions/$FUNC_ID/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"value": 100}'
```

### Devpack SDK

Functions have access to the Devpack SDK:

```javascript
// In function source
export default function(params, secrets) {
  // Oracle request
  Devpack.oracle.request({ dataSourceId: "src-1", payload: "{}" });

  // Price feed submission
  Devpack.priceFeeds.recordSnapshot({ feedId: "feed-1", price: 12.34 });

  // Random generation
  const rand = Devpack.random.generate({ length: 32 });

  // HTTP request
  const resp = Devpack.http.request({
    url: "https://api.example.com/data",
    method: "GET",
    headers: { "Authorization": `Bearer ${secrets.apiKey}` }
  });

  return Devpack.respond.success({ result: resp.body });
}
```

---

## 3. Automation Service

**Purpose**: Cron-style job scheduling for function execution.

**Location**: `internal/services/automation/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/automation/jobs` | Create job |
| GET | `/accounts/{account}/automation/jobs` | List jobs |
| GET | `/accounts/{account}/automation/jobs/{id}` | Get job |
| PATCH | `/accounts/{account}/automation/jobs/{id}` | Update job |
| DELETE | `/accounts/{account}/automation/jobs/{id}` | Delete job |

### Example Usage

```bash
# Create scheduled job (every 5 minutes)
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "'"$FUNC_ID"'",
    "schedule": "*/5 * * * *",
    "payload": {"action": "refresh"},
    "enabled": true
  }'

# List jobs
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/automation/jobs
```

### Schedule Formats

| Format | Example | Description |
|--------|---------|-------------|
| Cron | `*/5 * * * *` | Every 5 minutes |
| Interval | `@every 1h` | Every hour |
| Daily | `@daily` | Once per day at midnight |
| Hourly | `@hourly` | Every hour |

---

## 4. Triggers Service

**Purpose**: Event/webhook routing to functions.

**Location**: `internal/services/triggers/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/triggers` | Create trigger |
| GET | `/accounts/{account}/triggers` | List triggers |
| GET | `/accounts/{account}/triggers/{id}` | Get trigger |
| PATCH | `/accounts/{account}/triggers/{id}` | Update trigger |
| DELETE | `/accounts/{account}/triggers/{id}` | Delete trigger |

### Example Usage

```bash
# Create webhook trigger
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-webhook",
    "type": "webhook",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "path": "/hooks/price-update",
      "method": "POST"
    },
    "enabled": true
  }'

# Create event trigger
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "transfer-event",
    "type": "event",
    "function_id": "'"$FUNC_ID"'",
    "config": {
      "event_type": "neo.transfer",
      "filter": {"asset": "NEO"}
    },
    "enabled": true
  }'
```

---

## 5. Secrets Service

**Purpose**: Encrypted secret storage with ACL enforcement.

**Location**: `internal/services/secrets/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/secrets` | Create secret |
| GET | `/accounts/{account}/secrets` | List secrets (names only) |
| GET | `/accounts/{account}/secrets/{name}` | Get secret |
| PUT | `/accounts/{account}/secrets/{name}` | Update secret |
| DELETE | `/accounts/{account}/secrets/{name}` | Delete secret |

### Example Usage

```bash
# Create secret
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/secrets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "apiKey", "value": "sk-123456789"}'

# List secrets (values are hidden)
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/secrets

# Get secret (requires explicit access)
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/secrets/apiKey
```

### Security

- Secrets are encrypted at rest using AES-GCM
- Configure `SECRET_ENCRYPTION_KEY` (16/24/32 byte key)
- Secrets are injected into function execution automatically

---

## 6. Oracle Service

**Purpose**: HTTP adapter for external data sources with retry logic.

**Location**: `internal/services/oracle/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/oracle/sources` | Create data source |
| GET | `/accounts/{account}/oracle/sources` | List sources |
| GET | `/accounts/{account}/oracle/sources/{id}` | Get source |
| POST | `/accounts/{account}/oracle/requests` | Submit request |
| GET | `/accounts/{account}/oracle/requests` | List requests |
| PATCH | `/accounts/{account}/oracle/requests/{id}` | Update status |

### Example Usage

```bash
# Create data source
SRC_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/oracle/sources \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "coingecko",
    "url": "https://api.coingecko.com/api/v3/simple/price",
    "method": "GET",
    "headers": {"Accept": "application/json"}
  }' | jq -r .ID)

# Submit oracle request
REQ_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/oracle/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "data_source_id": "'"$SRC_ID"'",
    "payload": "{\"ids\":\"neo\",\"vs_currencies\":\"usd\"}"
  }' | jq -r .ID)

# Check request status
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/oracle/requests/$REQ_ID
```

### Request Lifecycle

```
pending → running → succeeded/failed → (retry) → succeeded/dlq
```

---

## 7. DataFeeds Service

**Purpose**: Chainlink-style signed data feeds with aggregation.

**Location**: `internal/services/datafeeds/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/datafeeds` | Create feed |
| GET | `/accounts/{account}/datafeeds` | List feeds |
| GET | `/accounts/{account}/datafeeds/{id}` | Get feed |
| PUT | `/accounts/{account}/datafeeds/{id}` | Update feed |
| POST | `/accounts/{account}/datafeeds/{id}/updates` | Submit update |
| GET | `/accounts/{account}/datafeeds/{id}/updates` | List updates |
| GET | `/accounts/{account}/datafeeds/{id}/latest` | Get latest |

### Example Usage

```bash
# Create feed with median aggregation
FEED_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datafeeds \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pair": "NEO/USD",
    "decimals": 8,
    "aggregation": "median",
    "heartbeat_seconds": 3600,
    "threshold_ppm": 5000,
    "signer_set": ["0xabc123...", "0xdef456..."]
  }' | jq -r .id)

# Submit signed update
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datafeeds/$FEED_ID/updates \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "round_id": 1,
    "price": "12.34567890",
    "signer": "0xabc123...",
    "signature": "0x..."
  }'
```

### Aggregation Strategies

| Strategy | Description |
|----------|-------------|
| `median` | Middle value (default) |
| `mean` | Average of all values |
| `min` | Minimum value |
| `max` | Maximum value |

See [DataFeeds Quickstart](examples/datafeeds.md) for detailed guide.

---

## 8. PriceFeed Service

**Purpose**: Decentralized oracle aggregation with deviation-based publishing.

**Location**: `internal/services/pricefeed/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/pricefeeds` | Create feed |
| GET | `/accounts/{account}/pricefeeds` | List feeds |
| GET | `/accounts/{account}/pricefeeds/{id}` | Get feed |
| PATCH | `/accounts/{account}/pricefeeds/{id}` | Update feed |
| DELETE | `/accounts/{account}/pricefeeds/{id}` | Delete feed |
| POST | `/accounts/{account}/pricefeeds/{id}/snapshots` | Submit observation |
| GET | `/accounts/{account}/pricefeeds/{id}/snapshots` | List snapshots |

### Example Usage

```bash
# Create price feed
FEED_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "base_asset": "NEO",
    "quote_asset": "USD",
    "deviation_percent": 1.0,
    "update_interval": "@every 5m",
    "heartbeat_interval": "@every 1h"
  }' | jq -r .ID)

# Submit price observation
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/pricefeeds/$FEED_ID/snapshots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"price": 12.34, "source": "binance"}'
```

See [PriceFeeds Quickstart](examples/pricefeeds.md) for detailed guide.

---

## 9. DataStreams Service

**Purpose**: Real-time high-frequency data streams.

**Location**: `internal/services/datastreams/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/datastreams` | Create stream |
| GET | `/accounts/{account}/datastreams` | List streams |
| GET | `/accounts/{account}/datastreams/{id}` | Get stream |
| PUT | `/accounts/{account}/datastreams/{id}` | Update stream |
| POST | `/accounts/{account}/datastreams/{id}/frames` | Publish frame |
| GET | `/accounts/{account}/datastreams/{id}/frames` | List frames |
| GET | `/accounts/{account}/datastreams/{id}/latest` | Get latest frame |

### Example Usage

```bash
# Create stream
STREAM_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datastreams \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "neo-ticker",
    "symbol": "NEO",
    "description": "NEO price ticker",
    "frequency": "1s",
    "sla_ms": 50,
    "status": "active"
  }' | jq -r .ID)

# Publish frame
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datastreams/$STREAM_ID/frames \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sequence": 1,
    "payload": {"price": 12.34, "volume": 1000000},
    "latency_ms": 5,
    "status": "delivered"
  }'
```

---

## 10. DataLink Service

**Purpose**: Data delivery channels with dispatcher pattern.

**Location**: `internal/services/datalink/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/datalink/channels` | Create channel |
| GET | `/accounts/{account}/datalink/channels` | List channels |
| GET | `/accounts/{account}/datalink/channels/{id}` | Get channel |
| PUT | `/accounts/{account}/datalink/channels/{id}` | Update channel |
| POST | `/accounts/{account}/datalink/channels/{id}/deliveries` | Queue delivery |
| GET | `/accounts/{account}/datalink/deliveries` | List deliveries |

### Example Usage

```bash
# Create channel
CH_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datalink/channels \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-publisher",
    "endpoint": "https://consumer.example.com/webhook",
    "signer_set": ["0xabc..."],
    "status": "active"
  }' | jq -r .ID)

# Queue delivery
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/datalink/channels/$CH_ID/deliveries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {"event": "price_update", "data": {"neo": 12.34}},
    "metadata": {"trace_id": "abc-123"}
  }'
```

See [DataLink Quickstart](examples/datalink.md) for detailed guide.

---

## 11. Random Service

**Purpose**: Cryptographically secure random number generation with ED25519 signatures.

**Location**: `internal/services/random/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/random` | Generate random bytes |
| GET | `/accounts/{account}/random/requests` | List history |

### Example Usage

```bash
# Generate 32 random bytes
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/random \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"length": 32, "request_id": "lottery-001"}'

# Response includes signed random data
{
  "request_id": "lottery-001",
  "random_bytes": "base64-encoded-bytes",
  "signature": "ed25519-signature",
  "public_key": "signer-public-key"
}
```

### CLI

```bash
slctl random generate --account $ACCOUNT_ID --length 64
slctl random list --account $ACCOUNT_ID --limit 10
```

---

## 12. VRF Service

**Purpose**: Verifiable Random Function for provably fair randomness.

**Location**: `internal/services/vrf/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/vrf/keys` | Create VRF key |
| GET | `/accounts/{account}/vrf/keys` | List keys |
| GET | `/accounts/{account}/vrf/keys/{id}` | Get key |
| POST | `/accounts/{account}/vrf/requests` | Submit VRF request |
| GET | `/accounts/{account}/vrf/requests` | List requests |
| GET | `/accounts/{account}/vrf/requests/{id}` | Get request |

### Example Usage

```bash
# Create VRF key
KEY_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/vrf/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "lottery-key", "algorithm": "secp256k1"}' | jq -r .ID)

# Submit VRF request
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/vrf/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key_id": "'"$KEY_ID"'",
    "seed": "lottery-round-123",
    "callback_url": "https://app.example.com/vrf-callback"
  }'
```

### VRF vs Random

| Feature | Random | VRF |
|---------|--------|-----|
| Verification | Signature-based | Cryptographic proof |
| Use case | General randomness | Provably fair gaming |
| Speed | Fast | Slower (proof generation) |

---

## 13. GasBank Service

**Purpose**: Service-owned gas accounts for transaction subsidization.

**Location**: `internal/services/gasbank/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/gasbank/deposit` | Deposit funds |
| POST | `/accounts/{account}/gasbank/withdraw` | Request withdrawal |
| GET | `/accounts/{account}/gasbank/balance` | Get balance |
| GET | `/accounts/{account}/gasbank/transactions` | List transactions |
| GET | `/accounts/{account}/gasbank/summary` | Get summary |

### Example Usage

```bash
# Deposit funds
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/gasbank/deposit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 10.0, "tx_id": "0xabc123..."}'

# Request withdrawal
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/gasbank/withdraw \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 5.0,
    "to_address": "NeoAddress123...",
    "scheduled_at": "2025-01-20T10:00:00Z"
  }'

# Check balance
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/accounts/$ACCOUNT_ID/gasbank/balance
```

See [GasBank Workflows](gasbank-workflows.md) for detailed guide.

---

## 14. CCIP Service

**Purpose**: Cross-Chain Interoperability Protocol for multi-chain messaging.

**Location**: `internal/services/ccip/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/ccip/lanes` | Create lane |
| GET | `/accounts/{account}/ccip/lanes` | List lanes |
| GET | `/accounts/{account}/ccip/lanes/{id}` | Get lane |
| PUT | `/accounts/{account}/ccip/lanes/{id}` | Update lane |
| POST | `/accounts/{account}/ccip/messages` | Send message |
| GET | `/accounts/{account}/ccip/messages` | List messages |
| GET | `/accounts/{account}/ccip/messages/{id}` | Get message |

### Example Usage

```bash
# Create cross-chain lane
LANE_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/ccip/lanes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "neo-to-eth",
    "source_chain": "neo-mainnet",
    "dest_chain": "ethereum-mainnet",
    "source_contract": "0xneo...",
    "dest_contract": "0xeth...",
    "status": "active"
  }' | jq -r .ID)

# Send cross-chain message
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/ccip/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "lane_id": "'"$LANE_ID"'",
    "payload": {"action": "transfer", "amount": "100"},
    "gas_limit": 200000
  }'
```

### Message Lifecycle

```
pending → inflight → delivered/failed → (retry) → confirmed
```

---

## 15. CRE Service

**Purpose**: Composable Run Engine for complex workflow orchestration.

**Location**: `internal/services/cre/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/cre/playbooks` | Create playbook |
| GET | `/accounts/{account}/cre/playbooks` | List playbooks |
| GET | `/accounts/{account}/cre/playbooks/{id}` | Get playbook |
| PUT | `/accounts/{account}/cre/playbooks/{id}` | Update playbook |
| POST | `/accounts/{account}/cre/executors` | Create executor |
| GET | `/accounts/{account}/cre/executors` | List executors |
| POST | `/accounts/{account}/cre/runs` | Start run |
| GET | `/accounts/{account}/cre/runs` | List runs |
| GET | `/accounts/{account}/cre/runs/{id}` | Get run |

### Example Usage

```bash
# Create playbook
PB_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/cre/playbooks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-aggregation",
    "description": "Multi-source price aggregation workflow",
    "steps": [
      {"action": "fetch", "source": "oracle-1"},
      {"action": "fetch", "source": "oracle-2"},
      {"action": "aggregate", "method": "median"},
      {"action": "publish", "target": "pricefeed-1"}
    ],
    "status": "active"
  }' | jq -r .ID)

# Start run
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/cre/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "playbook_id": "'"$PB_ID"'",
    "params": {"pair": "NEO/USD"}
  }'
```

---

## 16. Confidential Service

**Purpose**: TEE enclave management and sealed key operations.

**Location**: `internal/services/confidential/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/confidential/enclaves` | Create enclave |
| GET | `/accounts/{account}/confidential/enclaves` | List enclaves |
| GET | `/accounts/{account}/confidential/enclaves/{id}` | Get enclave |
| PUT | `/accounts/{account}/confidential/enclaves/{id}` | Update enclave |
| POST | `/accounts/{account}/confidential/enclaves/{id}/keys` | Seal key |
| GET | `/accounts/{account}/confidential/enclaves/{id}/keys` | List keys |
| POST | `/accounts/{account}/confidential/enclaves/{id}/attest` | Request attestation |
| GET | `/accounts/{account}/confidential/attestations` | List attestations |

### Example Usage

```bash
# Create enclave
ENCLAVE_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/confidential/enclaves \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "secure-signer",
    "type": "sgx",
    "measurement": "mrenclave-hash...",
    "status": "active"
  }' | jq -r .ID)

# Seal key in enclave
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "signing-key",
    "algorithm": "secp256k1",
    "policy": {"require_attestation": true}
  }'

# Request attestation
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/attest \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"challenge": "random-nonce-123"}'
```

---

## 17. DTA Service

**Purpose**: Decentralized Trading Architecture for product/order management.

**Location**: `internal/services/dta/`

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/accounts/{account}/dta/products` | Create product |
| GET | `/accounts/{account}/dta/products` | List products |
| GET | `/accounts/{account}/dta/products/{id}` | Get product |
| PUT | `/accounts/{account}/dta/products/{id}` | Update product |
| POST | `/accounts/{account}/dta/orders` | Submit order |
| GET | `/accounts/{account}/dta/orders` | List orders |
| GET | `/accounts/{account}/dta/orders/{id}` | Get order |

### Example Usage

```bash
# Create product
PROD_ID=$(curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/dta/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NEO-PERP",
    "type": "perpetual",
    "base_asset": "NEO",
    "quote_asset": "USDT",
    "tick_size": "0.01",
    "lot_size": "0.1",
    "status": "active"
  }' | jq -r .ID)

# Submit order
curl -s -X POST http://localhost:8080/accounts/$ACCOUNT_ID/dta/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "'"$PROD_ID"'",
    "side": "buy",
    "type": "limit",
    "price": "12.50",
    "quantity": "10.0",
    "time_in_force": "GTC"
  }'
```

---

## CLI Quick Reference

```bash
# Account management
slctl accounts list|create|get|delete

# Functions
slctl functions list|create|get|delete|execute --account <id>

# Automation
slctl automation jobs list|create|get|update|delete --account <id>

# Oracle
slctl oracle sources|requests list|create|get --account <id>

# Data services
slctl datafeeds list|create|submit|latest --account <id>
slctl pricefeeds list|create|get|update|delete|snapshots --account <id>
slctl datastreams list|create|publish --account <id>
slctl datalink channels|deliveries --account <id>

# Randomness
slctl random generate|list --account <id>
slctl vrf keys|requests --account <id>

# Financial
slctl gasbank deposit|withdraw|balance|summary --account <id>

# Cross-chain
slctl ccip lanes|messages --account <id>
slctl cre playbooks|executors|runs --account <id>

# Confidential
slctl confcompute enclaves|keys|attestations --account <id>

# Trading
slctl dta products|orders --account <id>

# System
slctl status
slctl services list
slctl bus events|data|compute
```

---

## Related Documentation

- [Architecture Layers](architecture-layers.md)
- [Service Engine Architecture](service-engine-architecture.md)
- [API Examples](examples/services.md)
- [Operations Runbook](ops-runbook.md)
- [Security Hardening](security-hardening.md)
