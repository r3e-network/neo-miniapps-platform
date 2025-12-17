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
- `SUPABASE_SERVICE_ROLE_KEY`

Notes:

- These functions are scaffolds; wire them into your Supabase project under
  `supabase/functions/*` (or symlink/copy from here).
- In strict identity / production mode, the TEE services will only trust
  identity headers (`X-User-ID`, `X-Service-ID`) when protected by verified mTLS.

Wallet onboarding:

- `wallet-nonce` + `wallet-bind` implement an OAuth-first flow where users must
  bind a Neo N3 address (once signature) before accessing on-chain actions.
