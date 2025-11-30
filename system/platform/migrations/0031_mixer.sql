-- Migration: 0031_mixer
-- Description: Privacy Mixer Service tables

-- Mix requests table
CREATE TABLE IF NOT EXISTS mixer_requests (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',

    -- Input configuration
    source_wallet TEXT NOT NULL,
    amount TEXT NOT NULL,
    token_address TEXT DEFAULT '',
    mix_duration TEXT NOT NULL DEFAULT '1h',
    split_count INTEGER NOT NULL DEFAULT 1,

    -- Target configuration (JSON array of {address, amount, delivered, tx_hash})
    targets JSONB NOT NULL DEFAULT '[]',

    -- Deposit tracking
    deposit_tx_hashes TEXT[] DEFAULT '{}',
    deposit_pool_ids TEXT[] DEFAULT '{}',
    deposited_amount TEXT DEFAULT '0',

    -- Proof and security
    zk_proof_hash TEXT DEFAULT '',
    tee_signature TEXT DEFAULT '',
    on_chain_proof_tx TEXT DEFAULT '',

    -- Timing
    mix_start_at TIMESTAMPTZ,
    mix_end_at TIMESTAMPTZ,
    withdrawable_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Completion tracking
    completion_proof_tx TEXT DEFAULT '',
    delivered_amount TEXT DEFAULT '0',

    -- Error handling
    error TEXT DEFAULT '',
    refund_tx_hash TEXT DEFAULT '',

    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Pool accounts table (TEE-managed mixing wallets)
CREATE TABLE IF NOT EXISTS mixer_pool_accounts (
    id TEXT PRIMARY KEY,
    wallet_address TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active',

    -- Balance tracking
    balance TEXT NOT NULL DEFAULT '0',
    pending_in TEXT NOT NULL DEFAULT '0',
    pending_out TEXT NOT NULL DEFAULT '0',

    -- TEE management
    tee_key_id TEXT NOT NULL,
    encrypted_priv_key TEXT NOT NULL,

    -- Activity tracking
    total_received TEXT NOT NULL DEFAULT '0',
    total_sent TEXT NOT NULL DEFAULT '0',
    transaction_count BIGINT NOT NULL DEFAULT 0,

    -- Lifecycle
    retire_after TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Mix transactions table (internal obfuscation transactions)
CREATE TABLE IF NOT EXISTS mixer_transactions (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'scheduled',

    from_pool_id TEXT REFERENCES mixer_pool_accounts(id),
    to_pool_id TEXT REFERENCES mixer_pool_accounts(id),
    amount TEXT NOT NULL DEFAULT '0',

    -- For user-related transactions
    request_id TEXT REFERENCES mixer_requests(id),
    target_address TEXT DEFAULT '',

    -- Blockchain info
    tx_hash TEXT DEFAULT '',
    block_number BIGINT DEFAULT 0,
    gas_used TEXT DEFAULT '0',

    error TEXT DEFAULT '',
    scheduled_at TIMESTAMPTZ,
    executed_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Withdrawal claims table (emergency withdrawals when service unavailable)
CREATE TABLE IF NOT EXISTS mixer_withdrawal_claims (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES mixer_requests(id),
    account_id TEXT NOT NULL,
    claim_amount TEXT NOT NULL,
    claim_address TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',

    -- On-chain claim
    claim_tx_hash TEXT DEFAULT '',
    claim_block_number BIGINT DEFAULT 0,
    claimable_at TIMESTAMPTZ,

    -- Resolution
    resolution_tx_hash TEXT DEFAULT '',
    resolved_at TIMESTAMPTZ,

    error TEXT DEFAULT '',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Service deposit table (collateral/guarantee)
CREATE TABLE IF NOT EXISTS mixer_service_deposit (
    id TEXT PRIMARY KEY,
    amount TEXT NOT NULL DEFAULT '0',
    locked_amount TEXT NOT NULL DEFAULT '0',
    available_amount TEXT NOT NULL DEFAULT '0',
    wallet_address TEXT NOT NULL,
    last_top_up_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for mixer_requests
CREATE INDEX IF NOT EXISTS idx_mixer_requests_account_id ON mixer_requests(account_id);
CREATE INDEX IF NOT EXISTS idx_mixer_requests_status ON mixer_requests(status);
CREATE INDEX IF NOT EXISTS idx_mixer_requests_zk_proof_hash ON mixer_requests(zk_proof_hash) WHERE zk_proof_hash != '';
CREATE INDEX IF NOT EXISTS idx_mixer_requests_withdrawable_at ON mixer_requests(withdrawable_at) WHERE status NOT IN ('completed', 'refunded');
CREATE INDEX IF NOT EXISTS idx_mixer_requests_created_at ON mixer_requests(created_at DESC);

-- Indexes for mixer_pool_accounts
CREATE INDEX IF NOT EXISTS idx_mixer_pool_accounts_status ON mixer_pool_accounts(status);
CREATE INDEX IF NOT EXISTS idx_mixer_pool_accounts_retire_after ON mixer_pool_accounts(retire_after) WHERE status = 'active';

-- Indexes for mixer_transactions
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_request_id ON mixer_transactions(request_id);
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_from_pool_id ON mixer_transactions(from_pool_id);
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_to_pool_id ON mixer_transactions(to_pool_id);
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_status ON mixer_transactions(status);
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_scheduled_at ON mixer_transactions(scheduled_at) WHERE status = 'scheduled';
CREATE INDEX IF NOT EXISTS idx_mixer_transactions_tx_hash ON mixer_transactions(tx_hash) WHERE tx_hash != '';

-- Indexes for mixer_withdrawal_claims
CREATE INDEX IF NOT EXISTS idx_mixer_withdrawal_claims_request_id ON mixer_withdrawal_claims(request_id);
CREATE INDEX IF NOT EXISTS idx_mixer_withdrawal_claims_account_id ON mixer_withdrawal_claims(account_id);
CREATE INDEX IF NOT EXISTS idx_mixer_withdrawal_claims_status ON mixer_withdrawal_claims(status);
CREATE INDEX IF NOT EXISTS idx_mixer_withdrawal_claims_claimable_at ON mixer_withdrawal_claims(claimable_at) WHERE status = 'pending';

-- Add tenant_id column for multi-tenancy support
ALTER TABLE mixer_requests ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT '';
ALTER TABLE mixer_pool_accounts ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT '';
ALTER TABLE mixer_transactions ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT '';
ALTER TABLE mixer_withdrawal_claims ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_mixer_requests_tenant_id ON mixer_requests(tenant_id) WHERE tenant_id != '';
CREATE INDEX IF NOT EXISTS idx_mixer_pool_accounts_tenant_id ON mixer_pool_accounts(tenant_id) WHERE tenant_id != '';
