# JAM Multitenancy & Isolation Design

Purpose: describe how to run JAM in a multi-tenant environment with clear isolation boundaries for data, auth, and resource limits.

## Isolation Domains
- **Service-level**: each service owns its code/state/balance and its work packages/reports.
- **Account-level (optional)**: map service owners to existing accounts to reuse tokens/policies.
- **Operator-level**: admin/operator tokens can bypass per-service restrictions for support.

## Tenancy Controls
- **AuthZ (per service)**:
  - Enable `runtime.jam.authz_enabled`.
  - If `runtime.jam.owner_is_account`, require tokens associated with the owning account.
  - Delegate table (PG) or in-memory map for additional authorized tokens per service.
- **Data access**:
  - Package submit/list/report endpoints enforce service ownership/delegates.
  - Preimages optionally scoped to service (future: add `service_id` to preimage metadata and enforce).
- **Resource limits**:
  - Per-service quotas (future): max pending packages, max storage bytes, compute budgets; today use global caps and service filters.
  - Rate limits per token; tokens are per-tenant to avoid cross-tenant impact.

## Namespacing
- Key service fields:
  - `service_id` in packages/reports/attestations.
  - (Future) `service_id` in preimages to scope blobs.
- State backend: prefix state namespace per service to avoid collisions.

## Cross-tenant Leakage Risks & Mitigations
- **Listing endpoints**:
  - Ensure filters default to service scope when authz enabled; deny cross-service access.
  - Add `403` when token attempts to access other service IDs.
- **Preimages**:
  - If unscoped, any JAM token can fetch known hashes; mitigate by scoping preimages to services when authz enabled.
- **Logs/metrics**:
  - Avoid logging raw tokens; hash tokens.
  - Avoid high-cardinality labels with service_id unless sampled or bounded.

## Configuration
- `runtime.jam.authz_enabled` (bool)
- `runtime.jam.owner_is_account` (bool)
- `runtime.jam.allowed_tokens` (global operator tokens)
- `runtime.jam.service_delegates` (in-memory) or PG table for delegates
- Future: per-service quotas config store/table.

## Implementation Steps
1) Enforce service-level authz on submit/get/list/report when authz_enabled.
2) Add optional `service_id` field to preimage metadata and scope access when authz_enabled.
3) Add delegate store (PG + memory) and CLI/admin hooks to manage delegates.
4) Introduce per-service quotas (pending count, storage bytes) and rate limits keyed by service/token.
5) Update tests to cover cross-service access denials and scoped listings.
