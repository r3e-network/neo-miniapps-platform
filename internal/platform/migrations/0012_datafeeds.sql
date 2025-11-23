-- Chainlink Data Feeds (centralized)

CREATE TABLE IF NOT EXISTS chainlink_data_feeds (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    pair TEXT NOT NULL,
    description TEXT,
    decimals INTEGER NOT NULL,
    heartbeat_seconds BIGINT NOT NULL,
    threshold_ppm INTEGER NOT NULL DEFAULT 0,
    signer_set JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB,
    tags JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chainlink_data_feeds_account ON chainlink_data_feeds(account_id);
CREATE INDEX IF NOT EXISTS idx_chainlink_data_feeds_pair ON chainlink_data_feeds(pair);

CREATE TABLE IF NOT EXISTS chainlink_data_feed_updates (
    id UUID PRIMARY KEY,
    feed_id UUID NOT NULL REFERENCES chainlink_data_feeds(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    round_id BIGINT NOT NULL,
    price TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    signature TEXT,
    status TEXT NOT NULL,
    error TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(feed_id, round_id)
);

CREATE INDEX IF NOT EXISTS idx_chainlink_data_feed_updates_feed_round ON chainlink_data_feed_updates(feed_id, round_id DESC);
