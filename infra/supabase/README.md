# Self-hosted Supabase (TEE-friendly)

This repo ships a lightweight Supabase-style stack (Postgres + PostgREST + GoTrue + Storage + Realtime + Kong proxy) for local/self-hosted use. All secrets/keys stay out of env once deployed; services load sealed configs inside the TEE.

## Quick start
1) Create `.env` in `infra/supabase/` with values (see below).
2) From `infra/supabase/`: `docker compose up -d`
3) APIs:
   - Kong proxy: http://localhost:8000 (routes to rest/auth/storage/realtime)
   - PostgREST direct: http://localhost:3000
   - Auth: http://localhost:9999
   - Storage: http://localhost:5000
   - Realtime: ws://localhost:4000
   - DB: localhost:54322 (postgres)
   - Studio: http://localhost:54323 (optional admin UI)

## .env template
```
POSTGRES_PASSWORD=supabase
POSTGRES_USER=supabase_admin
POSTGRES_DB=postgres
JWT_SECRET=super-secret-jwt
ANON_KEY=change-me   # must be a valid JWT signed with JWT_SECRET
SERVICE_ROLE_KEY=change-me  # must be a valid JWT signed with JWT_SECRET
SITE_URL=http://localhost:8000
```

If you regenerate the JWTs, keep `ANON_KEY` (role `anon`) and `SERVICE_ROLE_KEY` (role `service_role`) consistent with `JWT_SECRET`. A helper script rewrites `.env` with valid tokens:
```
cd infra/supabase && ./setup_local.sh   # if not already run
```

To embed the service role key directly into the Supabase configs (avoids secret store for local dev):
```
cd infra/supabase && ./render_configs_with_env.sh
```
Rendered configs will appear under `infra/supabase/rendered/*.json` with `api_key` populated from `.env`.

To seal the rendered configs and service role key into the enclave-backed storage (so services can load Supabase configs/secrets automatically), use the seeder:
```
# seeds into sealed storage (defaults: ./sealed_store and ./sealing.key)
go run ./cmd/seed_supabase \
  -storage-path ./sealed_store \
  -sealing-key ./sealing.key
```
This writes:
- secret `supabase/api_key` for namespaces `oracle` and `datafeeds`
- config blobs `oracle/supabase` and `datafeeds/supabase` from `infra/supabase/rendered/*.json`

## Schema
`init/` seeds roles and two tables:
- `oracle_prices(symbol, price, volume, source, fetched_at)`
- `datafeed_prices(symbol, price, sources, confidence, fetched_at)`

Adjust/add tables in `init/02_tables.sql` as needed.

## Connecting services (TEE)
- Use `project_url = http://localhost:8000` (or your host) in sealed configs.
- Store the service role key as a secret (e.g., `supabase/api_key`) in the service namespace.
- `allowed_hosts`: set to your host (e.g., `localhost`) in sealed config if you want explicit allowlists; otherwise derived from the URL by the Supabase client.
- Oracle/DataFeeds sample sealed configs: `configs/oracle_supabase.json`, `configs/datafeeds_supabase.json`. Store them as sealed Storage keys `oracle/supabase` and `datafeeds/supabase` after adjusting host/keys.

## Verify
- Quick REST check via Kong: `cd infra/supabase && ./health.sh` (uses `.env` ANON_KEY and SITE_URL).
- DB check: `docker exec supabase-db psql -U supabase_admin -d postgres -c '\dt auth.*'`

## Notes
- This is a minimal stack; for production, harden JWT/keys, SSL termination, and RLS policies.
- Kong config (`kong/kong.yml`) is declarative; update routes if you change service ports/paths.
