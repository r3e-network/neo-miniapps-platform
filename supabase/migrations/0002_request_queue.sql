-- =============================================================================
-- Neo Service Layer - Request Queue Architecture
-- =============================================================================
-- This migration adds request queue tables for Frontend ↔ Supabase ↔ Service Layer
-- communication pattern. Frontend writes requests, Service Layer processes them,
-- results are pushed back via Supabase Realtime.

-- =============================================================================
-- Service Request Queue (Generic)
-- =============================================================================

CREATE TABLE IF NOT EXISTS service_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Request details
    service_type TEXT NOT NULL CHECK (service_type IN ('oracle', 'vrf', 'secrets', 'gasbank', 'mixer', 'datafeeds', 'accounts', 'automation', 'ccip', 'confidential', 'cre', 'datalink', 'datastreams', 'dta')),
    operation TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',

    -- Processing status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')),
    priority INTEGER NOT NULL DEFAULT 0,

    -- Result
    result JSONB,
    error_message TEXT,

    -- TEE attestation
    tee_quote BYTEA,
    tee_signature BYTEA,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '1 hour'),

    -- Metadata
    metadata JSONB DEFAULT '{}'
);

-- Indexes for efficient polling
CREATE INDEX idx_service_requests_pending ON service_requests(service_type, status, priority DESC, created_at ASC)
    WHERE status = 'pending';
CREATE INDEX idx_service_requests_user ON service_requests(user_id, created_at DESC);
CREATE INDEX idx_service_requests_status ON service_requests(status, created_at DESC);

-- Enable RLS
ALTER TABLE service_requests ENABLE ROW LEVEL SECURITY;

-- Users can only see their own requests
CREATE POLICY service_requests_select ON service_requests
    FOR SELECT USING (auth.uid() = user_id);

-- Users can insert their own requests
CREATE POLICY service_requests_insert ON service_requests
    FOR INSERT WITH CHECK (auth.uid() = user_id);

-- Users can cancel their own pending requests
CREATE POLICY service_requests_update ON service_requests
    FOR UPDATE USING (auth.uid() = user_id AND status = 'pending');

-- =============================================================================
-- Oracle Requests (Specific)
-- =============================================================================

CREATE TABLE IF NOT EXISTS oracle_request_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Request
    url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'GET',
    headers JSONB DEFAULT '{}',
    body TEXT,
    json_path TEXT, -- Optional JSON path extraction

    -- Callback (optional Neo N3 contract callback)
    callback_contract TEXT,
    callback_method TEXT,

    -- Status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),

    -- Result
    response_status INTEGER,
    response_headers JSONB,
    response_body TEXT,
    extracted_value JSONB, -- If json_path was specified
    response_hash TEXT, -- SHA256 of response

    -- TEE
    tee_signature BYTEA,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- Error
    error_message TEXT
);

CREATE INDEX idx_oracle_queue_pending ON oracle_request_queue(status, created_at ASC) WHERE status = 'pending';
CREATE INDEX idx_oracle_queue_user ON oracle_request_queue(user_id, created_at DESC);

ALTER TABLE oracle_request_queue ENABLE ROW LEVEL SECURITY;

CREATE POLICY oracle_queue_select ON oracle_request_queue FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY oracle_queue_insert ON oracle_request_queue FOR INSERT WITH CHECK (auth.uid() = user_id);

-- =============================================================================
-- VRF Requests
-- =============================================================================

CREATE TABLE IF NOT EXISTS vrf_request_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Request
    seed TEXT NOT NULL,
    num_values INTEGER DEFAULT 1,

    -- Callback
    callback_contract TEXT,
    callback_method TEXT,

    -- Status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),

    -- Result
    random_values JSONB, -- Array of random values
    proof BYTEA,
    public_key BYTEA,

    -- TEE
    tee_signature BYTEA,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- Error
    error_message TEXT
);

CREATE INDEX idx_vrf_queue_pending ON vrf_request_queue(status, created_at ASC) WHERE status = 'pending';
CREATE INDEX idx_vrf_queue_user ON vrf_request_queue(user_id, created_at DESC);

ALTER TABLE vrf_request_queue ENABLE ROW LEVEL SECURITY;

CREATE POLICY vrf_queue_select ON vrf_request_queue FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY vrf_queue_insert ON vrf_request_queue FOR INSERT WITH CHECK (auth.uid() = user_id);

-- =============================================================================
-- Secrets Requests
-- =============================================================================

CREATE TABLE IF NOT EXISTS secrets_request_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Request
    operation TEXT NOT NULL CHECK (operation IN ('store', 'get', 'delete', 'list')),
    secret_name TEXT,
    encrypted_value BYTEA, -- For store operation

    -- Status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),

    -- Result (encrypted)
    result_encrypted BYTEA,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- Error
    error_message TEXT
);

CREATE INDEX idx_secrets_queue_pending ON secrets_request_queue(status, created_at ASC) WHERE status = 'pending';
CREATE INDEX idx_secrets_queue_user ON secrets_request_queue(user_id, created_at DESC);

ALTER TABLE secrets_request_queue ENABLE ROW LEVEL SECURITY;

CREATE POLICY secrets_queue_select ON secrets_request_queue FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY secrets_queue_insert ON secrets_request_queue FOR INSERT WITH CHECK (auth.uid() = user_id);

-- =============================================================================
-- GasBank Operations
-- =============================================================================

CREATE TABLE IF NOT EXISTS gasbank_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Operation
    operation TEXT NOT NULL CHECK (operation IN ('deposit', 'withdraw', 'transfer', 'check_balance')),
    amount NUMERIC(78, 0),
    recipient TEXT, -- For transfer

    -- Neo N3 transaction
    neo_tx_hash TEXT,
    neo_block_height INTEGER,

    -- Status
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirming', 'completed', 'failed')),
    confirmations INTEGER DEFAULT 0,

    -- Result
    new_balance NUMERIC(78, 0),

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,

    -- Error
    error_message TEXT
);

