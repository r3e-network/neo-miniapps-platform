-- Enhanced requests table with full state machine support.
-- Adds retry tracking, chain tx reference, and detailed status.

-- Add new columns to existing service_requests table if they don't exist
DO $$
BEGIN
  -- Add retry_count column
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'service_requests' AND column_name = 'retry_count') THEN
    ALTER TABLE service_requests ADD COLUMN retry_count INT NOT NULL DEFAULT 0;
  END IF;

  -- Add chain_tx_id column
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'service_requests' AND column_name = 'chain_tx_id') THEN
    ALTER TABLE service_requests ADD COLUMN chain_tx_id BIGINT REFERENCES chain_txs(id);
  END IF;

  -- Add last_error column
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'service_requests' AND column_name = 'last_error') THEN
    ALTER TABLE service_requests ADD COLUMN last_error TEXT;
  END IF;

  -- Add signature column
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'service_requests' AND column_name = 'signature') THEN
    ALTER TABLE service_requests ADD COLUMN signature BYTEA;
  END IF;

  -- Add signer_key_id column
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                 WHERE table_name = 'service_requests' AND column_name = 'signer_key_id') THEN
    ALTER TABLE service_requests ADD COLUMN signer_key_id TEXT;
  END IF;
END $$;

-- Create index for chain_tx_id if not exists
CREATE INDEX IF NOT EXISTS service_requests_chain_tx_id_idx
  ON service_requests (chain_tx_id);

-- Create index for retry tracking
CREATE INDEX IF NOT EXISTS service_requests_retry_idx
  ON service_requests (status, retry_count)
  WHERE status = 'retrying';

COMMENT ON TABLE service_requests IS 'Enhanced service request state machine with chain tx tracking';
