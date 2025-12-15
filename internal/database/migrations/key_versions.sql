-- Key versions table for TEE signer rotation.
-- Status lifecycle: active -> deprecated -> expired

CREATE TABLE IF NOT EXISTS key_versions (
  id BIGSERIAL PRIMARY KEY,
  key_version TEXT UNIQUE NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  valid_from TIMESTAMPTZ NOT NULL,
  valid_until TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT key_versions_status_check CHECK (status IN ('active', 'deprecated', 'expired'))
);

-- Enforce a single active key version at any time.
CREATE UNIQUE INDEX IF NOT EXISTS key_versions_single_active
  ON key_versions (status)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS key_versions_status_valid_from_idx
  ON key_versions (status, valid_from DESC);

CREATE INDEX IF NOT EXISTS key_versions_valid_until_idx
  ON key_versions (valid_until);

