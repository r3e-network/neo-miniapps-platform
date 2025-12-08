#!/usr/bin/env bash
# Stop Neo Fairy test environment
set -euo pipefail

echo "Stopping Neo Fairy..."
pkill -f "neo-cli.dll" 2>/dev/null || true
echo "Done"
