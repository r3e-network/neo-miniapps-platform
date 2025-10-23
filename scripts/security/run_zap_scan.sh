#!/bin/bash

# run_zap_scan.sh - Script to run OWASP ZAP for API security scanning
# This script performs automated security scanning of API endpoints

set -e

echo "===== Running OWASP ZAP API Security Scan ====="

# Define output directory
OUTPUT_DIR="./security-reports"
mkdir -p "$OUTPUT_DIR"

# Default target URL (local API server)
TARGET_URL="http://localhost:8080"

# ZAP Docker image
ZAP_IMAGE="owasp/zap2docker-stable"

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -t|--target) TARGET_URL="$2"; shift ;;
        -s|--swagger) SWAGGER_URL="$2"; shift ;;
        *) echo "Unknown parameter: $1"; exit 1 ;;
    esac
    shift
done

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Docker not found. Please install Docker to run ZAP scans."
    exit 1
fi

# Pull the ZAP Docker image
echo "Pulling latest ZAP Docker image..."
docker pull "$ZAP_IMAGE"

# Check if we need to start the API server
if [[ "$TARGET_URL" == "http://localhost"* ]]; then
    # Check if the server is already running
    if ! curl -s "$TARGET_URL/health" > /dev/null; then
        echo "Local API server not detected. Please start the API server before running this script."
        echo "You can use: make run"
        echo "Alternatively, specify a different target URL with -t or --target"
        exit 1
    fi

    echo "Local API server detected at $TARGET_URL"
fi

# Determine if we have a Swagger/OpenAPI specification
if [ -n "$SWAGGER_URL" ]; then
    echo "Running API scan with OpenAPI specification at $SWAGGER_URL..."
    docker run --rm -v "$(pwd)/$OUTPUT_DIR:/zap/wrk" $ZAP_IMAGE zap-api-scan.py -t "$SWAGGER_URL" -f openapi -r zap-api-report.html
else
    echo "No OpenAPI specification found. Running baseline scan against $TARGET_URL..."
    docker run --rm -v "$(pwd)/$OUTPUT_DIR:/zap/wrk" $ZAP_IMAGE zap-baseline.py -t "$TARGET_URL" -r zap-baseline-report.html
fi

# Generate additional report formats
echo "Generating JSON report..."
docker run --rm -v "$(pwd)/$OUTPUT_DIR:/zap/wrk" $ZAP_IMAGE zap-cli report -o /zap/wrk/zap-report.json -f json

echo "===== ZAP Security Scanning Complete ====="

# Check if high severity issues were found
if grep -q "High" "$OUTPUT_DIR/zap-report.json"; then
    echo "WARNING: High severity security issues found!"
    jq '.site[].alerts[] | select(.risk == "High") | {risk: .risk, name: .name, url: .instances[0].uri}' "$OUTPUT_DIR/zap-report.json"
    exit 1
else
    echo "No high severity security issues found."
    exit 0
fi
