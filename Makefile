# =============================================================================
# Neo Service Layer - Makefile
# MarbleRun + EGo + Supabase + Netlify Architecture
# =============================================================================

.PHONY: all build test clean docker frontend deploy help

# Variables
SERVICES := gateway oracle vrf mixer secrets datafeeds gasbank automation confidential accounts ccip datalink datastreams dta cre
DOCKER_COMPOSE := docker compose -f docker/docker-compose.yaml

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
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration: ## Run integration tests
	go test -v -tags=integration ./test/integration/...

# =============================================================================
# Docker
# =============================================================================

docker-build: ## Build all Docker images
	$(DOCKER_COMPOSE) build

docker-up: ## Start all services in simulation mode
	OE_SIMULATION=1 $(DOCKER_COMPOSE) up -d

docker-up-sgx: ## Start all services with SGX hardware
	OE_SIMULATION=0 $(DOCKER_COMPOSE) up -d

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
	marblerun manifest set manifests/manifest.json localhost:4433 --insecure

marblerun-status: ## Check MarbleRun status
	marblerun status localhost:4433 --insecure

marblerun-recover: ## Recover MarbleRun coordinator
	marblerun recover manifests/recovery-key.json localhost:4433 --insecure

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

frontend-deploy: ## Deploy frontend to Netlify
	cd frontend && netlify deploy --prod

# =============================================================================
# Development
# =============================================================================

dev: ## Start development environment
	@echo "Starting development environment..."
	OE_SIMULATION=1 $(DOCKER_COMPOSE) up -d coordinator
	@sleep 5
	@echo "Setting manifest..."
	marblerun manifest set manifests/manifest.json localhost:4433 --insecure || true
	@echo "Starting gateway..."
	OE_SIMULATION=1 go run ./cmd/gateway

dev-gateway: ## Run gateway in development mode
	OE_SIMULATION=1 go run ./cmd/gateway

lint: ## Run linter
	golangci-lint run ./...

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
	rm -rf bin/
	rm -rf coverage.out coverage.html
	rm -rf frontend/dist

generate: ## Generate code
	go generate ./...

docs: ## Generate documentation
	godoc -http=:6060

version: ## Show version
	@echo "Neo Service Layer v1.0.0"
	@echo "MarbleRun + EGo + Supabase + Netlify"

# =============================================================================
# Help
# =============================================================================

help: ## Show this help
	@echo "Neo Service Layer - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
