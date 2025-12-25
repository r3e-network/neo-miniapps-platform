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
- `services/requests` (`service_id`: `neorequests`)
- `services/gasbank` (`service_id`: `neogasbank`, optional)
- `services/simulation` (`service_id`: `neosimulation`, dev-only)

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

For the full local k3s stack (Supabase + Edge + MarbleRun), run:

```bash
./scripts/bootstrap_k3s_dev.sh --env-file .env --edge-env-file .env.local
```

Or see `docs/LOCAL_DEV.md` for detailed steps.

## Key Environment Variables

- `SUPABASE_URL`: Supabase project URL.
- `SUPABASE_SERVICE_KEY`: Supabase service role key (used by Go services and tooling).
- `SUPABASE_SERVICE_ROLE_KEY`: Supabase service role key (used by Supabase Edge functions).
- `SECRETS_MASTER_KEY`: encryption master key for secrets APIs (`platform/edge/functions/secrets-*`) and secret injection into TEE services.
- `NEO_RPC_URL` / `NEO_RPC_URLS`, `NEO_NETWORK_MAGIC`: Neo RPC configuration (services).
- `CONTRACT_PAYMENTHUB_HASH`, `CONTRACT_GOVERNANCE_HASH`, `CONTRACT_PRICEFEED_HASH`, `CONTRACT_RANDOMNESSLOG_HASH`, `CONTRACT_APPREGISTRY_HASH`, `CONTRACT_AUTOMATIONANCHOR_HASH`, `CONTRACT_SERVICEGATEWAY_HASH`: MiniApp platform contract hashes.
- `CONTRACT_MINIAPP_CONSUMER_HASH` (optional): MiniApp callback test contract hash for workflow scripts.
- `TXPROXY_ALLOWLIST`: tx-proxy allowlist JSON (contract+method policy).
- `GASBANK_URL` (optional): GasBank service URL for fee deduction.
- `GASBANK_DEPOSIT_ADDRESS` (optional): deposit address for GasBank verification.
- `NEOACCOUNTS_SERVICE_URL` (optional): account pool service URL.

See `.env.example` for a full list.

## Docs

- `docs/ARCHITECTURE.md`: current end-to-end architecture and TEE boundary
- `docs/WORKFLOWS.md`: MiniApp lifecycle + callback workflows
- `docs/DATAFLOWS.md`: request/dataflow + audit tables
- `docs/LAYERING.md`: layering rules + boundaries (what goes where)
- `docs/MODULE_RESPONSIBILITIES.md`: per-module responsibilities + dependency rules
- `docs/API_DOCUMENTATION.md`: gateway/service API reference
- `docs/DEPLOYMENT_GUIDE.md`: deployment paths (Docker, MarbleRun, K8s)
- `docs/MASTER_KEY_ATTESTATION.md`: GlobalSigner key + attestation workflow
- `docs/sdk-guide.md`: MiniApp SDK integration guide
- `docs/service-api.md`: Service API reference

## Smart Contracts (Neo N3 Testnet)

| Contract            | Hash                                         | Description               |
| ------------------- | -------------------------------------------- | ------------------------- |
| PaymentHub          | `0x0bb8f09e6d3611bc5c8adbd79ff8af1e34f73193` | GAS payments for MiniApps |
| Governance          | `0xc8f3bbe1c205c932aab00b28f7df99f9bc788a05` | NEO staking and voting    |
| PriceFeed           | `0xc5d9117d255054489d1cf59b2c1d188c01bc9954` | Oracle price data         |
| RandomnessLog       | `0x76dfee17f2f4b9fa8f32bd3f4da6406319ab7b39` | VRF attestation anchoring |
| AppRegistry         | `0x79d16bee03122e992bb80c478ad4ed405f33bc7f` | MiniApp registration      |
| AutomationAnchor    | `0x1c888d699ce76b0824028af310d90c3c18adeab5` | Automation triggers       |
| ServiceLayerGateway | `0x27b79cf631eff4b520dd9d95cd1425ec33025a53` | Service request routing   |

## Builtin MiniApps (23 Apps)

**Phase 1 - Gaming:**

- `builtin-lottery` - Neo Lottery with provable randomness
- `builtin-coin-flip` - 50/50 double-or-nothing
- `builtin-dice-game` - Roll dice, win up to 6x
- `builtin-scratch-card` - Instant win scratch cards

**Phase 2 - DeFi & Social:**

- `builtin-prediction-market` - Price movement predictions
- `builtin-flashloan` - Instant borrow and repay
- `builtin-price-ticker` - Real-time price feeds
- `builtin-gas-spin` - Lucky wheel with VRF
- `builtin-price-predict` - Binary options trading
- `builtin-secret-vote` - Privacy-preserving voting
- `builtin-secret-poker` - TEE Texas Hold'em
- `builtin-micro-predict` - 60-second predictions
- `builtin-red-envelope` - Social GAS red packets
- `builtin-gas-circle` - Daily savings circle

**Phase 3 - Advanced:**

- `builtin-fog-chess` - Chess with fog of war
- `builtin-gov-booster` - NEO governance tools
- `builtin-turbo-options` - Ultra-fast binary options
- `builtin-il-guard` - Impermanent loss protection
- `builtin-guardian-policy` - TEE transaction security

**Phase 4 - Long-Running Processes:**

- `builtin-ai-trader` - Autonomous AI trading agent
- `builtin-grid-bot` - Automated grid trading
- `builtin-nft-evolve` - Dynamic NFT evolution
- `builtin-bridge-guardian` - Cross-chain asset bridge
