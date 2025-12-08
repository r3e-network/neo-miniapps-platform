-- =============================================================================
-- Neo Service Layer - Initial Database Schema
-- MarbleRun + EGo + Supabase Architecture
-- =============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Users & Authentication
-- =============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    address VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_users_address ON users(address);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    prefix VARCHAR(8) NOT NULL,
    scopes TEXT[] DEFAULT '{}',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used TIMESTAMPTZ,
    revoked BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(prefix);

-- =============================================================================
-- Secrets Management
-- =============================================================================

CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_secrets_user ON secrets(user_id);

-- =============================================================================
-- Service Requests
-- =============================================================================

CREATE TYPE service_type AS ENUM (
    'oracle', 'vrf', 'mixer', 'secrets', 'datafeeds',
    'gasbank', 'automation', 'confidential', 'accounts',
    'ccip', 'datalink', 'datastreams', 'dta', 'cre'
);

CREATE TYPE request_status AS ENUM (
    'pending', 'processing', 'completed', 'failed', 'timeout'
);

CREATE TABLE service_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_type service_type NOT NULL,
    status request_status DEFAULT 'pending',
    payload JSONB NOT NULL,
    result JSONB,
    error TEXT,
    gas_used BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_requests_user ON service_requests(user_id);
CREATE INDEX idx_requests_status ON service_requests(status);
CREATE INDEX idx_requests_service ON service_requests(service_type);
CREATE INDEX idx_requests_created ON service_requests(created_at DESC);

-- =============================================================================
-- Price Feeds (DataFeeds Service)
-- =============================================================================

CREATE TABLE price_feeds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    feed_id VARCHAR(64) NOT NULL,
    pair VARCHAR(32) NOT NULL,
    price BIGINT NOT NULL,
    decimals SMALLINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    sources TEXT[] DEFAULT '{}',
    signature BYTEA,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_feeds_feed_id ON price_feeds(feed_id);
CREATE INDEX idx_feeds_pair ON price_feeds(pair);
CREATE INDEX idx_feeds_timestamp ON price_feeds(timestamp DESC);

-- Latest price view
CREATE VIEW latest_prices AS
SELECT DISTINCT ON (feed_id) *
FROM price_feeds
ORDER BY feed_id, timestamp DESC;

-- =============================================================================
-- Gas Bank
-- =============================================================================

CREATE TABLE gasbank_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance BIGINT DEFAULT 0,
    reserved BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE gasbank_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES gasbank_accounts(id) ON DELETE CASCADE,
    tx_type VARCHAR(32) NOT NULL,
    amount BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reference_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_gasbank_tx_account ON gasbank_transactions(account_id);
CREATE INDEX idx_gasbank_tx_created ON gasbank_transactions(created_at DESC);

-- =============================================================================
-- Automation Triggers
-- =============================================================================

CREATE TYPE trigger_type AS ENUM (
    'cron', 'condition', 'event', 'price_threshold'
);

CREATE TABLE automation_triggers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    trigger_type trigger_type NOT NULL,
    schedule VARCHAR(255),
    condition JSONB,
    action JSONB NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    last_execution TIMESTAMPTZ,
    next_execution TIMESTAMPTZ,
    execution_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_triggers_user ON automation_triggers(user_id);
CREATE INDEX idx_triggers_enabled ON automation_triggers(enabled) WHERE enabled = TRUE;
CREATE INDEX idx_triggers_next ON automation_triggers(next_execution) WHERE enabled = TRUE;

CREATE TABLE automation_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trigger_id UUID NOT NULL REFERENCES automation_triggers(id) ON DELETE CASCADE,
    status request_status NOT NULL,
    result JSONB,
    error TEXT,
    gas_used BIGINT DEFAULT 0,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_executions_trigger ON automation_executions(trigger_id);

-- =============================================================================
-- VRF Requests
-- =============================================================================

