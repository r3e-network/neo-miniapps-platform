# Neo Service Layer

A service layer for Neo N3 that combines a user-facing **Gateway** (Supabase Edge) with enclave workloads (MarbleRun + EGo) for signing and confidential computation.

For the canonical, up-to-date architecture overview see `docs/ARCHITECTURE.md`.

For the target MiniApp platform blueprint/spec, see `docs/neo-miniapp-platform-blueprint.md` and `docs/neo-miniapp-platform-full.md`.
For the reviewed English architectural blueprint, see `docs/neo-miniapp-platform-architectural-blueprint.md`.

## Scope (Current)

**Product services** (only these are in scope right now):

- `services/datafeed` (`service_id`: `neofeeds`)
- `services/automation` (`service_id`: `neoflow`)
- `services/confcompute` (`service_id`: `neocompute`)
- `services/vrf` (`service_id`: `neovrf`)
- `services/conforacle` (`service_id`: `neooracle`)
- `services/txproxy` (`service_id`: `txproxy`)

Randomness is provided via `services/vrf` (NeoVRF) inside the enclave.

**Infrastructure marbles** (shared capabilities):

- `infrastructure/globalsigner` (`service_id`: `globalsigner`)
- `infrastructure/accountpool` (`service_id`: `neoaccounts`)

## Runtime Boundary (TEE vs Non‑TEE)

- **Outside TEE (default)**: user workflows (Supabase Auth), wallet bindings, secrets UX + API.
- **Inside TEE**: service execution that needs confidentiality/integrity, enclave-held keys, and signing (GlobalSigner + service workloads).

Secrets are **not** a separate service: they are managed by the gateway and stored in Supabase encrypted with `SECRETS_MASTER_KEY`.

## Repository Layout

- `cmd/`: binaries (`cmd/marble`, deploy tooling, bundle verification helpers)
- `infrastructure/`: shared building blocks (runtime, middleware, chain I/O, secrets, storage helpers, account pool, global signer)
- `services/`: product services only (see “Scope”)
- `contracts/`: Neo N3 MiniApp platform contracts
- `platform/`: platform layer (Supabase Edge functions, JS SDK, Next.js host app)
- Export targets (intentionally empty in git; generated via scripts):
  - `platform/host-app/public/miniapps/` + `platform/host-app/public/sdk/` (run `make export-miniapps`)
  - `supabase/functions/` (run `make export-supabase-functions`)
  - `supabase/migrations/` (run `make export-supabase-migrations`)
- `docker/`, `k8s/`, `manifests/`, `deploy/`: deployment and operations

For enforced responsibility boundaries, see `docs/LAYERING.md`.

## Quick Start (Local Simulation)

Prereqs: Go, Docker, Node.js.

```bash
make docker-up
```

Run a single service locally (outside MarbleRun) for debugging:

```bash
SERVICE_TYPE=neocompute go run ./cmd/marble
# Or run VRF:
# SERVICE_TYPE=neovrf go run ./cmd/marble
```

Supabase Edge functions are the intended public gateway. See `platform/edge/README.md` for setup and required env vars.

## Key Environment Variables

- `SUPABASE_URL`: Supabase project URL.
- `SUPABASE_SERVICE_KEY`: Supabase service role key (used by Go services and tooling).
- `SUPABASE_SERVICE_ROLE_KEY`: Supabase service role key (used by Supabase Edge functions).
- `SECRETS_MASTER_KEY`: encryption master key for secrets APIs (`platform/edge/functions/secrets-*`) and secret injection into TEE services.
- `NEO_RPC_URL` / `NEO_RPC_URLS`, `NEO_NETWORK_MAGIC`: Neo RPC configuration (services).
- `CONTRACT_PAYMENTHUB_HASH`, `CONTRACT_GOVERNANCE_HASH`, `CONTRACT_PRICEFEED_HASH`, `CONTRACT_RANDOMNESSLOG_HASH`, `CONTRACT_APPREGISTRY_HASH`, `CONTRACT_AUTOMATIONANCHOR_HASH`: MiniApp platform contract hashes.
- `TXPROXY_ALLOWLIST`: tx-proxy allowlist JSON (contract+method policy).

See `.env.example` for a full list.

## Docs

- `docs/ARCHITECTURE.md`: current end-to-end architecture and TEE boundary
- `docs/LAYERING.md`: layering rules + boundaries (what goes where)
- `docs/MODULE_RESPONSIBILITIES.md`: per-module responsibilities + dependency rules
- `docs/API_DOCUMENTATION.md`: gateway/service API reference
- `docs/DEPLOYMENT_GUIDE.md`: deployment paths (Docker, MarbleRun, K8s)
- `docs/MASTER_KEY_ATTESTATION.md`: GlobalSigner key + attestation workflow
