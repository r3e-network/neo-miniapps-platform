# AccountPool (NeoAccounts) Service

HD-derived pool account management for the Neo Service Layer.

This is **infrastructure**, not a product-facing service: other enclave services
use it to allocate/lock accounts, sign payloads, track balances, and rotate/retire
accounts over time.

## Responsibilities

- Maintain a large pool of Neo N3 accounts (target: 10,000+).
- Allocate + lock accounts for a requesting service (`service_id`).
- Provide signing using derived account keys (private keys never leave the enclave).
- Track per-token balances (GAS/NEO today; extensible).
- Rotate/retire accounts and archive old ones.

## API Endpoints (Marble)

Standard:

- `GET /health`, `GET /ready`, `GET /info`

AccountPool-specific:

- `GET /pool-info`: pool stats + per-token stats
- `GET /master-key`: master key metadata (pubkey/hash/attestation hash; no secrets)
- `GET /accounts?service_id=...&token=...&min_balance=...`: list locked accounts
- `POST /request`: request + lock accounts
- `POST /release`: release locked accounts (or release all for a service)
- `POST /sign`: sign a tx hash with a pool account key
- `POST /batch-sign`: sign multiple tx hashes
- `POST /balance`: update tracked token balances
- `POST /transfer`: construct/sign/broadcast a token transfer from a pool account

## Example: Request Accounts

```json
POST /request
{
  "service_id": "neocompute",
  "count": 2,
  "purpose": "compute jobs"
}
```

## Multi-Token Balances

Balances are stored in `pool_account_balances` keyed by:

- `account_id`
- `token_type` (e.g. `GAS`, `NEO`)
- `script_hash` (NEP-17 contract hash)

## Code Layout

- `infrastructure/accountpool/marble`: enclave runtime + HTTP API
- `infrastructure/accountpool/supabase`: Supabase/PostgREST persistence
- `infrastructure/accountpool/types`: canonical request/response DTOs
- `infrastructure/accountpool/client`: client SDK used by other services/tools

## Security Notes

- In strict identity mode (production/SGX/MarbleRun TLS), the `service_id` is
  derived from verified mTLS peer identity and the API rejects spoofed headers.
- Master key material is injected via MarbleRun and never leaves the enclave.

## Testing

```bash
go test ./infrastructure/accountpool/... -v
```
