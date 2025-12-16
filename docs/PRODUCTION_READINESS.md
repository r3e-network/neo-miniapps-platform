# Production Readiness (Current)

This document is the **current** production readiness checklist for the Neo
Service Layer as described in `docs/ARCHITECTURE.md`.

## Scope

**Gateway (edge)**:
- Auth (wallet + OAuth), sessions/JWT, API keys, wallet bindings
- Secrets API + permissions (stored in Supabase; not a separate service)
- Delegated payments / gas bank (stored in Supabase)
- Service proxy routes (mTLS inside the mesh)

**Enclave workloads (MarbleRun + EGo)**:
- Infrastructure marbles: `infrastructure/accountpool`, `infrastructure/globalsigner`
- Product services: `services/datafeed`, `services/automation`, `services/confcompute`, `services/conforacle`

## Required External Dependencies

- **Supabase** (Postgres + PostgREST): migrations applied, service role key available.
- **Neo N3 RPC**: one or more reliable endpoints configured.
- **Deployed contracts**: gateway + service contracts deployed and hashes set.

## Required Secrets / Config

### Gateway (recommended outside TEE)

- `JWT_SECRET` (>= 32 bytes recommended)
- `SECRETS_MASTER_KEY` (hex-encoded 32 bytes)
- `OAUTH_TOKENS_MASTER_KEY` (hex-encoded 32 bytes) when OAuth is enabled
- OAuth provider secrets (Google/GitHub/etc) if enabled

### Enclave Workloads

Injected via MarbleRun secrets (values depend on which services you run):

- `POOL_MASTER_KEY` (+ `POOL_MASTER_KEY_HASH` in enclave mode) for AccountPool
- `GLOBALSIGNER_MASTER_SEED` for GlobalSigner
- `NEOFEEDS_SIGNING_KEY` for Datafeeds
- `COMPUTE_MASTER_KEY` for Confidential Compute
- `TEE_PRIVATE_KEY` if you enable on-chain fulfillments/callback tx submission

## Chain / Contract Configuration

Contract hashes can be set with the preferred names (legacy fallbacks are still
supported by the codebase):

- `CONTRACT_GATEWAY_HASH`
- `CONTRACT_DATAFEEDS_HASH` (fallback: `CONTRACT_NEOFEEDS_HASH`)
- `CONTRACT_AUTOMATION_HASH` (fallback: `CONTRACT_NEOFLOW_HASH`)
- `CONTRACT_CONFIDENTIAL_HASH` (fallback: `CONTRACT_NEOCOMPUTE_HASH`)
- `CONTRACT_ORACLE_HASH` (fallback: `CONTRACT_NEOORACLE_HASH`)

## Identity / Trust Boundary

- **Production should run in strict identity mode** (MarbleRun TLS injected).
- Public clients must not be able to spoof identity headers.
- Gateway is the trust boundary: it authenticates users and forwards derived
  identity into the mesh over mTLS.

## Validation Commands

```bash
go test ./...
go vet ./...
```

Local simulation:

```bash
make docker-up
```
