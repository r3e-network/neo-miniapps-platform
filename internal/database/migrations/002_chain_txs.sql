-- Chain transactions audit table.
-- Records all transactions submitted through TxSubmitter.

CREATE TABLE IF NOT EXISTS chain_txs (
  id BIGSERIAL PRIMARY KEY,
  tx_hash TEXT UNIQUE,
  request_id TEXT NOT NULL,
  from_service TEXT NOT NULL,
  tx_type TEXT NOT NULL,
  contract_address TEXT NOT NULL,
  method_name TEXT NOT NULL,
  params JSONB NOT NULL,
  gas_consumed BIGINT,
  status TEXT NOT NULL DEFAULT 'pending',
  retry_count INT NOT NULL DEFAULT 0,
  error_message TEXT,
  rpc_endpoint TEXT,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  confirmed_at TIMESTAMPTZ,
  CONSTRAINT chain_txs_status_check CHECK (status IN ('pending', 'submitted', 'confirmed', 'failed', 'timeout'))
);

-- Index for querying by status
CREATE INDEX IF NOT EXISTS chain_txs_status_idx
  ON chain_txs (status);

-- Index for querying by service
CREATE INDEX IF NOT EXISTS chain_txs_from_service_idx
  ON chain_txs (from_service);

-- Index for querying by request
CREATE INDEX IF NOT EXISTS chain_txs_request_id_idx
  ON chain_txs (request_id);

-- Index for querying pending transactions
CREATE INDEX IF NOT EXISTS chain_txs_pending_idx
  ON chain_txs (status, submitted_at)
  WHERE status IN ('pending', 'submitted');

COMMENT ON TABLE chain_txs IS 'Transaction audit table for TxSubmitter';
