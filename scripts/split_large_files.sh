#!/bin/bash
# Large File Analyzer and Splitter
# Finds large Go files and suggests/performs logical splits

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Default thresholds
LINE_THRESHOLD=${LINE_THRESHOLD:-500}
FUNCTION_THRESHOLD=${FUNCTION_THRESHOLD:-15}

usage() {
    echo "Usage: $0 [OPTIONS] [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  scan              Scan for large files (default)"
    echo "  analyze <file>    Analyze a specific file's structure"
    echo "  split <file>      Generate split suggestions for a file"
    echo "  execute <file>    Execute the split (creates new files)"
    echo ""
    echo "Options:"
    echo "  -l, --lines N     Line count threshold (default: $LINE_THRESHOLD)"
    echo "  -f, --funcs N     Function count threshold (default: $FUNCTION_THRESHOLD)"
    echo "  -h, --help        Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 scan"
    echo "  $0 analyze services/mixer/handlers.go"
    echo "  $0 -l 300 scan"
}

# Parse options
while [[ $# -gt 0 ]]; do
    case $1 in
        -l|--lines)
            LINE_THRESHOLD="$2"
            shift 2
            ;;
        -f|--funcs)
            FUNCTION_THRESHOLD="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            break
            ;;
    esac
done

COMMAND=${1:-scan}
TARGET_FILE=${2:-}

