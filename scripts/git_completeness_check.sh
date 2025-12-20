#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if ! command -v git >/dev/null 2>&1; then
  echo "[SKIP] git not found"
  exit 0
fi

if ! git -C "$PROJECT_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "[SKIP] not a git work tree: $PROJECT_ROOT"
  exit 0
fi

untracked="$(
  git -C "$PROJECT_ROOT" status --porcelain=v1 |
    awk '/^\?\?/ {print substr($0,4)}'
)"

if [[ -z "$untracked" ]]; then
  echo "[OK] no untracked files"
  exit 0
fi

declare -a important=()
while IFS= read -r path; do
  [[ -n "$path" ]] || continue
  case "$path" in
    platform/edge/functions/*|miniapps/*|platform/host-app/public/*|supabase/*|deploy/wallets/*|test/e2e/*|scripts/export_host_miniapps.sh|scripts/export_supabase_functions.sh|services/*/contract/*)
      important+=("$path")
      ;;
  esac
done <<<"$untracked"

if [[ ${#important[@]} -eq 0 ]]; then
  echo "[OK] untracked files exist, but none match canonical source/export paths"
  exit 0
fi

echo "[WARN] untracked files/directories found in canonical source or export paths:"
for p in "${important[@]}"; do
  echo "  - $p"
done

cat <<'EOF'

Suggested next step (review first):
  git add \
    platform/edge/functions \
    miniapps \
    scripts/export_host_miniapps.sh scripts/export_supabase_functions.sh \
    platform/host-app/public \
    supabase \
    deploy/wallets \
    services/*/contract \
    test/e2e
EOF
