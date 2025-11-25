-- Enhance NEO indexer schema with richer execution details and reorg-friendly fields.

ALTER TABLE IF EXISTS neo_blocks ADD COLUMN IF NOT EXISTS prev_hash TEXT;
ALTER TABLE IF EXISTS neo_blocks ADD COLUMN IF NOT EXISTS next_hash TEXT;
ALTER TABLE IF EXISTS neo_blocks ADD COLUMN IF NOT EXISTS size BIGINT;
ALTER TABLE IF EXISTS neo_blocks ADD COLUMN IF NOT EXISTS block_time TIMESTAMPTZ;

ALTER TABLE IF EXISTS neo_transactions ADD COLUMN IF NOT EXISTS vm_state TEXT;
ALTER TABLE IF EXISTS neo_transactions ADD COLUMN IF NOT EXISTS exception TEXT;
ALTER TABLE IF EXISTS neo_transactions ADD COLUMN IF NOT EXISTS gas_consumed NUMERIC;
ALTER TABLE IF EXISTS neo_transactions ADD COLUMN IF NOT EXISTS stack JSONB;

ALTER TABLE IF EXISTS neo_notifications ADD COLUMN IF NOT EXISTS exec_index INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS neo_transaction_executions (
    tx_hash TEXT NOT NULL REFERENCES neo_transactions(hash) ON DELETE CASCADE,
    exec_index INTEGER NOT NULL,
    vm_state TEXT,
    exception TEXT,
    gas_consumed NUMERIC,
    stack JSONB,
    notifications JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tx_hash, exec_index)
);
