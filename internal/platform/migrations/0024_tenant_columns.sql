-- Add tenant columns to core tables for multi-tenant enforcement without breaking historical migrations.

ALTER TABLE IF EXISTS app_accounts ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_functions ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_triggers ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_automation_jobs ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_price_feeds ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_oracle_sources ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_oracle_requests ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_vrf_keys ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_vrf_requests ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_ccip_lanes ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS app_ccip_messages ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS chainlink_datalink_channels ADD COLUMN IF NOT EXISTS tenant TEXT;
ALTER TABLE IF EXISTS chainlink_datalink_deliveries ADD COLUMN IF NOT EXISTS tenant TEXT;
