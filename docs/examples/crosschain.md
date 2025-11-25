# Cross-Chain Services Quickstart

Guide to CCIP (Cross-Chain Interoperability Protocol) and CRE (Composable Run Engine) services.

## Overview

| Service | Purpose | Use Case |
|---------|---------|----------|
| **CCIP** | Cross-chain messaging | Bridge assets, sync state |
| **CRE** | Workflow orchestration | Complex multi-step operations |

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
export ACCOUNT_ID=<your-account-id>
```

---

# CCIP Service

## Overview

CCIP enables secure cross-chain communication between Neo N3 and other blockchains.

### Key Concepts

- **Lane**: A bidirectional path between two chains
- **Message**: Data payload sent across chains
- **Finality**: Confirmation that message was delivered

## Quick Start

### 1. Create Cross-Chain Lane

```bash
LANE_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/ccip/lanes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "neo-to-ethereum",
    "source_chain": "neo-mainnet",
    "dest_chain": "ethereum-mainnet",
    "source_contract": "0xNeoContractHash...",
    "dest_contract": "0xEthContractAddress...",
    "fee_token": "GAS",
    "status": "active"
  }' | jq -r .ID)

echo "Lane ID: $LANE_ID"
```

### 2. Send Cross-Chain Message

```bash
MSG_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/ccip/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "lane_id": "'"$LANE_ID"'",
    "receiver": "0xReceiverAddress...",
    "payload": {
      "action": "transfer",
      "token": "NEO",
      "amount": "100",
      "recipient": "0xEthRecipient..."
    },
    "gas_limit": 200000,
    "strict": false
  }' | jq -r .ID)

echo "Message ID: $MSG_ID"
```

### 3. Track Message Status

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/ccip/messages/$MSG_ID | jq
```

**Response**:
```json
{
  "ID": "msg-uuid",
  "LaneID": "lane-uuid",
  "SourceTxHash": "0xneo...",
  "DestTxHash": "0xeth...",
  "Status": "delivered",
  "Payload": {"action": "transfer", "...": "..."},
  "GasLimit": 200000,
  "GasUsed": 150000,
  "CreatedAt": "2025-01-15T10:00:00Z",
  "DeliveredAt": "2025-01-15T10:02:00Z"
}
```

## Message Lifecycle

```
pending → inflight → delivered
                  → failed → retry → delivered
                           → dlq (dead letter)
```

## API Reference

### Create Lane

```http
POST /accounts/{account}/ccip/lanes
```

```json
{
  "name": "neo-eth-bridge",
  "source_chain": "neo-mainnet",
  "dest_chain": "ethereum-mainnet",
  "source_contract": "0x...",
  "dest_contract": "0x...",
  "fee_token": "GAS",
  "min_confirmations": 12,
  "status": "active"
}
```

### Send Message

```http
POST /accounts/{account}/ccip/messages
```

```json
{
  "lane_id": "lane-uuid",
  "receiver": "0x...",
  "payload": {"key": "value"},
  "gas_limit": 200000,
  "strict": false,
  "extra_args": {}
}
```

### List Messages

```http
GET /accounts/{account}/ccip/messages?limit=20&status=pending
```

## CLI Usage

```bash
# List lanes
slctl ccip lanes list --account $ACCOUNT_ID

# Create lane
slctl ccip lanes create --account $ACCOUNT_ID \
  --name "neo-eth" \
  --source neo-mainnet \
  --dest ethereum-mainnet

# Send message
slctl ccip messages send --account $ACCOUNT_ID \
  --lane $LANE_ID \
  --payload '{"action":"transfer"}'

# List messages
slctl ccip messages list --account $ACCOUNT_ID --status pending
```

## Use Cases

### Token Bridge

```bash
# Bridge NEO to Ethereum
curl -s -X POST $API/accounts/$ACCOUNT_ID/ccip/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "lane_id": "'"$LANE_ID"'",
    "receiver": "0xBridgeContract...",
    "payload": {
      "action": "bridge_tokens",
      "source_token": "NEO",
      "amount": "1000000000",
      "recipient": "0xUserWallet..."
    },
    "gas_limit": 300000
  }'
```

### Cross-Chain NFT

```bash
# Transfer NFT metadata cross-chain
curl -s -X POST $API/accounts/$ACCOUNT_ID/ccip/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "lane_id": "'"$LANE_ID"'",
    "receiver": "0xNFTBridge...",
    "payload": {
      "action": "mint_nft",
      "source_contract": "NeoNFTHash...",
      "token_id": "42",
      "metadata_uri": "ipfs://Qm...",
      "owner": "0xNewOwner..."
    },
    "gas_limit": 500000
  }'
```

---

# CRE Service (Composable Run Engine)

## Overview

CRE orchestrates complex multi-step workflows through playbooks and executors.

### Key Concepts

- **Playbook**: Workflow definition with steps
- **Executor**: Runtime environment for playbook execution
- **Run**: Single execution instance of a playbook

## Quick Start

### 1. Create Playbook

```bash
PB_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/cre/playbooks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-aggregation-workflow",
    "description": "Fetch prices from multiple sources and publish",
    "version": "1.0.0",
    "steps": [
      {
        "id": "fetch-binance",
        "action": "http.get",
        "params": {
          "url": "https://api.binance.com/api/v3/ticker/price",
          "query": {"symbol": "NEOUSDT"}
        }
      },
      {
        "id": "fetch-coingecko",
        "action": "http.get",
        "params": {
          "url": "https://api.coingecko.com/api/v3/simple/price",
          "query": {"ids": "neo", "vs_currencies": "usd"}
        }
      },
      {
        "id": "aggregate",
        "action": "transform.median",
        "depends_on": ["fetch-binance", "fetch-coingecko"],
        "params": {
          "inputs": ["${fetch-binance.price}", "${fetch-coingecko.neo.usd}"]
        }
      },
      {
        "id": "publish",
        "action": "pricefeed.submit",
        "depends_on": ["aggregate"],
        "params": {
          "feed_id": "neo-usd-feed",
          "price": "${aggregate.result}",
          "source": "cre-aggregator"
        }
      }
    ],
    "status": "active"
  }' | jq -r .ID)

echo "Playbook ID: $PB_ID"
```

