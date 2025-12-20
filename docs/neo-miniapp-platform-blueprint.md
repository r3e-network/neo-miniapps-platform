# Neo N3 Mini‑App Platform (Architectural Blueprint)

This is the **reviewed, polished, and expanded** blueprint the repository is
converging to. It explicitly locks the technology choices and enforces the hard
constraints:

- **Settlement:** **GAS only** (payments/settlement must reject other assets)
- **Governance:** **NEO only** (voting/staking; no bNEO)
- **Confidential services:** **MarbleRun + EGo (SGX TEE)** with attested TLS
- **Gateway/Data:** **Supabase** (Auth + Postgres + RLS + Edge Functions)
- **Frontend host:** **Vercel + Next.js** + micro‑frontends
- **High‑freq data:** **Datafeed** pushes on **≥ 0.1%** deviation

## Repo Notes (Current Implementation)

This repo implements the blueprint with a few deliberate adjustments to keep
responsibilities clear:

- **Dedicated VRF service:** randomness is provided via `neovrf` and can be
  anchored on‑chain via `RandomnessLog`.
- **Service naming:** runtime **service IDs** are kept stable (`neofeeds`,
  `neocompute`, `neovrf`, ...). See `docs/platform-mapping.md` for mapping to
  blueprint names (`datafeed-service`, `compute-service`, `vrf-service`, ...).
- **Edge ↔ TEE mTLS:** Supabase Edge functions support optional mTLS via
  `Deno.createHttpClient`. Enclave services can additionally trust a gateway
  client CA via `MARBLE_EXTRA_CLIENT_CA`.

For the expanded Chinese spec, see:

- `docs/neo-miniapp-platform-full.md`

---

## 1. Complete Tech Stack & Tooling Selection

### A. Blockchain Layer (Neo N3)

- **Network:** Neo N3 MainNet (prod) / TestNet (stage)
- **Contracts:** **C#** via `neo-devpack-dotnet`
- **Local chain:** `neo-express` (Neo Express)
- **Contract testing:** `Neo.TestingFramework`
- **Client SDK:** `neon-js` (TS/JS transaction construction)
- **Wallet integration:** NeoLine / O3 / OneGate via dAPI

### B. Service Layer (Confidential Computing)

- **Hardware:** Intel SGX nodes (or simulation for dev)
- **Enclave runtime:** **EGo** (Go inside enclaves)
- **Orchestration:** **MarbleRun** (attestation, secrets distribution, scaling)
- **Language:** **Go**
- **Networking:** REST (or gRPC) over attested TLS

### C. Backend & Gateway

- **Platform:** **Supabase**
- **Auth:** Supabase Auth (OAuth: Google/GitHub/Twitter/etc.)
- **DB:** Postgres + RLS
- **API/Gateway:** Supabase Edge Functions (Deno) as the thin gateway
- **Storage:** Supabase Storage

### D. Frontend Platform

- **Framework:** Next.js
- **Hosting:** Vercel
- **Micro‑frontend strategy:**
  - Built‑ins: Module Federation
  - Untrusted 3rd‑party apps: `<iframe sandbox>` + strict `postMessage` protocol

### E. DevOps & CI/CD

- **Repo:** GitHub monorepo
- **CI:** GitHub Actions
- **Security:** dependency scans, CSP checks, TEE measurement builds

---

## 2. Repository Structure (Target Layout)

Target platform shape:

```text
neo-miniapp-platform/
├── contracts/                  # [C#] Neo N3 Contracts (GAS-only / NEO-only enforced)
│   ├── PaymentHub/             # Payments & settlement (GAS ONLY)
│   ├── Governance/             # Voting & staking (NEO ONLY)
│   ├── PriceFeed/              # On-chain datafeed anchor (0.1% trigger)
│   ├── RandomnessLog/          # Randomness anchoring (TEE report hash)
│   ├── AppRegistry/            # App manifest hash + allowlist anchors
│   └── AutomationAnchor/       # Task registry + anti-replay nonce
│
├── services/                   # [Go] EGo + MarbleRun TEE services
│   ├── datafeed-service/       # Price aggregation + publish policy
│   ├── oracle-gateway/         # Allowlisted HTTP fetch + secret injection
│   ├── vrf-service/            # Verifiable randomness generation
│   ├── compute-service/        # Restricted scripts/compute
│   ├── automation-service/     # Triggers + anchored tasks execution
│   ├── tx-proxy/               # Allowlisted sign+broadcast gatekeeper
│   └── marblerun/              # Manifests/policies (policy.json, manifest.json, CA)
│
├── platform/                   # Host platform layer
│   ├── host-app/               # Next.js host app (Vercel)
│   ├── builtin-app/            # Built-in MiniApps (Module Federation remote)
│   ├── sdk/                    # MiniAppSDK (TS)
│   ├── edge/                   # Supabase Edge functions
│   └── rls/                    # RLS SQL (schema lives in migrations/)
│
├── miniapps/                   # Mini-apps + starter kits
├── docker/                     # Local dev compose (Supabase, MarbleRun, services)
├── deploy/                     # neo-express config + deployment scripts
└── .github/                    # CI workflows (GitHub Actions)
```

Current repo mapping (actual folder names and service IDs):

