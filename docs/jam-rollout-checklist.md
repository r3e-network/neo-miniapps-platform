# JAM Rollout Checklist

Use this checklist to verify the JAM prototype before enabling in shared environments.

## Preflight
- [ ] `runtime.jam.enabled` set appropriately; store configured (memory vs postgres).
- [ ] Auth tokens configured; allowed list (if used) includes operators.
- [ ] Database migrations applied (jam_* tables present) when using Postgres.

## Security & Limits
- [ ] Auth required on `/jam/*` (bearer token).
- [ ] Rate limit configured (`rate_limit_per_minute`), tested 429 path.
- [ ] Quotas set (`max_preimage_bytes`, `max_pending_packages`); oversized uploads rejected.
- [ ] Secrets/tokens hashed in logs; no raw blobs logged.

## API Behavior
- [ ] Preimage endpoints: PUT/GET/HEAD working; meta endpoint (if enabled) returns JSON.
- [ ] Package submit/list/get works; filters/pagination (if enabled) return expected envelope.
- [ ] Report fetch returns report + attestations.
- [ ] Process endpoint handles empty queue (204) and errors (400) correctly.

## Observability
- [ ] Metrics exported for preimage, package submit/process, rate-limit/quota hits.
- [ ] Logs include pkg_id/service_id and token hash for auth/quota failures.
- [ ] `/system/status` shows JAM enabled/store/config fields.

## Persistence & Retention
- [ ] Postgres connectivity healthy; cleanup job configured (if retention enabled).
- [ ] Preimage refcounts sane; no orphaned blobs (manual spot-check).
- [ ] Pending count reflects reality after submits/processes.

## CLI & UX
- [ ] `slctl jam status` shows enabled/store (and limits if exposed).
- [ ] `slctl jam preimage` upload/stat/meta tested.
- [ ] `slctl jam packages` and `slctl jam reports` (if present) handle filters/envelope.
- [ ] `slctl jam process` processes a package end-to-end.

## Rollback Plan
- [ ] Feature flag off path validated (`JAM_ENABLED=0` removes endpoints).
- [ ] Memory store warning acknowledged if used (ephemeral data).

## Open Items
- [ ] Decision on retention window and cleanup cadence.
- [ ] Decision on external blob store vs in-DB blobs for large preimages.
