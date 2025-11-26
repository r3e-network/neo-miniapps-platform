#!/bin/bash
# Comprehensive test runner for service_layer
# Usage: ./test/run_tests.sh [unit|integration|smoke|neoexpress|all]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
export PROJECT_ROOT

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Run unit tests
run_unit_tests() {
    log_info "Running unit tests..."
    cd "$PROJECT_ROOT"

    go test -v -race -short ./... 2>&1 | tee test-unit.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "Unit tests passed"
    else
        log_error "Unit tests failed"
        return 1
    fi
}

# Run unit tests with coverage
run_unit_coverage() {
    log_info "Running unit tests with coverage..."
    cd "$PROJECT_ROOT"

    go test -v -race -short -coverprofile=coverage.out ./... 2>&1 | tee test-coverage.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        go tool cover -func=coverage.out | tail -1
        go tool cover -html=coverage.out -o coverage.html
        log_info "Coverage report generated: coverage.html"
    else
        log_error "Coverage tests failed"
        return 1
    fi
}

# Run integration tests
run_integration_tests() {
    log_info "Running integration tests..."
    cd "$PROJECT_ROOT"

    # Check if API is available
    API_URL="${TEST_API_URL:-http://localhost:8080}"
    if ! curl -sf "$API_URL/healthz" > /dev/null 2>&1; then
        log_warn "API not available at $API_URL - skipping integration tests"
        log_warn "Start the server with: make run"
        return 0
    fi

    go test -v -tags=integration ./test/integration/... 2>&1 | tee test-integration.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "Integration tests passed"
    else
        log_error "Integration tests failed"
        return 1
    fi
}

# Run smoke tests
run_smoke_tests() {
    log_info "Running smoke tests..."
    cd "$PROJECT_ROOT"

    # Check if API is available
    API_URL="${SMOKE_API_URL:-${TEST_API_URL:-http://localhost:8080}}"
    if ! curl -sf "$API_URL/healthz" > /dev/null 2>&1; then
        log_warn "API not available at $API_URL - skipping smoke tests"
        return 0
    fi

    go test -v -tags=smoke ./test/smoke/... 2>&1 | tee test-smoke.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "Smoke tests passed"
    else
        log_error "Smoke tests failed"
        return 1
    fi
}

# Run Neo Express contract tests
run_neoexpress_tests() {
    log_info "Running Neo Express tests..."
    cd "$PROJECT_ROOT"

    # Check if neoxp is available
    NEOXP=""
    if command -v neoxp &> /dev/null; then
        NEOXP="neoxp"
    elif [ -x "$HOME/.dotnet/tools/neoxp" ]; then
        NEOXP="$HOME/.dotnet/tools/neoxp"
    else
        log_warn "Neo Express (neoxp) not found - skipping Neo Express tests"
        log_warn "Install with: dotnet tool install Neo.Express -g"
        return 0
    fi

    log_info "Using Neo Express: $NEOXP"

    go test -v -tags=neoexpress ./test/neo-express/... 2>&1 | tee test-neoexpress.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "Neo Express tests passed"
    else
        log_error "Neo Express tests failed"
        return 1
    fi
}

# Run postgres integration tests
run_postgres_tests() {
    log_info "Running Postgres integration tests..."
    cd "$PROJECT_ROOT"

    if [ -z "$TEST_POSTGRES_DSN" ] && [ -z "$DATABASE_URL" ]; then
        log_warn "TEST_POSTGRES_DSN or DATABASE_URL not set - skipping Postgres tests"
        return 0
    fi

    DSN="${TEST_POSTGRES_DSN:-$DATABASE_URL}"
    export TEST_POSTGRES_DSN="$DSN"
    export DATABASE_URL="$DSN"

    go test -v -tags=integration,postgres ./internal/app/storage/postgres/... 2>&1 | tee test-postgres.log
    go test -v -tags=integration,postgres ./internal/app/httpapi/... 2>&1 | tee -a test-postgres.log

    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "Postgres integration tests passed"
    else
        log_error "Postgres integration tests failed"
        return 1
    fi
}

# Run all tests
run_all_tests() {
    local failed=0

    run_unit_tests || failed=1
    run_smoke_tests || failed=1
    run_integration_tests || failed=1
    run_postgres_tests || failed=1
    run_neoexpress_tests || failed=1

    if [ $failed -eq 0 ]; then
        log_info "All tests passed!"
    else
        log_error "Some tests failed"
        return 1
    fi
}

# Print usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  unit          Run unit tests (default)"
    echo "  coverage      Run unit tests with coverage"
    echo "  integration   Run integration tests (requires running server)"
    echo "  smoke         Run smoke tests (requires running server)"
    echo "  neoexpress    Run Neo Express contract tests"
    echo "  postgres      Run Postgres integration tests"
    echo "  all           Run all tests"
    echo ""
    echo "Environment variables:"
    echo "  TEST_API_URL       API URL for integration tests (default: http://localhost:8080)"
    echo "  TEST_DEV_TOKEN     Dev token for authentication (default: dev-token)"
    echo "  TEST_TENANT_ID     Tenant ID for tests (default: integration-tenant)"
    echo "  TEST_POSTGRES_DSN  PostgreSQL connection string"
    echo "  DATABASE_URL       Alternative PostgreSQL connection string"
}

# Main
case "${1:-unit}" in
    unit)
        run_unit_tests
        ;;
    coverage)
        run_unit_coverage
        ;;
    integration)
        run_integration_tests
        ;;
    smoke)
        run_smoke_tests
        ;;
    neoexpress|neo-express|neo)
        run_neoexpress_tests
        ;;
    postgres|pg)
        run_postgres_tests
        ;;
    all)
        run_all_tests
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        log_error "Unknown command: $1"
        usage
        exit 1
        ;;
esac
