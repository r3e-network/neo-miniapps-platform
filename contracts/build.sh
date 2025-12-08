#!/bin/bash
# Build script for Neo N3 Smart Contracts

set -e

echo "Building Neo N3 Smart Contracts..."

# Check for nccs
if ! command -v nccs &> /dev/null; then
    echo "Error: nccs (Neo Contract Compiler) not found"
    echo "Install with: dotnet tool install -g Neo.Compiler.CSharp"
    exit 1
fi

# Create build directory
mkdir -p build

# Build each contract
contracts=(
    "gateway/ServiceLayerGateway"
    "vrf/VRFService"
    "mixer/MixerService"
    "datafeeds/DataFeedsService"
    "automation/AutomationService"
)

# Example contracts
examples=(
    "examples/ExampleConsumer"
    "examples/VRFLottery"
    "examples/MixerClient"
    "examples/DeFiPriceConsumer"
)

echo "=== Building Core Contracts ==="
for contract in "${contracts[@]}"; do
    name=$(basename "$contract")
    dir=$(dirname "$contract")

    echo "Building $name..."

    if [ -f "$contract.cs" ]; then
        nccs "$contract.cs" -o "build/${name}" 2>/dev/null || echo "  Warning: Build may have warnings"
        if [ -f "build/${name}/${name}.nef" ]; then
            mv "build/${name}/${name}.nef" "build/${name}.nef"
            mv "build/${name}/${name}.manifest.json" "build/${name}.manifest.json"
            rm -rf "build/${name}"
            echo "  ✓ $name.nef"
            echo "  ✓ $name.manifest.json"
        else
            echo "  ✗ Compilation failed for $name"
        fi
    else
        echo "  ⚠ $contract.cs not found, skipping"
    fi
done

echo ""
echo "=== Building Example Contracts ==="
for contract in "${examples[@]}"; do
    name=$(basename "$contract")

    echo "Building $name..."

    if [ -f "$contract.cs" ]; then
        nccs "$contract.cs" -o "build/${name}" 2>/dev/null || echo "  Warning: Build may have warnings"
        if [ -f "build/${name}/${name}.nef" ]; then
            mv "build/${name}/${name}.nef" "build/${name}.nef"
            mv "build/${name}/${name}.manifest.json" "build/${name}.manifest.json"
            rm -rf "build/${name}"
            echo "  ✓ $name.nef"
            echo "  ✓ $name.manifest.json"
        else
            echo "  ✗ Compilation failed for $name"
        fi
    else
        echo "  ⚠ $contract.cs not found, skipping"
    fi
done

echo ""
echo "Build complete! Output in ./build/"
ls -la build/ 2>/dev/null || echo "No files in build directory"
