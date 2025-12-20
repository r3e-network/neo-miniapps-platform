-- =============================================================================
-- MiniApp daily usage tracking (Edge-enforced caps)
-- =============================================================================

CREATE TABLE miniapp_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id TEXT NOT NULL,
    usage_date DATE NOT NULL,
    gas_used BIGINT NOT NULL DEFAULT 0,
    governance_used BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, app_id, usage_date)
);

CREATE INDEX idx_miniapp_usage_user ON miniapp_usage(user_id, app_id, usage_date);

ALTER TABLE miniapp_usage ENABLE ROW LEVEL SECURITY;
CREATE POLICY service_all ON miniapp_usage FOR ALL TO service_role USING (true);

CREATE OR REPLACE FUNCTION miniapp_usage_bump(
    p_user_id UUID,
    p_app_id TEXT,
    p_gas_delta BIGINT DEFAULT 0,
    p_governance_delta BIGINT DEFAULT 0,
    p_gas_cap BIGINT DEFAULT NULL,
    p_governance_cap BIGINT DEFAULT NULL
)
RETURNS TABLE(gas_used BIGINT, governance_used BIGINT) AS $$
DECLARE
    v_gas BIGINT;
    v_governance BIGINT;
BEGIN
    INSERT INTO miniapp_usage (
        user_id,
        app_id,
        usage_date,
        gas_used,
        governance_used,
        updated_at
    )
    VALUES (
        p_user_id,
        p_app_id,
        CURRENT_DATE,
        GREATEST(COALESCE(p_gas_delta, 0), 0),
        GREATEST(COALESCE(p_governance_delta, 0), 0),
        NOW()
    )
    ON CONFLICT (user_id, app_id, usage_date)
    DO UPDATE SET
        gas_used = miniapp_usage.gas_used + EXCLUDED.gas_used,
        governance_used = miniapp_usage.governance_used + EXCLUDED.governance_used,
        updated_at = NOW()
    RETURNING gas_used, governance_used INTO v_gas, v_governance;

    IF p_gas_cap IS NOT NULL AND p_gas_cap > 0 AND v_gas > p_gas_cap THEN
        RAISE EXCEPTION 'CAP_EXCEEDED: daily GAS cap exceeded';
    END IF;

    IF p_governance_cap IS NOT NULL AND p_governance_cap > 0 AND v_governance > p_governance_cap THEN
        RAISE EXCEPTION 'CAP_EXCEEDED: governance cap exceeded';
    END IF;

    RETURN QUERY SELECT v_gas, v_governance;
END;
$$ LANGUAGE plpgsql;
