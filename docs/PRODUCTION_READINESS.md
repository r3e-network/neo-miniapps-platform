# Production Readiness (Current)

This document is the **current** production readiness checklist for the Neo
Service Layer as described in `docs/ARCHITECTURE.md`.

## Scope

**Gateway (edge)**:
- Auth (Supabase Auth: OAuth providers), sessions/JWT, API keys, wallet bindings
- Secrets API + permissions (stored in Supabase; not a separate service)
- Delegated payments / gas bank (stored in Supabase)
- Service proxy routes (mTLS inside the mesh)

**Enclave workloads (MarbleRun + EGo)**:
- Infrastructure marbles: `infrastructure/accountpool`, `infrastructure/globalsigner`
- Product services: `services/datafeed`, `services/automation`, `services/confcompute`, `services/conforacle`, `services/txproxy`

## Required External Dependencies

- **Supabase** (Postgres + PostgREST): migrations applied, service role key available.
- **Neo N3 RPC**: one or more reliable endpoints configured.
- **Deployed contracts**: MiniApp platform contracts deployed and hashes set (`PaymentHub`, `Governance`, `PriceFeed`, `RandomnessLog`, `AppRegistry`, `AutomationAnchor`).

## Required Secrets / Config

### Gateway (recommended outside TEE)

- `SUPABASE_URL`
- `SUPABASE_ANON_KEY` (Edge validates `Authorization: Bearer <jwt>`)
- `SUPABASE_SERVICE_ROLE_KEY` (Edge reads/writes `public.*` platform tables)
- `SECRETS_MASTER_KEY` (hex-encoded 32 bytes)
- Optional: `TEE_MTLS_CERT_PEM`, `TEE_MTLS_KEY_PEM`, `TEE_MTLS_ROOT_CA_PEM` when connecting Edge → TEE services over mTLS

### Enclave Workloads

Injected via MarbleRun secrets (values depend on which services you run):

- `POOL_MASTER_KEY` (+ `POOL_MASTER_KEY_HASH` in enclave mode) for AccountPool
- `GLOBALSIGNER_MASTER_SEED` for GlobalSigner
- `NEOFEEDS_SIGNING_KEY` for Datafeeds
- `COMPUTE_MASTER_KEY` for Confidential Compute
- `TEE_PRIVATE_KEY` (fallback only) if `txproxy` cannot use GlobalSigner and must sign/broadcast directly

## Chain / Contract Configuration

Contract hashes are configured via environment variables (0x-prefixed Uint160 strings):

- `CONTRACT_PAYMENTHUB_HASH` (**payments/settlement = GAS only**, enforced on-chain)
- `CONTRACT_GOVERNANCE_HASH` (**governance = NEO only**, enforced on-chain)
- `CONTRACT_PRICEFEED_HASH` (datafeed anchoring)
- `CONTRACT_RANDOMNESSLOG_HASH` (optional randomness anchoring)
- `CONTRACT_APPREGISTRY_HASH` (app allowlist + manifest hashes)
- `CONTRACT_AUTOMATIONANCHOR_HASH` (automation task registry + anti-replay)

The gateway for user workflows is **Supabase Edge** (there is no on-chain
gateway contract in the current blueprint).

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
