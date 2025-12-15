-- Processed events table for chain event idempotency.
-- Ensures each chain event is processed exactly once.

CREATE TABLE IF NOT EXISTS processed_events (
  id BIGSERIAL PRIMARY KEY,
  chain_id TEXT NOT NULL,
  tx_hash TEXT NOT NULL,
  log_index INT NOT NULL,
  block_height BIGINT NOT NULL,
  block_hash TEXT NOT NULL,
  contract_address TEXT NOT NULL,
  event_name TEXT NOT NULL,
  payload JSONB NOT NULL,
  confirmations INT NOT NULL DEFAULT 0,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT processed_events_unique UNIQUE (chain_id, tx_hash, log_index)
);

-- Index for querying by block height (for reorg detection)
CREATE INDEX IF NOT EXISTS processed_events_block_height_idx
  ON processed_events (block_height DESC);

-- Index for querying by contract and event
CREATE INDEX IF NOT EXISTS processed_events_contract_event_idx
  ON processed_events (contract_address, event_name);

-- Index for querying recent events
CREATE INDEX IF NOT EXISTS processed_events_processed_at_idx
  ON processed_events (processed_at DESC);

COMMENT ON TABLE processed_events IS 'Chain event idempotency table for NeoIndexer';
