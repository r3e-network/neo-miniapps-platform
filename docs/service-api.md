# Service API (Draft)

This document describes the intended API surface for the MiniApp platform.

## Layers

- **Supabase Edge**: authentication, nonce/replay protection, rate limits, manifest policy enforcement, routing.
- **TEE Services**: trusted execution + key custody + signing + verifiable origin.
- **Neo N3 Contracts**: final authorization, state updates, and public audit trail.

## Edge Endpoints (Gateway)

Supabase deploys Edge functions under:

- `/functions/v1/<function-name>`

The architectural blueprint sometimes describes these as `/api/rpc/<name>`. In
production, Supabase uses `/functions/v1/<name>`. For local development, the
repo’s Edge dev server supports both prefixes.

The JS SDK (`platform/sdk`) is expected to set `edgeBaseUrl` to:

- `https://<project>.supabase.co/functions/v1`

Most endpoints require authentication via:

- `Authorization: Bearer <jwt>`, or
- `X-API-Key: <key>`

Exceptions:

- `GET /functions/v1/datafeed-price` is currently a public read proxy (no auth).

Host-only endpoints (oracle/compute/automation/secrets) require **API keys with
explicit scopes** in production; bearer JWTs are rejected there.

Most state-changing endpoints also require a **verified primary wallet binding**
(`wallet-nonce` + `wallet-bind`).

### Payments (GAS only)

- `POST /functions/v1/pay-gas`
  - body: `{ app_id: "...", amount_gas: "1.5", memo?: "..." }`
  - returns: a PaymentHub `pay` invocation (GAS-only) for the wallet/SDK to sign and submit
  - enforces: manifest `permissions.payments` and `limits.max_gas_per_tx` (when present)
  - enforces: `limits.daily_gas_cap_per_user` via `miniapp_usage_bump(...)`

### Governance (NEO only)

- `POST /functions/v1/vote-neo`
  - body: `{ app_id: "...", proposal_id: "...", neo_amount: "10", support?: true }`
  - returns: a Governance `vote` invocation (NEO-only) for the wallet/SDK to sign and submit
  - enforces: manifest `permissions.governance` and `limits.governance_cap` (when present)
  - tracks: `limits.governance_cap` via `miniapp_usage_bump(...)` (per-day enforcement)

### RNG / VRF

- `POST /functions/v1/rng-request`
  - body: `{ app_id: "..." }`
  - requests randomness from `neovrf` (`/random`) with signature + attestation hash
  - returns: `{ randomness, signature, public_key, attestation_hash }`
  - enforces: manifest `permissions.rng`
  - optional: anchors to `RandomnessLog` via `txproxy` when enabled

### Apps (App Registry)

- `POST /functions/v1/app-register`
  - body: `{ manifest: { ... } }`
  - gateway computes `manifest_hash = sha256(canonical_json(manifest))`
  - enforces: `assets_allowed == ["GAS"]` and `governance_assets_allowed == ["NEO"]`
  - persists: canonical manifest in Supabase `miniapps` table for runtime enforcement
  - returns: an AppRegistry `register` invocation for the developer wallet to sign and submit
- `POST /functions/v1/app-update-manifest`
  - body: `{ manifest: { ... } }`
  - gateway computes `manifest_hash = sha256(canonical_json(manifest))`
  - enforces: `assets_allowed == ["GAS"]` and `governance_assets_allowed == ["NEO"]`
  - persists: updated canonical manifest in Supabase `miniapps` table
  - returns: an AppRegistry `updateManifest` invocation for the developer wallet to sign and submit

### Wallet Binding

- `POST /functions/v1/wallet-nonce`
  - issues `{ nonce, message }` to be signed by a Neo N3 wallet
- `POST /functions/v1/wallet-bind`
  - body: `{ address, public_key, signature, message, nonce, label? }`
  - verifies wallet ownership and binds the address to the authenticated user

### API Keys

API key management endpoints require `Authorization: Bearer <jwt>` (cannot be called using an API key).

- `POST /functions/v1/api-keys-create`
  - body: `{ name, scopes?: string[], description?: string, expires_at?: string }`
  - returns: the raw key once (never stored in plaintext)
- `GET /functions/v1/api-keys-list`
  - returns: metadata only (no raw key)
- `POST /functions/v1/api-keys-revoke`
  - body: `{ id }`

Scope notes:

- Scopes are optional. If omitted (or empty), the key is treated as full access for that user.
- Recommended convention: set scopes to the Edge function names you want the key to call (e.g. `["pay-gas","rng-request"]`).
- `["*"]` can be used as an explicit “full access” scope.
- Host-only endpoints require **explicit scopes** (non-empty) in production.

