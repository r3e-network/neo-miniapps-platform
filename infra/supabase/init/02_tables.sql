-- Oracle prices table
create table if not exists oracle_prices (
    id serial primary key,
    symbol text not null,
    price numeric not null,
    volume numeric,
    source text,
    fetched_at timestamptz not null default now()
);
create index if not exists idx_oracle_prices_symbol on oracle_prices(symbol);

-- Datafeeds aggregated prices
create table if not exists datafeed_prices (
    id serial primary key,
    symbol text not null,
    price numeric not null,
    sources text[],
    confidence numeric,
    fetched_at timestamptz not null default now()
);
create index if not exists idx_datafeed_prices_symbol on datafeed_prices(symbol);