# -----------------------------------------------------------------------------
# Scan for large files
# -----------------------------------------------------------------------------
scan_large_files() {
    echo "========================================"
    echo "  Large File Scanner"
    echo "========================================"
    echo ""
    echo "Thresholds: >$LINE_THRESHOLD lines OR >$FUNCTION_THRESHOLD functions"
    echo ""

    echo -e "${BLUE}=== Files by Line Count ===${NC}"
    echo ""

    local found=0
    while IFS= read -r file; do
        [[ -f "$file" ]] || continue

        local lines=$(wc -l < "$file" 2>/dev/null | tr -d '[:space:]')
        local funcs=$(grep -c "^func " "$file" 2>/dev/null | tr -d '[:space:]')
        local types=$(grep -c "^type " "$file" 2>/dev/null | tr -d '[:space:]')

        # Ensure numeric values (default to 0)
        [[ -z "$lines" || ! "$lines" =~ ^[0-9]+$ ]] && lines=0
        [[ -z "$funcs" || ! "$funcs" =~ ^[0-9]+$ ]] && funcs=0
        [[ -z "$types" || ! "$types" =~ ^[0-9]+$ ]] && types=0

        if (( lines > LINE_THRESHOLD || funcs > FUNCTION_THRESHOLD )); then
            local rel_path="${file#$PROJECT_ROOT/}"
            local color=$YELLOW
            (( lines > 1000 )) && color=$RED

            printf "${color}%-50s${NC} %5d lines, %3d funcs, %3d types\n" \
                "$rel_path" "$lines" "$funcs" "$types"
            found=$((found + 1))
        fi
    done < <(find "$PROJECT_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        ! -path "*_test.go" \
        -type f 2>/dev/null | sort)

    echo ""
    if [[ $found -eq 0 ]]; then
        echo -e "${GREEN}No files exceed thresholds.${NC}"
    else
        echo -e "${YELLOW}Found $found file(s) exceeding thresholds.${NC}"
        echo ""
        echo "Run '$0 analyze <file>' for detailed analysis."
    fi
}

# -----------------------------------------------------------------------------
# Analyze file structure
# -----------------------------------------------------------------------------
analyze_file() {
    local file="$1"

    if [[ ! -f "$file" ]]; then
        # Try relative to project root
        file="$PROJECT_ROOT/$file"
    fi

    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File not found: $1${NC}"
        exit 1
    fi

    local rel_path="${file#$PROJECT_ROOT/}"
    local lines=$(wc -l < "$file")
    local package=$(grep "^package " "$file" | head -1 | awk '{print $2}')

    echo "========================================"
    echo "  File Analysis: $rel_path"
    echo "========================================"
    echo ""
    echo "Package: $package"
    echo "Lines: $lines"
    echo ""

    # Extract sections with line numbers
    echo -e "${BLUE}=== Constants & Variables ===${NC}"
    grep -n "^const \|^var " "$file" 2>/dev/null | head -10 || echo "  (none)"
    echo ""

    echo -e "${BLUE}=== Type Definitions ===${NC}"
    grep -n "^type " "$file" 2>/dev/null | while read -r line; do
        echo "  $line"
    done
    echo ""

    echo -e "${BLUE}=== Functions/Methods ===${NC}"
    grep -n "^func " "$file" 2>/dev/null | while read -r line; do
        # Extract function name and receiver
        local linenum=$(echo "$line" | cut -d: -f1)
        local funcdef=$(echo "$line" | cut -d: -f2-)

        # Get function body size (approximate)
        local func_end=$(tail -n +$linenum "$file" | grep -n "^}" | head -1 | cut -d: -f1)
        local func_size=${func_end:-0}

        printf "  %4d: %-60s (~%d lines)\n" "$linenum" "${funcdef:0:60}" "$func_size"
    done
    echo ""

    # Detect logical sections by comments
    echo -e "${BLUE}=== Section Comments ===${NC}"
    grep -n "^// ===\|^// ---\|^// ###" "$file" 2>/dev/null | while read -r line; do
        echo "  $line"
    done || echo "  (no section markers found)"
    echo ""

    # Summary
    local func_count=$(grep -c "^func " "$file" 2>/dev/null || echo 0)
    local type_count=$(grep -c "^type " "$file" 2>/dev/null || echo 0)
    local method_count=$(grep -c "^func (.*) " "$file" 2>/dev/null || echo 0)
    local standalone_func=$((func_count - method_count))

    echo -e "${CYAN}=== Summary ===${NC}"
    echo "  Total functions: $func_count"
    echo "    - Methods: $method_count"
    echo "    - Standalone: $standalone_func"
    echo "  Types: $type_count"
    echo ""

    if [[ $lines -gt 500 ]]; then
        echo -e "${YELLOW}Recommendation: Consider splitting this file.${NC}"
        echo "Run '$0 split $rel_path' for suggestions."
    fi
}

# -----------------------------------------------------------------------------
# Generate split suggestions
# -----------------------------------------------------------------------------
suggest_split() {
    local file="$1"

    if [[ ! -f "$file" ]]; then
        file="$PROJECT_ROOT/$file"
    fi

    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File not found: $1${NC}"
        exit 1
    fi

    local rel_path="${file#$PROJECT_ROOT/}"
    local dir=$(dirname "$file")
    local base=$(basename "$file" .go)
    local package=$(grep "^package " "$file" | head -1 | awk '{print $2}')

    echo "========================================"
    echo "  Split Suggestions: $rel_path"
    echo "========================================"
    echo ""

    # Detect logical groupings
    declare -A groups
    declare -A group_lines
    local current_group="main"
    local line_num=0

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Detect section markers
        if [[ "$line" =~ ^//[[:space:]]*===.*===$ ]] || \
           [[ "$line" =~ ^//[[:space:]]*---.*---$ ]] || \
           [[ "$line" =~ ^//[[:space:]]*[A-Z][A-Za-z\ ]+$ ]]; then
            # Extract section name
            local section=$(echo "$line" | sed 's/^\/\/[[:space:]]*//;s/[=\-]//g' | xargs)
            if [[ -n "$section" && ${#section} -gt 3 ]]; then
                current_group=$(echo "$section" | tr '[:upper:]' '[:lower:]' | tr ' ' '_' | tr -cd '[:alnum:]_')
            fi
        fi

        # Track functions per group
        if [[ "$line" =~ ^func ]]; then
            groups[$current_group]="${groups[$current_group]} $line_num"
            group_lines[$current_group]=$((${group_lines[$current_group]:-0} + 1))
        fi
    done < "$file"

    # Analyze receiver types for methods
    echo -e "${BLUE}=== Suggested Splits by Receiver Type ===${NC}"
    echo ""

    declare -A receivers
    while IFS= read -r line; do
        if [[ "$line" =~ ^func[[:space:]]*\(([a-z]+)[[:space:]]+\*?([A-Za-z]+)\) ]]; then
            local receiver="${BASH_REMATCH[2]}"
            receivers[$receiver]=$((${receivers[$receiver]:-0} + 1))
        fi
    done < <(grep "^func " "$file")

    for receiver in "${!receivers[@]}"; do
        local count=${receivers[$receiver]}
        if [[ $count -gt 3 ]]; then
            local suggested_file="${base}_${receiver,,}.go"
            echo -e "  ${GREEN}$suggested_file${NC}"
            echo "    Receiver: $receiver ($count methods)"
            grep -n "^func (.*\*\?$receiver)" "$file" | head -5 | while read -r fn; do
                echo "      - $(echo "$fn" | cut -d: -f2- | sed 's/func //' | cut -d'(' -f1-2)..."
            done
            [[ $count -gt 5 ]] && echo "      ... and $((count - 5)) more"
            echo ""
        fi
    done

    # Suggest by section comments
    if [[ ${#groups[@]} -gt 1 ]]; then
        echo -e "${BLUE}=== Suggested Splits by Section ===${NC}"
        echo ""

        for group in "${!groups[@]}"; do
            [[ "$group" == "main" ]] && continue
            local count=${group_lines[$group]:-0}
            [[ $count -lt 3 ]] && continue

            local suggested_file="${base}_${group}.go"
            echo -e "  ${GREEN}$suggested_file${NC}"
            echo "    Section: $group ($count functions)"
            echo ""
        done
    fi

    # Suggest by function prefix patterns
    echo -e "${BLUE}=== Suggested Splits by Function Prefix ===${NC}"
    echo ""

    declare -A prefixes
    while IFS= read -r func_name; do
        # Extract prefix (e.g., handleXxx -> handle, processXxx -> process)
        local prefix=$(echo "$func_name" | sed 's/^\([a-z]*\)[A-Z].*/\1/')
        if [[ ${#prefix} -gt 2 && ${#prefix} -lt 15 ]]; then
            prefixes[$prefix]=$((${prefixes[$prefix]:-0} + 1))
        fi
    done < <(grep "^func " "$file" | sed 's/^func \(([^)]*) \)\?//' | cut -d'(' -f1)

    for prefix in "${!prefixes[@]}"; do
        local count=${prefixes[$prefix]}
        if [[ $count -gt 4 ]]; then
            local suggested_file="${base}_${prefix}.go"
            echo -e "  ${GREEN}$suggested_file${NC}"
            echo "    Prefix: ${prefix}* ($count functions)"
            grep "^func.*${prefix}[A-Z]" "$file" | head -3 | while read -r fn; do
                echo "      - $(echo "$fn" | sed 's/func //' | cut -d'(' -f1)..."
            done
            echo ""
        fi
    done

    echo "========================================"
    echo "  Recommended Split Plan"
    echo "========================================"
    echo ""
    echo "1. Keep core types and initialization in: ${base}.go"
    echo "2. Move HTTP handlers to: ${base}_handlers.go"
    echo "3. Move business logic to: ${base}_logic.go (or by domain)"
    echo "4. Move helper/utility functions to: ${base}_helpers.go"
    echo ""
    echo -e "${YELLOW}Note: Manual review required before splitting.${NC}"
    echo "Ensure imports and dependencies are properly handled."
}

# -----------------------------------------------------------------------------
# Execute split (creates new files)
# -----------------------------------------------------------------------------
execute_split() {
    local file="$1"

    if [[ ! -f "$file" ]]; then
        file="$PROJECT_ROOT/$file"
    fi

    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File not found: $1${NC}"
        exit 1
    fi

    local rel_path="${file#$PROJECT_ROOT/}"
    local dir=$(dirname "$file")
    local base=$(basename "$file" .go)
    local package=$(grep "^package " "$file" | head -1)

    echo "========================================"
    echo "  Auto-Split: $rel_path"
    echo "========================================"
    echo ""
    echo -e "${YELLOW}WARNING: This will create new files and modify the original.${NC}"
    echo ""

    read -p "Continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi

    # Create backup
    cp "$file" "${file}.bak"
    echo "Backup created: ${file}.bak"

    # Extract handlers (functions starting with 'handle')
    local handlers_file="$dir/${base}_handlers.go"
    if grep -q "^func.*handle[A-Z]" "$file"; then
        echo ""
        echo "Creating: $handlers_file"

        {
            echo "// Code generated by split_large_files.sh - handlers extracted"
            echo ""
            echo "$package"
            echo ""

            # Extract imports used by handlers
            echo "import ("
            grep "^func.*handle[A-Z]" "$file" -A 100 | grep -oE '"[^"]+"|[a-z]+\.' | \
                sort -u | sed 's/\.$//' | while read -r imp; do
                [[ "$imp" =~ ^\" ]] && echo "	$imp"
            done
            echo ")"
            echo ""

            # Extract handler functions with their comments
            awk '/^\/\/ handle[A-Z]|^func.*handle[A-Z]/{p=1} p{print; if(/^}$/){p=0; print ""}}' "$file"
        } > "$handlers_file"

        echo "  Extracted handler functions"
    fi

    # Extract helper functions (unexported, not methods)
    local helpers_file="$dir/${base}_helpers.go"
    local helper_count=$(grep -c "^func [a-z]" "$file" 2>/dev/null || echo 0)

    if [[ $helper_count -gt 3 ]]; then
        echo ""
        echo "Creating: $helpers_file"

        {
            echo "// Code generated by split_large_files.sh - helpers extracted"
            echo ""
            echo "$package"
            echo ""

            # Extract helper functions
            awk '/^\/\/ [a-z]|^func [a-z][a-zA-Z]*\(/{p=1} p{print; if(/^}$/){p=0; print ""}}' "$file"
        } > "$helpers_file"

        echo "  Extracted $helper_count helper functions"
    fi

    echo ""
    echo -e "${GREEN}Split complete.${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Review generated files and fix imports"
    echo "2. Remove extracted functions from original file"
    echo "3. Run 'go build ./...' to verify"
    echo "4. Run 'gofmt -w $dir/*.go' to format"
    echo ""
    echo "To revert: mv ${file}.bak $file"
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
cd "$PROJECT_ROOT"

case $COMMAND in
    scan)
        scan_large_files
        ;;
    analyze)
        if [[ -z "$TARGET_FILE" ]]; then
            echo -e "${RED}Error: File path required${NC}"
            usage
            exit 1
        fi
        analyze_file "$TARGET_FILE"
        ;;
    split)
        if [[ -z "$TARGET_FILE" ]]; then
            echo -e "${RED}Error: File path required${NC}"
            usage
            exit 1
        fi
        suggest_split "$TARGET_FILE"
        ;;
    execute)
        if [[ -z "$TARGET_FILE" ]]; then
            echo -e "${RED}Error: File path required${NC}"
            usage
            exit 1
        fi
        execute_split "$TARGET_FILE"
        ;;
    *)
        echo -e "${RED}Unknown command: $COMMAND${NC}"
        usage
        exit 1
        ;;
esac
