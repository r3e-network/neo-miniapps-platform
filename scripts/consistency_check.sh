#!/bin/bash
# Consistency Validation Script
# Validates code style, configuration, and structural consistency across the codebase

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

echo "========================================"
echo "  Consistency Validation Check"
echo "========================================"
echo ""
echo "Project: $PROJECT_ROOT"
echo ""

# -----------------------------------------------------------------------------
# Go Code Formatting
# -----------------------------------------------------------------------------
check_go_formatting() {
    echo -e "${BLUE}=== Go Code Formatting ===${NC}"

    local unformatted=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec gofmt -l {} \; 2>/dev/null)

    if [[ -n "$unformatted" ]]; then
        echo -e "${RED}[ERROR] Unformatted Go files:${NC}"
        echo "$unformatted" | while read -r file; do
            echo "  ${file#$PROJECT_ROOT/}"
        done
        echo ""
        echo "  Run: gofmt -w <file>"
        ERRORS=$((ERRORS + $(echo "$unformatted" | wc -l)))
    else
        echo -e "${GREEN}[OK] All Go files are properly formatted${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Go Vet
# -----------------------------------------------------------------------------
check_go_vet() {
    echo -e "${BLUE}=== Go Vet Analysis ===${NC}"

    local vet_output
    vet_output=$(cd "$PROJECT_ROOT" && go vet ./... 2>&1) || true

    if [[ -n "$vet_output" ]]; then
        echo -e "${RED}[ERROR] Go vet issues found:${NC}"
        echo "$vet_output" | head -20
        ERRORS=$((ERRORS + 1))
    else
        echo -e "${GREEN}[OK] No go vet issues${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Go Module Consistency
# -----------------------------------------------------------------------------
check_go_mod() {
    echo -e "${BLUE}=== Go Module Consistency ===${NC}"

    cd "$PROJECT_ROOT"

    # Check if go.mod is tidy
    cp go.sum go.sum.backup 2>/dev/null || true
    go mod tidy 2>/dev/null

    if ! diff -q go.sum go.sum.backup >/dev/null 2>&1; then
        echo -e "${YELLOW}[WARNING] go.sum changed after 'go mod tidy'${NC}"
        echo "  Run: go mod tidy"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}[OK] Go modules are tidy${NC}"
    fi

    mv go.sum.backup go.sum 2>/dev/null || true
    echo ""
}

