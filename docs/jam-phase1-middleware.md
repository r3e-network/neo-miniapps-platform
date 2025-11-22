# JAM Phase 1: Auth & Rate-Limit Middleware (Implementation Notes)

Purpose: implement the first hardening slice for JAM endpoints—auth checks, token allowlist, per-token rate limiting, and surface config in status—without changing existing data models.

## Scope
- Apply middleware to `/jam/*` only.
- Config-driven behaviour; defaults safe but non-breaking when JAM disabled.

## Config (runtime.jam)
- `enabled` (existing)
- `auth_required` (bool; default true when enabled)
- `allowed_tokens` ([]string; optional allowlist; fallback to global tokens)
- `rate_limit_per_minute` (int; default 60; <=0 disables)
- (Later phases: size/quotas/retention)

## Middleware Behaviour
1) **Auth**:
   - Require bearer token when `auth_required` is true.
   - If `allowed_tokens` non-empty, token must be in allowlist; else use global token list.
   - 401 on missing; 403 on disallowed.
2) **Rate Limit**:
   - Leaky/bucket per token (fallback IP) stored in-process.
   - Allow `rate_limit_per_minute` tokens per minute.
   - 429 with `Retry-After` when exceeded.

Error payload: `{"error":"message","code":"jam_auth|jam_rate_limit"}`.

## Status Surface
- Extend `/system/status` `jam` section with `enabled`, `store`, `rate_limit_per_minute`, `auth_required`.
- `slctl status`/`slctl jam status` should render new fields.

## Implementation Steps
- Add config fields to `runtime.jam` structs; bridge into `app.RuntimeConfig`.
- Build middleware in `internal/app/jam/http.go`:
  - Extract token from `Authorization: Bearer`.
  - Validate against allowlist/global tokens.
  - Rate-limit bucket map with mutex; configurable limit; include `Retry-After`.
- Wire middleware into JAM handler before route dispatch.
- Update status response to include new fields.
- Add tests:
  - 401/403 cases (missing token, not allowed).
  - 429 rate limit using small limit.
  - Status includes new fields.
  - Ensure disabled JAM bypasses middleware.
- CLI: `slctl jam status` prints auth/rate-limit fields (already prints store/enabled; extend).

## Non-Goals (Phase 1)
- No request filters/pagination yet.
- No preimage size caps or pending quotas.
- No persistence/index changes.