CREATE TABLE vrf_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seed BYTEA NOT NULL,
    num_words INTEGER DEFAULT 1,
    callback_address VARCHAR(64),
    status request_status DEFAULT 'pending',
    random_words BYTEA[],
    proof BYTEA,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    fulfilled_at TIMESTAMPTZ
);

CREATE INDEX idx_vrf_user ON vrf_requests(user_id);
CREATE INDEX idx_vrf_status ON vrf_requests(status);

-- =============================================================================
-- Oracle Requests
-- =============================================================================

CREATE TABLE oracle_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    method VARCHAR(16) DEFAULT 'GET',
    headers JSONB,
    body TEXT,
    json_path TEXT,
    status request_status DEFAULT 'pending',
    response JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_oracle_user ON oracle_requests(user_id);
CREATE INDEX idx_oracle_status ON oracle_requests(status);

-- =============================================================================
-- Mixer (Privacy) Transactions
-- =============================================================================

CREATE TABLE mixer_pools (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    denomination BIGINT NOT NULL,
    asset VARCHAR(64) NOT NULL,
    merkle_root BYTEA,
    leaf_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(denomination, asset)
);

CREATE TABLE mixer_commitments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pool_id UUID NOT NULL REFERENCES mixer_pools(id),
    commitment BYTEA NOT NULL UNIQUE,
    leaf_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_mixer_pool ON mixer_commitments(pool_id);

CREATE TABLE mixer_nullifiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pool_id UUID NOT NULL REFERENCES mixer_pools(id),
    nullifier BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =============================================================================
-- Confidential Compute Jobs
-- =============================================================================

CREATE TABLE compute_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    script TEXT NOT NULL,
    entry_point VARCHAR(255) DEFAULT 'main',
    input JSONB,
    secret_refs TEXT[],
    status request_status DEFAULT 'pending',
    output JSONB,
    logs TEXT[],
    error TEXT,
    gas_used BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_compute_user ON compute_jobs(user_id);
CREATE INDEX idx_compute_status ON compute_jobs(status);

-- =============================================================================
-- Attestation Records
-- =============================================================================

CREATE TABLE attestation_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    marble_type VARCHAR(64) NOT NULL,
    marble_uuid VARCHAR(64) NOT NULL,
    mr_enclave BYTEA NOT NULL,
    mr_signer BYTEA NOT NULL,
    product_id SMALLINT NOT NULL,
    security_version SMALLINT NOT NULL,
    quote BYTEA,
    verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_attestation_marble ON attestation_records(marble_type);

-- =============================================================================
-- Row Level Security (RLS)
-- =============================================================================

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE gasbank_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE gasbank_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_triggers ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE vrf_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE oracle_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE compute_jobs ENABLE ROW LEVEL SECURITY;

-- Service role can access all data
CREATE POLICY service_all ON users FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON api_keys FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON secrets FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON service_requests FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON gasbank_accounts FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON gasbank_transactions FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON automation_triggers FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON automation_executions FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON vrf_requests FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON oracle_requests FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON compute_jobs FOR ALL TO service_role USING (true);

-- =============================================================================
-- Functions
-- =============================================================================

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_secrets_updated_at
    BEFORE UPDATE ON secrets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_gasbank_updated_at
    BEFORE UPDATE ON gasbank_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_triggers_updated_at
    BEFORE UPDATE ON automation_triggers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Gas bank balance check
CREATE OR REPLACE FUNCTION check_gasbank_balance()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.balance < 0 THEN
        RAISE EXCEPTION 'Insufficient balance';
    END IF;
    IF NEW.reserved < 0 THEN
        RAISE EXCEPTION 'Invalid reserved amount';
    END IF;
    IF NEW.reserved > NEW.balance THEN
        RAISE EXCEPTION 'Reserved amount exceeds balance';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER check_gasbank_balance_trigger
    BEFORE UPDATE ON gasbank_accounts
    FOR EACH ROW EXECUTE FUNCTION check_gasbank_balance();
