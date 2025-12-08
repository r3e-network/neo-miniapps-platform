#!/bin/bash
# Comprehensive Duplication Detection Script for Go Codebase
# Detects: Types, Functions, Constants, Patterns, Error Messages

set -e

PROJECT_ROOT="${1:-.}"
OUTPUT_DIR="${PROJECT_ROOT}/duplication_report"
mkdir -p "$OUTPUT_DIR"

echo "=========================================="
echo "  Duplication Detection Report"
echo "  Project: $PROJECT_ROOT"
echo "  Date: $(date)"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# 1. TYPE DUPLICATIONS (structs, interfaces)
# =============================================================================
echo -e "\n${BLUE}[1/6] Checking Type Duplications...${NC}"

# Find all struct definitions
echo "=== STRUCT DUPLICATIONS ===" > "$OUTPUT_DIR/type_duplications.txt"
grep -rn "^type [A-Z][a-zA-Z0-9]* struct" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    sed 's/:type /\t/' | \
    awk -F'\t' '{
        match($2, /^[A-Z][a-zA-Z0-9]*/)
        name = substr($2, RSTART, RLENGTH)
        files[name] = files[name] ? files[name] "\n  " $1 : $1
        count[name]++
    }
    END {
        for (name in count) {
            if (count[name] > 1) {
                print "\n[DUPLICATE] " name " (" count[name] " occurrences):"
                print "  " files[name]
            }
        }
    }' >> "$OUTPUT_DIR/type_duplications.txt"

# Find all interface definitions
echo -e "\n=== INTERFACE DUPLICATIONS ===" >> "$OUTPUT_DIR/type_duplications.txt"
grep -rn "^type [A-Z][a-zA-Z0-9]* interface" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    sed 's/:type /\t/' | \
    awk -F'\t' '{
        match($2, /^[A-Z][a-zA-Z0-9]*/)
        name = substr($2, RSTART, RLENGTH)
        files[name] = files[name] ? files[name] "\n  " $1 : $1
        count[name]++
    }
    END {
        for (name in count) {
            if (count[name] > 1) {
                print "\n[DUPLICATE] " name " (" count[name] " occurrences):"
                print "  " files[name]
            }
        }
    }' >> "$OUTPUT_DIR/type_duplications.txt"

TYPE_DUPS=$(grep -c "\[DUPLICATE\]" "$OUTPUT_DIR/type_duplications.txt" 2>/dev/null || echo "0")
echo -e "  Found ${RED}$TYPE_DUPS${NC} type duplications"

# =============================================================================
# 2. CONSTANT DUPLICATIONS
# =============================================================================
echo -e "\n${BLUE}[2/6] Checking Constant Duplications...${NC}"

echo "=== CONSTANT DUPLICATIONS ===" > "$OUTPUT_DIR/constant_duplications.txt"

# Find all const definitions with values
grep -rn "^\s*[A-Z][a-zA-Z0-9_]*\s*=\s*" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    grep -v "func\|:=\|var " | \
    awk -F':' '{
        file = $1
        line = $2
        rest = $0
        sub(/^[^:]*:[^:]*:/, "", rest)
        # Extract constant name
        match(rest, /[A-Z][a-zA-Z0-9_]*/)
        name = substr(rest, RSTART, RLENGTH)
        if (name != "") {
            files[name] = files[name] ? files[name] "\n  " file ":" line : file ":" line
            count[name]++
        }
    }
    END {
        for (name in count) {
            if (count[name] > 1) {
                print "\n[DUPLICATE] " name " (" count[name] " occurrences):"
                print "  " files[name]
            }
        }
    }' >> "$OUTPUT_DIR/constant_duplications.txt"

CONST_DUPS=$(grep -c "\[DUPLICATE\]" "$OUTPUT_DIR/constant_duplications.txt" 2>/dev/null || echo "0")
echo -e "  Found ${RED}$CONST_DUPS${NC} constant duplications"

