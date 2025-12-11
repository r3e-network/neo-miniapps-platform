-- Migration: Multi-Token Balance Support for NeoAccounts
-- Version: 2.0.0
-- Date: 2025-12-10
-- Description: Adds pool_account_balances table for per-token balance tracking

-- =============================================================================
-- Step 1: Create pool_account_balances table
-- =============================================================================

CREATE TABLE IF NOT EXISTS pool_account_balances (
    account_id   UUID NOT NULL REFERENCES pool_accounts(id) ON DELETE CASCADE,
    token_type   VARCHAR(32) NOT NULL,
    script_hash  VARCHAR(66) NOT NULL,
    amount       BIGINT NOT NULL DEFAULT 0,
    decimals     INT NOT NULL DEFAULT 8,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, token_type)
);

-- Index for token-based queries
CREATE INDEX IF NOT EXISTS idx_pool_account_balances_token
ON pool_account_balances(token_type);

-- Index for balance filtering (e.g., find accounts with min balance)
CREATE INDEX IF NOT EXISTS idx_pool_account_balances_amount
ON pool_account_balances(token_type, amount);

-- =============================================================================
-- Step 2: Migrate existing balances (if any data exists)
-- =============================================================================

-- Insert GAS balances from existing accounts (if balance column exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'pool_accounts'
        AND column_name = 'balance'
    ) THEN
        INSERT INTO pool_account_balances (account_id, token_type, script_hash, amount, decimals, updated_at)
        SELECT
            id,
            'GAS',
            '0xd2a4cff31913016155e38e474a2c06d08be276cf',
            COALESCE(balance, 0),
            8,
            NOW()
        FROM pool_accounts
        ON CONFLICT (account_id, token_type) DO NOTHING;
    END IF;
END $$;

-- =============================================================================
-- Step 3: Remove balance column from pool_accounts (clean slate)
-- =============================================================================

-- Note: This is a breaking change. Only execute if clean slate migration is desired.
-- For production, you may want to keep this column during transition period.

ALTER TABLE pool_accounts
DROP COLUMN IF EXISTS balance;

-- =============================================================================
-- Step 4: Add default NEO balances (zero) for existing accounts
-- =============================================================================

INSERT INTO pool_account_balances (account_id, token_type, script_hash, amount, decimals, updated_at)
SELECT
    id,
    'NEO',
    '0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5',
    0,
    0,
    NOW()
FROM pool_accounts
ON CONFLICT (account_id, token_type) DO NOTHING;

-- =============================================================================
-- Rollback Script (if needed)
-- =============================================================================
--
-- To rollback this migration:
--
-- 1. Add balance column back:
--    ALTER TABLE pool_accounts ADD COLUMN balance BIGINT DEFAULT 0;
--
-- 2. Restore GAS balances:
--    UPDATE pool_accounts p
--    SET balance = COALESCE(
--        (SELECT amount FROM pool_account_balances
--         WHERE account_id = p.id AND token_type = 'GAS'),
--        0
--    );
--
-- 3. Drop balances table:
--    DROP TABLE IF EXISTS pool_account_balances;
--
