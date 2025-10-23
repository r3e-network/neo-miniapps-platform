ALTER TABLE app_function_executions
    ADD COLUMN actions JSONB NOT NULL DEFAULT '[]'::jsonb;

