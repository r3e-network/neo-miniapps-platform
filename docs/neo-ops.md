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
make neo-up     # docker compose --profile neo up -d neo-mainnet neo-testnet
make neo-down   # docker compose --profile neo down --remove-orphans
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

## Next steps
- Wire the `neo-indexer` to these RPC endpoints.
- Wire the `neo-snapshot` job to fetch state roots + storage and publish manifests.
