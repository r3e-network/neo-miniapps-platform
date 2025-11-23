# NEO Layering Plan

Goal: make the service layer sit on top of fully-synced NEO full nodes (mainnet + testnet), expose indexed data, and publish stateless execution state snapshots (key-value sets + trusted state roots) per block.

## Target Architecture
1) **Node layer (per network)**  
   - Run `neo-cli` full nodes for mainnet and testnet (LevelDB store, persistent volumes).  
   - Enable RPC plugins: `ApplicationLogs`, `RpcServer`, `RpcClient`, `RpcStates` (for stateroots), `StatesDumper`/`StateService` for exporting contract storage.  
   - Expose RPC endpoints (JSON-RPC over HTTP) on separate ports per network.

2) **Indexer layer**  
   - Consume node RPC and new blocks; store normalized chain data (blocks, transactions, notifications, contract storage diffs) into Postgres (or MongoDB if preferred for logs).  
   - Provide APIs/feeds to the service layer for “read-side” operations (balances, contract reads, event history).

3) **State snapshot layer (“stateless execution state”)**  
   - For each block `N`, publish:
     - Trusted state root (from `RpcStates` / consensus header).  
     - Compact key-value set containing all contract storage keys required to execute block `N+1` (or a window), signed and chunked for download.  
   - Store snapshots in object storage (S3/minio) with metadata (block height, root hash, content hash, signature).

4) **Service layer integration**  
   - Inject indexer and snapshot URLs into service-layer config so APIs can serve fast reads without hitting full nodes.  
   - Add health checks to verify node sync height vs. NEO seed peers; fail if lag exceeds threshold.

## Concrete Steps
1) **Node containers (compose)**  
   - Add two services to `docker-compose`: `neo-mainnet` and `neo-testnet` based on `neocli` image.  
   - Mount plugin directory and volumes for `Chains/MainNet` and `Chains/TestNet`.  
   - Expose ports (e.g., 10332 mainnet RPC, 10342 testnet RPC).  
   - Seed peers via `config.json` per network; enable ApplicationLogs/RpcStates plugins.

2) **Indexer service**  
   - Implement a small Go worker that:  
     - Polls `getblock`/`getapplicationlog` via RPC, normalizes into Postgres tables (blocks, txs, notifications, storage diffs).  
     - Persists last processed height; supports catch-up and reorg handling using persisted state roots.  
   - Expose REST endpoints for the service layer (or reuse existing APIs with a new “neo-index” module).

3) **Snapshot generator**  
   - Use `RpcStates` to fetch state roots and `dumpstates`/`getcontractstate` to pull storage.  
   - For each block (or every N blocks), compute:
     - KV bundle of storage slots touched in that block (or full contract storage for determinism).  
     - Hash of bundle + state root; sign bundle with operator key; upload to object storage.  
   - Publish manifest JSON per block: `{ height, state_root, kv_url, kv_hash, signature, generated_at }`.

4) **Trust + validation**  
   - Verify state roots against consensus headers (RpcStates).  
   - Before serving a snapshot, validate bundle hash and signature; expose `/neo/snapshots/:height` API.

5) **Service layer wiring**  
   - Extend configuration to include `neo` block source (indexer URL + snapshot URL + preferred network).  
   - Add health checks: node reachable, synced within X blocks, state root matches latest header.

## Operational Notes
- Mainnet sync is heavy; keep compose defaults off (use profiles) and document disk/CPU requirements.  
- Persist volumes for chain data; do not rebuild on every compose up.  
- For production, run nodes on dedicated hosts; compose is for local/staging only.

## Next Implementation Tasks
1) Add compose services for `neo-mainnet` and `neo-testnet` (disabled by default) with plugins mounted.  
2) Scaffold a Go indexer module (poll RPC, store blocks/logs/storage into Postgres).  
3) Scaffold snapshot generator job (pull state root + storage, write bundle + manifest).  
4) Add service-layer config knobs for indexer/snapshot endpoints and health checks.  
5) Add docs for operating the nodes (ports, volumes, sample configs) and for consuming snapshots from the service layer.