CREATE INDEX idx_gasbank_ops_pending ON gasbank_operations(status, created_at ASC) WHERE status IN ('pending', 'confirming');
CREATE INDEX idx_gasbank_ops_user ON gasbank_operations(user_id, created_at DESC);

ALTER TABLE gasbank_operations ENABLE ROW LEVEL SECURITY;

CREATE POLICY gasbank_ops_select ON gasbank_operations FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY gasbank_ops_insert ON gasbank_operations FOR INSERT WITH CHECK (auth.uid() = user_id);

-- =============================================================================
-- Automation Tasks & Runs
-- =============================================================================

-- Update existing automation_tasks to work with queue pattern
ALTER TABLE automation_tasks ADD COLUMN IF NOT EXISTS trigger_type TEXT DEFAULT 'cron'
    CHECK (trigger_type IN ('cron', 'event', 'manual', 'webhook'));
ALTER TABLE automation_tasks ADD COLUMN IF NOT EXISTS webhook_secret TEXT;
ALTER TABLE automation_tasks ADD COLUMN IF NOT EXISTS max_retries INTEGER DEFAULT 3;

-- =============================================================================
-- Realtime Notifications Table
-- =============================================================================

CREATE TABLE IF NOT EXISTS realtime_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id),

    -- Notification
    type TEXT NOT NULL, -- request_completed, request_failed, balance_updated, etc.
    title TEXT NOT NULL,
    message TEXT,
    data JSONB DEFAULT '{}',

    -- Reference
    reference_type TEXT, -- oracle_request, vrf_request, etc.
    reference_id UUID,

    -- Status
    read BOOLEAN DEFAULT FALSE,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON realtime_notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON realtime_notifications(user_id, read) WHERE read = FALSE;

ALTER TABLE realtime_notifications ENABLE ROW LEVEL SECURITY;

CREATE POLICY notifications_select ON realtime_notifications FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY notifications_update ON realtime_notifications FOR UPDATE USING (auth.uid() = user_id);

-- =============================================================================
-- Service Layer Processing Functions
-- =============================================================================

-- Function to claim a pending request (called by Service Layer)
CREATE OR REPLACE FUNCTION claim_pending_request(
    p_service_type TEXT,
    p_processor_id TEXT
)
RETURNS TABLE(request_id UUID, payload JSONB) AS $$
DECLARE
    v_request_id UUID;
    v_payload JSONB;
BEGIN
    -- Atomically claim the oldest pending request
    UPDATE service_requests
    SET status = 'processing',
        started_at = NOW(),
        metadata = metadata || jsonb_build_object('processor_id', p_processor_id)
    WHERE id = (
        SELECT id FROM service_requests
        WHERE service_type = p_service_type
          AND status = 'pending'
          AND (expires_at IS NULL OR expires_at > NOW())
        ORDER BY priority DESC, created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING id, service_requests.payload INTO v_request_id, v_payload;

    IF v_request_id IS NOT NULL THEN
        RETURN QUERY SELECT v_request_id, v_payload;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to complete a request
CREATE OR REPLACE FUNCTION complete_request(
    p_request_id UUID,
    p_result JSONB,
    p_tee_signature BYTEA DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
    v_user_id UUID;
    v_service_type TEXT;
BEGIN
    UPDATE service_requests
    SET status = 'completed',
        result = p_result,
        tee_signature = p_tee_signature,
        completed_at = NOW()
    WHERE id = p_request_id AND status = 'processing'
    RETURNING user_id, service_type INTO v_user_id, v_service_type;

    IF v_user_id IS NOT NULL THEN
        -- Create notification
        INSERT INTO realtime_notifications (user_id, type, title, message, reference_type, reference_id, data)
        VALUES (
            v_user_id,
            'request_completed',
            v_service_type || ' request completed',
            'Your ' || v_service_type || ' request has been processed successfully.',
            'service_request',
            p_request_id,
            p_result
        );
        RETURN TRUE;
    END IF;

    RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- Function to fail a request
CREATE OR REPLACE FUNCTION fail_request(
    p_request_id UUID,
    p_error_message TEXT
)
RETURNS BOOLEAN AS $$
DECLARE
    v_user_id UUID;
    v_service_type TEXT;
BEGIN
    UPDATE service_requests
    SET status = 'failed',
        error_message = p_error_message,
        completed_at = NOW()
    WHERE id = p_request_id AND status = 'processing'
    RETURNING user_id, service_type INTO v_user_id, v_service_type;

    IF v_user_id IS NOT NULL THEN
        -- Create notification
        INSERT INTO realtime_notifications (user_id, type, title, message, reference_type, reference_id)
        VALUES (
            v_user_id,
            'request_failed',
            v_service_type || ' request failed',
            p_error_message,
            'service_request',
            p_request_id
        );
        RETURN TRUE;
    END IF;

    RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Triggers for Realtime Updates
-- =============================================================================

-- Notify on request status change
CREATE OR REPLACE FUNCTION notify_request_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Supabase Realtime will automatically broadcast changes
    -- This trigger can be used for additional logic if needed
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER service_requests_change
    AFTER UPDATE ON service_requests
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION notify_request_change();

-- =============================================================================
-- Service Layer Access (Bypass RLS for service account)
-- =============================================================================

-- Create a service role for Service Layer
-- Note: In production, use Supabase service_role key which bypasses RLS

-- Grant Service Layer access to claim and complete requests
-- This is done via service_role key, not through policies
