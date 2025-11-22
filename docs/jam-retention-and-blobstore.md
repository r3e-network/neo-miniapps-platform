# JAM Retention, Cleanup, and Blob Store Options

This document proposes how to manage data lifecycle for JAM artifacts (preimages, packages, reports) and how to evolve storage beyond the current in-DB blobs.

## Goals
- Define retention defaults and cleanup mechanics for JAM data.
- Avoid unbounded growth in Postgres and memory stores.
- Outline options for external blob storage and migration path.

## Data Classes
- **Preimages**: content-addressed blobs (code/data), currently stored inline in Postgres (jam_preimages.data) or memory.
- **Packages/Reports/Attestations**: metadata and compact outputs; stored in jam_work_* tables or memory.

## Retention Policy (proposed defaults)
- `runtime.jam.retention_days` (default 30):
  - Delete packages/reports/attestations older than retention window.
  - Delete preimages that are unreferenced by any package/report within the window.
- Configurable per deploy; disabling (<=0) turns off cleanup.

## Cleanup Mechanism
- Background job (goroutine) started when JAM is enabled and store is Postgres:
  - Runs every `retention_interval` (default 24h).
  - Steps:
    1. Delete from jam_work_reports where created_at < cutoff; cascade removes attestations.
    2. Delete from jam_work_packages where created_at < cutoff.
    3. Identify preimages with refcount = 0 and created_at < cutoff; delete.
  - Log counts and duration; emit metrics.
- In-memory store: log warning that data is ephemeral; cleanup is no-op.
- Safe mode: dry-run option to log would-be deletions (config flag) before enabling hard deletes.

## Refcounting Preimages
- On package submit: increment refcount for any package-level preimage_hashes.
- On report save: increment refcount for any referenced preimages in refine_output_compact (if applicable) and traces.
- On deletion of packages/reports: decrement refcount accordingly.
- Ensure refcount never drops below zero; guard with DB constraints or safe updates.

## Blob Store Evolution
- **Phase 1 (current)**: Postgres column `jam_preimages.data` holds bytes; fine for prototypes and small blobs.
- **Phase 2 (external object store)**:
  - Add config for S3-compatible endpoint/bucket/credentials.
  - Store metadata in jam_preimages; upload bytes to object store keyed by hash.
  - On PUT: write to object store, then metadata row with storage_class = "s3" and size/hash.
  - On GET/HEAD: fetch metadata, stream from object store; fall back to DB if storage_class = "db".
  - Migration: background job to offload large preimages from DB to object store; keep metadata intact.

## Indexing
- Add/ensure indexes to support cleanup and listing:
  - `jam_work_packages(created_at)`
  - `jam_work_reports(created_at)`
  - `jam_preimages(created_at, refcount)`

## Metrics and Logging
- Metrics:
  - `jam_cleanup_runs_total`, `jam_cleanup_deleted_preimages`, `jam_cleanup_deleted_packages`, `jam_cleanup_deleted_reports`.
  - `jam_preimage_refcount_adjust_total`.
- Logs:
  - Structured entries for each cleanup run (counts, duration, cutoff).
  - Warnings on refcount anomalies.

## Open Questions
- Should retention differ per artifact type (e.g., preimages longer than packages)?
- Do we need soft-delete (archived flag) before hard delete for auditability?
- What is the acceptable max blob size before requiring external storage?
