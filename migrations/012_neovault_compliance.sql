-- =============================================================================
-- Neo Service Layer - NeoVault Compliance & Registration System
-- Adds registration approval workflow for legal compliance
-- =============================================================================

-- =============================================================================
-- Registration Status Enum
-- =============================================================================
CREATE TYPE neovault_registration_status AS ENUM (
    'pending',    -- Application submitted, awaiting review
    'approved',   -- Approved for service use
    'rejected',   -- Application rejected
    'suspended',  -- Previously approved, now suspended
    'revoked'     -- Permanently revoked access
);

-- =============================================================================
-- NeoVault Registrations Table
-- Tracks user registration applications for the mixing service
-- =============================================================================
CREATE TABLE IF NOT EXISTS neovault_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Registration Details
    status neovault_registration_status NOT NULL DEFAULT 'pending',

    -- Compliance Information (encrypted or hashed for privacy)
    email VARCHAR(255),
    jurisdiction VARCHAR(64),          -- Country/region code
    terms_version VARCHAR(32) NOT NULL, -- Version of terms accepted
    terms_accepted_at TIMESTAMPTZ NOT NULL,

    -- Application Details
    purpose TEXT,                       -- Stated purpose for using service
    expected_volume VARCHAR(32),        -- Expected monthly volume tier

    -- Admin Review
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    review_notes TEXT,
    rejection_reason TEXT,

    -- Usage Limits (can be customized per user)
    max_daily_amount BIGINT,            -- Daily limit (null = use default)
    max_monthly_amount BIGINT,          -- Monthly limit (null = use default)
    max_single_amount BIGINT,           -- Per-request limit (null = use default)

    -- Audit Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one active registration per user
    CONSTRAINT unique_user_registration UNIQUE (user_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_neovault_reg_status
    ON neovault_registrations(status);
CREATE INDEX IF NOT EXISTS idx_neovault_reg_user
    ON neovault_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_neovault_reg_created
    ON neovault_registrations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_neovault_reg_pending
    ON neovault_registrations(status, created_at)
    WHERE status = 'pending';

-- =============================================================================
-- NeoVault Audit Log Table
-- Immutable audit trail for all significant actions
-- =============================================================================
CREATE TABLE IF NOT EXISTS neovault_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Actor Information
    user_id UUID REFERENCES users(id),
    admin_id UUID REFERENCES users(id),  -- If action by admin

    -- Action Details
    action VARCHAR(64) NOT NULL,         -- e.g., 'registration_submitted', 'request_created', 'request_delivered'
    entity_type VARCHAR(32) NOT NULL,    -- e.g., 'registration', 'request'
    entity_id UUID,                      -- ID of affected entity

    -- Context
    ip_address INET,
    user_agent TEXT,
    request_path TEXT,

    -- Payload (sensitive data should be redacted)
    details JSONB,

    -- Timestamp (immutable)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for audit queries
CREATE INDEX IF NOT EXISTS idx_neovault_audit_user
    ON neovault_audit_log(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_neovault_audit_action
    ON neovault_audit_log(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_neovault_audit_entity
    ON neovault_audit_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_neovault_audit_created
    ON neovault_audit_log(created_at DESC);

-- =============================================================================
-- Rename mixer_requests to neovault_requests (if not already done)
-- =============================================================================
DO $$
BEGIN
    -- Check if old table exists and new one doesn't
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'mixer_requests'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'neovault_requests'
    ) THEN
        -- Rename table
        ALTER TABLE public.mixer_requests RENAME TO neovault_requests;

        -- Rename indexes
        ALTER INDEX IF EXISTS mixer_requests_tx_hash_idx RENAME TO neovault_requests_tx_hash_idx;
        ALTER INDEX IF EXISTS mixer_requests_request_hash_idx RENAME TO neovault_requests_request_hash_idx;
        ALTER INDEX IF EXISTS mixer_requests_deadline_idx RENAME TO neovault_requests_deadline_idx;
    END IF;
END
$$;

-- Create the table if it doesn't exist at all (fresh install)
CREATE TABLE IF NOT EXISTS neovault_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_address TEXT,
    token_type TEXT NOT NULL DEFAULT 'GAS',
    status TEXT NOT NULL DEFAULT 'pending',
    total_amount BIGINT NOT NULL,
    service_fee BIGINT NOT NULL DEFAULT 0,
    net_amount BIGINT NOT NULL,
    target_addresses JSONB NOT NULL DEFAULT '[]'::jsonb,
    initial_splits INT NOT NULL DEFAULT 3,
    mixing_duration_seconds BIGINT NOT NULL DEFAULT 1800,
    deposit_address TEXT NOT NULL,
    deposit_tx_hash TEXT,
    pool_accounts JSONB DEFAULT '[]'::jsonb,
    -- TEE Commitment fields
    request_hash TEXT,
    tee_signature TEXT,
    deadline BIGINT,
    output_tx_ids JSONB DEFAULT '[]'::jsonb,
    completion_proof_json TEXT,
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deposited_at TIMESTAMPTZ,
    mixing_start_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    error TEXT
);

-- Create indexes if table was just created
CREATE INDEX IF NOT EXISTS neovault_requests_user_idx
    ON neovault_requests(user_id);
CREATE INDEX IF NOT EXISTS neovault_requests_status_idx
    ON neovault_requests(status);
CREATE INDEX IF NOT EXISTS neovault_requests_tx_hash_idx
    ON neovault_requests(deposit_tx_hash)
    WHERE deposit_tx_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS neovault_requests_request_hash_idx
    ON neovault_requests(request_hash)
    WHERE request_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS neovault_requests_deadline_idx
    ON neovault_requests(deadline)
    WHERE deadline IS NOT NULL;

-- =============================================================================
-- Row Level Security
-- =============================================================================
ALTER TABLE neovault_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE neovault_audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE neovault_requests ENABLE ROW LEVEL SECURITY;

-- Service role policies (full access for backend)
CREATE POLICY service_all_registrations ON neovault_registrations
    FOR ALL TO service_role USING (true);
CREATE POLICY service_all_audit ON neovault_audit_log
    FOR ALL TO service_role USING (true);
CREATE POLICY service_all_requests ON neovault_requests
    FOR ALL TO service_role USING (true);

-- =============================================================================
-- Triggers
-- =============================================================================
CREATE TRIGGER update_neovault_registrations_updated_at
    BEFORE UPDATE ON neovault_registrations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- Comments
-- =============================================================================
COMMENT ON TABLE neovault_registrations IS 'User registration applications for NeoVault privacy service - requires approval before use';
COMMENT ON TABLE neovault_audit_log IS 'Immutable audit trail for all NeoVault service actions';
COMMENT ON TABLE neovault_requests IS 'NeoVault mixing requests with full transaction history';
COMMENT ON COLUMN neovault_registrations.jurisdiction IS 'User jurisdiction for compliance (ISO country code)';
COMMENT ON COLUMN neovault_registrations.terms_version IS 'Version of terms of service accepted by user';
COMMENT ON COLUMN neovault_audit_log.details IS 'JSON payload with action details (sensitive data redacted)';
