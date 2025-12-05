# Service Layer Contracts Makefile
# Build, deploy, and test Neo N3 smart contracts

.PHONY: all build deploy test clean help express-start express-stop express-reset

# Default target
all: build

# =============================================================================
# Build Targets
# =============================================================================

# Build all contracts
build:
	@echo "=== Building Service Layer Contracts ==="
	@go run ../tools/contract-cli/main.go build -all

# Build specific contract
build-%:
	@echo "=== Building $* ==="
	@go run ../tools/contract-cli/main.go build -contract $*

# =============================================================================
# Deploy Targets
# =============================================================================

# Deploy to neo-express (local development)
deploy-express: express-start
	@echo "=== Deploying to neo-express ==="
	@go run ../tools/contract-cli/main.go deploy -network neo-express -init

# Deploy to testnet
deploy-testnet:
	@echo "=== Deploying to testnet ==="
	@echo "WARNING: This will deploy to Neo N3 TestNet"
	@read -p "Continue? [y/N] " confirm && [ "$$confirm" = "y" ]
	@go run ../tools/contract-cli/main.go deploy -network testnet -init

# Deploy to mainnet
deploy-mainnet:
	@echo "=== Deploying to mainnet ==="
	@echo "WARNING: This will deploy to Neo N3 MainNet"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ]
	@go run ../tools/contract-cli/main.go deploy -network mainnet -init

# =============================================================================
# Initialize Targets
# =============================================================================

# Initialize contracts on neo-express
init-express:
	@echo "=== Initializing contracts on neo-express ==="
	@go run ../tools/contract-cli/main.go init -network neo-express

# Initialize with custom service layer address
init-express-with-tee:
	@echo "=== Initializing with TEE address ==="
	@go run ../tools/contract-cli/main.go init -network neo-express -service-layer $(TEE_ADDRESS)

# =============================================================================
# Test Targets
# =============================================================================

# Run all tests (unit + integration)
test: test-unit test-integration

# Run unit tests only
test-unit:
	@echo "=== Running Unit Tests ==="
	@go test -v ./...

# Run integration tests with neo-express
test-integration: express-start build deploy-express
	@echo "=== Running Integration Tests ==="
	@go test -v -tags=integration ../test/contracts/...

# Run quick smoke test
test-smoke: express-start
	@echo "=== Running Smoke Tests ==="
	@go run ../tools/contract-cli/main.go status

# =============================================================================
# Neo-Express Management
# =============================================================================

# Start neo-express
express-start:
	@echo "=== Starting neo-express ==="
	@go run ../tools/contract-cli/main.go express -action start

# Stop neo-express
express-stop:
	@echo "=== Stopping neo-express ==="
	@go run ../tools/contract-cli/main.go express -action stop

# Reset neo-express (clean state)
express-reset:
	@echo "=== Resetting neo-express ==="
	@go run ../tools/contract-cli/main.go express -action reset

# Create checkpoint
express-checkpoint:
	@echo "=== Creating checkpoint ==="
	@go run ../tools/contract-cli/main.go express -action checkpoint

# =============================================================================
# Status and Info
# =============================================================================

# Show deployment status
status:
	@go run ../tools/contract-cli/main.go status

# Show help
help:
	@go run ../tools/contract-cli/main.go help

# =============================================================================
# Clean
# =============================================================================

# Clean build artifacts
clean:
	@echo "=== Cleaning build artifacts ==="
	@rm -rf build/*.nef build/*.manifest.json build/deployment-*.json
	@echo "Done"

# Clean everything including neo-express data
clean-all: clean express-reset
	@echo "=== Full clean complete ==="

# =============================================================================
# Development Workflow
# =============================================================================

# Full development cycle: build, deploy, test
dev: clean build deploy-express test-integration
	@echo "=== Development cycle complete ==="

# Quick rebuild and redeploy
rebuild: build deploy-express
	@echo "=== Rebuild complete ==="

# Watch for changes and rebuild (requires entr)
watch:
	@echo "=== Watching for changes ==="
	@find . -name "*.cs" | entr -c make build
