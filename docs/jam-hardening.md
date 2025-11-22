# JAM Prototype Hardening Plan

Scope: move the current JAM prototype (work packages, reports, preimages, JAM HTTP/CLI) toward production readiness with auth, quotas, observability, and safer defaults.

## Goals
- Lock down access (authN/authZ) and add per-tenant quotas/rate limits.
- Improve API ergonomics (listing/filtering, JSON metadata).
- Ensure persistence is durable and lifecycle-managed (preimages, packages, reports).
- Add observability and operational hooks for operators and SREs.

## Current Prototype (recap)
- Endpoints under `/jam/*`: upload/get preimages; submit packages; process next; fetch report.
- Stores: in-memory (default) and Postgres (jam_* tables) selected by `runtime.jam`.
- CLI: `slctl jam preimage|package|packages|process|report|status`.
- Status surface: `/system/status` exposes JAM enablement/store.

## Gaps / Risks
- No authZ scoping: any bearer token can hit `/jam/*`.
- No per-tenant quotas or rate limits; in-memory mode is ephemeral and silent.
- No listing/filtering of reports/attestations; no JSON stat for preimages (only HEAD/GET).
- No audit/event stream; no metrics specific to JAM.
- No TTL/cleanup on packages/reports/preimages.
- No request/response schemas in `requirements.md`.

## Design Proposals

### AuthN/AuthZ
- Reuse existing bearer tokens; add optional JAM scope check (e.g., `JAM_ENABLED` + `JAM_ALLOWED_TOKENS`, or reuse account-bound tokens).
- For multitenancy: associate `account_id` or `service_id` ownership with packages; enforce token claims.
- Return 403 when JAM is enabled but token lacks scope.

### Quotas and Rate Limits
- Per-service quotas: max packages per minute, max bytes per preimage, max outstanding pending packages.
- Global defaults, overridable per service via config table or env.
- HTTP 429 with `Retry-After` on rate limit hit; 413 on payload too large.

### API Enhancements
- **Preimages**: add `GET /jam/preimages/{hash}/meta` JSON stat; keep HEAD/GET.
- **Packages**: list with filters (`status`, `service_id`, `limit`, `offset`); add `GET /jam/packages/{id}/report` (already) plus `GET /jam/packages/{id}/attestations`.
- **Reports**: `GET /jam/reports?service_id=...&status=...` for operator views.
- Error model: consistent JSON `{error, code}`.

### Persistence & Lifecycle
- Add TTL/cleanup for old packages/reports/preimages (soft-delete with archived flag + retention job).
- Enforce preimage size cap and supported media types.
- Optional external blob store later (S3-compatible) keyed by hash; DB holds metadata only.

### Observability
- Metrics: counters/latency for preimage put/get/head; package submit/list; process success/failure; attestation counts; quota/rate-limit hits.
- Logging: structured logs for submit/process/report apply with package_id, service_id, store type.
- Tracing hooks (trace id propagation) for refine/attest/accumulate calls.

### CLI Improvements
- `slctl jam preimage --stat --hash <h>` (added); extend with `--meta` to show JSON.
- `slctl jam packages --status pending|applied --service <id> --limit N`.
- `slctl jam report --package <id>` (existing) plus `slctl jam status` (existing).

### Config & Defaults
- `runtime.jam` gains:
  - `auth_required` (bool), `allowed_tokens` (optional list)
  - `rate_limit_per_minute` (default), `max_preimage_bytes`, `max_pending_packages`
  - `retention_days` for cleanup jobs
- Default remains disabled; memory store logs “ephemeral” warning; rate limits default conservative in memory mode.

### Rollout Plan
1) Add schemas to `requirements.md` for JAM endpoints (request/response, error).
2) Add authZ scope check + rate limiting middleware on `/jam/*`.
3) Implement package/report listing filters; preimage meta JSON endpoint.
4) Add metrics/logging; expose in `/metrics` and logs.
5) Add retention/cleanup job and size caps.
6) Enhance CLI (`slctl jam packages --status ...`, `--service ...`, `preimage --meta`).
7) Add migration for any new columns/indexes (e.g., status indexes for listing).

## Open Questions
- Should JAM be tied to an account/service owner, or global operators only?
- Do we need slashing/incentives for attestations in this phase, or just logging?
- Is S3/object storage required for preimages soon, or is Postgres acceptable with caps?
- How long should we retain applied packages/reports by default (days vs. weeks)?
