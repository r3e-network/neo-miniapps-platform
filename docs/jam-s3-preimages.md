# JAM External Blob Store (S3) Design

Purpose: move JAM preimage storage out of Postgres/in-memory into an S3-compatible object store, while keeping metadata and hash validation intact.

## Goals
- Store preimage bytes in an object store keyed by hash.
- Keep Postgres metadata (hash, size, media_type, created_at, uploader, storage_class).
- Preserve current API semantics (PUT/GET/HEAD/meta) and hash validation.
- Support migration from in-DB blobs to object store.

## Config
- `runtime.jam.blob_store`:
  - `enabled` (bool)
  - `endpoint` (S3-compatible URL)
  - `bucket`
  - `region`
  - `access_key`, `secret_key` (or use env/iam)
  - `force_path_style` (bool, for MinIO/compat)
- `runtime.jam.storage_class`: `"db"` (default) or `"s3"` for new uploads.

## Storage Rules
- Object key: `preimages/{hash}`.
- PUT flow:
  1. Read body; compute sha256; compare with provided hash.
  2. If storage_class == "s3":
     - Upload to S3 bucket with content-type and size.
     - Insert/UPSERT metadata row in `jam_preimages` with `storage_class="s3"`; data column optional/empty.
  3. If storage_class == "db": keep existing behaviour (store bytes in DB).
- GET/HEAD/meta flow:
  - Lookup metadata in DB.
  - If storage_class == "s3": fetch HEAD/GET from object store.
  - Else fallback to DB column.

## Migration Strategy
- Background migrator:
  - Scan DB preimages where `storage_class="db"` and `size > threshold`.
  - Upload to S3; update row `storage_class="s3"`, clear `data` column (optional).
  - Track progress and log metrics; retry on failures.
- Rollback: keep original data until migration succeeds; only clear `data` when confirmed in S3.

## Error Handling
- 500 on S3 connectivity issues; log and metric.
- Validate hash before upload; 400 on mismatch.
- Timeouts: configurable S3 client timeout.

## Security
- Use bucket policies to restrict access; server is the only writer/reader.
- Avoid logging credentials; redact endpoint when necessary.

## Testing
- Use MinIO in integration tests to simulate S3.
- Verify PUT/GET/HEAD for s3-stored preimages; migration dry-run test.
- Validate fallback to DB when storage_class="db".

## Open Questions
- Do we ever store both DB and S3 copies for redundancy?
- Should we allow per-upload override of storage class?
