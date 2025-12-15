# NeoIndexer Service

Unified chain event indexing service for NEO N3 blockchain.

## Overview

NeoIndexer is a Non-TEE service that:

1. Polls NEO N3 RPC nodes for new blocks
2. Extracts events from application logs
3. Waits for confirmation depth (default 3 blocks)
4. Publishes standardized events to JetStream
5. Records processed events to Supabase for idempotency

## Architecture

```
NEO N3 RPC ──▶ NeoIndexer ──▶ JetStream ──▶ Executors
                   │
                   ▼
              Supabase
         (processed_events)
```

## Configuration

```yaml
indexer:
  chain_id: "neo3-mainnet"
  rpc_endpoints:
    - url: "https://rpc1.neo.org"
      priority: 1
    - url: "https://rpc2.neo.org"
      priority: 2
  confirmation_depth: 3
  poll_interval: "1s"
  batch_size: 100
  contract_addresses:
    - "0x..." # NeoRand contract
    - "0x..." # NeoOracle contract
```

## JetStream Topics

| Topic                          | Description                  |
| ------------------------------ | ---------------------------- |
| `neo.events.oracle.requested`  | Oracle request events        |
| `neo.events.rand.requested`    | Random number request events |
| `neo.events.feeds.updated`     | Price feed update events     |
| `neo.events.compute.requested` | Compute request events       |
| `neo.events.vault.requested`   | Vault request events         |
| `neo.events.gas.deposited`     | GAS deposit events           |
| `neo.events.flow.triggered`    | Flow trigger events          |

## API Endpoints

| Endpoint      | Method | Description                 |
| ------------- | ------ | --------------------------- |
| `/health`     | GET    | Health check                |
| `/ready`      | GET    | Readiness check             |
| `/info`       | GET    | Service info and statistics |
| `/status`     | GET    | Indexer status              |
| `/replay`     | POST   | Trigger replay from block   |
| `/rpc/health` | GET    | RPC endpoint health         |

## Idempotency

Events are deduplicated using the `processed_events` table:

- Unique key: `(chain_id, tx_hash, log_index)`
- Events are only published once
- Replay is safe and idempotent

## RPC Failover

- Multiple RPC endpoints supported
- Automatic health checking (30s interval)
- Automatic failover on errors
- Priority-based endpoint selection

## Metrics

- `blocks_processed`: Total blocks processed
- `events_published`: Total events published
- `last_processed_block`: Last processed block height
- `rpc_latency_ms`: RPC endpoint latency
