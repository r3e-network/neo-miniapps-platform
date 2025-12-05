#!/bin/bash
# Service Layer Contract Integration Test Runner
# This script builds, deploys, and tests all contracts on neo-express

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NEO_EXPRESS_CONFIG="$PROJECT_ROOT/test/neo-express/default.neo-express"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check for dotnet
    if ! command -v dotnet &> /dev/null; then
        log_error "dotnet not found. Install .NET SDK 7.0+"
        exit 1
    fi

    # Check for neo-express (neoxp)
    if ! command -v neoxp &> /dev/null; then
        log_warning "neoxp not found. Installing neo-express..."
        dotnet tool install -g Neo.Express
    fi

    # Check for nccs (Neo C# Compiler)
    if ! command -v nccs &> /dev/null; then
        log_warning "nccs not found. Installing Neo.Compiler.CSharp..."
        dotnet tool install -g Neo.Compiler.CSharp
    fi

    # Check for Go
    if ! command -v go &> /dev/null; then
        log_error "go not found. Install Go 1.21+"
        exit 1
    fi

    log_success "All prerequisites satisfied"
}

# Setup neo-express
setup_neo_express() {
    log_info "Setting up neo-express..."

    mkdir -p "$PROJECT_ROOT/test/neo-express"

    # Create neo-express config if not exists
    if [ ! -f "$NEO_EXPRESS_CONFIG" ]; then
        log_info "Creating neo-express configuration..."
        cd "$PROJECT_ROOT/test/neo-express"
        neoxp create -f -o default.neo-express
    fi

    log_success "Neo-express configured"
}

# Start neo-express
start_neo_express() {
    log_info "Starting neo-express..."

    # Check if already running
    if neoxp show node --input "$NEO_EXPRESS_CONFIG" &> /dev/null; then
        log_info "Neo-express is already running"
        return 0
    fi

    # Start neo-express in background
    cd "$PROJECT_ROOT/test/neo-express"
    neoxp run --input "$NEO_EXPRESS_CONFIG" --seconds-per-block 1 &
    NEO_EXPRESS_PID=$!

    # Wait for startup
    log_info "Waiting for neo-express to start..."
    sleep 5

    # Verify it's running
    if neoxp show node --input "$NEO_EXPRESS_CONFIG" &> /dev/null; then
        log_success "Neo-express started (PID: $NEO_EXPRESS_PID)"
    else
        log_error "Failed to start neo-express"
        exit 1
    fi
}

# Reset neo-express
reset_neo_express() {
    log_info "Resetting neo-express to clean state..."
    neoxp reset -f --input "$NEO_EXPRESS_CONFIG" || true
    log_success "Neo-express reset"
}

# Build contracts
build_contracts() {
    log_info "Building contracts..."

    cd "$PROJECT_ROOT"

    # Build contract CLI tool
    go build -o bin/contract-cli ./tools/contract-cli

    # Build all contracts
    ./bin/contract-cli build -all

    log_success "Contracts built"
}

# Deploy contracts
deploy_contracts() {
    log_info "Deploying contracts to neo-express..."

    cd "$PROJECT_ROOT"
    ./bin/contract-cli deploy -network neo-express -init

    log_success "Contracts deployed and initialized"
}

# Run integration tests
run_integration_tests() {
    log_info "Running integration tests..."

    cd "$PROJECT_ROOT"

    # Set environment variables
    export NEO_EXPRESS=true
    export NEO_EXPRESS_CONFIG="$NEO_EXPRESS_CONFIG"

    # Run tests
    go test -v -tags=integration ./test/contracts/...

    log_success "Integration tests completed"
}

# Run unit tests
run_unit_tests() {
    log_info "Running unit tests..."

    cd "$PROJECT_ROOT"
    go test -v ./...

    log_success "Unit tests completed"
}

# Stop neo-express
stop_neo_express() {
    log_info "Stopping neo-express..."
    neoxp stop --input "$NEO_EXPRESS_CONFIG" || true
    log_success "Neo-express stopped"
}

# Show status
show_status() {
    log_info "Contract deployment status:"
    cd "$PROJECT_ROOT"
    ./bin/contract-cli status
}

# Main execution
main() {
    echo "=============================================="
    echo "  Service Layer Contract Test Runner"
    echo "=============================================="
    echo ""

    case "${1:-full}" in
        "full")
            check_prerequisites
            setup_neo_express
            start_neo_express
            reset_neo_express
            build_contracts
            deploy_contracts
            run_integration_tests
            show_status
            log_success "Full test cycle completed!"
            ;;
        "build")
            check_prerequisites
            build_contracts
            ;;
        "deploy")
            check_prerequisites
            setup_neo_express
            start_neo_express
            deploy_contracts
            show_status
            ;;
        "test")
            run_integration_tests
            ;;
        "unit")
            run_unit_tests
            ;;
        "start")
            setup_neo_express
            start_neo_express
            ;;
        "stop")
            stop_neo_express
            ;;
        "reset")
            reset_neo_express
            ;;
        "status")
            show_status
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  full    - Run full test cycle (default)"
            echo "  build   - Build contracts only"
            echo "  deploy  - Deploy contracts to neo-express"
            echo "  test    - Run integration tests"
            echo "  unit    - Run unit tests"
            echo "  start   - Start neo-express"
            echo "  stop    - Stop neo-express"
            echo "  reset   - Reset neo-express"
            echo "  status  - Show deployment status"
            echo "  help    - Show this help"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Run '$0 help' for usage"
            exit 1
            ;;
    esac
}

# Run main
main "$@"
