# Service Layer Contract Deployment Makefile
# Usage: make [target] [NETWORK=neoexpress|testnet]

NETWORK ?= neoexpress
SHELL := /bin/bash

.PHONY: help setup build deploy init test clean all

help:
	@echo "Service Layer Contract Deployment"
	@echo ""
	@echo "Usage: make [target] [NETWORK=neoexpress|testnet]"
	@echo ""
	@echo "Targets:"
	@echo "  setup     - Setup Neo Express environment (wallets, config)"
	@echo "  build     - Build all contracts"
	@echo "  deploy    - Deploy contracts to network"
	@echo "  init      - Initialize deployed contracts"
	@echo "  test      - Run contract tests"
	@echo "  all       - Run full deployment (setup, build, deploy, init)"
	@echo "  clean     - Clean build artifacts"
	@echo ""
	@echo "Networks:"
	@echo "  neoexpress (default) - Local Neo Express"
	@echo "  testnet              - Neo N3 TestNet"
	@echo ""
	@echo "Examples:"
	@echo "  make setup                    # Setup Neo Express"
	@echo "  make all                      # Full local deployment"
	@echo "  make deploy NETWORK=testnet   # Deploy to testnet"

# Setup Neo Express environment
setup:
	@echo "=== Setting up Neo Express environment ==="
	chmod +x deploy/scripts/setup_neoexpress.sh
	./deploy/scripts/setup_neoexpress.sh

# Build all contracts
build:
	@echo "=== Building contracts ==="
	cd contracts && ./build.sh

# Deploy contracts to network
deploy:
	@echo "=== Deploying contracts to $(NETWORK) ==="
	chmod +x deploy/scripts/deploy_all.sh
	./deploy/scripts/deploy_all.sh $(NETWORK)

# Initialize deployed contracts
init:
	@echo "=== Initializing contracts on $(NETWORK) ==="
	python3 deploy/scripts/initialize.py $(NETWORK)

# Run contract tests
test:
	@echo "=== Running contract tests on $(NETWORK) ==="
	python3 deploy/scripts/run_tests.py $(NETWORK)

# Run Go tests
test-go:
	@echo "=== Running Go tests ==="
	go test ./test/contract/... -v -short

# Full deployment
all: setup build deploy init
	@echo "=== Full deployment complete ==="

# Clean build artifacts
clean:
	@echo "=== Cleaning build artifacts ==="
	rm -rf contracts/build/*.nef
	rm -rf contracts/build/*.manifest.json
	rm -f deploy/config/deployed_contracts.json

# Start Neo Express
run-neoexpress:
	@echo "=== Starting Neo Express ==="
	neoxp run -i deploy/config/default.neo-express

# Check dependencies
check-deps:
	@echo "=== Checking dependencies ==="
	@command -v neoxp >/dev/null 2>&1 || { echo "neoxp not found. Install: dotnet tool install -g Neo.Express"; exit 1; }
	@command -v nccs >/dev/null 2>&1 || { echo "nccs not found. Install: dotnet tool install -g Neo.Compiler.CSharp"; exit 1; }
	@command -v python3 >/dev/null 2>&1 || { echo "python3 not found"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "go not found"; exit 1; }
	@echo "All dependencies installed."

# Show deployed contracts
show-deployed:
	@echo "=== Deployed Contracts ==="
	@cat deploy/config/deployed_contracts.json 2>/dev/null || echo "No contracts deployed yet"

# Test single service
test-datafeeds:
	python3 deploy/scripts/run_tests.py $(NETWORK) datafeeds
