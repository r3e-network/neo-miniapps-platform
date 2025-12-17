# Service Layer Architecture

This document describes the **current** architecture of the Neo Service Layer.
For a quick map of directory responsibilities, see `docs/LAYERING.md`.

## Goals

- **Clean layering**: one module = one responsibility.
- **Minimal TEE surface**: only sensitive computation + signing runs in enclaves.
- **No duplicated chain I/O**: Neo RPC, tx building, and event monitoring live in one place.
- **Consistent service shape**: same patterns for config, routing, storage, and workers.

## High-Level Topology

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                               DAPPS / FRONTEND                               │
│                           (Vercel / browsers / CLI)                          │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │ HTTPS
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                  GATEWAY                                     │
│     Supabase Edge Functions (“thin gateway”): auth, wallet binding, routing, │
│           rate limits, nonce/replay protection, secrets API (RLS)            │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │ mTLS (mesh, optional)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           MARBLERUN COORDINATOR                              │
│         Attestation, topology verification, mTLS certs, secret injection     │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │ mTLS (MarbleRun-issued)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                             ENCLAVE WORKLOADS                                │
│                                                                             │
│  Infrastructure marbles:                                                    │
│   - GlobalSigner  (TEE-managed domain-separated signing + rotation)         │
│   - NeoAccounts   (account pool management + rotation)                      │
│                                                                             │
│  Product services:                                                          │
│   - NeoFeeds    (data feeds)                                                │
│   - NeoFlow     (automation)                                                │
│   - NeoCompute  (confidential compute)                                      │
│   - NeoOracle   (confidential oracle)                                       │
│   - TxProxy     (allowlisted tx signing/broadcast)                           │
│   - Randomness  (via NeoCompute scripts, optional on-chain anchoring)        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                   SUPABASE                                   │
│     Auth metadata + sessions + secrets + service state (Postgres + RLS)      │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                  NEO N3 CHAIN                                │
│                          MiniApp platform contracts                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Layering (Code)

The repo is split into:

- `infrastructure/`: shared building blocks (runtime, middleware, storage, chain I/O, secrets, signing).
- `services/`: product services only (`datafeed`, `automation`, `confcompute`, `conforacle`, `txproxy`).
- `cmd/`: binaries (`cmd/marble`, deployment tooling, bundle verification helpers).
- `platform/`: Supabase Edge gateway + host app + SDK.

See `docs/LAYERING.md` for the concrete mapping.

## Identity & User Workflow (Outside the Enclave)

User-facing workflow lives **outside the enclave** and can run directly on Vercel/Supabase:

- **Auth**: Supabase Auth (OAuth providers: Google/GitHub/etc.).
- **Wallet binding**: users bind a Neo N3 address after OAuth registration (Edge nonce + signature verification).
- **Sessions**: Supabase JWT/cookie sessions (Edge validates).
- **Secrets UX**: users create secrets and manage which internal services may read them.

Enclave services should not implement login/registration flows.

### Strict Identity Mode

In production/SGX mode, internal services only trust identity headers over verified
mTLS. This is enforced by `infrastructure/runtime.StrictIdentityMode()` and
`infrastructure/middleware`.

## Secrets (Gateway + Supabase, Not a Separate Service)

User secrets are stored in Supabase, encrypted with `SECRETS_MASTER_KEY`.

- **Write path**: Supabase Edge functions under `/functions/v1/secrets-*`.
- **Encryption + policy**: `infrastructure/secrets.Manager` (Go) and `platform/edge/functions/_shared/secrets.ts` (Deno), using compatible AES‑GCM envelopes.
- **Storage**: `infrastructure/secrets/supabase`.

### Service Access (Secret Injection)

Enclave services do not implement user-facing secret workflows. They receive a
`secrets.Provider` implementation (injected by `cmd/marble`) that enforces:

- per-user ownership
- per-secret allowed services (permissions)
- audit logging

Compute and oracle support secret injection via request fields (`secret_refs`,
`secret_name`).

## Chain Module (Single Source of Truth)

All Neo chain communication belongs to `infrastructure/chain`:

- RPC client + pooling
- transaction building + signing helpers
- event monitoring/listeners
- shared contract parsing helpers

Contract wrappers and typed event parsing also live in `infrastructure/chain`
(`contracts_*.go`, `listener_events_*.go`) to keep services free of duplicated
chain bindings.

State-changing on-chain writes are centralized behind `services/txproxy`, which
uses `infrastructure/chain` for tx building/broadcast and enforces an explicit
contract+method allowlist. Other services should only use `infrastructure/chain`
for **read-only** calls and event monitoring.

## Global Signer (TEE-Managed Signing)

`infrastructure/globalsigner` provides a single place to manage enclave-held
master key material and derive **domain-separated** signing keys.

Use cases:

- signing service-layer on-chain fulfillments / callbacks
- signing off-chain service receipts (future)
- key rotation with auditability

## Account Pool (Large-Scale Neo N3 Accounts)

`infrastructure/accountpool` manages a large pool of Neo N3 accounts (target:
10,000+ accounts) and provides:

- account allocation + locking (`service_id`)
- balance tracking + updates
- rotation/archival of accounts (move funds, retire old accounts)

This is an infrastructure capability used by multiple services; it is not a
product-facing API.

## Product Services (Responsibilities)

Only these services are considered product services right now:

- **NeoFeeds (`neofeeds`)**: aggregate prices, sign responses, optionally push updates on-chain.
- **NeoFlow (`neoflow`)**: schedule triggers, run webhooks, optionally execute on-chain actions.
- **NeoCompute (`neocompute`)**: execute JS with strict limits + optional secret injection.
- **NeoOracle (`neooracle`)**: fetch external data with allowlist + optional secret injection.
- **TxProxy (`txproxy`)**: allowlisted transaction signing + broadcast proxy (single point for tx policy).

Randomness is provided by running scripts in NeoCompute (`neocompute`) inside the enclave (optionally anchoring results via `RandomnessLog`).

Each service follows the same internal pattern:

- `services/<svc>/marble`: HTTP handlers + workers (enclave runtime).
- `services/<svc>/supabase`: service-specific persistence (only when needed).

Platform contracts live under `contracts/` and are written by the enclave-managed
signer (Updater pattern) when needed.

## EGo Boundary (What belongs in the enclave)

Keep enclave code focused on operations that need:

- confidentiality (private compute / secret-using fetch)
- integrity + verifiable origin (proofs/signatures)
- key custody (global signer)

Keep outside-TEE code focused on:

- user workflows and web-facing APIs
- data modeling and storage (Supabase)
- deployment glue and observability
