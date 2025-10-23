.PHONY: build run test clean tidy help docker-build docker-run docker-compose migrate-up migrate-down

# Default target
all: help

BIN?=./bin/appserver

build:
	@echo "Building appserver..."
	@go build -o $(BIN) ./cmd/appserver

run:
	@echo "Running appserver..."
	@go run ./cmd/appserver

test:
	@echo "Running tests..."
	@go test ./...

tidy:
	@echo "Tidying modules..."
	@go mod tidy

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ./bin

migrate-up:
	@echo "Running database migrations up..."
	@migrate -path internal/platform/migrations -database "$(shell grep DATABASE_URL .env 2>/dev/null | cut -d '=' -f2)" up

migrate-down:
	@echo "Running database migrations down..."
	@migrate -path internal/platform/migrations -database "$(shell grep DATABASE_URL .env 2>/dev/null | cut -d '=' -f2)" down

docker-build:
	@echo "Building Docker image for appserver..."
	@docker build -t service_layer:latest .

docker-run:
	@echo "Running Docker image..."
	@docker run -p 8080:8080 --env-file .env service_layer:latest

docker-compose:
	@echo "Running with Docker Compose..."
	@docker compose up -d

help:
	@echo "service_layer make commands:"
	@echo "  build          - Build the refactored appserver"
	@echo "  run            - Run the refactored appserver"
	@echo "  test           - Run tests for the refactored runtime"
	@echo "  tidy           - Run go mod tidy"
	@echo "  clean          - Remove build artifacts"
	@echo "  migrate-up     - Apply database migrations"
	@echo "  migrate-down   - Rollback database migrations"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker image"
	@echo "  docker-compose - Start Docker Compose stack"
