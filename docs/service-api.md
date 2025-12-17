# Service API (Draft)

This document describes the intended API surface for the MiniApp platform.

## Layers

- **Supabase Edge**: authentication, nonce/replay protection, rate limits, manifest policy enforcement, routing.
- **TEE Services**: trusted execution + key custody + signing + verifiable origin.
- **Neo N3 Contracts**: final authorization, state updates, and public audit trail.

## Edge Endpoints (Gateway)

All endpoints require either:

- Supabase session (cookie), or
- `Authorization: Bearer <jwt>`, or
- `X-API-Key: <key>`

### Payments (GAS only)

- `POST /v1/apps/{appId}/pay`
  - body: `{ amount_gas: "1.5", memo?: "..." }`

### Governance (NEO only)

- `POST /v1/apps/{appId}/governance/vote`
  - body: `{ proposal_id: "...", neo_amount: "10", memo?: "..." }`

### RNG / VRF

- `POST /v1/apps/{appId}/rng/request`
  - executes a randomness script in `neocompute` (no dedicated VRF service)

### Datafeed

- `GET /v1/datafeed/price/{symbol}`
- `GET /v1/datafeed/stream/{symbol}` (SSE/WebSocket proxy)

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

### `txproxy` (tx-proxy)

- `POST /invoke`: build+sign+broadcast allowlisted transactions.
  - hard rule: **payments only GAS**, **governance only NEO**, contract/method allowlists enforced.
