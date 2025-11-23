# JAM Quickstart (CLI)

Lightweight walkthrough for uploading preimages and submitting a package to JAM via `slctl`.

## Prerequisites
- JAM enabled on the server (`runtime.jam.enabled=true`).
- API token exported as `SERVICE_LAYER_TOKEN`.
- `slctl` available (`go run ./cmd/slctl ...`).

## 1) Check JAM status
```bash
slctl jam status
```

## 2) Upload a preimage
```bash
echo "hello jam" > /tmp/preimage.txt
slctl jam preimage --file /tmp/preimage.txt
```
Note the returned hash; use it for packages.

## 3) Submit a package
```bash
slctl jam package \
  --service demo \
  --kind example \
  --params-hash <HASH_FROM_PREIMAGE>
```

## 4) List packages
```bash
slctl jam packages --status pending --limit 5
```

## 5) Fetch a report
```bash
slctl jam report --package <PACKAGE_ID>
```

## Notes
- When `runtime.jam.auth_required` is true, JAM also enforces tokens from `runtime.jam.allowed_tokens` (falls back to global API tokens).
- Rate limits and quotas come from `runtime.jam.rate_limit_per_minute`, `runtime.jam.max_preimage_bytes`, and `runtime.jam.max_pending_packages`.
- Postgres store enables persistence; memory store is ephemeral.
- Dashboard users can view JAM status (enabled/store/limits/accumulators) on the
  system overview card when `/system/status` is reachable.
