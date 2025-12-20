# Supabase Edge (Gateway)

This folder contains **Supabase Edge Functions** for the MiniApp platform. The
intended architecture is a **thin gateway**:

- Auth via **Supabase Auth (GoTrue)**.
- Stateless request validation + rate limiting (backed by Postgres `rate_limits`).
- Enforce platform rules:
  - **payments/settlement = GAS only**
  - **governance = NEO only**
- Forward sensitive requests to the **TEE services** over **mTLS** (attested TLS
  inside MarbleRun; **required in production**).

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
- `app-register`: validates a `manifest`, computes `manifest_hash`, and returns an AppRegistry `register` invocation (developer wallet-signed).
- `app-update-manifest`: validates a `manifest`, computes `manifest_hash`, and returns an AppRegistry `updateManifest` invocation (developer wallet-signed).
- `rng-request`: runs RNG via `neovrf` (signature + attestation hash).
- `compute-execute`: runs a script via `neocompute` (`/execute`) (host-gated).
- `compute-jobs`: lists compute jobs via `neocompute` (`/jobs`) (host-gated).
- `compute-job`: gets a compute job via `neocompute` (`/jobs/{id}`) (host-gated; uses `?id=`).
- `datafeed-price`: read proxy to `neofeeds`.
- `oracle-query`: forwards allowlisted HTTP fetch requests to `neooracle` (optional secret injection).
- `automation-triggers`: list/create automation triggers via `neoflow` (host-gated).
- `automation-trigger`: get a trigger via `neoflow` (host-gated; uses `?id=`).
- `automation-trigger-update`: update a trigger via `neoflow` (host-gated; uses POST in Edge).
- `automation-trigger-delete`: delete a trigger via `neoflow` (host-gated; uses POST in Edge).
- `automation-trigger-enable`, `automation-trigger-disable`, `automation-trigger-resume`: lifecycle controls.
- `automation-trigger-executions`: list executions (audit log).

Supabase deploys functions under:

- `/functions/v1/<function-name>`

## Required Env Vars

At minimum, these functions expect:

- `SUPABASE_URL`
- `SUPABASE_ANON_KEY` (to validate `Authorization: Bearer <jwt>`)
- `SUPABASE_SERVICE_ROLE_KEY` (preferred) or `SUPABASE_SERVICE_KEY` (fallback; used by Go services)
- `SECRETS_MASTER_KEY` (required for `secrets-*` endpoints; AES-GCM envelope key)

`app-register` and `app-update-manifest` also persist canonical manifests into the
Supabase `miniapps` table. Runtime endpoints (`pay-gas`, `vote-neo`, `rng-request`)
use that table to enforce manifest permissions and per-tx limits.
Daily cap enforcement uses the `miniapp_usage` table and the
`miniapp_usage_bump(...)` RPC (see `migrations/026_miniapp_usage.sql`).

TEE routing env vars (required by functions that proxy to internal services):

- `NEOFEEDS_URL`
- `NEOCOMPUTE_URL`
- `NEOVRF_URL`
- `NEOORACLE_URL`
- `NEOFLOW_URL`
- `TXPROXY_URL`

Most endpoints accept either:

- `Authorization: Bearer <jwt>` (Supabase Auth), or
- `X-API-Key: <key>` (user API keys; used for secrets/gasbank/etc.)

Host-only endpoints (compute/automation/oracle/secrets) require **API keys with
explicit scopes** in production; bearer JWTs are rejected there.

API key management endpoints (`api-keys-*`) require `Authorization: Bearer <jwt>`.

## Optional Env Vars

- `RNG_ANCHOR`: set to `1` to record RNG results on-chain via `txproxy` (`RandomnessLog.record`).
- `EDGE_CORS_ORIGINS`: optional origin allowlist for browser clients (comma/space-separated). When unset, responses default to `Access-Control-Allow-Origin: *`.

## Rate Limiting

All Edge functions call `platform/edge/functions/_shared/ratelimit.ts`, which
implements a simple fixed-window limiter backed by the `public.rate_limits`
table (service-role only).

It requires the Postgres RPC function `public.rate_limit_bump(...)` (added in
`migrations/024_rate_limit_bump.sql`).

Env vars:

- `EDGE_RATELIMIT_DEFAULT_PER_MINUTE` (default: `60`)
- `EDGE_RATELIMIT_WINDOW_SECONDS` (default: `60`)
- Per-endpoint override: `EDGE_RATELIMIT_<ENDPOINT>_PER_MINUTE` (hyphens replaced with underscores),
  e.g. `EDGE_RATELIMIT_PAY_GAS_PER_MINUTE=10`.

## Typecheck (Deno)

The Edge functions are Deno code. To typecheck locally (requires Deno installed):

```bash
cd platform/edge
deno task check
```

`deno.json` sets `compilerOptions.skipLibCheck` to avoid upstream `.d.ts`
issues from `esm.sh` dependencies (Supabase SDK).

## Local Dev Server (No Supabase CLI)

This repo includes a lightweight local router that serves **all** Edge functions
from a single port, without needing `supabase functions serve`.

Start it with:

```bash
make edge-dev
```

Defaults:

- listens on `http://localhost:8787` (override with `EDGE_DEV_PORT`)
- serves function routes under both:
  - `http://localhost:8787/functions/v1/<function-name>` (Supabase-compatible)
  - `http://localhost:8787/api/rpc/<function-name>` (blueprint-compatible)
  - `http://localhost:8787/<function-name>` (convenient direct form)

Set your host/SDK base URL to:

- `http://localhost:8787/functions/v1`
  - or `http://localhost:8787/api/rpc` (blueprint form)

When running the simulation stack (`make docker-up`), the marbles expose their
ports on `127.0.0.1` (e.g. `neofeeds` on `8083`). For the local dev server,
set:

- `NEOFEEDS_URL=http://localhost:8083`
- `NEOCOMPUTE_URL=http://localhost:8086`
- `NEOORACLE_URL=http://localhost:8088`
- `NEOFLOW_URL=http://localhost:8084`
- `TXPROXY_URL=http://localhost:8090`

## mTLS to TEE

The shared helper `platform/edge/functions/_shared/tee.ts` enforces **mTLS in
production** and rejects non-HTTPS service URLs. Provide these env vars:

- `TEE_MTLS_CERT_PEM`: client certificate chain (PEM)
- `TEE_MTLS_KEY_PEM`: client private key (PEM)
- `TEE_MTLS_ROOT_CA_PEM`: trusted server root (PEM; MarbleRun root CA)

Alternatively `MARBLERUN_ROOT_CA_PEM` can be used as the root CA name.
