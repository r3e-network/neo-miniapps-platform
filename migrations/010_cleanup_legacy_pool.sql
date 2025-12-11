-- Cleanup legacy mixer_pool_accounts after pool_accounts adoption.
-- This is safe to run after 006_accountpool_schema has added lock columns to pool_accounts.

-- Drop legacy table if it still exists (data should already have been moved/renamed by 006 when needed).
drop table if exists public.mixer_pool_accounts;
