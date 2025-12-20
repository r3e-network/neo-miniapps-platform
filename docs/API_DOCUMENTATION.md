# API Documentation (Current)

The **MiniApp Platform** exposes its public gateway via **Supabase Edge Functions**.
TEE services are internal (mesh/mTLS) and should be reached through Edge routing.

For the full intended API surface, see:

- `docs/service-api.md`
- `platform/edge/functions/README.md`

## Supabase Edge (Gateway)

Supabase deploys Edge functions under:

- `/functions/v1/<function-name>`

Key gateway endpoints in this repo:

- `wallet-nonce`, `wallet-bind` (bind Neo N3 address to Supabase user)
- `api-keys-*` (user API keys: create/list/revoke; raw key returned once)
- `pay-gas` (PaymentHub intent; settlement **GAS only**)
- `vote-neo` (Governance intent; governance **NEO only**)
- `rng-request` (randomness via `neovrf`; optional RandomnessLog anchoring)
- `compute-execute`, `compute-jobs`, `compute-job` (host-gated `neocompute` script execution + job inspection)
- `automation-triggers`, `automation-trigger-*` (host-gated trigger management + audit via `neoflow`)
- `secrets-*` (user secrets management + per-service permissions)
- `gasbank-*` (delegated payments: balances, deposits, transactions)
- `datafeed-price` (read proxy for `neofeeds`)
- `oracle-query` (allowlisted HTTP fetch proxy for `neooracle`)

## TEE Services (Internal)

Stable service IDs (runtime) used throughout the repo:

- `neofeeds` (datafeed)
- `neooracle` (oracle fetch)
- `neocompute` (confidential compute)
- `neovrf` (verifiable randomness)
- `neoflow` (automation)
- `txproxy` (allowlisted tx signing/broadcast)

## Legacy

The previous “Gateway binary + legacy REST API” documentation has been moved to:

- `docs/legacy/API_DOCUMENTATION_LEGACY_GATEWAY.md`
