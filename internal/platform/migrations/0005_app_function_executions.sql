-- Function execution history tables

CREATE TABLE IF NOT EXISTS app_function_executions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    function_id TEXT NOT NULL REFERENCES app_functions(id) ON DELETE CASCADE,
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    logs JSONB NOT NULL DEFAULT '[]'::jsonb,
    error TEXT,
    status TEXT NOT NULL CHECK (status IN ('succeeded', 'failed')),
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    duration_ns BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_app_function_executions_function
    ON app_function_executions (function_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_app_function_executions_account
    ON app_function_executions (account_id, started_at DESC);
