# Supabase Edge Functions (Scaffold)

This folder contains **reference Supabase Edge functions** (Deno) for the MiniApp
platform.

Goals:

- keep the gateway **thin** (auth, limits, routing)
- enforce platform rules:
  - **payments = GAS only**
  - **governance = NEO only**
- forward sensitive operations to **TEE services** over **mTLS** in production

Required env vars:

- `SUPABASE_URL`
- `SUPABASE_ANON_KEY`
- `SUPABASE_SERVICE_ROLE_KEY` (preferred) or `SUPABASE_SERVICE_KEY` (fallback)
- `SECRETS_MASTER_KEY` (required for `secrets-*`)

Notes:

- These functions are scaffolds; wire them into your Supabase project under
  `supabase/functions/*` (or symlink/copy from here).
- In strict identity / production mode, the TEE services will only trust
  identity headers (`X-User-ID`, `X-Service-ID`) when protected by verified mTLS.
- Authentication: most endpoints accept either `Authorization: Bearer <jwt>` or
  `X-API-Key: <key>`. API key management endpoints (`api-keys-*`) require a JWT.

Wallet onboarding:

- `wallet-nonce` + `wallet-bind` implement an OAuth-first flow where users must
  bind a Neo N3 address (once signature) before accessing on-chain actions.

Secrets:

- `secrets-list`, `secrets-get`, `secrets-upsert`, `secrets-delete`: manage user secrets stored in Supabase (encrypted via `SECRETS_MASTER_KEY`).
- `secrets-permissions`: configure which internal service IDs may read a secret (`secret_policies` table).

API keys:

- `api-keys-create`, `api-keys-list`, `api-keys-revoke`: create/list/revoke user API keys (hashed in DB; raw key returned once).

Gas bank (delegated payments):

- `gasbank-account`, `gasbank-deposit`, `gasbank-deposits`, `gasbank-transactions`

On-chain invocations (wallet-signed):

- `pay-gas`: returns a `PaymentHub.pay` invocation (**GAS only**).
- `vote-neo`: returns a `Governance.vote` invocation (**NEO only**).
- `app-register`: validates a `manifest` payload, computes `manifest_hash`, and returns an `AppRegistry.register` invocation (developer wallet-signed).
- `app-update-manifest`: validates a `manifest` payload, computes `manifest_hash`, and returns an `AppRegistry.updateManifest` invocation (developer wallet-signed).

TEE-routed:

- `rng-request`: executes RNG via `neocompute` scripts and can optionally anchor to `RandomnessLog` through `txproxy`.
- `datafeed-price`: read proxy to `neofeeds` (future: cache/SSE/WebSocket).
