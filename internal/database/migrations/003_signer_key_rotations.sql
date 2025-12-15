-- Signer key rotations table for GlobalSigner.
-- Tracks 30-day automatic key rotation with 7-day overlap.

CREATE TABLE IF NOT EXISTS signer_key_rotations (
  id BIGSERIAL PRIMARY KEY,
  key_id TEXT UNIQUE NOT NULL,
  public_key TEXT NOT NULL,
  attestation_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  registry_tx_hash TEXT,
  activated_at TIMESTAMPTZ,
  overlap_ends_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT signer_key_rotations_status_check CHECK (status IN ('pending', 'active', 'overlapping', 'revoked'))
);

-- Index for querying active keys
CREATE INDEX IF NOT EXISTS signer_key_rotations_status_idx
  ON signer_key_rotations (status);

-- Index for querying by activation time
CREATE INDEX IF NOT EXISTS signer_key_rotations_activated_at_idx
  ON signer_key_rotations (activated_at DESC);

-- Ensure at most 2 active/overlapping keys at any time
CREATE INDEX IF NOT EXISTS signer_key_rotations_active_overlapping_idx
  ON signer_key_rotations (status)
  WHERE status IN ('active', 'overlapping');

COMMENT ON TABLE signer_key_rotations IS 'Key rotation state machine for GlobalSigner';
