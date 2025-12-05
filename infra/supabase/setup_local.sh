#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

ENV_FILE=".env"

rand() {
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c "${1:-32}"
}

if [[ -f "${ENV_FILE}" ]]; then
  echo "⚠️  ${ENV_FILE} already exists; refusing to overwrite."
  echo "    Remove it if you want to regenerate."
  exit 0
fi

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-$(rand 24)}"
POSTGRES_USER="${POSTGRES_USER:-supabase_admin}"
POSTGRES_DB="${POSTGRES_DB:-postgres}"
JWT_SECRET="${JWT_SECRET:-$(rand 48)}"
ANON_KEY="${ANON_KEY:-anon_$(rand 24)}"
SERVICE_ROLE_KEY="${SERVICE_ROLE_KEY:-service_$(rand 32)}"
SITE_URL="${SITE_URL:-http://localhost:8000}"

cat >"${ENV_FILE}" <<EOF
POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
POSTGRES_USER=${POSTGRES_USER}
POSTGRES_DB=${POSTGRES_DB}
JWT_SECRET=${JWT_SECRET}
ANON_KEY=${ANON_KEY}
SERVICE_ROLE_KEY=${SERVICE_ROLE_KEY}
SITE_URL=${SITE_URL}
EOF

echo "✅ Generated ${ENV_FILE} with random secrets."
echo "Next steps:"
echo "1) cd infra/supabase && docker compose up -d"
echo "2) Use project_url=http://localhost:8000 and api_key_secret=supabase/api_key in sealed configs:"
echo "   - oracle:   store configs/oracle_supabase.json as sealed key oracle/supabase"
echo "   - datafeeds: store configs/datafeeds_supabase.json as sealed key datafeeds/supabase"
echo "3) Store the SERVICE_ROLE_KEY from ${ENV_FILE} as secret 'supabase/api_key' in each service namespace."
