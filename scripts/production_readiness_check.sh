#!/bin/bash
# Production Readiness Check Script
# Scans codebase for TODO, FIXME, placeholder, and other non-production patterns

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "========================================"
echo "  Production Readiness Check"
echo "========================================"
echo ""
echo "Project: $PROJECT_ROOT"
echo ""

ISSUES_FOUND=0
WARNINGS_FOUND=0

# Patterns to search for (case-insensitive)
CRITICAL_PATTERNS=(
    "TODO"
    "FIXME"
    "XXX"
    "HACK"
    "BUG:"
)

WARNING_PATTERNS=(
    "placeholder"
    "for now"
    "in production"
    "simplified"
    "temporary"
    "workaround"
)

# Directories to exclude
EXCLUDE_DIRS="--exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=.git --exclude-dir=dist --exclude-dir=build --exclude-dir=__pycache__ --exclude-dir=.next --exclude-dir=coverage"

# Files to exclude
EXCLUDE_FILES="--exclude=*.md --exclude=*.txt --exclude=README* --exclude=CHANGELOG* --exclude=LICENSE* --exclude=.gitleaks.toml --exclude=*_test.go --exclude=*_test.cs --exclude=*.test.ts --exclude=*.test.tsx --exclude=*.spec.ts --exclude=*.spec.tsx --exclude=production_readiness_check.sh --exclude=go.sum --exclude=package-lock.json --exclude=yarn.lock"

check_pattern() {
    local pattern=$1
    local severity=$2
    local color=$RED
    local count=0

    if [[ "$severity" == "warning" ]]; then
        color=$YELLOW
    fi

    # Search with exclusions using find + grep (more reliable for exclusions)
    local results=$(find "$PROJECT_ROOT" \
        -type f \( -name "*.go" -o -name "*.cs" -o -name "*.ts" -o -name "*.tsx" -o -name "*.py" -o -name "*.sh" -o -name "*.yaml" -o -name "*.yml" \) \
        ! -path "*/node_modules/*" \
        ! -path "*/.git/*" \
        ! -path "*/vendor/*" \
        ! -path "*/dist/*" \
        ! -path "*/build/*" \
        ! -path "*/__pycache__/*" \
        ! -path "*/.next/*" \
        ! -path "*/coverage/*" \
        ! -name "*_test.go" \
        ! -name "*_test.cs" \
        ! -name "*.test.ts" \
        ! -name "*.test.tsx" \
        ! -name "*.spec.ts" \
        ! -name "*.spec.tsx" \
        ! -name "production_readiness_check.sh" \
        -exec grep -lni "$pattern" {} \; 2>/dev/null | while read -r file; do
            grep -ni "$pattern" "$file" 2>/dev/null | while read -r line; do
                echo "$file:$line"
            done
        done || true)

    # Filter out HTML placeholder attributes, StatusTemporaryRedirect, and Tailwind placeholder- classes
    if [[ -n "$results" ]]; then
        results=$(echo "$results" | grep -v 'placeholder="' | grep -v "placeholder='" | grep -v "StatusTemporaryRedirect" | grep -v "placeholder-" || true)
    fi

    if [[ -n "$results" ]]; then
        count=$(echo "$results" | wc -l)
        echo -e "${color}[$severity] Found '$pattern': $count occurrence(s)${NC}"

        echo "$results" | while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                # Make path relative
                local rel_line="${line#$PROJECT_ROOT/}"
                # Truncate long lines
                local display_line="${rel_line:0:150}"
                if [[ ${#rel_line} -gt 150 ]]; then
                    display_line="$display_line..."
                fi
                echo "  $display_line"
            fi
        done
        echo ""

        if [[ "$severity" == "critical" ]]; then
            ISSUES_FOUND=$((ISSUES_FOUND + count))
        else
            WARNINGS_FOUND=$((WARNINGS_FOUND + count))
        fi
    fi
}

echo -e "${BLUE}=== Checking for CRITICAL patterns ===${NC}"
echo ""

for pattern in "${CRITICAL_PATTERNS[@]}"; do
    check_pattern "$pattern" "critical"
done

echo -e "${BLUE}=== Checking for WARNING patterns ===${NC}"
echo ""

for pattern in "${WARNING_PATTERNS[@]}"; do
    check_pattern "$pattern" "warning"
done

echo "========================================"
echo "  Summary"
echo "========================================"
echo ""

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "${RED}CRITICAL ISSUES: $ISSUES_FOUND${NC}"
fi

if [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "${YELLOW}WARNINGS: $WARNINGS_FOUND${NC}"
fi

if [[ $ISSUES_FOUND -eq 0 && $WARNINGS_FOUND -eq 0 ]]; then
    echo -e "${GREEN}No production readiness issues found!${NC}"
    exit 0
elif [[ $ISSUES_FOUND -gt 0 ]]; then
    echo ""
    echo -e "${RED}FAILED: Critical issues must be resolved before production deployment${NC}"
    exit 1
else
    echo ""
    echo -e "${YELLOW}WARNING: Review warnings before production deployment${NC}"
    exit 0
fi
