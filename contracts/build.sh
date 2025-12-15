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
    "../services/neooracle/contract:NeoOracleService"
    "../services/neorand/contract:NeoRandService"
    "../services/neovault/contract:NeoVaultService"
    "../services/neofeeds/contract:NeoFeedsService"
    "../services/neoflow/contract:NeoFlowService"
    "../services/neocompute/contract:NeoComputeService"
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
        outdir="build/${name}"
        mkdir -p "$outdir"
        nccs "$contract.cs" -o "$outdir" 2>/dev/null || echo "  Warning: Build may have warnings"

        nef_file=$(find "$outdir" -maxdepth 1 -name "*.nef" -type f | head -n 1)
        manifest_file=$(find "$outdir" -maxdepth 1 -name "*.manifest.json" -type f | head -n 1)
        if [ -n "$nef_file" ] && [ -n "$manifest_file" ] && [ -f "$nef_file" ] && [ -f "$manifest_file" ]; then
            mv "$nef_file" "build/${name}.nef"
            mv "$manifest_file" "build/${name}.manifest.json"
            rm -rf "$outdir"
            echo "  ✓ ${name}.nef"
            echo "  ✓ ${name}.manifest.json"
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

    out_name="$name"
    main_file="${dir}/${name}.cs"
    if [ -f "$main_file" ]; then
        display_name=$(sed -n 's/.*\\[DisplayName(\"\\([^\"]*\\)\"\\)\\].*/\\1/p' "$main_file" | head -n 1)
        if [ -n "$display_name" ]; then
            out_name="$display_name"
        fi
    fi

    echo "Building $out_name..."

    if [ -d "$dir" ]; then
        # Collect all .cs files in the contract directory
        cs_files=$(find "$dir" -maxdepth 1 -name "${name}*.cs" -type f | sort)

        if [ -n "$cs_files" ]; then
            # Pass all .cs files to nccs
            outdir="build/${out_name}"
            mkdir -p "$outdir"
            nccs $cs_files -o "$outdir" 2>/dev/null || echo "  Warning: Build may have warnings"

            nef_file=$(find "$outdir" -maxdepth 1 -name "*.nef" -type f | head -n 1)
            manifest_file=$(find "$outdir" -maxdepth 1 -name "*.manifest.json" -type f | head -n 1)
            if [ -n "$nef_file" ] && [ -n "$manifest_file" ] && [ -f "$nef_file" ] && [ -f "$manifest_file" ]; then
                mv "$nef_file" "build/${out_name}.nef"
                mv "$manifest_file" "build/${out_name}.manifest.json"
                rm -rf "$outdir"
                echo "  ✓ ${out_name}.nef"
                echo "  ✓ ${out_name}.manifest.json"
            else
                echo "  ✗ Compilation failed for $out_name"
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
        outdir="build/${name}"
        mkdir -p "$outdir"
        nccs "$contract.cs" -o "$outdir" 2>/dev/null || echo "  Warning: Build may have warnings"

        nef_file=$(find "$outdir" -maxdepth 1 -name "*.nef" -type f | head -n 1)
        manifest_file=$(find "$outdir" -maxdepth 1 -name "*.manifest.json" -type f | head -n 1)
        if [ -n "$nef_file" ] && [ -n "$manifest_file" ] && [ -f "$nef_file" ] && [ -f "$manifest_file" ]; then
            mv "$nef_file" "build/${name}.nef"
            mv "$manifest_file" "build/${name}.manifest.json"
            rm -rf "$outdir"
            echo "  ✓ ${name}.nef"
            echo "  ✓ ${name}.manifest.json"
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
