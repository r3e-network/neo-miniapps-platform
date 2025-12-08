#!/usr/bin/env bash
# Start Neo Fairy test environment for service_layer contracts
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_LAYER_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
NEO_ROOT="${NEOROOT:-/home/neo/git/neo}"
NEO_CLI="$NEO_ROOT/bin/Neo.CLI/net10.0"

# Copy Fairy config
mkdir -p "$NEO_CLI/Plugins/Fairy"
cp "$SCRIPT_DIR/config.json" "$NEO_CLI/Plugins/Fairy/"

echo "Starting Neo Fairy test environment..."
echo "  Neo CLI: $NEO_CLI"
echo "  Service Layer: $SERVICE_LAYER_ROOT"
echo "  Fairy RPC: http://127.0.0.1:16868"
echo ""

cd "$NEO_CLI"

# Check if already running
if curl -s -X POST http://127.0.0.1:16868 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"hellofairy","params":[],"id":1}' 2>/dev/null | grep -q "result"; then
    echo "Fairy already running at http://127.0.0.1:16868"
    exit 0
fi

# Start neo-cli with Fairy (use script to provide pseudo-terminal)
echo "Starting neo-cli with Fairy plugin..."
nohup script -q -c "dotnet neo-cli.dll" /dev/null > /tmp/fairy.log 2>&1 &
echo $! > /tmp/fairy.pid

# Wait for startup
echo "Waiting for Fairy to start..."
for i in {1..30}; do
    if curl -s -X POST http://127.0.0.1:16868 \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"hellofairy","params":[],"id":1}' 2>/dev/null | grep -q "result"; then
        echo "Fairy is ready!"
        break
    fi
    sleep 1
done

echo ""
echo "Test Fairy with:"
echo "  curl -X POST http://127.0.0.1:16868 -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"method\":\"hellofairy\",\"params\":[],\"id\":1}'"
echo ""
echo "Run tests with:"
echo "  go test -v ./test/fairy/..."
