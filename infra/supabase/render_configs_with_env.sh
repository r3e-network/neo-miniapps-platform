#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f .env ]]; then
  echo "Missing .env; run ./setup_local.sh first." >&2
  exit 1
fi

# shellcheck disable=SC1091
source ./.env

if [[ -z "${SERVICE_ROLE_KEY:-}" ]]; then
  echo "SERVICE_ROLE_KEY missing in .env" >&2
  exit 1
fi

render() {
  local src=$1 dst=$2
  jq --arg key "$SERVICE_ROLE_KEY" '.api_key = $key' "$src" >"$dst"
  echo "Rendered $dst with api_key from .env SERVICE_ROLE_KEY"
}

mkdir -p rendered
render configs/oracle_supabase.json rendered/oracle_supabase.json
render configs/datafeeds_supabase.json rendered/datafeeds_supabase.json

echo "Rendered configs are in infra/supabase/rendered/*.json"