# =============================================================================
# 3. FUNCTION SIGNATURE DUPLICATIONS
# =============================================================================
echo -e "\n${BLUE}[3/6] Checking Function Signature Duplications...${NC}"

echo "=== FUNCTION SIGNATURE DUPLICATIONS ===" > "$OUTPUT_DIR/function_duplications.txt"

# Find functions with same name (excluding test files and methods)
grep -rn "^func [A-Z][a-zA-Z0-9]*(" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude="*_test.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    awk -F':' '{
        file = $1
        line = $2
        rest = $0
        sub(/^[^:]*:[^:]*:/, "", rest)
        match(rest, /func ([A-Z][a-zA-Z0-9]*)/, arr)
        name = arr[1]
        if (name != "") {
            files[name] = files[name] ? files[name] "\n  " file ":" line : file ":" line
            count[name]++
        }
    }
    END {
        for (name in count) {
            if (count[name] > 1) {
                print "\n[DUPLICATE] " name " (" count[name] " occurrences):"
                print "  " files[name]
            }
        }
    }' >> "$OUTPUT_DIR/function_duplications.txt"

FUNC_DUPS=$(grep -c "\[DUPLICATE\]" "$OUTPUT_DIR/function_duplications.txt" 2>/dev/null || echo "0")
echo -e "  Found ${RED}$FUNC_DUPS${NC} function name duplications"

# =============================================================================
# 4. ERROR MESSAGE DUPLICATIONS
# =============================================================================
echo -e "\n${BLUE}[4/6] Checking Error Message Duplications...${NC}"

echo "=== ERROR MESSAGE DUPLICATIONS ===" > "$OUTPUT_DIR/error_duplications.txt"

# Find duplicate error messages
grep -roh 'fmt\.Errorf("[^"]*"' "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    sort | uniq -c | sort -rn | \
    awk '$1 > 1 {print "[DUPLICATE] " $1 "x: " substr($0, index($0, "fmt"))}' \
    >> "$OUTPUT_DIR/error_duplications.txt"

grep -roh 'errors\.New("[^"]*"' "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    sort | uniq -c | sort -rn | \
    awk '$1 > 1 {print "[DUPLICATE] " $1 "x: " substr($0, index($0, "errors"))}' \
    >> "$OUTPUT_DIR/error_duplications.txt"

ERR_DUPS=$(grep -c "\[DUPLICATE\]" "$OUTPUT_DIR/error_duplications.txt" 2>/dev/null || echo "0")
echo -e "  Found ${RED}$ERR_DUPS${NC} error message duplications"

# =============================================================================
# 5. SIMILAR CODE PATTERNS
# =============================================================================
echo -e "\n${BLUE}[5/6] Checking Similar Code Patterns...${NC}"

echo "=== SIMILAR CODE PATTERNS ===" > "$OUTPUT_DIR/pattern_duplications.txt"

# Pattern: Validation patterns
echo -e "\n--- Validation Patterns (if err != nil return) ---" >> "$OUTPUT_DIR/pattern_duplications.txt"
grep -rn "if err != nil {" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    -A1 2>/dev/null | \
    grep -E "return.*err|return nil, err|return \"\", err" | \
    wc -l | \
    xargs -I {} echo "  Found {} error return patterns (consider error wrapping utility)" \
    >> "$OUTPUT_DIR/pattern_duplications.txt"

# Pattern: Context timeout patterns
echo -e "\n--- Context Timeout Patterns ---" >> "$OUTPUT_DIR/pattern_duplications.txt"
grep -rn "context.WithTimeout\|context.WithDeadline" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    2>/dev/null | wc -l | \
    xargs -I {} echo "  Found {} context timeout usages" \
    >> "$OUTPUT_DIR/pattern_duplications.txt"

