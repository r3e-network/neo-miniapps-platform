-- Align account_id columns with app_accounts.id TEXT type for new and existing deployments.
-- These guards only run when the tables/columns exist and are currently UUID-typed.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_data_feeds' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_data_feeds ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_data_feed_updates' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_data_feed_updates ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
END$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_datalink_channels' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_datalink_channels ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_datalink_deliveries' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_datalink_deliveries ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
END$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_datastreams' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_datastreams ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_datastream_frames' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_datastream_frames ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
END$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_dta_products' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_dta_products ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'chainlink_dta_orders' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE chainlink_dta_orders ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
END$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'confidential_enclaves' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE confidential_enclaves ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'confidential_sealed_keys' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE confidential_sealed_keys ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'confidential_attestations' AND column_name = 'account_id' AND data_type = 'uuid') THEN
        ALTER TABLE confidential_attestations ALTER COLUMN account_id TYPE TEXT USING account_id::text;
    END IF;
END$$;
