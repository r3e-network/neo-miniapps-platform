# Service Layer Layering & Responsibilities

This repo is intentionally split into three layers:

1. **Infrastructure**: reusable building blocks (storage, networking, chain I/O, runtime, auth primitives).
2. **Services**: product services that implement off-chain logic (typically running inside EGo/SGX).
3. **Dapps**: frontends and app integrations that consume the gateway + services.

The goal is **one module = one responsibility**, with **no duplicated chain I/O** or HTTP middleware across services.

## Directory Map

### `infrastructure/` (shared building blocks)

- `infrastructure/database`: Supabase/PostgREST client + generic repository helpers (**storage**).
- `infrastructure/marble`: MarbleRun/EGo integration (**execution engine + enclave runtime glue**).
- `infrastructure/middleware`: HTTP middleware used everywhere (**middleware only lives here**).
- `infrastructure/runtime`: environment/identity strictness helpers (**runtime**).
- `infrastructure/chain`: Neo N3 RPC, tx building/submission, event monitoring, contract invocation (**chain module**).
- `infrastructure/accountpool`: Neo N3 account pool management (**account pool**).
- `infrastructure/globalsigner`: TEE-managed domain-separated signing + rotation (**global signer**).
- `infrastructure/secrets`: user secrets encryption + permissions + audit (**secrets**).
- `infrastructure/service`, `infrastructure/httputil`, `infrastructure/errors`, `infrastructure/logging`, `infrastructure/metrics`, `infrastructure/serviceauth`: shared service framework + transport primitives (**network**).

### `services/` (product services)

Only these services are considered “product services” right now:

- `services/vrf`: verifiable randomness (VRF).
- `services/datafeed`: data feeds (push pattern).
- `services/automation`: automation / triggers.
- `services/confcompute`: confidential compute (JS execution).
- `services/conforacle`: confidential oracle (external fetch with controls).

Each service should follow the same internal pattern:

- `services/<svc>/marble`: enclave runtime + HTTP handlers + worker loops.
- `services/<svc>/supabase`: service-specific persistence repository (if needed).
- `services/<svc>/contract`: Neo N3 smart contract source (C#; compiled into `contracts/build/`).

### `cmd/` (binaries)

- `cmd/marble`: **single entrypoint** used to run any enclave service (`MARBLE_TYPE=...`).
- `cmd/gateway`: edge API (auth, user workflow, routing, rate-limits).

### `dapps/`, `frontend/`

User-facing apps consuming the gateway API. These should not contain service-layer business logic.

## EGo / SGX Boundary (What runs where)

### Inside EGo (enclave)

Keep enclave code focused on sensitive operations and verifiable execution:

- VRF computation
- confidential compute execution
- oracle fetch (when results need enclave-origin proofs)
- any key material that must not leave the enclave (e.g. “global signer”)
- signing of service-layer on-chain fulfillments / datafeed submissions

In code, this is primarily:

- `services/*/marble`
- `infrastructure/marble`
- `cmd/marble`

### Outside EGo (non-enclave)

Keep non-enclave code focused on user/product workflow and “web2 plumbing”:

- user auth (wallet + OAuth), session/JWT handling
- secrets management & permissions UX/API
- delegated payments / configuration stored in Supabase
- API gateway routing, request validation, rate limiting
- deployment glue (Docker/K8s), CLI tools

In code, this is primarily:

- `cmd/gateway`
- `frontend/`, `dapps/`
- `deploy/`, `docker/`, `k8s/`

## Responsibility Rules (enforced by refactoring)

- **Chain I/O lives in `infrastructure/chain`** (RPC client, tx submission, event monitoring).
- **Middleware lives in `infrastructure/middleware`** (no per-service copies).
- **Services may not talk to Neo RPC directly** except via `infrastructure/chain`.
- **Services may not duplicate “service base” lifecycle** (use `infrastructure/service.BaseService`).
- **Contract bindings/event parsing live in `infrastructure/chain`** (`contracts_*.go`, `listener_events_*.go`) to avoid duplication.
