-- Storage snapshots per height/contract (captured from notifications for stateless execution inputs).

CREATE TABLE IF NOT EXISTS neo_storage (
    height BIGINT NOT NULL,
    contract TEXT NOT NULL,
    kv JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (height, contract)
);
