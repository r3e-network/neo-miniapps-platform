# Operations Runbook (Local & Staging)

## Bring Up the Stack
- `make run` — copies `.env.example` if missing, then `docker compose up -d --build` for API (8080), dashboard (8081), marketing site (8082), Postgres (5432).
- Default auth: `Authorization: Bearer dev-token` (from `API_TOKENS`). JWT login via `/auth/login` with `admin/changeme` (from `AUTH_USERS` / `AUTH_JWT_SECRET`).
- Optional NEO nodes: `make neo-up` (profile `neo`) to start mainnet/testnet RPC nodes. Point `NEO_RPC_STATUS_URL` at the node RPC to surface lag in `/neo/status`.
- Full NEO stack: `make run-neo` to start API + dashboard + site + Postgres + `neo-indexer` + NEO nodes (profile `neo`).

## Health & Smoke
- Liveness: `curl -H "Authorization: Bearer dev-token" http://localhost:8080/livez`
- Readiness: `curl -H "Authorization: Bearer dev-token" http://localhost:8080/readyz` (returns 503 with modules list if any module is not ready). `/healthz` behaves the same as `/readyz`. `/system/status` provides the detailed modules view with readiness + timestamps.
- System status: `curl -H "Authorization: Bearer dev-token" http://localhost:8080/system/status`
- Dashboard: open `http://localhost:8081/?api=http://localhost:8080&token=dev-token&tenant=<id>` or generate via `slctl dashboard-link`.
- NEO smoke: `slctl neo status` and `slctl neo snapshots` (requires indexed data or manifests in `NEO_SNAPSHOT_DIR`).
- NEO checkpoint: `slctl neo checkpoint` for a concise latest height/node lag readout (same as `/neo/status`).
- Snapshot manifests are served from `NEO_SNAPSHOT_DIR` (compose mounts `./snapshots` into `/app/snapshots` for convenience).
- Validate manifests + bundles in one step: `slctl manifest --url http://localhost:8080/neo/snapshots/<height>`
- Inspect captured storage quickly: `slctl neo storage-summary <height>` (or `GET /neo/storage-summary/{height}`) shows per-contract KV and diff entry counts without streaming full blobs.
- Download KV bundles directly: `curl -H "Authorization: Bearer dev-token" -o block-<h>-kv.tar.gz http://localhost:8080/neo/snapshots/<h>/kv` (use `/kv-diff` for diff bundles).
- CLI download helper: `slctl neo download --height <h> [--diff] [--out file] [--sha <sha>]`.
- One-shot verify (manifest + bundles + signature): `slctl neo verify-all --manifest http://localhost:8080/neo/snapshots/<h> --download` (use `--download=false` to verify hashes without writing files).
- Convenience: `slctl neo verify-all --height <h>` auto-builds the manifest path (`/neo/snapshots/<h>`) and performs the same verification. Use `--heights 100,101,...` to batch-verify multiple manifests in one run (omit custom output paths to avoid clobbering).
- `verify-all` prints an OK/FAIL summary table and returns non-zero if any target fails.
- Dashboard snapshots: use “Verify all” in the NEO panel to hash-check KV/diff bundles and signatures; results persist per API endpoint across reloads.
- Engine modules: `curl -H "Authorization: Bearer dev-token" http://localhost:8080/system/status | jq '.modules'` to see registered components, lifecycle status, readiness, uptimes, interfaces, and any startup errors. `slctl status` also prints this list plus `modules_summary` (data/event/compute). The dashboard auto-refreshes status and shows a warning card if any module is failed/stopped or not ready.
- Engine bus fan-out: publish events/data/compute across services via `slctl bus events|data|compute ...` or the dashboard “Engine Bus Console”. See `docs/examples/bus.md` for payload shapes and smoke checks. Useful for injecting feed updates, datalink deliveries, stream frames, or invoking functions without hitting service-specific endpoints.

## Logs & Monitoring
- App logs: `make logs` (tails appserver in compose). All services: `docker compose logs -f`.
- Metrics: scrape `/metrics` on 8080. Configure Prometheus base URL in dashboard settings or via `?prom=http://localhost:9090`.

## Data & Persistence
- Postgres volume: `postgres-data` in compose; back it up before destructive changes.
- NEO node data volumes: `neo-mainnet-chain`, `neo-testnet-chain` (only when `neo` profile is used).
- To reset local data: `docker compose down -v` (destructive; removes DB and chain data).

## Shutdown / Restart
- Stop stack: `make down` (or `docker compose down --remove-orphans`).
- Restart after config changes: `make run` (rebuilds images if needed).

## Production Hardening (high level)
- Replace `dev-token`, `AUTH_JWT_SECRET`, and `SECRET_ENCRYPTION_KEY` with strong secrets.
- Enable TLS in front of the stack (reverse proxy/ingress).
- Require branch protection with `neo-smoke` as a required check (see `docs/branch-protection.md`).
- Set backups/retention for Postgres, centralize logs, and enable alerting on `/metrics` and `/healthz`.
