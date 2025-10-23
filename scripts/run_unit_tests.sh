#!/bin/bash

set -euo pipefail

PACKAGES=(
  "./cmd/appserver"
  "./internal/app/..."
  "./internal/config/..."
  "./internal/platform/..."
)

echo "Running unit tests for refactored runtime..."
go test -v "${PACKAGES[@]}" -short

cat <<SUMMARY
Tested the following packages:
$(printf '  - %s\n' "${PACKAGES[@]}")
SUMMARY
