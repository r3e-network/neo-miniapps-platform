# JAM Operator Runbook (Draft)

Purpose: quick-reference for enabling, observing, throttling, and disabling JAM in shared environments.

## Feature Flags / Config
- `runtime.jam.enabled`: enable/disable JAM endpoints.
- `runtime.jam.store`: memory | postgres.
- `runtime.jam.auth_required`: require bearer token for `/jam/*`.
- `runtime.jam.allowed_tokens`: optional allowlist; otherwise use global tokens.
- `runtime.jam.rate_limit_per_minute`: per-token rate limit (0=off).
- `runtime.jam.max_preimage_bytes`, `runtime.jam.max_pending_packages`: quota knobs.
- `runtime.jam.retention_days`: cleanup window (PG only).

## Startup Checklist
- Ensure DB reachable and `jam_*` tables migrated when using Postgres.
- Set API tokens and (optional) JAM allowed tokens.
- Confirm logs show JAM enabled/store; check `/system/status` `jam` block.

## Common Operations
- **Upload preimage**: `slctl jam preimage --file <path> [--hash]`.
- **Inspect preimage**: `slctl jam preimage --stat --hash <h>` (HEAD); `--meta` when JSON meta is available.
- **Submit package**: `slctl jam package --service <id> --kind <k> --params-hash <h>`.
- **List packages**: `slctl jam packages [--status ... --service ... --limit ...]`.
- **Process next**: `slctl jam process`.
- **Fetch report**: `slctl jam report --package <id>`.
- **Check status**: `slctl jam status` (enabled/store/limits).

## Observability
- Metrics: preimage put/get, package submit/process, rate-limit/quota hits (see jam-observability).
- Logs: watch for rate-limit/quota rejections and process failures; token hashes only.
- Status: `/system/status` shows JAM config; consider adding `/jam/healthz` once implemented.

## Rate/Quota Tuning
- To throttle: lower `rate_limit_per_minute`; reduce `max_pending_packages`.
- To pause uploads: set `max_preimage_bytes=1` and communicate downtime (temporary workaround) or disable JAM.
- To disable quickly: set `JAM_ENABLED=0` and restart; `/jam/*` will unmount.

## Troubleshooting
- 401/403: token missing or not in allowlist (when auth_required true).
- 429: rate limit hit; adjust limit or tokens; check `Retry-After`.
- 413: preimage too large; raise `max_preimage_bytes` or shrink payload.
- 409 on package submit: pending queue full; raise `max_pending_packages` or process backlog.
- DB errors: confirm `DATABASE_URL`/`JAM_PG_DSN`; rerun migrations.

## Rollback
- To revert JAM exposure: disable flag and redeploy; endpoints unmounted.
- Data in Postgres persists; memory mode data is lost on restart.

## Open Items
- Formal health endpoint for JAM store/cleanup runner.
- Admin endpoints for disputes and delegate token management (if authz enabled).
