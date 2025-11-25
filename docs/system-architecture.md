# System Architecture (Service Layer)

This doc describes how the Service Layer is structured so operators and developers
understand where to add features, debug issues, or deploy components.

## High-Level Layers
- **Edge / Auth**: HTTP API on port 8080 with bearer tokens (`API_TOKENS`) and optional JWT (`/auth/login`, `AUTH_USERS`/`AUTH_JWT_SECRET`). CORS is open for the dashboard. Tenants are enforced via `X-Tenant-ID`.
- **Application Services**: Modular domains under `internal/services` (accounts, automation, gasbank, oracle, datafeeds, datalink, datastreams, DTA, CRE, CCIP, VRF, random, confidential, JAM). The handler (`internal/app/httpapi`) routes requests and applies auth/tenant checks.
- **Service Core Engine**: `internal/engine` provides a lightweight registry/lifecycle driver with common interfaces (ServiceModule + Account/Store/Compute/Data/Event engines). Think Android-style: the engine is the OS with standard surfaces (lifecycle/readiness/store/account/compute/data/event); services are apps that plug in via those interfaces instead of inventing their own. Runtime wiring lives in `internal/engine/runtime`. All domain services/runners implement a common lifecycle (Name/Start/Stop/Ready) via `system.LifecycleService`, and `/system/status` surfaces health, readiness, timings, uptime, slow modules, and the active slow threshold (configurable via config/env/flag). Module entries now include an `apis` array so operators can see which OS-level surfaces each service participates in.
- **Engine-managed infrastructure**: optional modules for Neo full node + indexer (neo-go, testnet), multi-chain RPC hub (BTC/ETH/NeoX/etc. via configured RPC endpoints), shared data-source hub (feeds/oracle/trigger sources), service-owned GAS bank control, a contracts module for deploy/invoke/manage service-layer contracts, a crypto engine (ZKP/FHE/MPC helpers), and a RocketMQ-backed event bus (name servers/topic prefix/consumer group/namespace/max reconsume/consume batch/consume from). These register with the engine for lifecycle/readiness and surface their APIs via `/system/status`. Each infra module is tagged with `layer=infra` plus capabilities/quotas so operators can distinguish OS services from application services.
- **Descriptors/Manifests**: services and runners advertise descriptors with layer (`service|runner|infra`), capabilities, required APIs, and dependencies. `/system/descriptors` and `/system/status` surface these; startup can fail when required APIs are missing if `runtime.require_apis_strict=true`.
- Engine exposure: `/system/status` returns the registered module names (store, app, domain services, background runners) with domain/category, lifecycle status, readiness, timestamps, notes, and a summary of data/event/compute-capable modules to aid operations. Notes surface runtime observations such as collision-renamed modules.
- Engine typed interfaces: modules that implement `StoreEngine`, `AccountEngine`, `ComputeEngine`, `DataEngine`, or `EventEngine` are recorded and exposed via `/system/status` (module `interfaces` field) for typed lookups and operator visibility.
- Engine permissions/fan-out: `/system/events|data|compute` fan-out to Event/Data/Compute engines (and in-process subscribers) honoring per-module bus permissions (surfaced as `permissions` in `/system/status`). Dashboard “Engine Bus Console” and `slctl bus` wrap these for operators; see `docs/examples/bus.md` for payload shapes per service.
- RPC proxy: `/system/rpc` proxies JSON-RPC calls to the configured chain RPC hub (`runtime.chains.endpoints`). Payload: `{"chain":"eth","method":"eth_blockNumber","params":[...]}`; relies on `svc-chain-rpc` and RPCEndpoints exposed by the engine. Operators can enforce tenancy, rate limits, and method allowlists via `runtime.chains.require_tenant|per_tenant_per_minute|per_token_per_minute|burst|allowed_methods`.
- Required API surfaces: manifests can declare `requires_apis`; `/system/status` exposes `modules_requires_apis` and `modules_requires_missing`. When `runtime.require_apis_strict=true` the engine fails startup if any required surface is missing.
- **Persistence**: Postgres by default (compose service `postgres`); schemas evolved via embedded migrations in `internal/platform/migrations`. In-memory stores exist for dev-only.
- **NEO Layer**: Indexer (`cmd/neo-indexer`) captures blocks/txs/app logs/notifications and per-block storage/diffs into Postgres tables (`neo_blocks`, `neo_transactions`, `neo_storage`, `neo_storage_diffs`, `neo_meta`). Snapshot generator (`cmd/neo-snapshot`) emits stateless KV bundles/manifests (with hashes and optional signatures) reused by the API (`/neo/*`) and dashboard. Optional NEO nodes (compose profile `neo`) provide RPC for mainnet/testnet.
- **Observability**: Prometheus metrics at `/metrics`, health at `/healthz`, system summary at `/system/status` (includes NEO and JAM status). Dashboard consumes these and displays lag/height for NEO when `NEO_RPC_STATUS_URL` is set.

