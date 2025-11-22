# JAM Versioning & Compatibility Design

Purpose: define how JAM evolves without breaking clients, covering API versioning, feature flags, and data migrations.

## Goals
- Provide a stable surface for prototype consumers while allowing rapid iteration.
- Make breaking changes explicit and avoid silent behavioural shifts.
- Ensure DB migrations and CLI stay aligned with server capabilities.

## Versioning Strategy
- **API**: use feature flags and additive changes; avoid breaking existing endpoints. If a breaking change is unavoidable, gate it behind `runtime.jam.legacy_*` flags and announce a deprecation window.
- **Engine**: include `engine` and `engine_version` in WorkReports and attestations (already present) so reports are not replayed across engine upgrades.
- **DB Migrations**: additive migrations (`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`). Avoid destructive changes; if required, add new tables/columns and migrate data forward.
- **CLI**: detect envelope vs legacy list responses; tolerate unknown fields; expose server `jam` status to adapt behaviour.

## Feature Flags / Compatibility Toggles
- `legacy_list_response` — return raw arrays instead of envelope for list endpoints.
- Future flags:
  - `legacy_errors` — omit error codes (default off once error model is in place).
  - `legacy_reports` — include/omit certain report fields if needed.
- Flags should be documented in `/system/status jam` block so clients can branch.

## Deprecation Policy (proposed)
- Additive changes: no deprecation needed; document availability.
- Breaking changes: require a flag to opt-in for at least one release; then flip default with a release note; later remove flag after sunset period.
- Migrations: ensure zero-downtime by keeping readers/writers compatible across versions (e.g., nullable new columns, backfilled defaults).

## Compatibility Testing
- CLI tests against both legacy and new response shapes.
- Integration tests with `legacy_*` flags toggled.
- Migration tests to ensure old data remains readable after new schema.

## Status/Discovery
- `/system/status` should expose:
  - `jam.enabled`, `store`
  - `jam.legacy_list_response`
  - (future) `jam.error_codes_enabled`, `jam.report_fields`
- `slctl status` should render these to help operators know which behaviours are active.

## Implementation Steps
1) Add compatibility flags to `runtime.jam` and propagate to handlers.
2) Expose flags in `/system/status` and `slctl status`.
3) Ensure list endpoints return envelope unless `legacy_list_response` is true.
4) Keep migrations additive; avoid schema drops.
5) Add tests for legacy/new modes (list responses, error codes once added).
