-- =============================================================================
-- MiniApp Statistics and Notifications
-- =============================================================================

-- Aggregated statistics per MiniApp (updated periodically)
CREATE TABLE miniapp_stats (
    app_id TEXT PRIMARY KEY REFERENCES miniapps(app_id) ON DELETE CASCADE,
    total_transactions BIGINT NOT NULL DEFAULT 0,
    total_users INTEGER NOT NULL DEFAULT 0,
    total_gas_used NUMERIC(30,8) NOT NULL DEFAULT 0,
    total_gas_earned NUMERIC(30,8) NOT NULL DEFAULT 0,
    method_calls JSONB NOT NULL DEFAULT '{}'::jsonb,
    daily_active_users INTEGER NOT NULL DEFAULT 0,
    weekly_active_users INTEGER NOT NULL DEFAULT 0,
    last_activity_at TIMESTAMPTZ,
    stats_updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_miniapp_stats_activity ON miniapp_stats(last_activity_at DESC);

-- MiniApp notifications/news
CREATE TABLE miniapp_notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    app_id TEXT NOT NULL REFERENCES miniapps(app_id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    notification_type TEXT NOT NULL DEFAULT 'news',
    source TEXT NOT NULL DEFAULT 'contract',
    tx_hash TEXT,
    block_number BIGINT,
    priority INTEGER NOT NULL DEFAULT 0,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_notifications_app ON miniapp_notifications(app_id, created_at DESC);
CREATE INDEX idx_notifications_recent ON miniapp_notifications(created_at DESC);

-- RLS policies
ALTER TABLE miniapp_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE miniapp_notifications ENABLE ROW LEVEL SECURITY;

CREATE POLICY service_all ON miniapp_stats FOR ALL TO service_role USING (true);
CREATE POLICY service_all ON miniapp_notifications FOR ALL TO service_role USING (true);
CREATE POLICY public_read ON miniapp_stats FOR SELECT TO anon USING (true);
CREATE POLICY public_read ON miniapp_notifications FOR SELECT TO anon USING (true);
