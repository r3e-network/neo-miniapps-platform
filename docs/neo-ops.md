# Operating NEO nodes (local/staging)

This doc covers bringing up neo-cli full nodes (mainnet + testnet) using the optional `neo` compose profile, and the plugins/ports/volumes you need to keep.

## What gets started
- `neo-mainnet` (neo-cli 3.6.0) with:
  - RPC: `10332`
  - Data dir: volume `neo-mainnet-chain` mounted at `/neo-cli/Chains/MainNet`
  - Plugins: shared volume `neo-plugins` mounted at `/neo-cli/Plugins`
- `neo-testnet` (neo-cli 3.6.0) with:
  - RPC: `10342` (maps container 10332)
  - Data dir: volume `neo-testnet-chain` mounted at `/neo-cli/Chains/TestNet`
  - Plugins: shared volume `neo-plugins`

## Start/stop
```bash
make neo-up     # docker compose --profile neo up -d neo-mainnet neo-testnet neo-indexer
make neo-down   # docker compose --profile neo down --remove-orphans
# full stack (appserver + dashboard + site + postgres + neo-indexer + nodes)
make run-neo
```

## Plugins to install
Place plugin DLLs/configs into the `neo-plugins` volume (mounted at `/neo-cli/Plugins`):
- `ApplicationLogs` (required for tx execution traces)
- `RpcServer` (already included by `neo-cli` when `/rpc` flag is passed)
- `RpcStates` (state roots)
- `StateService` or `StatesDumper` (for contract storage/state export)
- `ImportBlocks` (optional, speeds up initial sync if you pre-download blocks)

Add plugin config JSON into the same volume; compose does not ship plugin binaries.

## Persistence and rebuilding
- Chain data lives in named volumes; `make neo-up` will reuse them.
- To force a resync, remove the volumes (`docker volume rm neo-mainnet-chain neo-testnet-chain`) — do this only when intentional.

## Networking
- RPC endpoints (host): `http://localhost:10332` (mainnet), `http://localhost:10342` (testnet).
- P2P ports are not exposed in compose; for real network participation expose 10333/10334 as needed.

## Health and sync
- Check height: `curl http://localhost:10332/?jsonrpc=2.0&id=1&method=getblockcount`
- Check state root: `curl -d '{"jsonrpc":"2.0","method":"getstateroot","params":[<height>],"id":1}' http://localhost:10332`
- Expect long initial sync; monitor disk/CPU.
- Expose node health/lag in `/neo/status` by setting `NEO_RPC_STATUS_URL` to your node RPC endpoint (e.g., `http://localhost:10332`).
- Shortcut: `/neo/checkpoint` returns the same payload as `/neo/status`; use `slctl neo checkpoint` for a concise readout (enabled, latest height/hash, node height/lag).
- `stable_height/hash/state_root` currently mirror `latest_*` until finality detection lands; plan to set stable values once a finalized block is detected.
- Stability buffer: set `NEO_STABLE_BUFFER` (default 12) to subtract a safety
  window from `latest_height` when reporting `stable_height` (uses `neo_blocks`
  at that height for hash/root). The buffer is also compared against node lag.
- Storage summary: `/neo/storage-summary/<height>` (and dashboard block detail) gives per-contract KV/diff counts without streaming blobs. Use `slctl neo storage-summary <height>` for the same quick view.
- Engine bus smoke: use `slctl bus events|data|compute ...` or the dashboard Engine Bus Console (see `docs/examples/bus.md`) to publish feed updates or stream frames against the running stack; helpful for quick ingestion sanity checks.
- Prometheus alert example for lag:
  - Record: `neo_indexer_lag = clamp_max((neo_status_node_height - neo_status_latest_height), 1e6)`
  - Alert:
    ```
    alert: NeoIndexerLagHigh
    expr: neo_indexer_lag > 50
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "NEO indexer is {{ $value }} blocks behind node"
      description: "Check node RPC and indexer logs."
    ```

## Next steps
- Wire the `neo-indexer` to these RPC endpoints (compose profile `neo` starts it with `NEO_RPC_URL`/`NEO_NETWORK`/`NEO_INDEXER_DSN` defaults).
- Wire the `neo-snapshot` job to fetch state roots + storage and publish manifests.
- Point the appserver at your manifest directory via `NEO_SNAPSHOT_DIR` (defaults to `./snapshots`) so `/neo/status`, `/neo/blocks`, and `/neo/snapshots` surface indexed data to operators.
