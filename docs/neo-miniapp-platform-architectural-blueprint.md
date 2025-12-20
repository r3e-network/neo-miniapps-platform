# Neo N3 Mini-App Platform (Architectural Blueprint)

This document is the **reviewed, polished, and fully expanded** design blueprint.
It explicitly lists the technology stack, open-source tools, and platforms to use
for every layer, ensuring the **Payment = GAS / Governance = NEO** constraint is
strictly enforced.

> **Core Constraints:**
> - **Settlement:** **GAS Only** (PaymentHub rejects all other assets).
> - **Governance:** **NEO Only** (Voting/Staking).
> - **Confidentiality:** Service layer via **MarbleRun + EGo (SGX TEE)**.
> - **Gateway:** **Supabase** (Auth, DB, Edge).
> - **Frontend:** **Vercel** + **Next.js** + **Micro-frontends**.
> - **High-Freq Data:** **Datafeed** (Push on ≥0.1% deviation).

## 1. Complete Tech Stack & Tooling Selection

### A. Blockchain Layer (Neo N3)
- **Network:** Neo N3 Mainnet (Prod) / Testnet (Stage).
- **Contract Language:** **C#**.
- **Compiler/Framework:** `neo-devpack-dotnet`.
- **Local Development:** `neo-express`.
- **Testing:** `Neo.TestingFramework`.
- **Client SDK:** `neon-js`.
- **Wallet Integration:** NeoLine, O3, or OneGate (via dAPI).

### B. Service Layer (Confidential Computing)
- **Hardware Requirement:** Intel SGX-enabled servers (Azure Confidential Computing or bare metal).
- **Enclave Runtime:** **EGo** (Go).
- **Orchestration:** **MarbleRun**.
- **Service Language:** **Go**.
- **Networking:** gRPC or REST (attested TLS).

### C. Backend & Gateway
- **Platform:** **Supabase**.
- **Authentication:** Supabase Auth (GoTrue).
- **Database:** PostgreSQL (Supabase-managed).
- **API/Gateway:** Supabase Edge Functions (Deno).
- **Storage:** Supabase Storage.

### D. Frontend Platform
- **Framework:** **Next.js**.
- **Hosting:** **Vercel**.
- **Micro-frontend Strategy:**
  - Built-ins: Module Federation (`@module-federation/nextjs-mf`).
  - 3rd Party Apps: `iframe sandbox` + `postMessage`.
- **State Management:** Zustand (SDK-side state, optional).

### E. DevOps & CI/CD
- **Repo:** GitHub (Monorepo).
- **CI Runner:** GitHub Actions.
- **Security Scanning:** `npm audit`, CSP checks, and enclave measurement builds.

## 2. Repository Structure (Monorepo)

```text
neo-miniapp-platform/
├── contracts/                  # [C#] Neo N3 Contracts
│   ├── PaymentHub/             # Logic: Pay/Split/Refund (GAS ONLY)
│   ├── Governance/             # Logic: Vote/Stake (NEO ONLY)
│   ├── PriceFeed/              # Logic: Datafeed storage (0.1% trigger)
│   ├── RandomnessLog/          # Logic: VRF verification storage
│   ├── AppRegistry/            # Logic: Manifest hash & Dev Allowlist
│   └── AutomationAnchor/       # Logic: Task registry
│
├── services/                   # [Go] EGo + MarbleRun TEE Services
│   ├── datafeed-service/       # High-freq polling, threshold logic
│   ├── oracle-gateway/         # General purpose fetcher
│   ├── vrf-service/            # Verifiable randomness generation
│   ├── compute-service/        # Confidential compute scripts
│   ├── tx-proxy/               # The "Key Holder". Signs transactions.
│   └── marblerun/              # Manifests & policies
│
├── platform/                   # [TS] Host Platform
│   ├── host-app/               # Next.js App (The "App Store" UI)
│   ├── builtin-app/            # Built-in MiniApps (Module Federation remote)
│   ├── sdk/                    # The "MiniAppSDK" npm package
│   ├── edge/                   # [Deno] Supabase Edge Functions
│   └── rls/                    # [SQL] RLS Policies & Migrations
│
├── miniapps/                   # [React/Vue] Mini-apps
│   ├── builtin/                # Trusted apps (Coin Flip, Dice, Lottery, etc.)
│   └── templates/              # Starter kits for users
│
└── infra/                      # Infrastructure Config
    ├── neo-express/            # Local chain setup (.neo-express)
    └── ci/                     # Github Workflows
```

## 3. Core Component Design

### A. Contracts (Asset & Logic Constraints)
1. **PaymentHub.cs**
   - Hardcoded check: reject any non-GAS asset.
   - Manages developer revenue splits.
2. **Governance.cs**
   - Hardcoded check: reject any non-NEO asset.
   - No bNEO support.
3. **PriceFeed.cs**
   - Stores `(Symbol, Price, Timestamp, RoundID, AttestationHash)`.
   - Enforces `RoundID` monotonicity.
   - Validates authorized updater (TEE node allowlist).

### B. TEE Services (The "Black Box")
All services run inside EGo enclaves. Keys **never** leave the enclave.

1. **tx-proxy**
   - Holds platform signing key(s).
   - Enforces contract+method allowlist and intent gates.
2. **datafeed-service**
   - Polls multiple sources, computes median.
   - Pushes updates when deviation ≥0.1% with hysteresis + throttling.
3. **vrf-service**
   - Signs `request_id` inside TEE and derives randomness from the signature.
   - Optionally anchors randomness in `RandomnessLog` via `tx-proxy`.
4. **compute-service**
   - Runs restricted scripts inside TEE (confidential jobs).

### C. Gateway (Supabase Edge)
- Stateless router + rate limiter.
- Validates Supabase Auth JWT / API keys.
- Enforces **GAS-only** (payments) and **NEO-only** (governance).
- Uses mTLS when calling TEE services.

## 4. Frontend & Security Sandbox

### The Host (Vercel)
- Loads mini-apps.
- Untrusted apps run in `iframe` with `sandbox` attributes.
- Uses strict CSP and a postMessage allowlist.

### The SDK (`window.MiniAppSDK`)

```ts
interface MiniAppSDK {
  getAddress(): Promise<string>;
  payments: { payGAS(appId: string, amount: string, memo?: string): Promise<unknown> };
  governance: { vote(appId: string, proposalId: string, amount: string): Promise<unknown> };
  rng: { requestRandom(appId: string): Promise<unknown> };
  datafeed: { getPrice(symbol: string): Promise<unknown> };
}
```

## 5. Deployment & User Flow

1. Developer builds a miniapp from a starter kit and writes `manifest.json`.
2. Platform validates/sanitizes and computes `manifest_hash`.
3. MarbleRun injects secrets and manages attestation for production enclaves.

## 6. Security Checklist (4-Layer Defense)

1. SDK enforces the high-level API shape.
2. Edge enforces auth, limits, and request validation.
3. TEE enforces signing policy + allowlists.
4. Contracts enforce GAS-only / NEO-only on-chain.

## 7. MVP Roadmap

1. Infra setup (neo-express, Supabase, CI).
2. Deploy PaymentHub + AppRegistry to local chain.
3. TEE skeleton (EGo hello world, tx-proxy).
4. End-to-end `payGAS` flow with a built-in MiniApp.
