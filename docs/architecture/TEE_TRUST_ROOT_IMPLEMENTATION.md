# TEE Trust Root Implementation (Current Code)

This repo now boots every service from a single TEE trust root. Secrets, secure networking, signing, and confidential compute all hang off the enclave and are exposed to services only through `platform/os`.

## Foundation
- `tee/trust_root.go`: owns the enclave runtime plus Vault, SecureNetwork, Keys, Compute, Attestation.
- `platform/bootstrap/foundation.go`: single entrypoint that creates/starts the TrustRoot and hands out `ServiceOS` instances; this makes the TEE the mandatory trust root for every service.
- `cmd/server/main.go`: uses the bootstrap foundation; flags now include `--mode` (simulation|hardware), `--sealing-key`, and `--sealed-storage` for sealed, enclave-backed persistence.

## Sealed Secret Store
- `tee/trust_root.go` now wires `tee/bridge.Storage` when `StoragePath` is provided, so vault data is sealed in the enclave and persisted only as encrypted blobs.
- `platform/os/context.go` exposes secrets solely through callbacks (`Use`, `UseMultiple`) so plaintext never leaves the enclave boundary.
- `platform/os/context.go` Storage API now supports callback access (`Use`) so stored values can remain enclave-internal when desired.
- `services/base/enclave.go` adds `UseStorage` helper so enclave logic can consume stored data via callback without exporting plaintext.

## Secure Networking & Confidential Compute
- `platform/os/context.go` routes all network calls to `tee/network.Client` (TLS/auth injected inside enclave). Outbound destinations can be locked down via `--allow-hosts`/`AllowedHosts` in the TrustRoot config, and further restricted per-service via `manifest.AllowedHosts`.
- `tee/network.Client` supports certificate pinning (host -> SHA256 leaf fingerprint) via TrustRoot config `PinnedCerts`; pins are enforced inside the enclave TLS path.
- Server CLI supports `--pinned-certs host=hexsha256,...` to feed `PinnedCerts` without code changes.
- `platform/os/context.go` routes confidential workloads to `tee/compute.Engine` so critical service execution runs inside the enclave with optional secret injection.
- `platform/supabase.Client` is TEE-friendly: API keys come from Secrets, allowed hosts are enforced, TLS/auth are inside the enclave; services load sealed Supabase config via `BaseEnclave.SupabaseClient(ctx, \"<svc>/supabase\")`.

## How services stay TEE-rooted
- Each service declares a manifest with required capabilities; `platform/bootstrap` builds a `ServiceOS` bound to the TrustRoot, enforcing that secrets/network/keys/compute all flow through the enclave-backed APIs.
- Base service/enclave helpers (`services/base`) already gate operations on the ServiceOS and enclave readiness, keeping critical paths inside TEE.
- Sealed configs: services with `CapStorage` perform best-effort enclave-side hydration of `service/config` via `UseStorage`; capability-denied errors are ignored for dev flows, and plaintext never leaves the enclave during hydration.
- `services/base/enclave.LoadConfigJSON` provides a common pattern for sealed JSON configs (enclave-only decode, optional if key absent).
