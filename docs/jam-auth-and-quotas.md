# JAM Auth, Rate Limits, and Quotas (Detailed Design)

This document details how to secure and meter the JAM prototype endpoints.

## Objectives
- Require explicit authorization for `/jam/*`.
- Enforce per-token rate limits and per-service quotas.
- Provide clear error responses and operator observability.
- Keep backward compatibility when JAM remains disabled.

## Config Surface
- `runtime.jam.enabled` (existing)
- `runtime.jam.auth_required` (bool, default true when enabled)
- `runtime.jam.allowed_tokens` ([]string, optional allowlist; fallback to global tokens when empty)
- `runtime.jam.rate_limit_per_minute` (int, default 60)
- `runtime.jam.max_preimage_bytes` (int64, default 10 MiB)
- `runtime.jam.max_pending_packages` (int, default 100)
- `runtime.jam.retention_days` (int, default 30)
- `runtime.jam.legacy_list_response` (bool, default false; controls paginated envelope vs raw array)

Expose effective values in `/system/status` under `jam`.

## Auth Flow
- If `auth_required` is true:
  - Require bearer token.
  - If `allowed_tokens` non-empty, token must match allowlist.
  - Else reuse global API tokens.
- Unauthorized: 401 (missing) / 403 (not allowed) with JSON `{"error":"...","code":"jam_auth"}`.

## Rate Limiting
- Token-scoped leaky bucket in memory (per-process). Keyed by bearer token; fall back to IP if no token.
- Configured by `rate_limit_per_minute`.
- Response on exceed: 429 with `Retry-After` header and `{"error":"rate limit exceeded","code":"jam_rate_limit"}`.
- No-op when limit <= 0.

## Quotas
- `max_preimage_bytes`: reject uploads > cap with 413 and `{"error":"preimage too large","code":"jam_too_large"}`.
- `max_pending_packages`: on package submit, if pending count >= cap, return 409 `{"error":"pending limit exceeded","code":"jam_pending_limit"}`.
- Future: per-service quotas (storage/compute) can reuse service records once available.

## API Impacts
- Add `GET /jam/preimages/{hash}/meta` JSON metadata; HEAD/GET unchanged.
- Add filters/pagination to `GET /jam/packages` (status, service_id, limit, offset) and respond with `{items, next_offset}` unless legacy flag set.
- Add `GET /jam/reports` (filters: service_id, status, limit, offset).
- Error payloads standardized to `{error, code}` across `/jam/*`.

## Observability
- Metrics: rate-limit hits, quota rejects, preimage put/get size counters, package submit/process counts.
- Logs: auth failures, rate-limit hits, quota rejects, package submit/process with token (hashed) and service_id.
- Status: include jam config and store.

## Backward Compatibility
- When `enabled=false`, nothing mounted.
- When `auth_required=false`, behavior matches current prototype (no extra checks).
- Legacy list response preserved with `legacy_list_response=true`.

## Rollout Steps
1) Add config fields and status exposure.
2) Add middleware for auth + rate limit; wire into JAM handler.
3) Enforce size/pending caps; add preimage meta endpoint; standardize errors.
4) Add filters/pagination to list endpoints; legacy flag for old clients.
5) Add metrics/logging; update docs and CLI as needed.
