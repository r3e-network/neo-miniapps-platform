# Supabase Edge (Gateway)

This folder contains **Supabase Edge Functions** scaffolds for the MiniApp
platform. The intended architecture is a **thin gateway**:

- Auth via **Supabase Auth (GoTrue)**.
- Stateless request validation + rate limiting.
- Enforce platform rules:
  - **payments/settlement = GAS only**
  - **governance = NEO only**
- Forward sensitive requests to the **TEE services** over **mTLS** (attested TLS
  inside MarbleRun).

## Functions

See `platform/edge/functions/`:

- `wallet-nonce`: issues a nonce + message for Neo N3 wallet binding.
- `wallet-bind`: verifies signature and binds a Neo N3 address to the authenticated user.
- `api-keys-create`: create a user API key (returned once; stored hashed).
- `api-keys-list`: list API keys (no raw key).
- `api-keys-revoke`: revoke an API key.
- `secrets-list`: list secret metadata (no values).
- `secrets-get`: decrypt and return a secret value.
- `secrets-upsert`: create/update a secret (AES-GCM envelope).
- `secrets-delete`: delete a secret and its policies.
- `secrets-permissions`: set allowed service IDs per secret.
- `gasbank-account`: get/create a GAS bank account (delegated payments).
- `gasbank-deposit`: create a deposit request record.
- `gasbank-deposits`: list deposit requests.
- `gasbank-transactions`: list gasbank transactions.
- `pay-gas`: returns a PaymentHub `pay` invocation (GAS-only).
- `vote-neo`: returns a Governance `vote` invocation (NEO-only).
- `app-register`: returns an AppRegistry `register` invocation (developer wallet-signed).
- `app-update-manifest`: returns an AppRegistry `updateManifest` invocation (developer wallet-signed).
- `rng-request`: runs RNG via `neocompute` (no dedicated `vrf-service` in this repo).
- `datafeed-price`: read proxy to `neofeeds`.

Supabase deploys functions under:

- `/functions/v1/<function-name>`

## Required Env Vars

At minimum, these functions expect:

- `SUPABASE_URL`
- `SUPABASE_ANON_KEY` (to validate `Authorization: Bearer <jwt>`)
- `SUPABASE_SERVICE_ROLE_KEY` (preferred) or `SUPABASE_SERVICE_KEY` (fallback; used by Go services)
- `SECRETS_MASTER_KEY` (required for `secrets-*` endpoints; AES-GCM envelope key)

Most endpoints accept either:

- `Authorization: Bearer <jwt>` (Supabase Auth), or
- `X-API-Key: <key>` (user API keys; used for secrets/gasbank/etc.)

API key management endpoints (`api-keys-*`) require `Authorization: Bearer <jwt>`.

## mTLS to TEE

The shared helper `platform/edge/functions/_shared/tee.ts` supports optional
mTLS when these env vars are present:

- `TEE_MTLS_CERT_PEM`: client certificate chain (PEM)
- `TEE_MTLS_KEY_PEM`: client private key (PEM)
- `TEE_MTLS_ROOT_CA_PEM`: trusted server root (PEM; MarbleRun root CA)

Alternatively `MARBLERUN_ROOT_CA_PEM` can be used as the root CA name.