### Engine Modules (OS + Apps)
Example `/system/status` modules after enabling infrastructure modules:
```
{
  "modules": [
    {"name":"store-postgres","domain":"store","status":"started","apis":[{"name":"lifecycle","surface":"lifecycle"},{"name":"readiness","surface":"readiness"},{"name":"store","surface":"store"}]},
    {"name":"svc-neo-node","domain":"neo","status":"started","ready_status":"ready","apis":[{"name":"neo-ledger","surface":"ledger"},{"name":"neo-rpc","surface":"rpc"}]},
    {"name":"svc-neo-indexer","domain":"neo","status":"started","ready_status":"ready","apis":[{"name":"neo-indexer","surface":"indexer"},{"name":"neo-rpc","surface":"rpc"}]},
    {"name":"svc-chain-rpc","domain":"chains","status":"started","ready_status":"ready","apis":[{"name":"chain-rpc","surface":"rpc"}]},
    {"name":"svc-data-sources","domain":"data-sources","status":"started","ready_status":"ready","apis":[{"name":"data-sources","surface":"data-source"}]},
    {"name":"svc-contracts","domain":"contracts","status":"started","ready_status":"ready","apis":[{"name":"contracts","surface":"contracts"}]},
    {"name":"svc-rocketmq","domain":"event","status":"started","ready_status":"ready","apis":[{"name":"event-bus","surface":"event"}]},
    {"name":"svc-accounts","domain":"accounts","status":"started","ready_status":"ready","apis":[{"name":"accounts","surface":"account"},{"name":"compute","surface":"compute"}]},
    {"name":"svc-service-bank","domain":"gasbank","status":"started","ready_status":"ready","apis":[{"name":"gasbank-ops","surface":"gasbank"}]},
    {"name":"svc-functions","domain":"functions","status":"started","ready_status":"ready","apis":[{"name":"compute","surface":"compute"}]},
    {"name":"svc-http","domain":"system","status":"started","ready_status":"ready","apis":[{"name":"lifecycle","surface":"lifecycle"},{"name":"readiness","surface":"readiness"}]}
  ],
  "modules_summary":{"data":["svc-datafeeds","svc-datalink","svc-datastreams","svc-data-sources"],"event":["svc-pricefeed","svc-oracle","svc-datalink","svc-datafeeds"],"compute":["svc-functions"]},
  "modules_api_summary":{"rpc":["svc-neo-node","svc-neo-indexer","svc-chain-rpc"],"ledger":["svc-neo-node"],"indexer":["svc-neo-indexer"],"data-source":["svc-data-sources"],"contracts":["svc-contracts"],"gasbank":["svc-service-bank"],"event":["svc-rocketmq"]},
  "modules_layers":{"service":["svc-accounts","svc-functions","svc-automation"],"runner":["runner-automation","runner-oracle"],"infra":["store-postgres","svc-neo-node","svc-neo-indexer","svc-chain-rpc","svc-data-sources","svc-service-bank","svc-crypto","svc-contracts","svc-rocketmq"]},
  "modules_slow_threshold_ms":1000
}
```
- **Operator UX**: React dashboard (`apps/dashboard`, port 8081) plus marketing site (`apps/site`, port 8082). Dashboard deep-links support `?api`, `?token`, `?tenant`, `?prom`.
- **Tooling**: CLI `slctl` mirrors API features and includes NEO tooling (status/blocks/snapshots/storage/diffs/verify) and dashboard-link generation.

## Deployment Topology (compose)
- `service-layer` (appserver, port 8080) depends on `postgres`.
- `dashboard` (port 8081) and `site` (port 8082) depend on `service-layer`.
- `neo-indexer` (profile `neo`, command `/app/neo-indexer`) depends on `postgres` and the NEO RPC URL.
- Optional: `neo-mainnet`, `neo-testnet` (profile `neo`) expose RPC on 10332/10342 and persist chains to volumes.
- Volumes: `postgres-data`, `neo-mainnet-chain`, `neo-testnet-chain`, `neo-plugins`.

## Data Flows
- Requests enter via API → handler → domain service → Postgres. Tenant header is required for account-scoped resources.
- JAM endpoints can persist to Postgres when `JAM_STORE=postgres` and `JAM_PG_DSN`/`DATABASE_URL` are set; otherwise in-memory.
- NEO indexer pulls from RPC, writes normalized rows; snapshotter reads Postgres (or RPC) to assemble manifests/bundles; API reads manifests (`NEO_SNAPSHOT_DIR`) and storage/diffs to expose `/neo/*`; dashboard/CLI consume those endpoints.

## Testing & Release Gates
- Required CI: `neo-smoke` workflow (Go tests, dashboard typecheck, mocked NEO curl). Make it required on `master` (see `docs/branch-protection.md`).
- Local smoke: `make run`, dashboard deep-link, `slctl status`, `slctl neo status`.

## Hardening Checklist (prod)
- Replace `dev-token`, `AUTH_USERS`, `AUTH_JWT_SECRET`, `SECRET_ENCRYPTION_KEY` with strong secrets; restrict tokens per tenant.
- Terminate TLS at ingress/reverse proxy; set proper CORS origins if locked down.
- Backup Postgres volumes; monitor `/metrics` and `/healthz`; enable log shipping.
- Run NEO nodes with persistent volumes and monitor lag; point `NEO_RPC_STATUS_URL` to your RPC endpoint so dashboard shows lag/height.
