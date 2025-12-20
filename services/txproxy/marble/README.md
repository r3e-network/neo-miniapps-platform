# TxProxy Service

TxProxy is the centralized transaction signing and broadcasting gatekeeper for the Neo N3 Mini-App Platform. It holds the platform's TEE signing keys and enforces strict allowlist policies before signing any transaction.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      TxProxy Service                        │
├─────────────────────────────────────────────────────────────┤
│  Security Layer                                             │
│  ├── Contract Allowlist - Only approved contracts           │
│  ├── Method Allowlist - Only approved methods per contract  │
│  ├── Intent Policy - Asset constraints (GAS/NEO)            │
│  └── Replay Protection - Request ID deduplication           │
├─────────────────────────────────────────────────────────────┤
│  Signing Layer                                              │
│  ├── GlobalSigner Client - Preferred (centralized TEE key)  │
│  └── Local TEE Signer - Fallback (enclave-held key)         │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```
services/txproxy/marble/
├── service.go      # Main service, initialization, replay cache
├── handlers.go     # HTTP request handler
├── allowlist.go    # Contract/method allowlist logic
└── types.go        # Type definitions
```

## API Endpoint

| Method | Endpoint  | Auth | Description                              |
| ------ | --------- | ---- | ---------------------------------------- |
| POST   | `/invoke` | mTLS | Sign and broadcast a contract invocation |

## Allowlist Configuration

The allowlist defines which contracts and methods TxProxy will sign transactions for.

### Format

```json
{
  "contracts": {
    "<contract_hash>": ["method1", "method2"],
    "<contract_hash>": ["*"]
  }
}
```

Example (allow platform contracts):

```json
{
  "contracts": {
    "<paymenthub_hash>": ["pay"],
    "<governance_hash>": ["stake", "unstake", "vote"],
    "<randomnesslog_hash>": ["record"],
    "<pricefeed_hash>": ["update"]
  }
}
```

### Rules

- Contract hashes are normalized to lowercase **without** `0x` prefix (40 hex chars)
- Method names are canonicalized by lowercasing the first character (to match Neo C# devpack manifest names like `getLatest`, `setUpdater`, `pay`)
- `"*"` allows all methods on a contract (not recommended in production)
- Empty array `[]` blocks all methods

### Loading Priority

1. `TXPROXY_ALLOWLIST` secret (MarbleRun injected)
2. `TXPROXY_ALLOWLIST` environment variable
3. Empty allowlist (blocks all)

## Intent Policy Gating

Request field `intent` enables stricter checks for platform user flows:

| Intent       | Asset Constraint | Contract   | Allowed Methods            |
| ------------ | ---------------- | ---------- | -------------------------- |
| `payments`   | GAS only         | PaymentHub | `pay`                      |
| `governance` | NEO only         | Governance | `stake`, `unstake`, `vote` |

Requires corresponding contract hash environment variables:

- `CONTRACT_PAYMENTHUB_HASH` for payments intent
- `CONTRACT_GOVERNANCE_HASH` for governance intent

## Replay Protection

- Each request includes a unique `request_id`
- TxProxy maintains an in-memory cache of seen request IDs
- Duplicate requests within the replay window (10 min) are rejected
- Cache cleanup runs periodically to remove expired entries

## Configuration

| Environment Variable       | Description           | Required              |
| -------------------------- | --------------------- | --------------------- |
| `NEO_RPC_URL`              | Neo N3 RPC endpoint   | Yes                   |
| `TXPROXY_ALLOWLIST`        | JSON allowlist config | Yes (strict mode)     |
| `GLOBALSIGNER_SERVICE_URL` | GlobalSigner URL      | Recommended           |
| `TEE_PRIVATE_KEY`          | Fallback signing key  | If no GlobalSigner    |
| `CONTRACT_PAYMENTHUB_HASH` | PaymentHub contract   | For payments intent   |
| `CONTRACT_GOVERNANCE_HASH` | Governance contract   | For governance intent |

## Security

### 4-Layer Defense

1. **SDK Layer**: Type signatures prevent invalid asset parameters
2. **Edge Layer**: Validates JWT, enforces rate limits
3. **TxProxy Layer**: Allowlist + intent policy + replay protection
4. **Contract Layer**: Hardcoded asset checks (`if asset != GAS throw`)

### Key Management

- **GlobalSigner (Preferred)**: Signing keys held in dedicated TEE service
- **Local Key (Fallback)**: Key injected via MarbleRun manifest
- Keys never leave the enclave boundary
