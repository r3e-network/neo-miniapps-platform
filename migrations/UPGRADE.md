# AccountPool Migration Notes

This repo uses a shared `pool_accounts` table managed by the AccountPool (NeoAccounts) service.
Older deployments may contain legacy tables from previous service scopes; migrations keep the
final schema consistent.

## Fresh deployments
- Apply migrations in order. `003_service_persistence.sql` creates `pool_accounts` with lock columns and indexes.

## Upgrades from older deployments
- Apply `006_accountpool_schema.sql`. It:
  - Renames legacy `neovault_pool_accounts` to `pool_accounts` if present.
  - Adds missing `locked_by` and `locked_at` columns.
  - Ensures lock/retiring indexes exist.
- No data is dropped; rows are preserved.
- Apply `007_secret_permissions.sql` for per-secret service allowlists.
- Apply `008_cleanup_legacy_pool.sql` to drop any leftover `neovault_pool_accounts` table after the rename/lock-column migration.
- Apply `019_remove_neovault.sql` to remove out-of-scope legacy NeoVault/Mixer tables (if present).
- Apply `020_remove_vrf.sql` to remove legacy `vrf_requests` persistence (randomness now uses NeoCompute scripts).
- Apply `022_neoflow_schema.sql` to canonicalize NeoFlow persistence (`neoflow_triggers`, `neoflow_executions`) and drop legacy `automation_*` tables.
- Apply `023_cleanup_legacy_request_tables.sql` to drop unused legacy request tables and convert `service_requests.service_type` to TEXT.

## Verification checklist
- Table `pool_accounts` exists with columns: `id`, `address`, `created_at`, `last_used_at`, `tx_count`, `is_retiring`, `locked_by`, `locked_at`.
- Indexes `pool_accounts_locked_by_idx` and `pool_accounts_is_retiring_idx` exist.
- Optional: table `pool_account_balances` exists when multi-token balances are enabled (`011_multi_token_balances.sql`).
