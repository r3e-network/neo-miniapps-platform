-- =============================================================================
-- Minimal Migration for Neo Simulation Service
-- Creates only the essential tables needed for neoaccounts and neosimulation
-- =============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Users Table (required for foreign keys)
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    address VARCHAR(64) UNIQUE,
    email VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_address ON users(address);

-- =============================================================================
-- Pool Accounts (required for neoaccounts service)
-- =============================================================================
CREATE TABLE IF NOT EXISTS pool_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    address TEXT NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tx_count BIGINT NOT NULL DEFAULT 0,
    is_retiring BOOLEAN NOT NULL DEFAULT FALSE,
    locked_by TEXT,
    locked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS pool_accounts_locked_by_idx ON pool_accounts (locked_by);
CREATE INDEX IF NOT EXISTS pool_accounts_is_retiring_idx ON pool_accounts (is_retiring);

-- =============================================================================
-- Account Balances (multi-token support)
-- =============================================================================
CREATE TABLE IF NOT EXISTS account_balances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES pool_accounts(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, token)
);

CREATE INDEX IF NOT EXISTS account_balances_account_idx ON account_balances(account_id);
CREATE INDEX IF NOT EXISTS account_balances_token_idx ON account_balances(token);

-- =============================================================================
-- Chain Transactions (audit trail)
-- =============================================================================
CREATE TABLE IF NOT EXISTS chain_txs (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    service TEXT NOT NULL,
    tx_hash TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS chain_txs_request_idx ON chain_txs(request_id);
CREATE INDEX IF NOT EXISTS chain_txs_service_idx ON chain_txs(service);
CREATE INDEX IF NOT EXISTS chain_txs_status_idx ON chain_txs(status);

-- =============================================================================
-- Contract Events (for MiniApp event tracking)
-- =============================================================================
CREATE TABLE IF NOT EXISTS contract_events (
    id BIGSERIAL PRIMARY KEY,
    app_id TEXT NOT NULL,
    event_name TEXT NOT NULL,
    tx_hash TEXT,
    block_number BIGINT,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS contract_events_app_idx ON contract_events(app_id);
CREATE INDEX IF NOT EXISTS contract_events_event_idx ON contract_events(event_name);

-- =============================================================================
-- Simulation Transactions (for neosimulation service)
-- =============================================================================
CREATE TABLE IF NOT EXISTS simulation_txs (
    id BIGSERIAL PRIMARY KEY,
    app_id TEXT NOT NULL,
    account_address TEXT NOT NULL,
    tx_type TEXT NOT NULL,
    amount BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'simulated',
    tx_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS simulation_txs_app_idx ON simulation_txs(app_id);
CREATE INDEX IF NOT EXISTS simulation_txs_created_idx ON simulation_txs(created_at DESC);
CREATE INDEX IF NOT EXISTS simulation_txs_tx_hash_idx ON simulation_txs(tx_hash);

-- =============================================================================
-- Row Level Security
-- =============================================================================
ALTER TABLE pool_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE account_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE chain_txs ENABLE ROW LEVEL SECURITY;
ALTER TABLE contract_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_txs ENABLE ROW LEVEL SECURITY;

-- Service role policies (allow all for service role)
DO $$
BEGIN
    -- Drop existing policies if they exist
    DROP POLICY IF EXISTS service_all ON pool_accounts;
    DROP POLICY IF EXISTS service_all ON account_balances;
    DROP POLICY IF EXISTS service_all ON chain_txs;
    DROP POLICY IF EXISTS service_all ON contract_events;
    DROP POLICY IF EXISTS service_all ON simulation_txs;
EXCEPTION WHEN undefined_object THEN
    NULL;
END $$;

CREATE POLICY service_all ON pool_accounts FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON account_balances FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON chain_txs FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON contract_events FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON simulation_txs FOR ALL TO service_role USING (true);

-- Grant permissions to authenticated and anon roles for read access
GRANT SELECT ON pool_accounts TO authenticated, anon;
GRANT SELECT ON account_balances TO authenticated, anon;
GRANT SELECT ON chain_txs TO authenticated, anon;
GRANT SELECT ON contract_events TO authenticated, anon;
GRANT SELECT ON simulation_txs TO authenticated, anon;

-- Grant all permissions to service_role
GRANT ALL ON pool_accounts TO service_role;
GRANT ALL ON account_balances TO service_role;
GRANT ALL ON chain_txs TO service_role;
GRANT ALL ON contract_events TO service_role;
GRANT ALL ON simulation_txs TO service_role;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO service_role;
