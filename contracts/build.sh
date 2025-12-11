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

# Service contracts (split into multiple partial class files)
# Format: "directory:ContractName"
service_contracts=(
    "../services/oracle/contract:OracleService"
    "../services/vrf/contract:VRFService"
    "../services/mixer/contract:NeoVaultService"
    "../services/datafeeds/contract:DataFeedsService"
    "../services/automation/contract:NeoFlowService"
    "../services/confidential/contract:ConfidentialService"
)

# Single-file contracts
single_contracts=(
    "gateway/ServiceLayerGateway"
)

# Example contracts
examples=(
    "examples/ExampleConsumer"
    "examples/VRFLottery"
    "examples/MixerClient"
    "examples/DeFiPriceConsumer"
)

echo "=== Building Gateway Contract ==="
for contract in "${single_contracts[@]}"; do
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
echo "=== Building Service Contracts (Multi-file) ==="
for entry in "${service_contracts[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    echo "Building $name..."

    if [ -d "$dir" ]; then
        # Collect all .cs files in the contract directory
        cs_files=$(find "$dir" -maxdepth 1 -name "${name}*.cs" -type f | sort)

        if [ -n "$cs_files" ]; then
            # Pass all .cs files to nccs
            nccs $cs_files -o "build/${name}" 2>/dev/null || echo "  Warning: Build may have warnings"
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
            echo "  ⚠ No .cs files found in $dir for $name"
        fi
    else
        echo "  ⚠ Directory $dir not found, skipping"
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
