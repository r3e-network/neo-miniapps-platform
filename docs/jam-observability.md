# JAM Observability Plan

## Goals
- Give operators visibility into JAM throughput, failures, and resource usage.
- Make debugging refine/attest/apply paths and quota/rate-limit rejections straightforward.
- Keep Prometheus cardinality manageable.

## Metrics (Prometheus)
- **Preimages**
  - `jam_preimage_put_total`, `jam_preimage_get_total`, `jam_preimage_head_total`
  - `jam_preimage_bytes_total` (counter)
  - `jam_preimage_put_duration_seconds` (histogram)
  - `jam_preimage_quota_reject_total{reason="too_large|unsupported_media"}`
- **Packages/Processing**
  - `jam_package_submit_total`, `jam_package_process_total`, `jam_package_process_fail_total`
  - `jam_package_pending_gauge`
  - `jam_package_process_duration_seconds` (histogram)
- **Attestations/Reports**
  - `jam_attestations_total`, `jam_reports_total`
- **Rate/Quotas**
  - `jam_rate_limit_hits_total`, `jam_pending_cap_hits_total`
- **Cleanup (if enabled)**
  - `jam_cleanup_runs_total`
  - `jam_cleanup_deleted_packages_total`, `jam_cleanup_deleted_reports_total`, `jam_cleanup_deleted_preimages_total`
- Labels: prefer `store` (memory/postgres); avoid unbounded `service_id` unless sampled.

## Logging
- Structured events for:
  - Package submit (`pkg_id`, `service_id`, `store`, `token_hash`, `pending_count`)
  - Process success/fail (`pkg_id`, `service_id`, `store`, `duration_ms`, `error`)
  - Preimage upload (`hash`, `size`, `media_type`, `store`, `token_hash`, `quota_hit`)
  - Rate-limit/quota rejects (`token_hash`, `reason`, `limit`)
  - Cleanup runs (`cutoff`, `deleted_counts`, `duration_ms`)
- Hash tokens before logging; never log raw blob data.

## Tracing
- Optional OpenTelemetry spans:
  - `jam.preimage.put/get/head`
  - `jam.package.submit/list/get`
  - `jam.process` with child spans `refine`, `attest`, `accumulate`
- Propagate trace IDs in logs; pass through `traceparent` header.

## Status/Health
- `/system/status` JAM block should include: `enabled`, `store`, `rate_limit_per_minute`, `max_preimage_bytes`, `max_pending_packages`, `cleanup_enabled`.
- Optional `/jam/healthz`: 200 when store reachable (PG ping) and cleanup runner healthy.

## Dashboards/Alerts
- Dashboards:
  - Request rates/error rates by endpoint
  - Pending package gauge over time
  - Preimage upload sizes/latencies
  - Rate-limit/quota rejects over time
- Alerts:
  - High process failure rate (>N% over 5m)
  - Sustained rate-limit hits
  - Cleanup failures
  - Pending queue above threshold

## Implementation Steps
1) Add metrics increments in JAM handler/refiner/accumulator/stores; register with existing Prometheus handler.
2) Add structured logging with token hashing helpers.
3) Wire optional OTEL spans (config flag).
4) Extend `/system/status` JAM section with observability fields.
5) Add tests ensuring metrics bump on happy path and quota/rate-limit rejects.
