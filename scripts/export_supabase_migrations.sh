#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SRC_DIR="$PROJECT_ROOT/migrations"
DEST_DIR="$PROJECT_ROOT/supabase/migrations"
BASE_VERSION="${SUPABASE_MIGRATIONS_BASE_VERSION:-20000101000000}"

if [[ ! -d "$SRC_DIR" ]]; then
  echo "ERROR: source migrations directory not found: $SRC_DIR" >&2
  exit 1
fi

mkdir -p "$DEST_DIR"

echo "Exporting migrations:"
echo "  from: $SRC_DIR"
echo "    to: $DEST_DIR"

if [[ ! "$BASE_VERSION" =~ ^[0-9]{14}$ ]]; then
  echo "ERROR: SUPABASE_MIGRATIONS_BASE_VERSION must be 14 digits (got: $BASE_VERSION)" >&2
  exit 1
fi

# Supabase CLI expects migration filenames to start with a 14-digit version, e.g.
# `20241218093000_init.sql`. The canonical source migrations in this repo use
# `NNN_name.sql` for readability. We export into a Supabase-compatible layout
# by mapping:
#   001_initial_schema.sql -> 20000101000001_initial_schema.sql
#   024_rate_limit_bump.sql -> 20000101000024_rate_limit_bump.sql
#
# The mapping is deterministic based on the numeric prefix.

# Keep tracked layout in dest, but remove previously-exported SQL files.
find "$DEST_DIR" -maxdepth 1 -type f -name "*.sql" -delete

copied=0
shopt -s nullglob
for src in "$SRC_DIR"/[0-9][0-9][0-9]_*.sql; do
  base="$(basename "$src")"
  prefix="${base%%_*}"
  rest="${base#*_}"

  if [[ ! "$prefix" =~ ^[0-9]{3}$ ]]; then
    echo "WARN: skipping migration with unexpected name: $base" >&2
    continue
  fi

  # Shell arithmetic: treat as base-10 even if there are leading zeros.
  n="$((10#$prefix))"
  version="$((10#$BASE_VERSION + n))"
  version="$(printf "%014d" "$version")"

  dest="$DEST_DIR/${version}_${rest}"
  cp -f "$src" "$dest"
  chmod 0644 "$dest" || true
  copied=$((copied + 1))
done

if [[ $copied -eq 0 ]]; then
  echo "WARN: no migrations were exported (expected files like 001_name.sql)" >&2
fi

echo "Done."
