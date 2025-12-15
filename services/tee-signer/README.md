# TEE Signer

Global gRPC signing service intended to run inside a MarbleRun-managed TEE.

## API

- gRPC server listens on `:9090`
- Proto: `api/proto/signer.proto`
- Method: `signer.TEESigner/Sign`
  - Input: `tx_hash` (hex string, 32 bytes, optional `0x` prefix)
  - Optional: `key_version` (sign using a specific active/deprecated key during overlap)
  - Output: `signature` (64-byte `r||s`), `key_version`
- Internal HTTP endpoint (optional):
  - Enabled when `INTERNAL_HTTP_PORT` is set
  - `POST /internal/rotate-key` triggers key rotation and returns the rotation result

## Environment

- `MASTER_KEY_SEED` (required): master seed used for HKDF key derivation
- `SUPABASE_URL` / `SUPABASE_SERVICE_KEY` (recommended): audit logging target
- `TEE_SIGNER_AUDIT_TABLE` (optional): defaults to `tee_signer_audit`
- `PORT` (optional): defaults to `9090`
- `INTERNAL_HTTP_PORT` (optional): enables internal HTTP server for key rotation

## Notes

- Per-client rate limiting is enforced at `100 req/s` using the mTLS client
  certificate common name (CN).
- Audit logging is asynchronous and non-blocking; when the audit queue is full,
  new audit records are dropped to avoid impacting signing latency.
- Key versions use the format `v{unix_timestamp}` and rotate with a 7-day overlap
  window (active + deprecated are both valid during overlap).
