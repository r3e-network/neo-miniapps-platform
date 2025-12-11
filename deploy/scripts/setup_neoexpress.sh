#!/bin/bash
# Setup Neo Express environment for Service Layer development
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$PROJECT_ROOT/deploy"
WALLETS_DIR="$DEPLOY_DIR/wallets"
CONFIG_DIR="$DEPLOY_DIR/config"

echo "=== Service Layer Neo Express Setup ==="
echo "Project root: $PROJECT_ROOT"

# Check dependencies
check_dependency() {
    if ! command -v $1 &> /dev/null; then
        echo "Error: $1 is not installed"
        echo "Install with: $2"
        exit 1
    fi
}

check_dependency "neoxp" "dotnet tool install -g Neo.Express"
check_dependency "nccs" "dotnet tool install -g Neo.Compiler.CSharp"

# Create directories
mkdir -p "$WALLETS_DIR" "$CONFIG_DIR"

# Initialize Neo Express if not exists
NEOEXPRESS_CONFIG="$CONFIG_DIR/default.neo-express"
if [ ! -f "$NEOEXPRESS_CONFIG" ]; then
    echo "Initializing Neo Express configuration..."
    neoxp create -o "$NEOEXPRESS_CONFIG" -f
fi

# Create wallets if not exist
create_wallet() {
    local name=$1
    local wallet_path="$WALLETS_DIR/${name}.json"

    if [ ! -f "$wallet_path" ]; then
        echo "Creating $name wallet..."
        neoxp wallet create "$name" -i "$NEOEXPRESS_CONFIG"
    else
        echo "Wallet $name already exists"
    fi
}

create_wallet "owner"
create_wallet "tee"
create_wallet "user"

# Fund wallets from genesis
echo "Funding wallets from genesis..."
neoxp transfer 1000 GAS genesis owner -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
neoxp transfer 100 NEO genesis owner -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
neoxp transfer 500 GAS genesis tee -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true
neoxp transfer 100 GAS genesis user -i "$NEOEXPRESS_CONFIG" 2>/dev/null || true

# Build contracts
echo ""
echo "Building contracts..."
cd "$PROJECT_ROOT/contracts"

# Create build directory
mkdir -p build

# Build core contracts
CONTRACTS=(
    "gateway/ServiceLayerGateway"
    "vrf/VRFService"
    "mixer/NeoVaultService"
    "datafeeds/DataFeedsService"
    "automation/NeoFlowService"
)

for contract in "${CONTRACTS[@]}"; do
    name=$(basename "$contract")
    if [ -f "${contract}.cs" ]; then
        echo "Building $name..."
        nccs "${contract}.cs" -o "build/${name}.nef" 2>/dev/null || echo "  Warning: Build may have warnings"
    fi
done

# Build example contracts
EXAMPLES=(
    "examples/ExampleConsumer"
    "examples/VRFLottery"
    "examples/MixerClient"
    "examples/DeFiPriceConsumer"
)

for contract in "${EXAMPLES[@]}"; do
    name=$(basename "$contract")
    if [ -f "${contract}.cs" ]; then
        echo "Building $name..."
        nccs "${contract}.cs" -o "build/${name}.nef" 2>/dev/null || echo "  Warning: Build may have warnings"
    fi
done

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
