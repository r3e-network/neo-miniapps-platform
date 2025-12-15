# TxSubmitter Service

Unified transaction submission service for NEO N3 blockchain, running in TEE (Marble/EGo).

## Overview

TxSubmitter is the **ONLY** service with chain write permission. All other services must submit transactions through TxSubmitter.

## Features

- **Centralized Chain Write**: Single point of chain interaction for all services
- **TEE Protection**: Private key never leaves the enclave
- **Multi-RPC Failover**: Automatic health checking and endpoint switching
- **Rate Limiting**: Global (50 TPS) and per-service limits
- **Retry with Backoff**: Exponential backoff with jitter (max 3 retries)
- **Audit Logging**: All transactions logged to `chain_txs` table
- **Confirmation Tracking**: Background worker tracks transaction confirmations

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  NeoOracle  │     │  NeoFeeds   │     │  NeoRand    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                           ▼
                   ┌───────────────┐
                   │  TxSubmitter  │
                   │    (TEE)      │
                   └───────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         ┌────────┐   ┌────────┐   ┌────────┐
         │ RPC 1  │   │ RPC 2  │   │ RPC 3  │
         └────────┘   └────────┘   └────────┘
```

## Configuration

```yaml
txsubmitter:
  rate_limit:
    global_tps: 50
    per_service_limits:
      neooracle: 20
      neofeeds: 10
      neorand: 10
      neovault: 5
    burst_multiplier: 1.5
  retry:
    max_retries: 3
    initial_backoff: "200ms"
    max_backoff: "30s"
    backoff_multiplier: 2.0
    jitter: 0.1
```

## API Endpoints

| Endpoint      | Method | Description                 |
| ------------- | ------ | --------------------------- |
| `/health`     | GET    | Health check                |
| `/ready`      | GET    | Readiness check             |
| `/info`       | GET    | Service info and statistics |
| `/status`     | GET    | Detailed status             |
| `/rpc/health` | GET    | RPC endpoint health         |

## Authorization

Services are authorized for specific transaction types:

| Service      | Allowed Transaction Types                      |
| ------------ | ---------------------------------------------- |
| neooracle    | fulfill_request, fail_request                  |
| neofeeds     | update_price, update_prices                    |
| neorand      | fulfill_request, fail_request                  |
| neocompute   | fulfill_request, fail_request                  |
| neovault     | fulfill_request, fail_request, resolve_dispute |
| neoflow      | execute_trigger                                |
| globalsigner | set_tee_master_key                             |

## Rate Limiting

- **Global**: 50 TPS across all services
- **Per-Service**: Configurable limits per service
- **Burst**: 1.5x multiplier for short bursts
- **Response**: Returns error when limit exceeded

## Retry Strategy

1. Initial attempt
2. Wait 200ms + jitter, retry
3. Wait 400ms + jitter, retry
4. Wait 800ms + jitter, retry (max)
5. Return error if all retries fail

## Audit Trail

All transactions are logged to `chain_txs` table:

```sql
SELECT * FROM chain_txs WHERE from_service = 'neooracle' ORDER BY submitted_at DESC;
```

## Metrics

- `txs_submitted`: Total transactions submitted
- `txs_confirmed`: Total transactions confirmed
- `txs_failed`: Total transactions failed
- `pending_txs`: Current pending transactions
- `rate_limit_status`: Current rate limit status per service