# Pattern: JSON marshal/unmarshal
echo -e "\n--- JSON Marshal/Unmarshal Patterns ---" >> "$OUTPUT_DIR/pattern_duplications.txt"
grep -rn "json.Marshal\|json.Unmarshal" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    2>/dev/null | wc -l | \
    xargs -I {} echo "  Found {} JSON operations (consider typed helpers)" \
    >> "$OUTPUT_DIR/pattern_duplications.txt"

# Pattern: HTTP client patterns
echo -e "\n--- HTTP Client Patterns ---" >> "$OUTPUT_DIR/pattern_duplications.txt"
grep -rn "http.NewRequest\|http.Get\|http.Post" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    2>/dev/null | wc -l | \
    xargs -I {} echo "  Found {} HTTP request patterns" \
    >> "$OUTPUT_DIR/pattern_duplications.txt"

echo "  Pattern analysis complete"

# =============================================================================
# 6. IMPORT ANALYSIS (packages imported in multiple files)
# =============================================================================
echo -e "\n${BLUE}[6/6] Analyzing Import Patterns...${NC}"

echo "=== IMPORT ANALYSIS ===" > "$OUTPUT_DIR/import_analysis.txt"

# Find commonly imported packages
grep -rh "^\s*\"" "$PROJECT_ROOT" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    --exclude-dir=node_modules \
    2>/dev/null | \
    grep -v "// " | \
    sort | uniq -c | sort -rn | head -30 \
    >> "$OUTPUT_DIR/import_analysis.txt"

echo "  Import analysis complete"

# =============================================================================
# SUMMARY REPORT
# =============================================================================
echo -e "\n${GREEN}=========================================="
echo "  SUMMARY REPORT"
echo "==========================================${NC}"

echo -e "\n${YELLOW}Duplications Found:${NC}"
echo "  - Type duplications:     $TYPE_DUPS"
echo "  - Constant duplications: $CONST_DUPS"
echo "  - Function duplications: $FUNC_DUPS"
echo "  - Error msg duplications: $ERR_DUPS"

echo -e "\n${YELLOW}Reports saved to:${NC}"
echo "  - $OUTPUT_DIR/type_duplications.txt"
echo "  - $OUTPUT_DIR/constant_duplications.txt"
echo "  - $OUTPUT_DIR/function_duplications.txt"
echo "  - $OUTPUT_DIR/error_duplications.txt"
echo "  - $OUTPUT_DIR/pattern_duplications.txt"
echo "  - $OUTPUT_DIR/import_analysis.txt"

# Create combined report
echo "=== COMBINED DUPLICATION REPORT ===" > "$OUTPUT_DIR/FULL_REPORT.txt"
echo "Generated: $(date)" >> "$OUTPUT_DIR/FULL_REPORT.txt"
echo "" >> "$OUTPUT_DIR/FULL_REPORT.txt"
cat "$OUTPUT_DIR/type_duplications.txt" >> "$OUTPUT_DIR/FULL_REPORT.txt"
echo "" >> "$OUTPUT_DIR/FULL_REPORT.txt"
cat "$OUTPUT_DIR/constant_duplications.txt" >> "$OUTPUT_DIR/FULL_REPORT.txt"
echo "" >> "$OUTPUT_DIR/FULL_REPORT.txt"
cat "$OUTPUT_DIR/function_duplications.txt" >> "$OUTPUT_DIR/FULL_REPORT.txt"
echo "" >> "$OUTPUT_DIR/FULL_REPORT.txt"
cat "$OUTPUT_DIR/error_duplications.txt" >> "$OUTPUT_DIR/FULL_REPORT.txt"
echo "" >> "$OUTPUT_DIR/FULL_REPORT.txt"
cat "$OUTPUT_DIR/pattern_duplications.txt" >> "$OUTPUT_DIR/FULL_REPORT.txt"

echo -e "\n${GREEN}Full report: $OUTPUT_DIR/FULL_REPORT.txt${NC}"
echo ""
