-- =============================================================================
-- Neo Service Layer - Core Schema for MarbleRun + Supabase Architecture
-- =============================================================================
-- This migration creates the core tables for the confidential microservices mesh.
-- All data is stored in Supabase PostgreSQL with Row Level Security (RLS).

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =============================================================================
-- Coordinator State
-- =============================================================================

-- Coordinator state and configuration
CREATE TABLE IF NOT EXISTS coordinator_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    state TEXT NOT NULL DEFAULT 'uninitialized' CHECK (state IN ('uninitialized', 'accepting_manifest', 'accepting_marbles', 'recovery')),
    manifest_fingerprint TEXT,
    root_ca_cert BYTEA,
    sealed_state BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Manifest storage (encrypted)
CREATE TABLE IF NOT EXISTS manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    fingerprint TEXT UNIQUE NOT NULL,
    content BYTEA NOT NULL, -- Encrypted manifest JSON
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES auth.users(id)
);

-- =============================================================================
-- Packages (Enclave Software)
-- =============================================================================

CREATE TABLE IF NOT EXISTS packages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    unique_id TEXT, -- MRENCLAVE
    signer_id TEXT, -- MRSIGNER
    product_id INTEGER,
    security_version INTEGER,
    debug BOOLEAN NOT NULL DEFAULT FALSE,
    accepted_tcb_statuses TEXT[] DEFAULT ARRAY['UpToDate'],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Marbles (Service Instances)
-- =============================================================================

CREATE TABLE IF NOT EXISTS marble_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    package_id UUID NOT NULL REFERENCES packages(id),
    max_activations INTEGER DEFAULT 0, -- 0 = unlimited
    parameters JSONB DEFAULT '{}', -- Env, Files, Argv
    tls_config JSONB DEFAULT '{}', -- Incoming, Outgoing
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Active marble instances
CREATE TABLE IF NOT EXISTS marble_instances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    marble_type TEXT NOT NULL,
    uuid TEXT UNIQUE NOT NULL,
    quote BYTEA,
    quote_verified BOOLEAN NOT NULL DEFAULT FALSE,
    certificate BYTEA,
    activated_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'revoked', 'expired')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_marble_instances_type ON marble_instances(marble_type);
CREATE INDEX idx_marble_instances_status ON marble_instances(status);

-- =============================================================================
-- Secrets Management
-- =============================================================================

-- Secret definitions from manifest
CREATE TABLE IF NOT EXISTS secret_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('symmetric-key', 'cert-rsa', 'cert-ecdsa', 'cert-ed25519', 'plain')),
    size INTEGER,
    shared BOOLEAN NOT NULL DEFAULT FALSE,
    user_defined BOOLEAN NOT NULL DEFAULT FALSE,
    cert_config JSONB, -- Subject, DNSNames, ValidityDays, etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Generated/stored secret values (encrypted)
CREATE TABLE IF NOT EXISTS secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    definition_id UUID NOT NULL REFERENCES secret_definitions(id),
    private_data BYTEA, -- Encrypted private key or secret value
    public_data BYTEA, -- Public key or certificate (if applicable)
    certificate BYTEA, -- X.509 certificate (if applicable)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    rotated_from UUID REFERENCES secrets(id)
);

CREATE INDEX idx_secrets_definition ON secrets(definition_id);

-- User-defined secrets (per tenant)
CREATE TABLE IF NOT EXISTS user_secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    name TEXT NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_user_secrets_user ON user_secrets(user_id);

-- =============================================================================
-- Recovery Keys
-- =============================================================================

CREATE TABLE IF NOT EXISTS recovery_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    public_key_pem TEXT NOT NULL, -- RSA public key
    encrypted_share BYTEA, -- Encrypted sealing key share
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Users and Roles (Coordinator Access Control)
-- =============================================================================

