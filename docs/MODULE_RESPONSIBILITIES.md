# Module Responsibilities & Dependency Rules

This document is the **responsibility map** for the repository. The goal is:

- one module = one responsibility
- no duplicated chain I/O / middleware implementations across services
- explicit TEE boundary (what runs inside MarbleRun/EGo vs outside)
- strict enforcement of constraints: **payments = GAS only**, **governance = NEO only**

For the end-to-end architecture, see `docs/ARCHITECTURE.md`. For the platform
blueprint/spec, see `docs/neo-miniapp-platform-blueprint.md` and
`docs/neo-miniapp-platform-full.md`.

---

## 1. Layers (Top-Level)

### `contracts/` (Neo N3, C#)

Platform contracts only:

- `PaymentHub` (**GAS-only** settlement)
- `Governance` (**NEO-only** staking/voting)
- `PriceFeed` (datafeed anchoring)
- `RandomnessLog` (randomness anchoring; randomness is provided via NeoVRF)
- `AppRegistry` (manifest hash + allowlist anchors)
- `AutomationAnchor` (task registry + nonce anti-replay)

Contracts enforce asset constraints at the final authorization layer.

### `platform/` (User Workflow, Outside TEE)

Outside-the-enclave “web2 plumbing” that can run on Supabase/Vercel:

- `platform/edge`: Supabase Edge Functions (thin gateway; auth, nonce, rate limits, routing)
- `platform/sdk`: TypeScript SDK shapes (`window.MiniAppSDK`)
- `platform/host-app`: Next.js host app (micro-frontends via iframe/Module Federation)
- `platform/builtin-app`: built-in MiniApps served as Module Federation remote
- `platform/rls`: RLS policy set (actual schema lives in `migrations/`)

Rules:

- no enclave secrets should be stored here (service role keys and master keys must be treated as sensitive)
- no chain signing here (wallet signs user actions; TEE signs service actions)

### `services/` (Product Services, Inside TEE by default)

Product services (only these):

- `services/datafeed` (`neofeeds`): multi-source price aggregation + optional on-chain anchoring (≥0.1% threshold)
- `services/conforacle` (`neooracle`): allowlisted external fetch + optional secret injection
- `services/vrf` (`neovrf`): verifiable randomness + signature + attestation
- `services/confcompute` (`neocompute`): restricted scripts + optional secret injection
- `services/automation` (`neoflow`): triggers/scheduler + optional anchored tasks via `AutomationAnchor`
- `services/txproxy` (`txproxy`): allowlisted sign+broadcast gatekeeper (single surface for chain writes)

Rules:

- services must not duplicate chain RPC/tx building logic (use `infrastructure/chain`)
- services must not implement shared middleware (use `infrastructure/middleware`)
- on-chain writes are centralized behind `txproxy` (other services should be read-only on chain)

### `infrastructure/` (Shared Building Blocks)

Reusable building blocks used by multiple services:

- `infrastructure/runtime`: strict identity mode + environment helpers (TEE vs non-TEE)
- `infrastructure/middleware`: HTTP middleware (logging/recovery/body limits/service identity)
- `infrastructure/httputil`: HTTP helpers (JSON envelopes, identity extraction helpers)
- `infrastructure/logging`: structured logging primitives
- `infrastructure/metrics`: Prometheus helpers
- `infrastructure/errors`: consistent error typing for services
- `infrastructure/database`: Supabase/PostgREST client + repositories
- `infrastructure/secrets`: secret encryption + permissions policy + audit hooks
- `infrastructure/marble`: MarbleRun/EGo glue (attested TLS, secret injection)
- `infrastructure/chain`: Neo N3 RPC, tx building/broadcast, typed stack parsing, event monitoring
- `infrastructure/txproxy`: shared txproxy client + request/response DTOs (delegating chain writes)
- `infrastructure/globalsigner`: enclave-held signing root + domain-separated signing + rotation
- `infrastructure/accountpool`: pool of Neo N3 accounts (target 10,000+) with locking + rotation
- `infrastructure/serviceauth`: service-to-service auth primitives (JWT claims, context helpers)
- `infrastructure/service`: common service framework (workers, hydration hooks, standard endpoints)

Rules:

- infrastructure must not depend on `services/` (keeps services swappable)

### `cmd/` (Composition Root)

Entry points and deployment tooling. `cmd/marble` is the primary composition
root: it wires infrastructure + services together based on `MARBLE_TYPE`.

`cmd/` is allowed to import both `infrastructure/` and `services/`.

---

## 2. Dependency Rules (Enforced)

These rules prevent “same functionality everywhere” drift.

1. `services/<svc>/...` may import:
   - `infrastructure/...`
   - `services/<svc>/...` (same service only)
2. `infrastructure/...` may import:
   - `infrastructure/...` only (plus standard library and external deps)
3. `cmd/...` may import both `infrastructure/...` and `services/...`.
4. Tests under `test/...` may import anything (tests are allowed to compose packages).

Enforcement lives in:

- `test/layering/layering_test.go`
