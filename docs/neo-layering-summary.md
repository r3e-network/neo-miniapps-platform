# NEO Layering: Operations to Production Readiness

This document distills the current implementation and the remaining steps to run the service layer as a production-grade, NEO-aware SaaS.

## Implemented
- **Node layer (compose profile `neo`)**: neo-cli mainnet/testnet containers with persisted chain volumes and plugin volume mounts. Operators can start/stop via `make neo-up` / `make neo-down`.
- **Indexer layer**: `cmd/neo-indexer` polls RPC, persists blocks, txs, VM executions, notifications, state roots, and checkpoints (`neo_meta`) into Postgres. Simple reorg handling rewrites diverging heights. State roots are cross-checked against `getstateroot` per height.
- **Storage capture**: Contracts touched in a block (via notifications) have their full storage persisted into `neo_storage` (Postgres) so `/neo/storage/{height}` and `slctl neo storage <height>` can serve KV blobs for stateless execution inputs.
- **Snapshot layer**: `cmd/neo-snapshot` fetches state roots and contract storage, emits manifests with SHA256 + size of KV bundles, optional public URL, and RPC used. Bundles are tar.gz with per-contract JSON. Manifests are served via `/neo/snapshots` when `NEO_SNAPSHOT_DIR` is set.
- **API surface**: `/neo/status`, `/neo/blocks`, `/neo/blocks/{height}`, `/neo/snapshots`, `/neo/snapshots/{height}` expose indexed data and manifests. `/system/status` embeds NEO status. CLI `slctl neo ...` queries these endpoints.
- **Docs**: `docs/neo-api.md`, `docs/neo-ops.md`, `docs/neo-layering.md` describe operations, goals, and API surfaces.

## Next priorities
1) **Storage diffs & validation**
   - Persist per-contract storage diffs per block (key/write set) alongside state root, to support stateless execution packages. Use `getcontractstorage` or plugin-provided diffs.
   - Compute and persist block content hash + KV bundle hash for integrity. Validate roots against consensus headers regularly.

2) **Reorg safety & checkpoints**
   - Store last stable height (finalized) separately from last processed. On reorg detection, walk back N blocks and replay with correct roots. `stable_height/hash/state_root` are currently a buffer over `latest_height`; replace with a confirmed/finalized height when a signal is available.
   - `/neo/checkpoint` already exposes stable/lag fields; tighten to finalized state when available.

3) **Serving stateless execution state**
   - API to fetch KV bundle metadata and signed hash (`/neo/snapshots/{height}/manifest`) and stream bundle downloads (signed URL or reverse-proxy from storage).
   - Include operator signature and hash chain in manifests for tamper evidence.
   - Provide KV diff bundle alongside full KV to reduce payload size for clients that already hold previous state.

4) **Dashboard integration**
   - New panels for NEO: status (height/root lag vs node), recent blocks (tx/notifications), snapshot list with download links, and integrity indicators.

5) **Neo node ops hardening**
   - Add readiness probes for RPC/plugins, disk usage alerts for chain volumes, and optional pruning/archival guidance.
   - Document recommended plugin config (ApplicationLogs, RpcStates, StateService) with version pinning.

6) **CI/QA**
   - Add smoke tests for `/neo/status` and `slctl neo status` against a lightweight mocked RPC (or recorded fixtures) to keep pipelines green without a full node.

7) **Multi-tenant awareness (read side)**
   - Keep NEO data global, but ensure admin queries remain auth-protected. If per-tenant views are needed, add filters keyed by tenant-owned addresses/contracts.

## Operational recipe (current)
```bash
# Run neo-cli nodes (mainnet/testnet) if needed
make neo-up

# Run indexer against a node and Postgres
go run ./cmd/neo-indexer \
  -rpc http://localhost:10332 \
  -dsn "postgres://service_layer:service_layer@localhost:5432/service_layer?sslmode=disable" \
  -network mainnet

# Generate a stateless snapshot (state root + KV bundle)
go run ./cmd/neo-snapshot \
  -rpc http://localhost:10332 \
  -height 123456 \
  -contracts 0xfffdc93764dbaddd97c48f252a53ea4643faa3fd \
  -dsn "postgres://service_layer:service_layer@localhost:5432/service_layer?sslmode=disable" \
  -out ./snapshots \
  -kv-url-base https://example.com/neo/snapshots

# Serve manifests/bundles via appserver
export NEO_SNAPSHOT_DIR=./snapshots
make run
```

Track these steps as we move toward a fully productionised, stateless NEO data plane layered under the service layer.