CREATE TABLE IF NOT EXISTS coordinator_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    certificate_fingerprint TEXT UNIQUE NOT NULL,
    roles TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS coordinator_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    resource_type TEXT NOT NULL,
    resource_names TEXT[] DEFAULT ARRAY[]::TEXT[],
    actions TEXT[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- Service-Specific Tables
-- =============================================================================

-- Oracle requests
CREATE TABLE IF NOT EXISTS oracle_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES auth.users(id),
    marble_id UUID REFERENCES marble_instances(id),
    url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'GET',
    headers JSONB DEFAULT '{}',
    body BYTEA,
    response_data BYTEA,
    response_status INTEGER,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_oracle_requests_user ON oracle_requests(user_id);
CREATE INDEX idx_oracle_requests_status ON oracle_requests(status);

-- VRF requests
CREATE TABLE IF NOT EXISTS vrf_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES auth.users(id),
    marble_id UUID REFERENCES marble_instances(id),
    seed BYTEA NOT NULL,
    proof BYTEA,
    random_value BYTEA,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_vrf_requests_user ON vrf_requests(user_id);
CREATE INDEX idx_vrf_requests_status ON vrf_requests(status);

-- Gas bank accounts
CREATE TABLE IF NOT EXISTS gasbank_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    balance NUMERIC(78, 0) NOT NULL DEFAULT 0,
    reserved NUMERIC(78, 0) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE TABLE IF NOT EXISTS gasbank_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES gasbank_accounts(id),
    type TEXT NOT NULL CHECK (type IN ('deposit', 'withdrawal', 'fee', 'refund')),
    amount NUMERIC(78, 0) NOT NULL,
    tx_hash TEXT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gasbank_transactions_account ON gasbank_transactions(account_id);

-- Data feeds
CREATE TABLE IF NOT EXISTS data_feeds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    source_url TEXT,
    update_interval INTEGER NOT NULL DEFAULT 60, -- seconds
    last_value JSONB,
    last_updated_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'error')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS data_feed_values (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    feed_id UUID NOT NULL REFERENCES data_feeds(id),
    value JSONB NOT NULL,
    marble_id UUID REFERENCES marble_instances(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_data_feed_values_feed ON data_feed_values(feed_id);
CREATE INDEX idx_data_feed_values_created ON data_feed_values(created_at DESC);

-- Automation tasks
CREATE TABLE IF NOT EXISTS automation_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    name TEXT NOT NULL,
    cron_expression TEXT,
    function_code TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'error')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_automation_tasks_user ON automation_tasks(user_id);
CREATE INDEX idx_automation_tasks_next_run ON automation_tasks(next_run_at) WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS automation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES automation_tasks(id),
    marble_id UUID REFERENCES marble_instances(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    output JSONB,
    error_message TEXT
);

CREATE INDEX idx_automation_runs_task ON automation_runs(task_id);

-- =============================================================================
-- Audit Log
-- =============================================================================

CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'marble', 'coordinator', 'system')),
    actor_id TEXT,
    resource_type TEXT,
    resource_id TEXT,
    action TEXT NOT NULL,
    details JSONB DEFAULT '{}',
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_event ON audit_log(event_type);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_type, actor_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);

-- =============================================================================
-- Row Level Security Policies
-- =============================================================================

-- Enable RLS on all tables
ALTER TABLE user_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE oracle_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE vrf_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE gasbank_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE gasbank_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_runs ENABLE ROW LEVEL SECURITY;

-- User secrets: users can only access their own secrets
CREATE POLICY user_secrets_policy ON user_secrets
    FOR ALL USING (auth.uid() = user_id);

-- Oracle requests: users can only access their own requests
CREATE POLICY oracle_requests_policy ON oracle_requests
    FOR ALL USING (auth.uid() = user_id);

-- VRF requests: users can only access their own requests
CREATE POLICY vrf_requests_policy ON vrf_requests
    FOR ALL USING (auth.uid() = user_id);

-- Gas bank accounts: users can only access their own accounts
CREATE POLICY gasbank_accounts_policy ON gasbank_accounts
    FOR ALL USING (auth.uid() = user_id);

-- Gas bank transactions: users can only see transactions for their accounts
CREATE POLICY gasbank_transactions_policy ON gasbank_transactions
    FOR SELECT USING (
        account_id IN (SELECT id FROM gasbank_accounts WHERE user_id = auth.uid())
    );

-- Automation tasks: users can only access their own tasks
CREATE POLICY automation_tasks_policy ON automation_tasks
    FOR ALL USING (auth.uid() = user_id);

-- Automation runs: users can only see runs for their tasks
CREATE POLICY automation_runs_policy ON automation_runs
    FOR SELECT USING (
        task_id IN (SELECT id FROM automation_tasks WHERE user_id = auth.uid())
    );

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

-- Apply update trigger to relevant tables
CREATE TRIGGER update_coordinator_state_updated_at
    BEFORE UPDATE ON coordinator_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_packages_updated_at
    BEFORE UPDATE ON packages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_marble_definitions_updated_at
    BEFORE UPDATE ON marble_definitions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_user_secrets_updated_at
    BEFORE UPDATE ON user_secrets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_gasbank_accounts_updated_at
    BEFORE UPDATE ON gasbank_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_automation_tasks_updated_at
    BEFORE UPDATE ON automation_tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Audit log function
CREATE OR REPLACE FUNCTION log_audit_event(
    p_event_type TEXT,
    p_actor_type TEXT,
    p_actor_id TEXT,
    p_resource_type TEXT,
    p_resource_id TEXT,
    p_action TEXT,
    p_details JSONB DEFAULT '{}'
)
RETURNS UUID AS $$
DECLARE
    v_id UUID;
BEGIN
    INSERT INTO audit_log (event_type, actor_type, actor_id, resource_type, resource_id, action, details)
    VALUES (p_event_type, p_actor_type, p_actor_id, p_resource_type, p_resource_id, p_action, p_details)
    RETURNING id INTO v_id;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Initial Data
-- =============================================================================

-- Insert initial coordinator state
INSERT INTO coordinator_state (state) VALUES ('uninitialized')
ON CONFLICT DO NOTHING;
