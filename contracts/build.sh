#!/bin/bash
# Build script for Neo N3 Smart Contracts

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$SCRIPT_DIR"

echo "Building Neo N3 Smart Contracts..."

# Ensure dotnet runtime can be resolved when tools are installed under ~/.dotnet.
if [ -z "${DOTNET_ROOT:-}" ] && [ -x "${HOME}/.dotnet/dotnet" ]; then
    export DOTNET_ROOT="${HOME}/.dotnet"
fi
if [ -n "${DOTNET_ROOT:-}" ]; then
    export PATH="${DOTNET_ROOT}:$PATH"
fi

# Resolve nccs (Neo Contract Compiler). Dotnet tools are commonly installed under ~/.dotnet/tools
# and that path may not be in $PATH in CI/containers.
NCCS_BIN="${NCCS_BIN:-}"
if [ -z "$NCCS_BIN" ]; then
    if command -v nccs &> /dev/null; then
        NCCS_BIN="nccs"
    elif [ -x "${HOME}/.dotnet/tools/nccs" ]; then
        NCCS_BIN="${HOME}/.dotnet/tools/nccs"
    else
        echo "Error: nccs (Neo Contract Compiler) not found"
        echo "Install with: dotnet tool install -g Neo.Compiler.CSharp"
        echo "Then ensure ~/.dotnet/tools is on PATH (or set NCCS_BIN)."
        exit 1
    fi
fi

# Create build directory
mkdir -p build
rm -rf build/*

failures=0

collect_artifacts() {
    local outdir=$1

    mapfile -t nef_files < <(find "$outdir" -maxdepth 1 -name "*.nef" -type f | sort)
    mapfile -t manifest_files < <(find "$outdir" -maxdepth 1 -name "*.manifest.json" -type f | sort)

    if [ "${#nef_files[@]}" -eq 0 ] || [ "${#manifest_files[@]}" -eq 0 ]; then
        return 1
    fi

    for nef_file in "${nef_files[@]}"; do
        local base
        base=$(basename "$nef_file" .nef)
        mv "$nef_file" "build/${base}.nef"
        echo "  ✓ ${base}.nef"
    done

    for manifest_file in "${manifest_files[@]}"; do
        local base
        base=$(basename "$manifest_file" .manifest.json)
        mv "$manifest_file" "build/${base}.manifest.json"
        echo "  ✓ ${base}.manifest.json"
    done

    return 0
}

build_sources() {
    local label=$1
    local outdir=$2
    shift 2

    local sources=("$@")
    if [ "${#sources[@]}" -eq 0 ]; then
        echo "  ⚠ No sources found for $label, skipping"
        return 0
    fi

    mkdir -p "$outdir"
    echo "Building $label..."

    if ! "$NCCS_BIN" "${sources[@]}" -o "$outdir"; then
        echo "  ✗ Compilation failed for $label"
        failures=$((failures + 1))
        rm -rf "$outdir"
        return 0
    fi

    if ! collect_artifacts "$outdir"; then
        echo "  ✗ Missing artifacts for $label"
        failures=$((failures + 1))
    fi

    rm -rf "$outdir"
    return 0
}

# Service contracts (split into multiple partial class files)
# Format: "directory:ContractName"
service_contracts=(
    "../services/conforacle/contract:NeoOracleService"
    "../services/datafeed/contract:NeoFeedsService"
    "../services/automation/contract:NeoFlowService"
    "../services/confcompute/contract:NeoComputeService"
)

# Gateway contract (multi-file partial class)
# Format: "directory:MainContractName"
gateway_contracts=(
    "gateway:ServiceLayerGateway"
)

# Platform contracts (single-file)
# Format: "directory:ContractName"
platform_contracts=(
    "PaymentHub:PaymentHub"
    "Governance:Governance"
    "PriceFeed:PriceFeed"
    "RandomnessLog:RandomnessLog"
    "AppRegistry:AppRegistry"
    "AutomationAnchor:AutomationAnchor"
)

# Example contracts
examples=(
    "examples/ExampleConsumer"
    "examples/DeFiPriceConsumer"
)

echo "=== Building Gateway Contract ==="
for entry in "${gateway_contracts[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    if [ -d "$dir" ]; then
        cs_files=$(find "$dir" -maxdepth 1 -name "*.cs" -type f | sort)
        # shellcheck disable=SC2086
        build_sources "$name" "build/${name}" $cs_files
    else
        echo "  ⚠ Directory $dir not found, skipping"
    fi
done

echo ""
echo "=== Building Service Contracts (Multi-file) ==="
for entry in "${service_contracts[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    if [ -d "$dir" ]; then
        # Include shared contract base types used by multiple service contracts.
        common_files=$(find "common" -maxdepth 1 -name "*.cs" -type f | sort)

        # Collect all .cs files in the contract directory
        cs_files=$(find "$dir" -maxdepth 1 -name "${name}*.cs" -type f | sort)
        # shellcheck disable=SC2086
        build_sources "$name" "build/${name}" $common_files $cs_files
    else
        echo "  ⚠ Directory $dir not found, skipping"
    fi
done

echo ""
echo "=== Building Platform Contracts ==="
for entry in "${platform_contracts[@]}"; do
    dir="${entry%%:*}"
    name="${entry##*:}"

    if [ -d "$dir" ]; then
        cs_files=$(find "$dir" -maxdepth 1 -name "*.cs" -type f | sort)
        # shellcheck disable=SC2086
        build_sources "$name" "build/${name}" $cs_files
    else
        echo "  ⚠ Directory $dir not found, skipping"
    fi
done

echo ""
echo "=== Building Example Contracts ==="
for contract in "${examples[@]}"; do
    name=$(basename "$contract")

    if [ -f "$contract.cs" ]; then
        build_sources "$name" "build/${name}" "$contract.cs"
    else
        echo "  ⚠ $contract.cs not found, skipping"
    fi
done

echo ""
echo "Build complete! Output in ./build/"
ls -la build/ 2>/dev/null || echo "No files in build directory"

if [ "$failures" -ne 0 ]; then
    echo ""
    echo "Build finished with ${failures} failure(s)."
    exit 1
fi
