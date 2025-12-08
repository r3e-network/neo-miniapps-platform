-- Optional index to accelerate deposit lookups by tx_hash.
create index if not exists mixer_requests_tx_hash_idx on public.mixer_requests (deposit_tx_hash);
