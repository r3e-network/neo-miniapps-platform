-- =============================================================================
-- MiniApps manifest registry (Supabase-side mirror)
-- =============================================================================

CREATE TYPE app_status AS ENUM ('active', 'disabled');

CREATE TABLE miniapps (
    app_id TEXT PRIMARY KEY,
    developer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    manifest_hash TEXT NOT NULL,
    entry_url TEXT NOT NULL,
    developer_pubkey TEXT NOT NULL,
    permissions JSONB NOT NULL DEFAULT '{}'::jsonb,
    limits JSONB NOT NULL DEFAULT '{}'::jsonb,
    assets_allowed TEXT[] NOT NULL DEFAULT '{}'::text[],
    governance_assets_allowed TEXT[] NOT NULL DEFAULT '{}'::text[],
    manifest JSONB NOT NULL,
    status app_status NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_miniapps_developer ON miniapps(developer_user_id);

ALTER TABLE miniapps ENABLE ROW LEVEL SECURITY;
CREATE POLICY service_all ON miniapps FOR ALL TO service_role USING (true);
