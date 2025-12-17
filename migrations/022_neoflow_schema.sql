-- =============================================================================
-- Neo Service Layer - NeoFlow (Automation) Canonical Schema
-- =============================================================================
--
-- The current codebase uses `neoflow_triggers` / `neoflow_executions` as the
-- canonical tables for the automation-service (`services/automation`, service_id
-- `neoflow`).
--
-- Older schemas used `automation_triggers` / `automation_executions`. Those are
-- out of scope for the current platform design and are dropped at the end of
-- this migration to avoid duplicated functionality.

-- =============================================================================
-- NeoFlow Triggers
-- =============================================================================

CREATE TABLE IF NOT EXISTS public.neoflow_triggers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    trigger_type TEXT NOT NULL, -- e.g. "cron", "event", "price_threshold"
    schedule TEXT,
    condition JSONB,
    action JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_execution TIMESTAMPTZ,
    next_execution TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS neoflow_triggers_user_id_idx
    ON public.neoflow_triggers(user_id);
CREATE INDEX IF NOT EXISTS neoflow_triggers_enabled_idx
    ON public.neoflow_triggers(enabled) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS neoflow_triggers_next_execution_idx
    ON public.neoflow_triggers(next_execution) WHERE enabled = TRUE;

-- =============================================================================
-- NeoFlow Executions
-- =============================================================================

CREATE TABLE IF NOT EXISTS public.neoflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trigger_id UUID NOT NULL REFERENCES public.neoflow_triggers(id) ON DELETE CASCADE,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT,
    action_type TEXT,
    action_payload JSONB
);

CREATE INDEX IF NOT EXISTS neoflow_executions_trigger_id_idx
    ON public.neoflow_executions(trigger_id);
CREATE INDEX IF NOT EXISTS neoflow_executions_executed_at_idx
    ON public.neoflow_executions(executed_at DESC);

-- =============================================================================
-- RLS
-- =============================================================================

ALTER TABLE public.neoflow_triggers ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.neoflow_executions ENABLE ROW LEVEL SECURITY;

-- Service role access (TEE services, Edge functions with service key)
CREATE POLICY service_all ON public.neoflow_triggers FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON public.neoflow_executions FOR ALL TO service_role USING (true);

-- NOTE: User-facing trigger management should be implemented via Supabase Edge
-- functions using the service role key. This migration intentionally does not
-- grant direct `authenticated` role access to these tables.

-- =============================================================================
-- Cleanup Legacy Tables
-- =============================================================================

DROP TABLE IF EXISTS public.automation_executions;
DROP TABLE IF EXISTS public.automation_triggers;
DROP TYPE IF EXISTS public.trigger_type;
