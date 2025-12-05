#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f .env ]]; then
  echo "Missing .env. Run ./setup_local.sh first." >&2
  exit 1
fi

# load env
set -a
source ./.env
set +a

BASE_URL="${SITE_URL:-http://localhost:8000}"
ANON="${ANON_KEY:-}"

if [[ -z "${ANON}" ]]; then
  echo "ANON_KEY is empty; check .env" >&2
  exit 1
fi

echo "Checking PostgREST via Kong at ${BASE_URL}/oracle_prices..."
status=$(curl -s -o /tmp/supabase_check.json -w "%{http_code}" \
  -H "apikey: ${ANON}" \
  -H "Authorization: Bearer ${ANON}" \
  "${BASE_URL}/oracle_prices?select=*&limit=1")

echo "HTTP ${status}"
echo "Body:"
cat /tmp/supabase_check.json
echo
