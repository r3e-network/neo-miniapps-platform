-- Chainlink DataLink tables

CREATE TABLE IF NOT EXISTS chainlink_datalink_channels (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    auth_token TEXT,
    signer_set JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_datalink_channels_account ON chainlink_datalink_channels(account_id);

CREATE TABLE IF NOT EXISTS chainlink_datalink_deliveries (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES chainlink_datalink_channels(id) ON DELETE CASCADE,
    payload JSONB,
    attempts INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    error TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_datalink_deliveries_account ON chainlink_datalink_deliveries(account_id);
CREATE INDEX IF NOT EXISTS idx_datalink_deliveries_channel ON chainlink_datalink_deliveries(channel_id);
