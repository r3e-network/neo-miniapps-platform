# JAM Quotas & Rate Limits (Detailed Design)

This doc zooms in on quota/rate-limit enforcement for JAM, complementing `jam-auth-and-quotas.md` and `jam-phase1-middleware.md`.

## Objectives
- Prevent runaway usage on `/jam/*` via per-token rate limits.
- Enforce payload and queue size caps (preimages, pending packages).
- Provide consistent error signalling and operator visibility.

## Config Knobs (`runtime.jam`)
- `rate_limit_per_minute` (int): per-token bucket; 0/neg = disabled. Default 60.
- `max_preimage_bytes` (int64): reject uploads above this size. Default 10 MiB.
- `max_pending_packages` (int): cap pending packages before accept. Default 100.
- Future: `max_preimage_media_types` ([]string) to restrict MIME types.

## Enforcement Model
- **Rate limit**: in-process leaky bucket keyed by bearer token (fallback to IP if no token). Decrement on each `/jam/*` request before handler; 429 + `Retry-After` when exceeded.
- **Preimage size**: check `Content-Length` if present; stream-and-count otherwise; reject with 413.
- **Pending cap**: package submit queries current pending count; if >= cap, return 409 and do not enqueue.
- **Error payload**: `{"error":"...","code":"jam_rate_limit|jam_too_large|jam_pending_limit"}`.

## Status & Observability
- `/system/status` `jam` section: include `rate_limit_per_minute`, `max_preimage_bytes`, `max_pending_packages`.
- Metrics:
  - `jam_rate_limit_hits_total{token_hash}` (cardinality guarded by hashing/truncation).
  - `jam_quota_reject_total{reason="preimage_size|pending_cap"}`.
  - `jam_preimage_put_bytes_total`, `jam_preimage_put_total`, `jam_package_submit_total`.
- Logs: structured events on rate-limit/quota rejects with token hash, remote IP, and service_id if present.

## API Behavior Changes
- Preimage PUT: 413 on oversize; otherwise unchanged.
- Package POST: 409 when pending cap reached.
- All `/jam/*`: 429 on rate limit exceed.
- Status endpoint surfaces config for clients/ops.

## Compatibility
- When limits are disabled (defaults or zero), behavior matches current prototype.
- No schema changes to DB; pending cap uses count query in PG/memory.

## Implementation Steps
1) Add config fields + defaults; expose in status.
2) Implement rate-limit middleware on JAM mux.
3) Add size check to preimage PUT; pending-cap check to package POST.
4) Add metrics/logs; update CLI to print new status fields if present.
5) Add tests: rate-limit 429, preimage 413, pending 409 (memory + PG store).
