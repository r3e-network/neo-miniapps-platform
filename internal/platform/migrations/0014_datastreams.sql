-- Chainlink Data Streams tables

CREATE TABLE IF NOT EXISTS chainlink_datastreams (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    description TEXT,
    frequency TEXT,
    sla_ms INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_datastreams_account ON chainlink_datastreams(account_id);

CREATE TABLE IF NOT EXISTS chainlink_datastream_frames (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    stream_id UUID NOT NULL REFERENCES chainlink_datastreams(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    payload JSONB,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(stream_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_datastream_frames_stream_seq ON chainlink_datastream_frames(stream_id, sequence DESC);
CREATE INDEX IF NOT EXISTS idx_datastream_frames_account ON chainlink_datastream_frames(account_id);
