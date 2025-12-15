-- Gas ledger entries table for GasAccounting.
-- Immutable append-only ledger for GAS balance changes.

CREATE TABLE IF NOT EXISTS gas_ledger_entries (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL,
  amount BIGINT NOT NULL,
  entry_type TEXT NOT NULL,
  reference_id TEXT,
  reference_type TEXT,
  description TEXT,
  balance_after BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT gas_ledger_entries_type_check CHECK (entry_type IN ('deposit', 'withdraw', 'fee', 'refund', 'adjustment'))
);

-- Index for querying by user
CREATE INDEX IF NOT EXISTS gas_ledger_entries_user_id_idx
  ON gas_ledger_entries (user_id);

-- Index for querying by reference
CREATE INDEX IF NOT EXISTS gas_ledger_entries_reference_idx
  ON gas_ledger_entries (reference_type, reference_id);

-- Index for querying recent entries
CREATE INDEX IF NOT EXISTS gas_ledger_entries_created_at_idx
  ON gas_ledger_entries (created_at DESC);

-- Index for querying by entry type
CREATE INDEX IF NOT EXISTS gas_ledger_entries_type_idx
  ON gas_ledger_entries (entry_type);

COMMENT ON TABLE gas_ledger_entries IS 'Immutable GAS ledger for GasAccounting service';