- `services/datafeed` (`neofeeds`) → `datafeed-service`
- `services/conforacle` (`neooracle`) → `oracle-gateway`
- `services/confcompute` (`neocompute`) → `compute-service`
- `services/vrf` (`neovrf`) → `vrf-service`
- `services/automation` (`neoflow`) → `automation-service`
- `services/txproxy` (`txproxy`) → `tx-proxy`

The repository also contains an explicit shared **infrastructure** layer:

- `infrastructure/chain`: single source of truth for Neo RPC/tx building/events
- `infrastructure/middleware`: shared HTTP middleware only (no per-service copies)
- `infrastructure/runtime`: strict identity mode + environment helpers
- `infrastructure/secrets`: user secrets encryption + permissions + audit
- `infrastructure/globalsigner`: TEE-held signing root + domain-separated keys
- `infrastructure/accountpool`: large pool of Neo N3 accounts + rotation

---

## 3. Core Component Design

### A. Contracts (Asset & Logic Constraints)

1. **PaymentHub**
   - Rejects non‑GAS payments (NEP‑17 callback checks `Runtime.CallingScriptHash`)
   - Supports app config + split settlement + receipts
2. **Governance**
   - Accepts **NEO only** for staking
   - Supports proposal skeleton + vote tracking (stake‑weighted)
3. **PriceFeed**
   - Stores `(symbol, round_id, price, ts, attestation_hash, sourceset_id)`
   - Enforces monotonic `round_id` to prevent replay
4. **RandomnessLog**
   - Anchors `(request_id, randomness, attestation_hash, timestamp)`
5. **AppRegistry**
   - Stores manifest hash + status + allowlist anchor hash
6. **AutomationAnchor**
   - Stores tasks + marks executed nonces to prevent replay

### B. TEE Services (The “Black Box”)

All signing keys and secret material stay **inside the enclave**.

1. **`tx-proxy` (Gatekeeper)**
   - Single point for enclave‑origin chain writes
   - Enforces contract+method allowlist and replay protection
   - Optional intent gates:
     - `payments` → PaymentHub `pay` only
     - `governance` → Governance `stake/unstake/vote` only
2. **`datafeed-service`**
   - Polls multiple sources frequently (default 1s)
   - Aggregates (median/weighted) and publishes on ≥ 0.1% deviation
   - Hysteresis 0.08%, throttle ≥ 5s min interval, cap ~30/min/symbol
   - Anchors updates via `tx-proxy` → `PriceFeed.update`
3. **`vrf-service`**
   - Signs `request_id` inside the enclave and derives randomness from the signature
   - Returns `randomness`, `signature`, `public_key`, and `attestation_hash`
   - Optional anchoring via `RandomnessLog.record` using `tx-proxy`
4. **`compute-service`**
   - Restricted scripts with resource limits + optional secret injection
   - Used for confidential computation workloads (non-randomness)
5. **`oracle-gateway`**
   - Allowlisted outbound HTTP fetch + optional secret injection
   - Used for “confidential oracle” workflows
6. **`automation-service`**
   - Scheduler + triggers
   - Optional anchored tasks via `AutomationAnchor` and on-chain event monitoring

### C. Gateway (Supabase Edge)

Supabase Edge is a stateless thin router:

- validates JWT / API key
- enforces nonce/replay + rate limits
- enforces “must bind wallet” for state-changing flows
- routes to enclave services over mTLS when configured

Canonical endpoints:

- `POST /functions/v1/pay-gas`
- `POST /functions/v1/vote-neo`
- `POST /functions/v1/rng-request`
- `POST /functions/v1/wallet-nonce`, `POST /functions/v1/wallet-bind`
- `POST /functions/v1/app-register`, `POST /functions/v1/app-update-manifest`
- `GET /functions/v1/datafeed-price`
- `POST /functions/v1/oracle-query`
- `POST /functions/v1/compute-execute`, `GET /functions/v1/compute-jobs`, `GET /functions/v1/compute-job?id=<job_id>`
- `GET/POST /functions/v1/automation-triggers` (trigger CRUD/lifecycle via `neoflow`)
- `secrets-*`, `api-keys-*`, `gasbank-*`

---

## 4. Frontend & Security Sandbox

- Host app loads miniapps.
- Untrusted apps run in `iframe sandbox` and only talk to the platform via a
  strict postMessage protocol.
- Platform enforces strict CSP (no `eval`, no uncontrolled external scripts).

---

## 5. Security Checklist (4-Layer Defense)

1. **SDK:** strict typed APIs (no asset param for `payGAS`/`vote`)
2. **Edge:** auth + nonce + caps + routing (GAS-only / NEO-only enforced)
3. **TEE:** allowlists + enclave-held keys + attested origin
4. **Contracts:** hardcoded asset checks and monotonicity checks (replay resistance)

---

## 6. MVP Roadmap

1. Deploy contracts to local `neo-express` / TestNet:
   - PaymentHub, Governance, PriceFeed, RandomnessLog, AppRegistry, AutomationAnchor
2. Bring up enclave services in simulation:
   - tx-proxy, vrf-service, compute-service, datafeed-service, automation-service, oracle-gateway
3. Wire Supabase Edge → services (auth + routing) and the Next.js host app.