# -----------------------------------------------------------------------------
# Import Consistency
# -----------------------------------------------------------------------------
check_import_consistency() {
    echo -e "${BLUE}=== Import Consistency ===${NC}"

    # Check for mixed import styles (should use project module path)
    local module_name=$(grep "^module " "$PROJECT_ROOT/go.mod" | cut -d' ' -f2)

    # Find files importing with wrong paths
    local bad_imports=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec grep -l "\"service_layer/" {} \; 2>/dev/null | head -5)

    if [[ -n "$bad_imports" ]]; then
        echo -e "${YELLOW}[WARNING] Files with potentially inconsistent imports:${NC}"
        echo "$bad_imports" | while read -r file; do
            echo "  ${file#$PROJECT_ROOT/}"
        done
        echo "  Expected module: $module_name"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}[OK] Import paths are consistent${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Configuration Consistency
# -----------------------------------------------------------------------------
check_config_consistency() {
    echo -e "${BLUE}=== Configuration Consistency ===${NC}"

    local config_dir="$PROJECT_ROOT/deploy/config"
    local issues=0

    if [[ -d "$config_dir" ]]; then
        # Check that all environment configs have same keys
        local base_keys=""
        local first_file=""

        for config in "$config_dir"/*.json; do
            [[ -f "$config" ]] || continue
            local filename=$(basename "$config")

            # Extract top-level keys
            local keys=$(jq -r 'keys[]' "$config" 2>/dev/null | sort | tr '\n' ' ')

            if [[ -z "$first_file" ]]; then
                first_file="$filename"
                base_keys="$keys"
            elif [[ "$keys" != "$base_keys" ]]; then
                echo -e "${YELLOW}[WARNING] Config key mismatch: $filename vs $first_file${NC}"
                issues=$((issues + 1))
            fi
        done

        if [[ $issues -eq 0 ]]; then
            echo -e "${GREEN}[OK] Configuration files are structurally consistent${NC}"
        else
            WARNINGS=$((WARNINGS + issues))
        fi
    else
        echo -e "${YELLOW}[SKIP] No config directory found${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Contract Interface Consistency
# -----------------------------------------------------------------------------
check_contract_consistency() {
    echo -e "${BLUE}=== Contract Interface Consistency ===${NC}"

    local contracts_dir="$PROJECT_ROOT/contracts"
    local issues=0

    if [[ -d "$contracts_dir" ]]; then
        # Check Gateway contract has required methods (C# syntax: public static)
        local gateway="$contracts_dir/gateway/ServiceLayerGateway.cs"
        if [[ -f "$gateway" ]]; then
            local required_methods=("RequestService" "FulfillRequest" "RegisterService")
            for method in "${required_methods[@]}"; do
                if ! grep -q "public static.*$method\|public.*static.*$method" "$gateway"; then
                    echo -e "${RED}[ERROR] Gateway missing method: $method${NC}"
                    issues=$((issues + 1))
                fi
            done
        fi

        # Check example contracts implement callbacks
        for example in "$contracts_dir/examples"/*.cs; do
            [[ -f "$example" ]] || continue
            local filename=$(basename "$example")

            # Check for callback pattern if contract uses Gateway
            if grep -q "RequestService\|requestService" "$example" && ! grep -q "Callback\|callback" "$example"; then
                echo -e "${YELLOW}[WARNING] $filename uses RequestService but may lack callback${NC}"
                WARNINGS=$((WARNINGS + 1))
            fi
        done

        if [[ $issues -eq 0 ]]; then
            echo -e "${GREEN}[OK] Contract interfaces are consistent${NC}"
        else
            ERRORS=$((ERRORS + issues))
        fi
    else
        echo -e "${YELLOW}[SKIP] No contracts directory found${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Service Registration Consistency
# -----------------------------------------------------------------------------
check_service_consistency() {
    echo -e "${BLUE}=== Service Registration Consistency ===${NC}"

    local services_dir="$PROJECT_ROOT/services"
    local issues=0

    if [[ -d "$services_dir" ]]; then
        # Check each service has required components
        for service_dir in "$services_dir"/*/; do
            [[ -d "$service_dir" ]] || continue
            local service_name=$(basename "$service_dir")

            # Skip if not a Go service
            [[ -f "$service_dir"/*.go ]] || continue

            # Check for ServiceID constant
            if ! grep -rq "ServiceID.*=" "$service_dir"/*.go 2>/dev/null; then
                echo -e "${YELLOW}[WARNING] Service $service_name missing ServiceID constant${NC}"
                WARNINGS=$((WARNINGS + 1))
            fi

            # Check for handlers
            if ! grep -rq "func.*Handler\|HandleFunc\|ServeHTTP" "$service_dir"/*.go 2>/dev/null; then
                echo -e "${YELLOW}[WARNING] Service $service_name may lack HTTP handlers${NC}"
                WARNINGS=$((WARNINGS + 1))
            fi
        done

        echo -e "${GREEN}[OK] Service structure check complete${NC}"
    else
        echo -e "${YELLOW}[SKIP] No services directory found${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Error Handling Consistency
# -----------------------------------------------------------------------------
check_error_handling() {
    echo -e "${BLUE}=== Error Handling Consistency ===${NC}"

    # Check for inconsistent error handling patterns
    local unchecked=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        ! -path "*_test.go" \
        -exec grep -l "_ = .*err\|, _ :=.*(" {} \; 2>/dev/null | wc -l)

    if [[ $unchecked -gt 5 ]]; then
        echo -e "${YELLOW}[WARNING] Found $unchecked files with potentially ignored errors${NC}"
        echo "  Pattern: '_ = err' or ', _ := func()'"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}[OK] Error handling appears consistent${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Logging Consistency
# -----------------------------------------------------------------------------
check_logging_consistency() {
    echo -e "${BLUE}=== Logging Consistency ===${NC}"

    # Check for mixed logging packages
    local std_log=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec grep -l '"log"' {} \; 2>/dev/null | wc -l)

    local zap_log=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec grep -l '"go.uber.org/zap"' {} \; 2>/dev/null | wc -l)

    local logrus=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec grep -l '"github.com/sirupsen/logrus"' {} \; 2>/dev/null | wc -l)

    echo "  Standard log: $std_log files"
    echo "  Zap: $zap_log files"
    echo "  Logrus: $logrus files"

    local total=$((std_log + zap_log + logrus))
    local packages=0
    [[ $std_log -gt 0 ]] && packages=$((packages + 1))
    [[ $zap_log -gt 0 ]] && packages=$((packages + 1))
    [[ $logrus -gt 0 ]] && packages=$((packages + 1))

    if [[ $packages -gt 1 ]]; then
        echo -e "${YELLOW}[WARNING] Multiple logging packages in use${NC}"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}[OK] Logging is consistent${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# JSON Tag Consistency
# -----------------------------------------------------------------------------
check_json_tags() {
    echo -e "${BLUE}=== JSON Tag Consistency ===${NC}"

    # Check for struct fields that might be missing JSON tags in API types
    local missing_tags=$(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec grep -l "type.*struct" {} \; 2>/dev/null | \
        xargs grep -A20 "type.*Request\|type.*Response\|type.*Payload" 2>/dev/null | \
        grep -E "^\s+[A-Z][a-zA-Z]+\s+(string|int|bool|\[\])" | \
        grep -v 'json:' | head -5)

    if [[ -n "$missing_tags" ]]; then
        echo -e "${YELLOW}[WARNING] Potential missing JSON tags in API structs:${NC}"
        echo "$missing_tags" | head -5
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "${GREEN}[OK] JSON tags appear consistent${NC}"
    fi
    echo ""
}

# -----------------------------------------------------------------------------
# Run All Checks
# -----------------------------------------------------------------------------
main() {
    check_go_formatting
    check_go_vet
    check_go_mod
    check_import_consistency
    check_config_consistency
    check_contract_consistency
    check_service_consistency
    check_error_handling
    check_logging_consistency
    check_json_tags

    echo "========================================"
    echo "  Summary"
    echo "========================================"
    echo ""

    if [[ $ERRORS -gt 0 ]]; then
        echo -e "${RED}ERRORS: $ERRORS${NC}"
    fi

    if [[ $WARNINGS -gt 0 ]]; then
        echo -e "${YELLOW}WARNINGS: $WARNINGS${NC}"
    fi

    if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
        echo -e "${GREEN}All consistency checks passed!${NC}"
        exit 0
    elif [[ $ERRORS -gt 0 ]]; then
        echo ""
        echo -e "${RED}FAILED: Fix errors before proceeding${NC}"
        exit 1
    else
        echo ""
        echo -e "${YELLOW}PASSED with warnings${NC}"
        exit 0
    fi
}

main "$@"
