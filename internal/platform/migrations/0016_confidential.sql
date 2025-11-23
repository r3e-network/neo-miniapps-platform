-- Confidential compute tables

CREATE TABLE IF NOT EXISTS confidential_enclaves (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    attestation TEXT,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_confidential_enclaves_account ON confidential_enclaves(account_id);

CREATE TABLE IF NOT EXISTS confidential_sealed_keys (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    enclave_id UUID NOT NULL REFERENCES confidential_enclaves(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    blob BYTEA NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_confidential_sealed_keys_account ON confidential_sealed_keys(account_id);
CREATE INDEX IF NOT EXISTS idx_confidential_sealed_keys_enclave ON confidential_sealed_keys(enclave_id);

CREATE TABLE IF NOT EXISTS confidential_attestations (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    enclave_id UUID NOT NULL REFERENCES confidential_enclaves(id) ON DELETE CASCADE,
    report TEXT NOT NULL,
    valid_until TIMESTAMPTZ,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_confidential_attestations_account ON confidential_attestations(account_id);
CREATE INDEX IF NOT EXISTS idx_confidential_attestations_enclave ON confidential_attestations(enclave_id);
