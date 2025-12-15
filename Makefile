# =============================================================================
# Neo Service Layer - Makefile
# MarbleRun + EGo + Supabase + Vercel Architecture
# =============================================================================

.PHONY: all build test clean docker frontend deploy help

# Variables
SERVICES := gateway neooracle neorand neovault neostore neofeeds gasbank neoflow neocompute neoaccounts ccip datalink datastreams dta cre
DOCKER_COMPOSE_SIM := docker compose -f docker/docker-compose.simulation.yaml
DOCKER_COMPOSE_SGX := docker compose -f docker/docker-compose.yaml
# Default to simulation mode for local development.
DOCKER_COMPOSE := $(DOCKER_COMPOSE_SIM)

GOBIN ?= $(shell go env GOPATH)/bin
GOLANGCI_LINT_VERSION ?= v1.64.8
GOLANGCI_LINT ?= $(GOBIN)/golangci-lint

COORDINATOR_CLIENT_ADDR ?= localhost:4433
INSECURE ?= 1
MARBLERUN_FLAGS :=
ifneq ($(filter 1 true yes,$(INSECURE)),)
  MARBLERUN_FLAGS += --insecure
endif

# =============================================================================
# Build
# =============================================================================

all: build

build: ## Build all services
	@echo "Building all services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		go build -o bin/$$service ./cmd/$$service 2>/dev/null || \
		go build -o bin/$$service ./services/$$service 2>/dev/null || true; \
	done
	@echo "Build complete"

build-gateway: ## Build gateway service
	go build -o bin/gateway ./cmd/gateway

build-ego: ## Build with EGo for SGX
	@echo "Building with EGo..."
	@for service in $(SERVICES); do \
		echo "Building $$service with EGo..."; \
		ego-go build -o bin/$$service ./cmd/$$service 2>/dev/null || \
		ego-go build -o bin/$$service ./services/$$service 2>/dev/null || true; \
	done

sign-enclaves: ## Sign all enclave binaries
	@echo "Signing enclaves..."
	@for service in $(SERVICES); do \
		if [ -f bin/$$service ]; then \
			ego sign bin/$$service; \
		fi; \
	done

# =============================================================================
# Test
# =============================================================================

test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	go test -v -short ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

test-e2e: ## Run end-to-end tests
	@echo "Running e2e tests..."
	go test -v -tags=e2e ./test/e2e/...

test-watch: ## Run tests in watch mode
	@echo "Running tests in watch mode..."
	@which gotestsum > /dev/null || go install gotest.tools/gotestsum@latest
	gotestsum --watch

# =============================================================================
# Docker
# =============================================================================

docker-build: ## Build all Docker images
	$(DOCKER_COMPOSE) build

docker-up: ## Start all services in simulation mode
	./scripts/up.sh --insecure

docker-up-sgx: ## Start all services with SGX hardware
	./scripts/up.sh

docker-up-tee: docker-up-sgx ## Alias for docker-up-sgx

docker-down: ## Stop all services
	$(DOCKER_COMPOSE) down

docker-logs: ## View logs
	$(DOCKER_COMPOSE) logs -f

docker-ps: ## List running containers
	$(DOCKER_COMPOSE) ps

docker-clean: ## Remove all containers and volumes
	$(DOCKER_COMPOSE) down -v --rmi local

# =============================================================================
# MarbleRun
# =============================================================================

marblerun-install: ## Install MarbleRun CLI
	curl -fsSL https://github.com/edgelesssys/marblerun/releases/latest/download/marblerun-linux-amd64 -o /usr/local/bin/marblerun
	chmod +x /usr/local/bin/marblerun

marblerun-manifest: ## Set MarbleRun manifest
	marblerun manifest set manifests/manifest.json $(COORDINATOR_CLIENT_ADDR) $(MARBLERUN_FLAGS)

marblerun-status: ## Check MarbleRun status
	marblerun status $(COORDINATOR_CLIENT_ADDR) $(MARBLERUN_FLAGS)

marblerun-recover: ## Recover MarbleRun coordinator
	marblerun recover manifests/recovery-key.json $(COORDINATOR_CLIENT_ADDR) $(MARBLERUN_FLAGS)

# =============================================================================
# Database
# =============================================================================

db-migrate: ## Run database migrations
	@echo "Running migrations..."
	psql "$(DATABASE_URL)" -f migrations/001_initial_schema.sql

db-seed: ## Seed database with test data
	@echo "Seeding database..."
	go run scripts/seed.go

# =============================================================================
# Frontend
# =============================================================================

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-dev: ## Start frontend development server
	cd frontend && npm run dev

frontend-build: ## Build frontend for production
	cd frontend && npm run build

frontend-deploy: ## Deploy frontend to Vercel
	cd frontend && npm ci && npm run build
	vercel deploy --prod

# =============================================================================
# Development
# =============================================================================

dev: ## Start development environment
	@echo "Starting development environment..."
	@./scripts/install_dev_env.sh --skip-k8s || echo "Dependencies already installed"
	OE_SIMULATION=1 $(DOCKER_COMPOSE) up -d coordinator
	@sleep 5
	@echo "Setting manifest..."
	INSECURE=1 $(MAKE) marblerun-manifest || true
	@echo "Starting gateway..."
	OE_SIMULATION=1 go run ./cmd/gateway

dev-full: ## Start full development environment with all services
	@echo "Starting full development environment..."
	@./scripts/deploy_k8s.sh --env dev

dev-stop: ## Stop development environment
	@echo "Stopping development environment..."
	$(DOCKER_COMPOSE) down

dev-gateway: ## Run gateway in development mode
	OE_SIMULATION=1 go run ./cmd/gateway

lint: ## Run linter
	@test -x $(GOLANGCI_LINT) || (echo "Installing golangci-lint..." && GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))
	$(GOLANGCI_LINT) run ./...

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

tidy: ## Tidy go modules
	go mod tidy

# =============================================================================
# Deployment
# =============================================================================

deploy-staging: ## Deploy to staging
	@echo "Deploying to staging..."
	$(DOCKER_COMPOSE) -f docker/docker-compose.staging.yaml up -d

deploy-production: ## Deploy to production
	@echo "Deploying to production..."
	$(DOCKER_COMPOSE) -f docker/docker-compose.production.yaml up -d

# =============================================================================
# Utilities
# =============================================================================

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf coverage.out coverage.html
	rm -rf frontend/dist
	rm -rf tmp/
	@echo "Clean complete"

clean-all: ## Clean everything including Docker images
	@echo "Cleaning everything..."
	$(MAKE) clean
	$(DOCKER_COMPOSE) down -v --rmi local
	docker system prune -f
	@echo "Deep clean complete"

generate: ## Generate code
	go generate ./...

docs: ## Generate documentation
	godoc -http=:6060

version: ## Show version
	@echo "Neo Service Layer v1.0.0"
	@echo "MarbleRun + EGo + Supabase + Vercel"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install gotest.tools/gotestsum@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Tools installed"

setup: ## Setup development environment
	@echo "Setting up development environment..."
	@./scripts/install_dev_env.sh --all
	$(MAKE) install-tools
	@echo "Setup complete"

check: ## Run all checks (lint, test, build)
	@echo "Running all checks..."
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) build
	@echo "All checks passed"

metrics: ## Show code metrics
	@echo "Code metrics:"
	@echo "Lines of code:"
	@find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1
	@echo ""
	@echo "Test coverage:"
	@go test -cover ./... | grep coverage || echo "Run 'make test-coverage' first"

# =============================================================================
# Help
# =============================================================================

help: ## Show this help
	@echo "Neo Service Layer - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