### Secrets

These endpoints manage user secrets stored in Supabase:

- `GET /functions/v1/secrets-list`
  - returns secret metadata (no values)
- `GET /functions/v1/secrets-get?name=...`
  - returns `{ name, value, version }` (decrypted in Edge using `SECRETS_MASTER_KEY`)
- `POST /functions/v1/secrets-upsert`
  - body: `{ name, value }`
- `POST /functions/v1/secrets-delete`
  - body: `{ name }`
- `POST /functions/v1/secrets-permissions`
  - body: `{ name, services: ["neocompute","neooracle"] }`

### GasBank (Delegated Payments)

- `GET /functions/v1/gasbank-account`
  - returns: `{ account }` (creates account row if missing)
- `POST /functions/v1/gasbank-deposit`
  - body: `{ amount, from_address, tx_hash? }`
  - returns: `{ deposit }` (records a deposit request; settlement runs elsewhere)
- `GET /functions/v1/gasbank-deposits`
  - returns: `{ deposits }`
- `GET /functions/v1/gasbank-transactions`
  - returns: `{ transactions }`

### Datafeed

- `GET /functions/v1/datafeed-price?symbol=BTC-USD`
  - read proxy to `neofeeds` (or a future cache)
- `GET /functions/v1/datafeed-stream?symbol=BTC-USD` (future: SSE/WebSocket proxy)

### Oracle

- `POST /functions/v1/oracle-query`
  - allowlisted HTTP fetch via `neooracle` (`/query`) with optional `secret_name` injection

### Compute

- `POST /functions/v1/compute-execute`
  - host-gated compute via `neocompute` (`/execute`) with optional `secret_refs` injection
- `GET /functions/v1/compute-jobs`
  - lists the authenticated user's recent compute jobs (proxy for `neocompute` `/jobs`)
- `GET /functions/v1/compute-job?id=<job_id>`
  - returns a compute job by id (proxy for `neocompute` `/jobs/{id}`)

### Automation

- `GET /functions/v1/automation-triggers`
  - lists the authenticated user's triggers (proxy for `neoflow` `/triggers`)
- `POST /functions/v1/automation-triggers`
  - creates a trigger (proxy for `neoflow` `/triggers`)
- `GET /functions/v1/automation-trigger?id=<trigger_id>`
  - gets a trigger by id (proxy for `neoflow` `/triggers/{id}`)
- `POST /functions/v1/automation-trigger-update`
  - updates a trigger (proxy for `neoflow` `PUT /triggers/{id}`)
- `POST /functions/v1/automation-trigger-delete`
  - deletes a trigger (proxy for `neoflow` `DELETE /triggers/{id}`)
- `POST /functions/v1/automation-trigger-enable|automation-trigger-disable|automation-trigger-resume`
  - lifecycle controls (proxy for `neoflow` `/triggers/{id}/...`)
- `GET /functions/v1/automation-trigger-executions?id=<trigger_id>&limit=50`
  - lists executions for a trigger (proxy for `neoflow` `/triggers/{id}/executions`)

## TEE Service Endpoints

This repo uses stable **service IDs** (runtime) and maps them to the target
platform naming in docs:

- `neofeeds` → datafeed-service
- `neooracle` → oracle-gateway
- `neocompute` → compute-service
- `neoflow` → automation-service
- `txproxy` → tx-proxy

### `neofeeds` (datafeed-service)

- `GET /price/{pair}`: signed price for a pair (canonical: `BTC-USD`; legacy `BTC/USD` accepted)
- `GET /prices`: signed prices (bulk)
- `GET /feeds`, `GET /sources`, `GET /config`: configuration inspection

### `neooracle` (oracle-gateway)

- `POST /query`: allowlisted HTTP fetch + optional secret injection
- `POST /fetch`: alias (backward compatible)

### `neocompute` (compute-service)

- `POST /execute`: run restricted script/wasm with optional secret injection.
- `GET /jobs`, `GET /jobs/{id}`: job inspection

### `neoflow` (automation-service)

- `GET/POST /triggers`: manage triggers
- `POST /triggers/{id}/enable|disable|resume`: control lifecycle
- `GET /triggers/{id}/executions`: audit

Note: in the current implementation, Supabase-backed triggers execute only `cron`
triggers, and the supported action type is `webhook`.

### `txproxy` (tx-proxy)

- `POST /invoke`: build+sign+broadcast allowlisted transactions.
  - hard rule: **payments only GAS**, **governance only NEO**, contract/method allowlists enforced.
  - optional `intent` field enables stricter gates for `payments` (PaymentHub.pay) and `governance` (Governance stake/unstake/vote) when contract hashes are configured.
