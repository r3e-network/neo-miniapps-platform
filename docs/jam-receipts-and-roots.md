# JAM Receipts and Accumulator Roots

## Overview
Receipts capture inclusion of packages or reports in the per-service accumulator. Each receipt includes:
- `hash`: package ID or report hash
- `service_id`: JAM service
- `entry_type`: `package` or `report`
- `seq`: monotonic sequence per service
- `prev_root` / `new_root`: accumulator roots
- `status`: applied status
- `processed_at`: timestamp
- `metadata_hash`: hash of canonical payload

Accumulator roots track the head of each service’s chain and can be listed via status when accumulators are enabled.

## API
- `GET /jam/status?service_id=<id>`: returns `accumulator_root` for the service; without service param, `accumulator_roots` (all) are returned when enabled.
- `/system/status`: when accumulators are enabled, includes `jam.accumulator_roots` alongside config values.
- Fields on `/system/status` under `jam`: `accumulators_enabled`, `accumulator_hash`, `accumulator_roots` (array of `{service_id, seq, root, updated_at}`).
- `GET /jam/receipts/{hash}`: fetch a specific receipt (auth + rate limit).
- `GET /jam/receipts?service_id=&limit=&offset=`: list receipts, optionally filtered by service; returns `items` and `next_offset`.
- Package/report endpoints: `include_receipt=true` returns receipts without mutating the accumulator.

## CLI (`slctl`)
- `slctl jam status --service <id>`: shows accumulator hash and root.
- `slctl status`: prints accumulator roots when provided by `/system/status`.
- `slctl jam receipt --hash <hash>`: fetch a receipt.
- `slctl jam receipts --service <id> --limit 50 --offset 0`: list receipts with pagination.
- `slctl jam packages --include-receipt`, `slctl jam reports --include-receipt`: include receipts in responses.
- `--table` flag: `slctl jam packages|reports|receipts` renders concise rows instead of raw JSON envelopes.

## Notes
- Accumulators are gated by config (`JAM_ACCUMULATORS_ENABLED`, `JAM_ACCUMULATOR_HASH`).
- Receipts are created at ingest/process time; get/list calls do not create new receipts.
- Listing endpoints are paginated and auth/rate-limited like other JAM routes.
