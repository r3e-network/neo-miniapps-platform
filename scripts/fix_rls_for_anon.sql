-- =============================================================================
-- Fix RLS Policies for Development (Allow anon role full access)
-- =============================================================================
-- This is a DEVELOPMENT WORKAROUND when the correct service_role JWT key is not available.
-- In production, use the proper service_role key from Supabase dashboard.
-- =============================================================================

-- Add policies for anon role to perform all operations
-- These policies allow the anon key to bypass RLS for development purposes

-- Pool Accounts
DROP POLICY IF EXISTS anon_all ON pool_accounts;
CREATE POLICY anon_all ON pool_accounts FOR ALL TO anon USING (true) WITH CHECK (true);

-- Account Balances
DROP POLICY IF EXISTS anon_all ON account_balances;
CREATE POLICY anon_all ON account_balances FOR ALL TO anon USING (true) WITH CHECK (true);

-- Chain Transactions
DROP POLICY IF EXISTS anon_all ON chain_txs;
CREATE POLICY anon_all ON chain_txs FOR ALL TO anon USING (true) WITH CHECK (true);

-- Contract Events
DROP POLICY IF EXISTS anon_all ON contract_events;
CREATE POLICY anon_all ON contract_events FOR ALL TO anon USING (true) WITH CHECK (true);

-- Simulation Transactions
DROP POLICY IF EXISTS anon_all ON simulation_txs;
CREATE POLICY anon_all ON simulation_txs FOR ALL TO anon USING (true) WITH CHECK (true);

-- Grant all permissions to anon role
GRANT ALL ON pool_accounts TO anon;
GRANT ALL ON account_balances TO anon;
GRANT ALL ON chain_txs TO anon;
GRANT ALL ON contract_events TO anon;
GRANT ALL ON simulation_txs TO anon;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO anon;

-- Verify policies were created
SELECT schemaname, tablename, policyname, permissive, roles, cmd, qual
FROM pg_policies
WHERE tablename IN ('pool_accounts', 'account_balances', 'chain_txs', 'contract_events', 'simulation_txs')
ORDER BY tablename, policyname;
