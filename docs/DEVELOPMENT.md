# Development Guide

This guide covers setting up your development environment and working with the Neo Service Layer codebase.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go 1.24+**: Programming language
- **Docker**: Container runtime
- **kubectl**: Kubernetes CLI
- **k3s**: Lightweight Kubernetes (for local development)
- **MarbleRun**: NeoCompute computing orchestration
- **EGo**: MarbleRun development framework

### Optional Tools

- **golangci-lint**: Code linting
- **gotestsum**: Enhanced test output
- **swag**: API documentation generation

## Quick Start

### 1. Automated Setup

The easiest way to get started is using the automated setup script:

```bash
# Install all dependencies and tools
make setup

# Or manually run the installation script
./scripts/install_dev_env.sh --all
```

This will install:
- MarbleRun/EGo SDK and PSW
- EGo runtime
- MarbleRun CLI
- k3s (Kubernetes)
- Helm
- Development tools

### 2. Manual Setup

If you prefer manual installation:

```bash
# Install prerequisites
sudo apt-get update
sudo apt-get install -y build-essential libssl-dev curl wget

# Install Go
wget https://go.dev/dl/go1.24.11.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.11.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install k3s
curl -sfL https://get.k3s.io | sh -

# Install MarbleRun
curl -fsSL https://github.com/edgelesssys/marblerun/releases/latest/download/marblerun-linux-amd64 -o /tmp/marblerun
chmod +x /tmp/marblerun
sudo mv /tmp/marblerun /usr/local/bin/

# Install EGo
sudo snap install ego-dev --classic

# Install development tools
make install-tools
```

### 3. Verify Installation

```bash
# Check versions
go version
docker --version
kubectl version --client
marblerun version
ego version

# Check Kubernetes cluster
kubectl get nodes
```

## Development Workflow

### Starting Development Environment

#### Option 1: Gateway Only (Fastest)

Start just the gateway service for quick development:

```bash
make dev
```

This will:
1. Start MarbleRun coordinator in simulation mode
2. Set the manifest
3. Run the gateway service

#### Option 2: Full Environment

Start all services in Kubernetes:

```bash
make dev-full
```

This will:
1. Build all Docker images
2. Import images to k3s
3. Deploy all services to Kubernetes
4. Set up MarbleRun manifest

#### Option 3: Custom Service

Run a specific service:

```bash
# Run confidential compute (NeoCompute)
OE_SIMULATION=1 SERVICE_TYPE=neocompute go run ./cmd/marble

# Run confidential oracle (NeoOracle)
OE_SIMULATION=1 SERVICE_TYPE=neooracle go run ./cmd/marble
```

### Stopping Development Environment

```bash
# Stop Docker Compose services
make dev-stop

# Or clean up Kubernetes deployment
./scripts/deploy_k8s.sh cleanup
```

### Code Quality Checks

Run all checks before committing:

```bash
# Run linter, tests, and build
make check

# Or run individually
make lint        # Run linter
make test        # Run tests
make build       # Build binaries
```

### Code Formatting

```bash
# Format all Go code
make fmt

# Tidy Go modules
make tidy
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run unit tests only (skip integration tests)
make test-unit

# Run tests with coverage
make test-coverage

# View coverage report
open coverage.html
```

### Integration Tests

```bash
# Run integration tests
make test-integration
```

### End-to-End Tests

```bash
# Run e2e tests
make test-e2e
```

### Watch Mode

Automatically run tests when files change:

```bash
make test-watch
```

### Writing Tests

Follow these patterns:

```go
// Unit test example
func TestServiceCreate(t *testing.T) {
    // Arrange
    service := NewService()

    // Act
    result, err := service.Create(context.Background(), input)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

// Integration test example
// +build integration

func TestServiceIntegration(t *testing.T) {
    // Setup
    db := setupTestDatabase(t)
    defer db.Close()

    // Test
    // ...
}
```

## Deployment

### Local Development (k3s)

```bash
# Deploy to local k3s
./scripts/deploy_k8s.sh --env dev

# Check status
./scripts/deploy_k8s.sh status

# View logs
kubectl -n service-layer logs -f deployment/gateway
```

### Test Environment

```bash
# Deploy to test environment (OE simulation mode + MARBLE_ENV=testing)
./scripts/deploy_k8s.sh --env test --registry docker.io/myorg --push

# Perform rolling update
./scripts/deploy_k8s.sh --env test --rolling-update update
```

### Production Environment

```bash
# Production (SGX hardware) requires enclave images to be signed with stable keys
# that match `manifests/manifest.json` SignerIDs.
#
# Build + push signed images (recommended: one key per service in a directory)
./scripts/deploy_k8s.sh --env prod --registry docker.io/myorg --signing-key-dir /path/to/signing-keys --push

# Deploy to production
./scripts/deploy_k8s.sh --env prod --skip-build deploy

# Perform rolling update
./scripts/deploy_k8s.sh --env prod --rolling-update update
```

### Dry Run

Test deployment without making changes:

```bash
./scripts/deploy_k8s.sh --env prod --dry-run all
```

## Monitoring

### Metrics

The service exposes Prometheus metrics at `/metrics`:

```bash
# View metrics
curl http://localhost:8080/metrics

# Key metrics:
# - http_requests_total: Total HTTP requests
# - http_request_duration_seconds: Request latency
# - errors_total: Total errors
# - blockchain_transactions_total: Blockchain transactions
# - database_queries_total: Database queries
# - service_uptime_seconds: Service uptime
```

### Logging

