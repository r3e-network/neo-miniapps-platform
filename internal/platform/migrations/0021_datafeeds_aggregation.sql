-- Data feed aggregation strategy per-feed (median/mean/min/max)

ALTER TABLE chainlink_data_feeds
    ADD COLUMN IF NOT EXISTS aggregation TEXT NOT NULL DEFAULT 'median';

-- Backfill any existing rows with the default strategy.
UPDATE chainlink_data_feeds SET aggregation = 'median' WHERE aggregation IS NULL OR aggregation = '';
