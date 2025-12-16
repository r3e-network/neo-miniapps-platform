#!/bin/bash
# Setup Neo Express environment for Service Layer development
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$PROJECT_ROOT/deploy"
WALLETS_DIR="$DEPLOY_DIR/wallets"
CONFIG_DIR="$DEPLOY_DIR/config"

echo "=== Service Layer Neo Express Setup ==="
echo "Project root: $PROJECT_ROOT"

# Ensure dotnet runtime can be resolved when installed under ~/.dotnet (common in CI/containers).
if [ -z "${DOTNET_ROOT:-}" ] && [ -x "${HOME}/.dotnet/dotnet" ]; then
    export DOTNET_ROOT="${HOME}/.dotnet"
fi
if [ -n "${DOTNET_ROOT:-}" ]; then
    export PATH="${DOTNET_ROOT}:$PATH"
fi

# Resolve dotnet-tool style binaries. In CI/containers ~/.dotnet/tools may not be on PATH.
resolve_tool() {
    local name="$1"
    local install_hint="$2"

    local resolved=""
    resolved="$(command -v "$name" 2>/dev/null || true)"
    if [ -n "$resolved" ]; then
        echo "$resolved"
        return 0
    fi

    local dotnet_tool="${HOME}/.dotnet/tools/${name}"
    if [ -x "$dotnet_tool" ]; then
        echo "$dotnet_tool"
        return 0
    fi

    echo "Error: ${name} is not installed" >&2
    echo "Install with: ${install_hint}" >&2
    echo "Then ensure ~/.dotnet/tools is on PATH." >&2
    exit 1
}

NEOXP="$(resolve_tool "neoxp" "dotnet tool install -g Neo.Express")"
NCCS="$(resolve_tool "nccs" "dotnet tool install -g Neo.Compiler.CSharp")"

# Create directories
mkdir -p "$WALLETS_DIR" "$CONFIG_DIR"

# Initialize Neo Express if not exists
NEOEXPRESS_CONFIG="$CONFIG_DIR/default.neo-express"
if [ ! -f "$NEOEXPRESS_CONFIG" ]; then
    echo "Initializing Neo Express configuration..."
    "$NEOXP" create -o "$NEOEXPRESS_CONFIG" -f
fi

# Create wallets if not exist
create_wallet() {
    local name=$1

    echo "Ensuring $name wallet..."
    if out=$("$NEOXP" wallet create "$name" -i "$NEOEXPRESS_CONFIG" 2>&1); then
        echo "Created wallet $name"
        return 0
    fi

    if echo "$out" | grep -qi "already exists"; then
        echo "Wallet $name already exists"
        return 0
    fi

    echo "Failed to create wallet $name:" >&2
    echo "$out" >&2
    exit 1
}

create_wallet "owner"
create_wallet "tee"
create_wallet "user"

# Fund wallets from genesis
echo "Funding wallets from genesis..."
"$NEOXP" transfer 1000 GAS genesis owner -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
"$NEOXP" transfer 100 NEO genesis owner -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
"$NEOXP" transfer 500 GAS genesis tee -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
"$NEOXP" transfer 100 GAS genesis user -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true

# Build contracts
echo ""
echo "Building contracts..."
cd "$PROJECT_ROOT/contracts"
NCCS_BIN="$NCCS" ./build.sh

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Neo Express config: $NEOEXPRESS_CONFIG"
echo "Wallets directory: $WALLETS_DIR"
echo "Contract builds: $PROJECT_ROOT/contracts/build/"
echo ""
echo "Next steps:"
echo "  1. Start Neo Express: neoxp run -i $NEOEXPRESS_CONFIG"
echo "  2. Deploy contracts: ./deploy/scripts/deploy_all.sh"
echo "  3. Initialize contracts: python3 deploy/scripts/initialize.py"