Structured logging with trace ID support:

```bash
# View logs with trace ID
kubectl -n service-layer logs -f deployment/gateway | jq 'select(.trace_id)'

# Filter by log level
kubectl -n service-layer logs -f deployment/gateway | jq 'select(.level=="error")'

# Follow specific trace
kubectl -n service-layer logs -f deployment/gateway | jq 'select(.trace_id=="abc-123")'
```

### Health Checks

```bash
# Check service health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready
```

## Project Structure

```
service_layer/
├── cmd/                    # Service entry points
│   ├── gateway/           # API Gateway
│   └── marble/            # Generic marble runner
├── services/              # Product services (enclave workloads)
│   ├── vrf/               # VRF (NeoRand)
│   ├── datafeed/          # Data feeds (NeoFeeds)
│   ├── automation/        # Automation (NeoFlow)
│   ├── confcompute/       # Confidential compute (NeoCompute)
│   └── conforacle/        # Confidential oracle (NeoOracle)
├── infrastructure/        # Shared building blocks (chain, runtime, middleware, storage)
│   ├── chain/             # Neo N3 RPC + tx + event monitoring
│   ├── middleware/        # HTTP middleware
│   ├── runtime/           # strict identity + runtime helpers
│   ├── accountpool/       # Account pool service + repo
│   ├── globalsigner/      # Global signer service + repo
│   └── secrets/           # Secrets manager + repo
├── contracts/             # Smart contracts
├── manifests/             # MarbleRun manifests
├── k8s/                   # Kubernetes manifests
│   ├── base/             # Base configuration
│   └── overlays/         # Environment overlays
├── scripts/               # Deployment scripts
├── test/                  # Tests
│   ├── integration/      # Integration tests
│   └── contract/         # Contract-flow tests
└── docs/                  # Documentation
```

## Common Tasks

### Adding a New Service

1. Create service directory:
```bash
mkdir -p services/myservice/marble
```

2. Implement service:
```go
// services/myservice/marble/service.go
package marble

type Service struct {
    // ...
}

func NewService() *Service {
    return &Service{}
}
```

3. Register it in the marble entrypoint + config:
- Add the service ID to `cmd/marble/main.go` (`availableServices` + switch)
- Add a default entry to `config/services.yaml`

4. Add Kubernetes manifests:
```bash
# Add to k8s/base/services-deployment.yaml
```

### Adding a New Endpoint

1. Define handler:
```go
func (s *Service) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Log request
    logger.WithContext(ctx).Info("Handling request")

    // Record metrics
    metrics.Global().IncrementInFlight()
    defer metrics.Global().DecrementInFlight()

    // Process request
    // ...
}
```

2. Register route:
```go
router.HandleFunc("/api/myendpoint", s.HandleRequest).Methods("POST")
```

3. Add tests:
```go
func TestHandleRequest(t *testing.T) {
    // ...
}
```

### Updating Dependencies

```bash
# Update all dependencies
go get -u ./...
go mod tidy

# Update specific dependency
go get -u github.com/example/package@latest
go mod tidy
```

### Generating API Documentation

```bash
# Install swag
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g cmd/gateway/main.go

# View docs
# Navigate to http://localhost:8080/swagger/index.html
```

## Troubleshooting

### Common Issues

#### 1. MarbleRun Coordinator Not Ready

```bash
# Check MarbleRun status
marblerun check

# Reinstall MarbleRun
marblerun uninstall
marblerun install --simulation
```

#### 2. k3s Not Accessible

```bash
# Check k3s status
sudo systemctl status k3s

# Restart k3s
sudo systemctl restart k3s

# Fix kubeconfig permissions
sudo chown $(id -u):$(id -g) /etc/rancher/k3s/k3s.yaml
```

#### 3. Docker Build Fails

```bash
# Clean Docker cache
docker system prune -a

# Rebuild without cache
docker build --no-cache -t myimage .
```

#### 4. Tests Failing

```bash
# Run tests with verbose output
go test -v ./...

# Run specific test
go test -v -run TestMyFunction ./...

# Check test coverage
go test -cover ./...
```

#### 5. Port Already in Use

```bash
# Find process using port
sudo lsof -i :8080

# Kill process
sudo kill -9 <PID>
```

### Debug Mode

Enable debug logging:

```bash
# Set log level
export LOG_LEVEL=debug

# Run service
go run ./cmd/gateway
```

### MarbleRun Issues

```bash
# Check MarbleRun support
ls /dev/sgx*

# If MarbleRun not available, use simulation mode
export OE_SIMULATION=1
```

## Best Practices

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write meaningful commit messages

### Testing

- Write tests for all business logic
- Aim for >80% code coverage
- Use table-driven tests
- Mock external dependencies

### Security

- Never commit secrets/keys or `.env` files with real values
- Use environment variables for configuration
- Validate all inputs
- Use prepared statements for SQL queries

### Performance

- Profile code with `pprof`
- Use connection pooling
- Cache frequently accessed data
- Monitor metrics

## Resources

- [Go Documentation](https://go.dev/doc/)
- [MarbleRun Documentation](https://docs.edgeless.systems/marblerun/)
- [EGo Documentation](https://docs.edgeless.systems/ego/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)

## Getting Help

- Check existing documentation
- Search GitHub issues
- Ask in team chat
- Create a new issue with detailed information

## Contributing

1. Create a feature branch
2. Make your changes
3. Run `make check`
4. Commit with descriptive message
5. Create pull request
6. Wait for review

---

**Happy coding!**
