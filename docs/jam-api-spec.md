# JAM Prototype API Spec

Scope: document the current JAM prototype endpoints, payloads, and expectations for clients (slctl or custom HTTP callers). This spec tracks the implemented prototype, not the full JAM design.

## Base
- Mounted at `/jam/*` when `runtime.jam.enabled` (or `JAM_ENABLED=1`).
- Auth: same bearer token model as the rest of the API; no extra scopes yet.
- Stores: memory (default) or Postgres (`runtime.jam.store: postgres`, `runtime.jam.pg_dsn` or `DATABASE_URL`).

## Endpoints

### Preimages
- `PUT /jam/preimages/{hash}`
  - Body: raw bytes of the blob.
  - Headers: `Content-Type` optional (defaults to `application/octet-stream`).
  - Hash must be sha256 of the body; size enforced server-side when configured.
  - Responses: `201 Created` with JSON metadata `{hash,size,media_type,created_at,...}` or `400` on hash mismatch.
- `GET /jam/preimages/{hash}/meta`
  - Returns JSON metadata (hash, size, media type, created_at).

- `HEAD /jam/preimages/{hash}`
  - Returns headers: `X-Preimage-Hash`, `X-Preimage-Size`, `X-Preimage-Media-Type`; `200` if exists, `404` if missing.

- `GET /jam/preimages/{hash}`
  - Streams the blob. `Content-Type` matches stored media type.
  - `404` if missing.

### Packages
- `POST /jam/packages`
  - Body (JSON):
    ```json
    {
      "service_id": "svc-123",
      "items": [
        {
          "kind": "demo",
          "params_hash": "abc123",
          "preimage_hashes": ["...optional..."]
        }
      ],
      "preimage_hashes": ["...optional package-level hashes..."]
    }
    ```
  - Server assigns `id`, `status=pending`, timestamps, and item IDs if absent.
  - Responses: `201 Created` with the stored package, or `400` on validation error.

- `GET /jam/packages?limit=50`
  - Lists recent packages (default limit 50). Unfiltered in the prototype; filters/pagination are planned.
  - Response: `200 OK` with array of packages (envelope/pagination planned).

- `GET /jam/packages/{id}`
  - Fetch a single package by ID.
  - Response: `200 OK` with package, `404` if not found.

### Reports
- `GET /jam/packages/{id}/report`
  - Returns the refined report (if any) and attestations.
  - Response shape:
    ```json
    {
      "report": {
        "id": "rep-1",
        "package_id": "pkg-1",
        "service_id": "svc-1",
        "refine_output_hash": "deadbeef...",
        "refine_output_compact": "...base64...",
        "traces": "...optional base64...",
        "created_at": "..."
      },
      "attestations": [
        {"report_id":"rep-1","worker_id":"local","weight":1,"created_at":"...","engine":"hash-refiner","engine_version":"0.1"}
      ]
    }
    ```
  - `404` if not found.

### Processing
- `POST /jam/process`
  - Processes the next pending package (refine → attest → accumulate). Uses the in-memory/PG store and the configured engine (hash-refiner + static attestor in prototype).
  - Response: `200 {"processed": true}` when a package was processed; `204 No Content` when no pending work; `400` if processing fails.

## Status Surface
- `/system/status` includes `jam: {enabled: bool, store: "memory"|"postgres"}` so clients can discover availability.
- `slctl status` and `slctl jam status` show these fields.

## CLI (slctl) Quick Reference
- `slctl jam status` — show enablement/store.
- `slctl jam preimage --file path [--hash sha256]` — upload blob (hash auto-computed if omitted).
- `slctl jam preimage --stat --hash sha256` — HEAD metadata.
- `slctl jam package --service <id> --kind <k> --params-hash <h> [--preimages h1,h2]` — submit package.
- `slctl jam packages` — list recent packages.
- `slctl jam report --package <id>` — fetch report + attestations.
- `slctl jam process` — process next pending package.

## Known Limitations
- No filtering on list endpoints; no pagination tokens.
- No authZ/quotas/rate limits beyond bearer auth.
- Preimage size/type caps are not enforced server-side yet.
- In-memory mode is ephemeral; Postgres mode stores blobs in DB; no external object store.
- No retention/cleanup; no event stream; metrics are minimal.

## Next Steps (aligns with hardening doc)
- Add filters/pagination to package/report listing.
- Add preimage JSON metadata endpoint.
- Enforce quotas/rate limits and auth scopes on `/jam/*`.
- Add metrics/logs and retention policies.
