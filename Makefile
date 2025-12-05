# Service Layer Makefile
# Professional build automation for the entire project

.PHONY: all build clean test run dev help
.PHONY: build-server build-contracts build-frontend
.PHONY: build-coordinator build-marble build-ego
.PHONY: run-server run-frontend run-neo-express
.PHONY: run-coordinator run-marble
.PHONY: docker-build docker-up docker-down
.PHONY: docker-ego-build docker-ego-up
.PHONY: lint fmt check

# Default target
all: build

# =============================================================================
# Build Targets
# =============================================================================

build: build-server build-contracts build-frontend ## Build all components
	@echo "✓ All components built successfully"

build-server: ## Build the Go server
	@echo "Building server..."
	@go build -o bin/server ./cmd/server/
	@echo "✓ Server built: bin/server"

build-contracts: ## Build Neo N3 smart contracts
	@echo "Building contracts..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && \
	mkdir -p build && \
	nccs ServiceLayerGateway/ServiceLayerGateway.csproj -o build/ && \
	nccs OracleService/OracleService.csproj -o build/ && \
	nccs VRFService/VRFService.csproj -o build/ && \
	nccs DataFeedsService/DataFeedsService.csproj -o build/ && \
	nccs AutomationService/AutomationService.csproj -o build/
	@echo "✓ Contracts built: contracts/build/"

build-frontend: ## Build the frontend
	@echo "Building frontend..."
	@cd frontend && npm install && npm run build
	@echo "✓ Frontend built: frontend/dist/"

# =============================================================================
# MarbleRun + EGo Build Targets
# =============================================================================

build-coordinator: ## Build the Coordinator (MarbleRun control plane)
	@echo "Building Coordinator..."
	@go build -o bin/coordinator ./cmd/coordinator/
	@echo "✓ Coordinator built: bin/coordinator"

build-marble: ## Build the Marble runner (generic service runner)
	@echo "Building Marble runner..."
	@go build -o bin/marble ./cmd/marble/
	@echo "✓ Marble built: bin/marble"

build-marblerun: build-coordinator build-marble ## Build all MarbleRun components
	@echo "✓ MarbleRun components built"

build-ego-coordinator: ## Build Coordinator as SGX enclave with EGo
	@echo "Building Coordinator with EGo..."
	@ego-go build -o bin/coordinator-ego ./cmd/coordinator/
	@cp ego/coordinator-enclave.json bin/enclave.json
	@cd bin && ego sign coordinator-ego
	@echo "✓ EGo Coordinator built and signed: bin/coordinator-ego"

build-ego-marble: ## Build Marble as SGX enclave with EGo
	@echo "Building Marble with EGo..."
	@ego-go build -o bin/marble-ego ./cmd/marble/
	@cp ego/enclave.json bin/enclave.json
	@cd bin && ego sign marble-ego
	@echo "✓ EGo Marble built and signed: bin/marble-ego"

build-ego: build-ego-coordinator build-ego-marble ## Build all EGo SGX enclaves
	@echo "✓ All EGo enclaves built"

# =============================================================================
# Run Targets
# =============================================================================

run: build-server ## Run everything (server + frontend + neo-express)
	@echo "=============================================="
	@echo "  Starting Service Layer - Full Stack"
	@echo "=============================================="
	@echo ""
	@echo "Starting Neo Express..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && (neoxp run -i default.neo-express > /tmp/neo-express.log 2>&1 &) && sleep 2
	@echo "✓ Neo Express started (log: /tmp/neo-express.log)"
	@echo ""
	@echo "Starting Frontend..."
	@cd frontend && (npm run dev > /tmp/frontend.log 2>&1 &) && sleep 2
	@echo "✓ Frontend started (log: /tmp/frontend.log)"
	@echo ""
	@echo "=============================================="
	@echo "  Services:"
	@echo "    Backend:      http://localhost:8080"
	@echo "    Frontend:     http://localhost:3000"
	@echo "    Metrics:      http://localhost:9090"
	@echo "    API Docs:     http://localhost:8080/api/info"
	@echo "=============================================="
	@echo ""
	@echo "Starting Backend Server (foreground)..."
	@./bin/server --mode=simulation --debug

run-server: build-server ## Run only the server
	@echo "Starting Service Layer server..."
	@./bin/server --mode=simulation --debug

run-frontend: ## Run only the frontend dev server
	@echo "Starting frontend dev server..."
	@cd frontend && npm run dev

run-neo-express: ## Run only Neo Express local blockchain
	@echo "Starting Neo Express..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && neoxp run -i default.neo-express

run-all-bg: build-server ## Run all services in background
	@echo "Starting all services in background..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && (neoxp run -i default.neo-express > /tmp/neo-express.log 2>&1 &)
	@cd frontend && (npm run dev > /tmp/frontend.log 2>&1 &)
	@(./bin/server --mode=simulation --debug > /tmp/server.log 2>&1 &)
	@sleep 3
	@echo ""
	@echo "✓ All services started in background"
	@echo "  Logs:"
	@echo "    Server:       /tmp/server.log"
	@echo "    Frontend:     /tmp/frontend.log"
	@echo "    Neo Express:  /tmp/neo-express.log"
	@echo ""
	@echo "  URLs:"
	@echo "    Backend:      http://localhost:8080"
	@echo "    Frontend:     http://localhost:3000"
	@echo "    Metrics:      http://localhost:9090"

stop: ## Stop all background services
	@echo "Stopping all services..."
	@pkill -f "bin/server" || true
	@pkill -f "npm run dev" || true
	@pkill -f "neoxp run" || true
	@echo "✓ All services stopped"

