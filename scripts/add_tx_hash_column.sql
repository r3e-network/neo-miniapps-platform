-- Add tx_hash column to simulation_txs table
-- This is an incremental migration for existing databases

-- Add the column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'simulation_txs' AND column_name = 'tx_hash'
    ) THEN
        ALTER TABLE simulation_txs ADD COLUMN tx_hash TEXT;
        CREATE INDEX IF NOT EXISTS simulation_txs_tx_hash_idx ON simulation_txs(tx_hash);
        RAISE NOTICE 'Added tx_hash column to simulation_txs table';
    ELSE
        RAISE NOTICE 'tx_hash column already exists in simulation_txs table';
    END IF;
END $$;
