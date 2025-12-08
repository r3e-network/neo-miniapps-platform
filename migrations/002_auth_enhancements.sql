-- =============================================================================
-- Neo Service Layer - Authentication Enhancements
-- Adds: user sessions, wallet bindings, deposit tracking, API key improvements
-- =============================================================================

-- =============================================================================
-- User Sessions (JWT token tracking)
-- =============================================================================

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    device_info JSONB,
    ip_address INET,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_active TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_sessions_token ON user_sessions(token_hash);
CREATE INDEX idx_sessions_expires ON user_sessions(expires_at);

-- =============================================================================
-- User Wallets (multiple wallet support)
-- =============================================================================

CREATE TABLE user_wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address VARCHAR(64) NOT NULL,
    label VARCHAR(255),
    is_primary BOOLEAN DEFAULT FALSE,
    verified BOOLEAN DEFAULT FALSE,
    verification_message TEXT,
    verification_signature TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, address)
);

CREATE INDEX idx_wallets_user ON user_wallets(user_id);
CREATE INDEX idx_wallets_address ON user_wallets(address);

-- Ensure only one primary wallet per user
CREATE UNIQUE INDEX idx_wallets_primary ON user_wallets(user_id) WHERE is_primary = TRUE;

-- =============================================================================
-- Deposit Requests (track on-chain deposits)
-- =============================================================================

CREATE TYPE deposit_status AS ENUM (
    'pending', 'confirming', 'confirmed', 'failed', 'expired'
);

CREATE TABLE deposit_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES gasbank_accounts(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    tx_hash VARCHAR(66),
    from_address VARCHAR(64) NOT NULL,
    status deposit_status DEFAULT 'pending',
    confirmations INTEGER DEFAULT 0,
    required_confirmations INTEGER DEFAULT 1,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ DEFAULT NOW() + INTERVAL '24 hours'
);

CREATE INDEX idx_deposits_user ON deposit_requests(user_id);
CREATE INDEX idx_deposits_status ON deposit_requests(status);
CREATE INDEX idx_deposits_tx ON deposit_requests(tx_hash);

-- =============================================================================
-- Withdrawal Requests
-- =============================================================================

CREATE TYPE withdrawal_status AS ENUM (
    'pending', 'processing', 'completed', 'failed', 'cancelled'
);

CREATE TABLE withdrawal_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES gasbank_accounts(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL,
    to_address VARCHAR(64) NOT NULL,
    tx_hash VARCHAR(66),
    status withdrawal_status DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_withdrawals_user ON withdrawal_requests(user_id);
CREATE INDEX idx_withdrawals_status ON withdrawal_requests(status);

-- =============================================================================
-- API Key Usage Tracking
-- =============================================================================

CREATE TABLE api_key_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(16) NOT NULL,
    status_code INTEGER,
    response_time_ms INTEGER,
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_usage_key ON api_key_usage(api_key_id);
CREATE INDEX idx_usage_created ON api_key_usage(created_at DESC);

-- Partition by month for better performance (optional)
-- CREATE INDEX idx_usage_month ON api_key_usage(date_trunc('month', created_at));

-- =============================================================================
-- Rate Limiting
-- =============================================================================

CREATE TABLE rate_limits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    identifier VARCHAR(255) NOT NULL, -- API key prefix or IP address
    identifier_type VARCHAR(32) NOT NULL, -- 'api_key' or 'ip'
    window_start TIMESTAMPTZ NOT NULL,
    request_count INTEGER DEFAULT 1,
    UNIQUE(identifier, identifier_type, window_start)
);

CREATE INDEX idx_rate_limits_identifier ON rate_limits(identifier, identifier_type);
CREATE INDEX idx_rate_limits_window ON rate_limits(window_start);

-- =============================================================================
-- Enable RLS on new tables
-- =============================================================================

ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE deposit_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE withdrawal_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_key_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE rate_limits ENABLE ROW LEVEL SECURITY;

-- Service role policies
CREATE POLICY service_all ON user_sessions FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON user_wallets FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON deposit_requests FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON withdrawal_requests FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON api_key_usage FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON rate_limits FOR ALL TO service_role USING (true);

-- =============================================================================
-- Helper Functions
-- =============================================================================

-- Generate API key with prefix
CREATE OR REPLACE FUNCTION generate_api_key()
RETURNS TABLE(key TEXT, prefix VARCHAR(8), hash VARCHAR(64)) AS $$
DECLARE
    random_bytes BYTEA;
    full_key TEXT;
    key_prefix VARCHAR(8);
    key_hash VARCHAR(64);
BEGIN
    random_bytes := gen_random_bytes(32);
    full_key := 'sl_' || encode(random_bytes, 'hex');
    key_prefix := substring(full_key from 1 for 8);
    key_hash := encode(digest(full_key, 'sha256'), 'hex');

    RETURN QUERY SELECT full_key, key_prefix, key_hash;
END;
$$ LANGUAGE plpgsql;

-- Verify API key
CREATE OR REPLACE FUNCTION verify_api_key(input_key TEXT)
RETURNS TABLE(
    user_id UUID,
    key_id UUID,
    scopes TEXT[],
    valid BOOLEAN
) AS $$
DECLARE
    input_hash VARCHAR(64);
    key_record RECORD;
BEGIN
    input_hash := encode(digest(input_key, 'sha256'), 'hex');

    SELECT ak.id, ak.user_id, ak.scopes, ak.expires_at, ak.revoked
    INTO key_record
    FROM api_keys ak
    WHERE ak.key_hash = input_hash;

    IF key_record IS NULL THEN
        RETURN QUERY SELECT NULL::UUID, NULL::UUID, NULL::TEXT[], FALSE;
        RETURN;
    END IF;

    IF key_record.revoked THEN
        RETURN QUERY SELECT NULL::UUID, NULL::UUID, NULL::TEXT[], FALSE;
        RETURN;
    END IF;

    IF key_record.expires_at IS NOT NULL AND key_record.expires_at < NOW() THEN
        RETURN QUERY SELECT NULL::UUID, NULL::UUID, NULL::TEXT[], FALSE;
        RETURN;
    END IF;

    -- Update last_used
    UPDATE api_keys SET last_used = NOW() WHERE id = key_record.id;

    RETURN QUERY SELECT key_record.user_id, key_record.id, key_record.scopes, TRUE;
END;
$$ LANGUAGE plpgsql;

-- Clean up expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM user_sessions WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Clean up old rate limit records
CREATE OR REPLACE FUNCTION cleanup_rate_limits()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM rate_limits WHERE window_start < NOW() - INTERVAL '1 hour';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Add missing columns to existing tables
-- =============================================================================

-- Add nonce for signature verification
ALTER TABLE users ADD COLUMN IF NOT EXISTS nonce VARCHAR(64);

-- Add description to API keys
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS description TEXT;

-- Update gasbank_transactions to include more details
ALTER TABLE gasbank_transactions ADD COLUMN IF NOT EXISTS tx_hash VARCHAR(66);
ALTER TABLE gasbank_transactions ADD COLUMN IF NOT EXISTS from_address VARCHAR(64);
ALTER TABLE gasbank_transactions ADD COLUMN IF NOT EXISTS to_address VARCHAR(64);
ALTER TABLE gasbank_transactions ADD COLUMN IF NOT EXISTS status VARCHAR(32) DEFAULT 'completed';
