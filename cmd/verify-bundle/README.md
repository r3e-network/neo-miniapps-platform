# verify-bundle

Checks that a master-key attestation bundle hash matches the expected on-chain attestation hash.

Usage:
```
go run ./cmd/verify-bundle \
  --bundle file:///path/to/bundle.json \
  --expected-hash <sha256_bundle_hex>
```

Bundle expectations:
- `pubkey`: compressed master pubkey
- `hash`: `sha256(pubkey)`
- `quote`: optional; not parsed here (this tool only checks the bundle hash and required fields)

Exit non-zero on mismatch.
