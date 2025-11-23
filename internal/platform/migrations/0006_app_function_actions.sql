-- Make idempotent for reruns
ALTER TABLE app_function_executions
    ADD COLUMN IF NOT EXISTS actions JSONB NOT NULL DEFAULT '[]'::jsonb;