status: ## Check status of all services
	@echo "Service Status:"
	@echo -n "  Backend:     " && (curl -s http://localhost:8080/health > /dev/null 2>&1 && echo "✓ Running" || echo "✗ Stopped")
	@echo -n "  Frontend:    " && (curl -s http://localhost:3000 > /dev/null 2>&1 && echo "✓ Running" || echo "✗ Stopped")
	@echo -n "  Metrics:     " && (curl -s http://localhost:9090/metrics > /dev/null 2>&1 && echo "✓ Running" || echo "✗ Stopped")

dev: run ## Alias for run

# =============================================================================
# Test Targets
# =============================================================================

test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@go test -v ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v -tags=integration ./test/...

test-contracts: ## Run contract tests
	@echo "Running contract tests..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && \
	neoxp contract invoke test-getAdmin.neo-invoke.json node1 -r -i default.neo-express

# =============================================================================
# Docker Targets
# =============================================================================

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@docker build -t service-layer:latest -f docker/Dockerfile .
	@echo "✓ Docker image built: service-layer:latest"

docker-up: ## Start all services with Docker Compose
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "✓ Services started"

docker-down: ## Stop all Docker services
	@echo "Stopping Docker services..."
	@docker-compose down
	@echo "✓ Services stopped"

docker-logs: ## View Docker logs
	@docker-compose logs -f

# =============================================================================
# MarbleRun + EGo Run Targets
# =============================================================================

run-coordinator: build-coordinator ## Run Coordinator (simulation mode)
	@echo "Starting Coordinator (simulation mode)..."
	@SIMULATION_MODE=true MANIFEST_PATH=manifests/development.yaml ./bin/coordinator

run-marble: build-marble ## Run a Marble service (specify MARBLE_TYPE)
	@echo "Starting Marble: $(MARBLE_TYPE)..."
	@SIMULATION_MODE=true COORDINATOR_ADDR=localhost:4433 ./bin/marble -type=$(MARBLE_TYPE)

run-marblerun: build-marblerun ## Run full MarbleRun stack (simulation)
	@echo "=============================================="
	@echo "  Starting MarbleRun Stack (Simulation)"
	@echo "=============================================="
	@SIMULATION_MODE=true MANIFEST_PATH=manifests/development.yaml ./bin/coordinator &
	@sleep 3
	@echo "Coordinator started on :4433"
	@echo ""
	@echo "Start Marbles with:"
	@echo "  make run-marble MARBLE_TYPE=oracle"
	@echo "  make run-marble MARBLE_TYPE=vrf"
	@echo "  make run-marble MARBLE_TYPE=secrets"

run-ego-coordinator: build-ego-coordinator ## Run Coordinator in SGX enclave
	@echo "Starting Coordinator in SGX enclave..."
	@cd bin && ego run coordinator-ego

run-ego-marble: build-ego-marble ## Run Marble in SGX enclave
	@echo "Starting Marble in SGX enclave: $(MARBLE_TYPE)..."
	@cd bin && MARBLE_TYPE=$(MARBLE_TYPE) ego run marble-ego

# =============================================================================
# Docker EGo Targets
# =============================================================================

docker-ego-build: ## Build EGo Docker images
	@echo "Building EGo Docker images..."
	@docker build -t neo-coordinator-sgx:latest -f docker/Dockerfile.ego-coordinator .
	@docker build -t neo-marble-sgx:latest -f docker/Dockerfile.ego-marble .
	@echo "✓ EGo Docker images built"

docker-ego-up: ## Start EGo SGX stack with Docker Compose
	@echo "Starting EGo SGX stack..."
	@docker-compose -f docker/docker-compose.ego.yaml up -d
	@echo "✓ EGo SGX services started"

docker-ego-down: ## Stop EGo SGX stack
	@echo "Stopping EGo SGX stack..."
	@docker-compose -f docker/docker-compose.ego.yaml down
	@echo "✓ EGo SGX services stopped"

docker-supabase-up: ## Start Supabase stack with Docker Compose
	@echo "Starting Supabase stack..."
	@docker-compose -f docker/docker-compose.yaml up -d supabase-db supabase-auth supabase-rest supabase-realtime supabase-storage supabase-kong
	@echo "✓ Supabase services started"

# =============================================================================
# Code Quality Targets
# =============================================================================

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./... || true
	@cd frontend && npm run lint || true
	@echo "✓ Linting complete"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@cd frontend && npm run format || true
	@echo "✓ Code formatted"

check: lint test ## Run all checks (lint + test)
	@echo "✓ All checks passed"

# =============================================================================
# Clean Targets
# =============================================================================

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf contracts/build/
	@rm -rf frontend/dist/
	@rm -rf frontend/node_modules/
	@echo "✓ Clean complete"

clean-all: clean ## Clean everything including caches
	@rm -rf sealed_store/
	@rm -f sealing.key
	@go clean -cache
	@echo "✓ Full clean complete"

# =============================================================================
# Deploy Targets
# =============================================================================

deploy-contracts: ## Deploy contracts to Neo Express
	@echo "Deploying contracts to Neo Express..."
	@export PATH="$$PATH:$$HOME/.dotnet/tools" && \
	cd contracts && \
	neoxp transfer 1000 GAS genesis node1 -i default.neo-express && \
	sleep 2 && \
	neoxp contract deploy build/ServiceLayerGateway.nef node1 -i default.neo-express && \
	neoxp contract deploy build/OracleService.nef node1 -i default.neo-express && \
	neoxp contract deploy build/VRFService.nef node1 -i default.neo-express && \
	neoxp contract deploy build/DataFeedsService.nef node1 -i default.neo-express && \
	neoxp contract deploy build/AutomationService.nef node1 -i default.neo-express
	@echo "✓ Contracts deployed"

# =============================================================================
# Help
# =============================================================================

help: ## Show this help
	@echo "Service Layer - Build & Development Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
