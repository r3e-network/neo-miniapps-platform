-- Storage diffs per block/contract to support stateless execution without full blobs.

CREATE TABLE IF NOT EXISTS neo_storage_diffs (
    height BIGINT NOT NULL,
    contract TEXT NOT NULL,
    kv_diff JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (height, contract)
);
