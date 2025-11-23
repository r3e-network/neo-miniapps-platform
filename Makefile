.PHONY: build run run-local test clean docker docker-run docker-compose docker-compose-run help build-dashboard

BIN_DIR?=./bin
APP_BIN?=$(BIN_DIR)/appserver
CLI_BIN?=$(BIN_DIR)/slctl
APP_IMAGE?=service-layer:latest

build:
	@echo "Building appserver -> $(APP_BIN)"
	@mkdir -p $(BIN_DIR)
	@go build -o $(APP_BIN) ./cmd/appserver
	@echo "Building CLI -> $(CLI_BIN)"
	@go build -o $(CLI_BIN) ./cmd/slctl

run:
	@echo "Starting stack via docker compose (appserver + postgres + dashboard)..."
	@echo "API:       http://localhost:8080   (Authorization: Bearer dev-token or JWT from /auth/login)"
	@echo "Dashboard: http://localhost:8081   (use admin/changeme via /auth/login to obtain JWT)"
	@echo "Site:      http://localhost:8082   (public marketing/docs entry)"
	@echo "Ensuring .env exists (copying .env.example if missing)..."
	@[ -f .env ] || (cp .env.example .env && echo "  > created .env from .env.example")
	@echo "Stopping any existing stack..."
	@docker compose down --remove-orphans >/dev/null 2>&1 || true
	@docker compose up -d --build
	@docker compose ps

run-local:
	@echo "Running appserver locally (expects Postgres via DATABASE_URL or config)..."
	@go run ./cmd/appserver

test:
	@echo "Running tests..."
	@go test ./...

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ./bin

build-dashboard:
	@echo "Building dashboard..."
	@cd apps/dashboard && npm install && npm run build

docker:
	@echo "Building Docker images (appserver + dashboard)..."
	@docker build -t $(APP_IMAGE) .
	@docker build -t service-layer-dashboard:latest ./apps/dashboard

docker-run:
	@echo "Running appserver image..."
	@docker run -p 8080:8080 --env-file .env $(APP_IMAGE)

docker-compose:
	@echo "Starting stack with docker compose (appserver + postgres + dashboard)..."
	@docker compose up -d --build

docker-compose-run:
	@echo "Running stack in foreground (Ctrl+C to stop)..."
	@docker compose up --build

down:
	@echo "Stopping stack (docker compose down)..."
	@docker compose down --remove-orphans

ps:
	@docker compose ps

logs:
	@echo "Tailing appserver logs (Ctrl+C to stop)..."
	@docker compose logs -f service-layer

help:
	@echo "make build             - Build appserver and CLI binaries into $(BIN_DIR)"
	@echo "make run               - Start the stack with docker compose (detached; uses .env)"
	@echo "make run-local         - Run appserver locally (requires Postgres available)"
	@echo "make test              - Run Go tests"
	@echo "make typecheck         - Run dashboard typecheck (npm required)"
	@echo "make smoke             - Run Go tests + dashboard typecheck"
	@echo "make clean             - Remove build artifacts"
	@echo "make build-dashboard   - Build the React dashboard (needs npm)"
	@echo "make docker            - Build appserver and dashboard Docker images"
	@echo "make docker-run        - Run appserver image (reads .env)"
	@echo "make docker-compose    - Bring up appserver+postgres+dashboard"
	@echo "make docker-compose-run - Bring up stack in foreground"
	@echo "make down              - Stop the compose stack and remove orphans"
	@echo "make ps                - Show docker compose service status"
	@echo "make logs              - Tail appserver logs from the compose stack"
	@echo "make neo-up            - Start NEO mainnet/testnet nodes (compose profile 'neo')"
	@echo "make neo-down          - Stop NEO nodes (compose profile 'neo')"

typecheck:
	@echo "Running dashboard typecheck..."
	@cd apps/dashboard && npm install && npm run typecheck

smoke: test typecheck
	@echo "Smoke checks complete."

neo-up:
	@echo "Starting NEO mainnet/testnet nodes (profile: neo)..."
	@docker compose --profile neo up -d neo-mainnet neo-testnet

neo-down:
	@echo "Stopping NEO nodes (profile: neo)..."
	@docker compose --profile neo down --remove-orphans
