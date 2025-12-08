#!/bin/bash
# Deploy all Service Layer contracts to Neo Express or TestNet
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/contracts/build"
CONFIG_DIR="$PROJECT_ROOT/deploy/config"
DEPLOYED_FILE="$CONFIG_DIR/deployed_contracts.json"

# Network selection
NETWORK=${1:-neoexpress}
NEOEXPRESS_CONFIG="$CONFIG_DIR/default.neo-express"

echo "=== Service Layer Contract Deployment ==="
echo "Network: $NETWORK"
echo "Build directory: $BUILD_DIR"

# Check if contracts are built
if [ ! -d "$BUILD_DIR" ] || [ -z "$(ls -A $BUILD_DIR/*.nef 2>/dev/null)" ]; then
    echo "Error: No built contracts found in $BUILD_DIR"
    echo "Run setup_neoexpress.sh first to build contracts"
    exit 1
fi

# Initialize deployed contracts file
echo "{}" > "$DEPLOYED_FILE"

deploy_contract() {
    local name=$1
    local nef_path="$BUILD_DIR/${name}.nef"
    local manifest_path="$BUILD_DIR/${name}.manifest.json"

    if [ ! -f "$nef_path" ]; then
        echo "  Skipping $name (not built)"
        return
    fi

    echo "Deploying $name..."

    if [ "$NETWORK" = "neoexpress" ]; then
        # Deploy to Neo Express
        result=$(neoxp contract deploy "$nef_path" owner -i "$NEOEXPRESS_CONFIG" 2>&1)
        # Extract contract hash from output
        hash=$(echo "$result" | grep -oP '0x[a-fA-F0-9]{40}' | head -1 || echo "")

        if [ -n "$hash" ]; then
            echo "  Deployed: $hash"
            # Update deployed contracts file
            jq --arg name "$name" --arg hash "$hash" '.[$name] = $hash' "$DEPLOYED_FILE" > "$DEPLOYED_FILE.tmp" && mv "$DEPLOYED_FILE.tmp" "$DEPLOYED_FILE"
        else
            echo "  Warning: Could not extract contract hash"
            echo "  Output: $result"
        fi
    else
        echo "  TestNet deployment requires manual signing"
        echo "  Use: neo-go contract deploy -i $nef_path -m $manifest_path --rpc-endpoint <RPC_URL>"
    fi
}

# Deploy core contracts in order
echo ""
echo "=== Deploying Core Contracts ==="
deploy_contract "ServiceLayerGateway"
deploy_contract "VRFService"
deploy_contract "MixerService"
deploy_contract "DataFeedsService"
deploy_contract "AutomationService"

# Deploy example contracts
echo ""
echo "=== Deploying Example Contracts ==="
deploy_contract "ExampleConsumer"
deploy_contract "VRFLottery"
deploy_contract "MixerClient"
deploy_contract "DeFiPriceConsumer"

echo ""
echo "=== Deployment Complete ==="
echo "Deployed contracts saved to: $DEPLOYED_FILE"
cat "$DEPLOYED_FILE"

echo ""
echo "Next step: Initialize contracts with:"
echo "  python3 deploy/scripts/initialize.py $NETWORK"
