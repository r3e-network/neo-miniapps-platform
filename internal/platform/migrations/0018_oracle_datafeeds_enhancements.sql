-- Oracle & Data Feed durability/aggregation enhancements

-- Track oracle request attempts to support TTL/backoff/DLQ handling.
ALTER TABLE app_oracle_requests
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;

-- Allow multiple submissions per round and enforce signer uniqueness.
ALTER TABLE chainlink_data_feed_updates
    ADD COLUMN IF NOT EXISTS signer TEXT NOT NULL DEFAULT '';

-- Remove the per-round uniqueness constraint so multiple signers can submit.
ALTER TABLE chainlink_data_feed_updates
    DROP CONSTRAINT IF EXISTS chainlink_data_feed_updates_feed_id_round_id_key;

-- Backfill legacy rows with an explicit signer to avoid empty-string collisions.
UPDATE chainlink_data_feed_updates
SET signer = COALESCE(NULLIF(signer, ''), account_id)
WHERE signer IS NULL OR signer = '';

-- Enforce per-round signer uniqueness.
CREATE UNIQUE INDEX IF NOT EXISTS idx_chainlink_data_feed_updates_feed_round_signer
    ON chainlink_data_feed_updates(feed_id, round_id, signer);
