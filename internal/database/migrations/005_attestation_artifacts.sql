-- Attestation artifacts table for TEE registration.
-- Stores attestation quotes, reports, and certificates.

CREATE TABLE IF NOT EXISTS attestation_artifacts (
  id BIGSERIAL PRIMARY KEY,
  service_name TEXT NOT NULL,
  artifact_type TEXT NOT NULL,
  artifact_hash TEXT NOT NULL,
  artifact_data BYTEA NOT NULL,
  public_key TEXT,
  key_id TEXT,
  measurement_hash TEXT,
  policy_hash TEXT,
  metadata JSONB,
  verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT attestation_artifacts_type_check CHECK (artifact_type IN ('quote', 'report', 'certificate', 'manifest'))
);

-- Index for querying by service
CREATE INDEX IF NOT EXISTS attestation_artifacts_service_idx
  ON attestation_artifacts (service_name);

-- Index for querying by key_id
CREATE INDEX IF NOT EXISTS attestation_artifacts_key_id_idx
  ON attestation_artifacts (key_id);

-- Index for querying by artifact hash
CREATE INDEX IF NOT EXISTS attestation_artifacts_hash_idx
  ON attestation_artifacts (artifact_hash);

-- Index for querying recent artifacts
CREATE INDEX IF NOT EXISTS attestation_artifacts_created_at_idx
  ON attestation_artifacts (created_at DESC);

COMMENT ON TABLE attestation_artifacts IS 'TEE attestation artifacts for Marblerun services';
