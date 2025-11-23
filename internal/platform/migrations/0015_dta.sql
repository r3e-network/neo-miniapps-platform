-- Chainlink DTA products and orders

CREATE TABLE IF NOT EXISTS chainlink_dta_products (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    settlement_terms TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dta_products_account ON chainlink_dta_products(account_id);

CREATE TABLE IF NOT EXISTS chainlink_dta_orders (
    id UUID PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES app_accounts(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES chainlink_dta_products(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    amount TEXT NOT NULL,
    wallet_address TEXT NOT NULL,
    status TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dta_orders_account ON chainlink_dta_orders(account_id);
CREATE INDEX IF NOT EXISTS idx_dta_orders_product ON chainlink_dta_orders(product_id);
