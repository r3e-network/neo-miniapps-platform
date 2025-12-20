# MiniApp Manifest Specification (Neo N3)

This repository is evolving into a **Neo N3 MiniApp Platform**. MiniApps are loaded by a host application (Next.js) and interact with Neo N3 only through the platform SDK + attested services.

## Goals

- **Payments/settlement only GAS** (no other payment assets).
- **Governance only NEO** (no bNEO support).
- **No direct transaction construction in MiniApps**: all sensitive actions flow through `SDK → Edge → TEE → Chain`.
- **Deterministic permissions**: each MiniApp declares capabilities; runtime enforces allowlists and limits.

## Manifest File

- Format: JSON
- Distribution: hosted with the MiniApp bundle (CDN) and registered on-chain via `AppRegistry` (manifest hash stored on-chain).

## Manifest Hashing (AppRegistry)

The platform stores a `manifest_hash` (32-byte SHA-256) on-chain in `AppRegistry`.

In this repo, the hash is computed by **Supabase Edge** to avoid client-side
inconsistency. The canonical algorithm is implemented in:

- `platform/edge/functions/_shared/manifest.ts`

High level:

1. **Canonicalize** known fields:
   - trim `app_id`, `entry_url`, `name`, `version`
   - normalize `developer_pubkey` as lowercase hex (strip leading `0x`)
   - normalize lists as sets:
     - `assets_allowed`: uppercase + sort + unique
     - `governance_assets_allowed`: uppercase + sort + unique
     - `permissions`: validated + canonicalized deterministically
     - `sandbox_flags`: lowercase + sort + unique
     - `contracts_needed`: trim + sort + unique
   - normalize objects:
     - `limits`: normalize values to trimmed strings (for hashing stability)
2. **Stable JSON** encode:
   - recursively sort all object keys lexicographically
   - omit `undefined` values
3. **Hash**:
   - `sha256(utf8(stable_json))`
   - represented as lowercase hex (no `0x`) for `ByteArray` parameters.

### Example

```json
{
  "app_id": "your-app-id",
  "entry_url": "https://cdn.miniapps.com/apps/neo-game/index.html",
  "name": "Neo MiniApp",
  "version": "1.0.0",
  "developer_pubkey": "0x...",
  "permissions": {
    "wallet": ["read-address"],
    "payments": true,
    "governance": false,
    "rng": true,
    "datafeed": true,
    "storage": ["kv"]
  },
  "assets_allowed": ["GAS"],
  "governance_assets_allowed": ["NEO"],
  "limits": {
    "max_gas_per_tx": "5",
    "daily_gas_cap_per_user": "20",
    "governance_cap": "100"
  },
  "contracts_needed": [
    "PaymentHub",
    "RandomnessLog",
    "PriceFeed"
  ],
  "sandbox_flags": ["no-eval", "strict-csp"],
  "attestation_required": true
}
```

## Field Definitions

### Identity

- `app_id` (string, required): globally unique identifier.
- `name` (string, required): display name.
- `version` (string, required): semver.
- `developer_pubkey` (string, required): developer signing key (hex).

### Runtime

- `entry_url` (string, required): URL to the MiniApp entry (Module Federation or `iframe`).
- `sandbox_flags` (array of strings): e.g. `no-eval`, `strict-csp`.
- `attestation_required` (bool): host must enforce enclave attestation for sensitive services.

`entry_url` supports two modes:

- `https://...` for iframe-hosted MiniApps.
- `mf://<remote>?app=<app_id>` for Module Federation built-ins. The host resolves
  `<remote>` via `NEXT_PUBLIC_MF_REMOTES` and loads `builtin/App` without an iframe.

### Permissions

- `permissions.wallet`: allowed wallet reads (only `read-address`; no signing permissions here).
- `permissions.payments`: enables GAS payments via `PaymentHub`.
- `permissions.governance`: enables NEO governance calls via `Governance`.
- `permissions.rng`: enables VRF/RNG requests via the TEE services.
- `permissions.datafeed`: enables reading/subscribing to price feeds.
- `permissions.storage`: storage scopes (e.g. `kv`).

If you use the shorthand array form (`"permissions": ["wallet", "payments"]`),
the `wallet` entry is treated as `["read-address"]`.

### Allowlisted Assets

- `assets_allowed`: must contain only `["GAS"]`.
- `governance_assets_allowed`: must contain only `["NEO"]`.

### Limits

Suggested strings to avoid floating point ambiguity:

- `limits.max_gas_per_tx` (string): per-tx cap in GAS.
- `limits.daily_gas_cap_per_user` (string): per-user/day cap in GAS.
- `limits.governance_cap` (string): per-user governance cap (units defined by contract policy).

### Contracts

- `contracts_needed`: symbolic names resolved by the host using network config (testnet/mainnet).

## Validation Rules (enforced by Host/Edge)

- `assets_allowed` must be exactly `["GAS"]`.
- `governance_assets_allowed` must be exactly `["NEO"]`.
- `entry_url` must use `https://` in production unless it uses the `mf://` scheme.
- `permissions` must be a strict subset of supported platform permissions.
- `limits` must be within platform policy bounds (global caps set by governance).
- `limits.daily_gas_cap_per_user` and `limits.governance_cap` are enforced by the gateway
  via `miniapp_usage_bump(...)` (daily usage tracking).