### 2. Create Executor

```bash
EXEC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/cre/executors \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-executor",
    "type": "scheduled",
    "config": {
      "schedule": "@every 1m",
      "timeout_seconds": 30,
      "retry_count": 3
    },
    "status": "active"
  }' | jq -r .ID)

echo "Executor ID: $EXEC_ID"
```

### 3. Start Run

```bash
RUN_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/cre/runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "playbook_id": "'"$PB_ID"'",
    "executor_id": "'"$EXEC_ID"'",
    "params": {
      "pair": "NEO/USD",
      "feed_id": "feed-123"
    }
  }' | jq -r .ID)

echo "Run ID: $RUN_ID"
```

### 4. Track Run Status

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/cre/runs/$RUN_ID | jq
```

**Response**:
```json
{
  "ID": "run-uuid",
  "PlaybookID": "pb-uuid",
  "ExecutorID": "exec-uuid",
  "Status": "completed",
  "StartedAt": "2025-01-15T10:00:00Z",
  "CompletedAt": "2025-01-15T10:00:05Z",
  "Steps": [
    {"id": "fetch-binance", "status": "completed", "duration_ms": 150},
    {"id": "fetch-coingecko", "status": "completed", "duration_ms": 200},
    {"id": "aggregate", "status": "completed", "duration_ms": 5},
    {"id": "publish", "status": "completed", "duration_ms": 50}
  ],
  "Output": {"published_price": 12.34}
}
```

## Run Lifecycle

```
pending → running → completed
                 → failed → (manual retry)
                 → cancelled
```

## Playbook Actions

| Action | Description |
|--------|-------------|
| `http.get` | HTTP GET request |
| `http.post` | HTTP POST request |
| `transform.median` | Calculate median |
| `transform.mean` | Calculate mean |
| `transform.map` | Transform array |
| `pricefeed.submit` | Submit to price feed |
| `oracle.request` | Oracle data request |
| `function.execute` | Execute function |
| `condition.if` | Conditional branch |
| `loop.foreach` | Iterate collection |

## CLI Usage

```bash
# List playbooks
slctl cre playbooks list --account $ACCOUNT_ID

# Create playbook from file
slctl cre playbooks create --account $ACCOUNT_ID \
  --file playbook.yaml

# List executors
slctl cre executors list --account $ACCOUNT_ID

# Start run
slctl cre runs start --account $ACCOUNT_ID \
  --playbook $PB_ID \
  --params '{"pair":"NEO/USD"}'

# List runs
slctl cre runs list --account $ACCOUNT_ID --status completed
```

## Use Cases

### Multi-Source Price Aggregation

See playbook example above.

### Automated Trading Strategy

```json
{
  "name": "arbitrage-detector",
  "steps": [
    {"id": "get-cex-price", "action": "http.get", "params": {...}},
    {"id": "get-dex-price", "action": "http.get", "params": {...}},
    {"id": "calculate-spread", "action": "transform.subtract", "depends_on": ["get-cex-price", "get-dex-price"]},
    {"id": "check-threshold", "action": "condition.if", "depends_on": ["calculate-spread"],
      "params": {"condition": "${calculate-spread.result} > 0.01"}},
    {"id": "execute-trade", "action": "function.execute", "depends_on": ["check-threshold"],
      "params": {"function_id": "trade-executor"}}
  ]
}
```

### Data Pipeline

```json
{
  "name": "data-etl-pipeline",
  "steps": [
    {"id": "extract", "action": "http.get", "params": {"url": "https://source.api/data"}},
    {"id": "transform", "action": "function.execute", "depends_on": ["extract"],
      "params": {"function_id": "data-transformer", "input": "${extract.body}"}},
    {"id": "load", "action": "http.post", "depends_on": ["transform"],
      "params": {"url": "https://dest.api/ingest", "body": "${transform.result}"}}
  ]
}
```

## Best Practices

### CCIP

1. **Set Appropriate Gas Limits**: Over-estimate initially, optimize later
2. **Handle Failures**: Implement retry logic for failed messages
3. **Monitor Finality**: Wait for sufficient confirmations
4. **Test on Testnet**: Always test cross-chain flows on testnet first

### CRE

1. **Idempotent Steps**: Design steps to be safely re-runnable
2. **Timeout Configuration**: Set appropriate timeouts per step
3. **Dependency Management**: Clearly define step dependencies
4. **Error Handling**: Use conditional steps for error paths

## Error Handling

### CCIP Errors

| Status | Meaning | Action |
|--------|---------|--------|
| `failed` | Delivery failed | Check gas, retry |
| `timeout` | No response | Increase gas limit |
| `reverted` | Dest contract reverted | Check payload format |

### CRE Errors

| Status | Meaning | Action |
|--------|---------|--------|
| `step_failed` | Individual step failed | Check step logs |
| `timeout` | Executor timeout | Increase timeout |
| `dependency_failed` | Upstream step failed | Fix dependent step |

## Related Documentation

- [Service Catalog](../service-catalog.md)
- [Architecture Layers](../architecture-layers.md)
- [Operations Runbook](../ops-runbook.md)
