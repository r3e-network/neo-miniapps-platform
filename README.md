# Neo Service Layer

A service layer for Neo N3 that combines a user-facing **Gateway** (Supabase/Vercel-friendly) with enclave workloads (MarbleRun + EGo) for signing and confidential computation.

For the canonical, up-to-date architecture overview see `docs/ARCHITECTURE.md`.

## Scope (Current)

**Product services** (only these are in scope right now):

- `services/datafeed` (`service_id`: `neofeeds`)
- `services/automation` (`service_id`: `neoflow`)
- `services/confcompute` (`service_id`: `neocompute`)
- `services/conforacle` (`service_id`: `neooracle`)

Randomness is provided via `neocompute` scripts (e.g. `crypto.randomBytes(...)`); there is no standalone VRF service.

**Infrastructure marbles** (shared capabilities):

- `infrastructure/globalsigner` (`service_id`: `globalsigner`)
- `infrastructure/accountpool` (`service_id`: `neoaccounts`)

## Runtime Boundary (TEE vs Non‑TEE)

- **Outside TEE (default)**: user workflows (wallet/OAuth auth), sessions/JWT, API keys, wallet bindings, secrets UX + API, gas bank.
- **Inside TEE**: service execution that needs confidentiality/integrity, enclave-held keys, and signing (GlobalSigner + service workloads).

Secrets are **not** a separate service: they are managed by the gateway and stored in Supabase encrypted with `SECRETS_MASTER_KEY`.

## Repository Layout

- `cmd/`: binaries (`cmd/gateway`, `cmd/marble`, deploy tooling, CLI)
- `infrastructure/`: shared building blocks (runtime, middleware, chain I/O, secrets, storage helpers, account pool, global signer)
- `services/`: product services only (see “Scope”)
- `contracts/`: Neo N3 contracts (Gateway + service contracts + examples)
- `frontend/`, `dapps/`: consumers (no service-layer business logic)
- `docker/`, `k8s/`, `manifests/`, `deploy/`: deployment and operations

For enforced responsibility boundaries, see `docs/LAYERING.md`.

## Quick Start (Local Simulation)

Prereqs: Go, Docker, Node.js.

```bash
make docker-up
make marblerun-manifest
make frontend-dev   # optional
```

Run a single service locally (outside MarbleRun) for debugging:

```bash
SERVICE_TYPE=neocompute go run ./cmd/marble
```

Run the gateway locally:

```bash
OE_SIMULATION=1 go run ./cmd/gateway
```

## Key Environment Variables

- `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`: Supabase connectivity (gateway + services that persist state).
- `JWT_SECRET`: gateway auth signing key (required in production).
- `SECRETS_MASTER_KEY`: gateway encryption master key for `/api/v1/secrets/*`.
- `NEO_RPC_URL` / `NEO_RPC_URLS`, `NEO_NETWORK_MAGIC`: Neo RPC configuration (services).
- `CONTRACT_GATEWAY_HASH`, `CONTRACT_DATAFEEDS_HASH`, `CONTRACT_AUTOMATION_HASH`, `CONTRACT_CONFIDENTIAL_HASH`, `CONTRACT_ORACLE_HASH`: contract hashes for event monitoring/callbacks (legacy names are still accepted).

See `.env.example` for a full list.

## Docs

- `docs/ARCHITECTURE.md`: current end-to-end architecture and TEE boundary
- `docs/LAYERING.md`: module responsibility map (what goes where)
- `docs/API_DOCUMENTATION.md`: gateway/service API reference
- `docs/DEPLOYMENT_GUIDE.md`: deployment paths (Docker, MarbleRun, K8s)
- `docs/MASTER_KEY_ATTESTATION.md`: GlobalSigner key + attestation workflow
