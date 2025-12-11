#!/bin/bash
#
# Service Layer Kubernetes Deployment Script
# Supports multiple environments: dev, test, prod
# Features: Docker build, registry push, rolling updates, health checks
#
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# Default configuration
ENVIRONMENT="dev"
REGISTRY=""
PUSH_TO_REGISTRY=false
SKIP_BUILD=false
SKIP_TESTS=false
ROLLING_UPDATE=false
WAIT_TIMEOUT=300
DRY_RUN=false

# Services to build and deploy
SERVICES=(
    "gateway"
    "vrf"
    "mixer"
    "datafeeds"
    "automation"
    "accountpool"
    "confidential"
    "secrets"
    "oracle"
)

# =============================================================================
# Parse Arguments
# =============================================================================
usage() {
    cat << EOF
Usage: $0 [OPTIONS] [COMMAND]

Commands:
  build       Build all Docker images
  push        Push images to registry
  deploy      Deploy to Kubernetes
  update      Perform rolling update
  status      Show deployment status
  cleanup     Remove all deployments
  all         Build, push, and deploy (default)

Options:
  --env <env>           Environment: dev, test, prod (default: dev)
  --registry <url>      Docker registry URL (e.g., docker.io/myorg)
  --push                Push images to registry after build
  --skip-build          Skip Docker image build
  --skip-tests          Skip running tests before deployment
  --rolling-update      Perform rolling update instead of recreate
  --timeout <seconds>   Wait timeout for deployments (default: 300)
  --dry-run             Show what would be done without executing
  -h, --help            Show this help message

Examples:
  # Deploy to development (local k3s)
  $0 --env dev

  # Build and push to registry for production
  $0 --env prod --registry docker.io/myorg --push

  # Perform rolling update in production
  $0 --env prod --rolling-update update

  # Deploy to test environment
  $0 --env test deploy

EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --env)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --registry)
                REGISTRY="$2"
                PUSH_TO_REGISTRY=true
                shift 2
                ;;
            --push)
                PUSH_TO_REGISTRY=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            --rolling-update)
                ROLLING_UPDATE=true
                shift
                ;;
            --timeout)
                WAIT_TIMEOUT="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            build|push|deploy|update|status|cleanup|all)
                COMMAND="$1"
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Set default command
    COMMAND="${COMMAND:-all}"

    # Validate environment
    if [[ ! "$ENVIRONMENT" =~ ^(dev|test|prod)$ ]]; then
        log_error "Invalid environment: $ENVIRONMENT (must be dev, test, or prod)"
        exit 1
    fi

    # Set registry prefix if provided
    if [ -n "$REGISTRY" ]; then
        IMAGE_PREFIX="$REGISTRY/"
    else
        IMAGE_PREFIX="service-layer/"
    fi
}

# =============================================================================
# Pre-flight Checks
# =============================================================================
preflight_checks() {
    log_step "Running pre-flight checks..."

    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi

    # Check if docker is available
    if ! command -v docker &> /dev/null; then
        log_error "docker not found. Please install Docker."
        exit 1
    fi

    # Check if k3s is running (for dev environment)
    if [ "$ENVIRONMENT" == "dev" ]; then
        if ! kubectl get nodes &> /dev/null; then
            log_error "Kubernetes cluster not accessible. Is k3s running?"
            exit 1
        fi
    fi

    # Check if MarbleRun is installed
    if ! command -v marblerun &> /dev/null; then
        log_warn "marblerun CLI not found. Some features may not work."
    fi

    log_info "Pre-flight checks passed"
}

# =============================================================================
# Run Tests
# =============================================================================
run_tests() {
    if [ "$SKIP_TESTS" == "true" ]; then
        log_info "Skipping tests (--skip-tests)"
        return 0
    fi

    log_step "Running tests..."

    cd "$PROJECT_ROOT"

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would run: go test -v ./..."
        return 0
    fi

    if ! go test -v ./...; then
        log_error "Tests failed. Aborting deployment."
        exit 1
    fi

    log_info "All tests passed"
}

# =============================================================================
# Build Docker Images
# =============================================================================
build_images() {
    if [ "$SKIP_BUILD" == "true" ]; then
        log_info "Skipping build (--skip-build)"
        return 0
    fi

    log_step "Building Docker images for environment: $ENVIRONMENT..."

    cd "$PROJECT_ROOT"

    for service in "${SERVICES[@]}"; do
        log_info "Building $service..."

        local image_name="${IMAGE_PREFIX}${service}:${ENVIRONMENT}"
        local dockerfile=""

        if [ "$service" == "gateway" ]; then
            dockerfile="docker/Dockerfile.gateway"
        else
            dockerfile="docker/Dockerfile.service"
        fi

        if [ "$DRY_RUN" == "true" ]; then
            log_info "[DRY RUN] Would build: $image_name"
            continue
        fi

        if [ "$service" == "gateway" ]; then
            docker build -t "$image_name" -f "$dockerfile" . || {
                log_error "Failed to build $service"
                exit 1
            }
        else
            docker build -t "$image_name" \
                --build-arg SERVICE="$service" \
                -f "$dockerfile" . || {
                log_error "Failed to build $service"
                exit 1
            }
        fi

        # Also tag as latest for the environment
        docker tag "$image_name" "${IMAGE_PREFIX}${service}:latest"

        log_info "$service image built successfully"
    done

    log_info "All images built successfully"
}

# =============================================================================
# Push Images to Registry
# =============================================================================
push_images() {
    if [ "$PUSH_TO_REGISTRY" != "true" ]; then
        log_info "Skipping registry push (use --push to enable)"
        return 0
    fi

    if [ -z "$REGISTRY" ]; then
        log_error "Registry not specified. Use --registry <url>"
        exit 1
    fi

    log_step "Pushing images to registry: $REGISTRY..."

    for service in "${SERVICES[@]}"; do
        local image_name="${IMAGE_PREFIX}${service}:${ENVIRONMENT}"

        log_info "Pushing $image_name..."

        if [ "$DRY_RUN" == "true" ]; then
            log_info "[DRY RUN] Would push: $image_name"
            continue
        fi

        docker push "$image_name" || {
            log_error "Failed to push $service"
            exit 1
        }

        log_info "$service pushed successfully"
    done

    log_info "All images pushed successfully"
}

# =============================================================================
# Import Images to k3s (for local development)
# =============================================================================
import_images_k3s() {
    if [ "$ENVIRONMENT" != "dev" ]; then
        log_info "Skipping k3s import (not dev environment)"
        return 0
    fi

    if [ "$PUSH_TO_REGISTRY" == "true" ]; then
        log_info "Skipping k3s import (using registry)"
        return 0
    fi

    log_step "Importing images to k3s..."

    for service in "${SERVICES[@]}"; do
        local image_name="${IMAGE_PREFIX}${service}:${ENVIRONMENT}"

        log_info "Importing $service to k3s..."

        if [ "$DRY_RUN" == "true" ]; then
            log_info "[DRY RUN] Would import: $image_name"
            continue
        fi

        docker save "$image_name" | sudo k3s ctr images import - || {
            log_warn "Failed to import $service to k3s"
        }
    done

    log_info "All images imported to k3s"
}

# =============================================================================
# Setup MarbleRun Manifest
# =============================================================================
setup_marblerun_manifest() {
    log_step "Setting up MarbleRun manifest..."

    # Check if MarbleRun is ready
    if ! command -v marblerun &> /dev/null; then
        log_warn "MarbleRun CLI not found, skipping manifest setup"
        return 0
    fi

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would setup MarbleRun manifest"
        return 0
    fi

    # Check if MarbleRun is installed in cluster
    if ! kubectl get namespace marblerun &> /dev/null; then
        log_warn "MarbleRun not installed in cluster, skipping manifest setup"
        return 0
    fi

    marblerun check --timeout 60s || {
        log_warn "MarbleRun is not ready. Skipping manifest setup."
        return 0
    }

    # Port forward to coordinator
    log_info "Setting up port forwarding to MarbleRun Coordinator..."
    kubectl -n marblerun port-forward svc/marblerun-coordinator 4433:4433 &
    PF_PID=$!
    sleep 3

    # Set the manifest
    log_info "Setting MarbleRun manifest..."
    marblerun manifest set "$PROJECT_ROOT/manifests/manifest.json" \
        --coordinator localhost:4433 \
        --insecure || {
        log_warn "Manifest may already be set or coordinator not ready"
    }

    # Kill port forward
    kill $PF_PID 2>/dev/null || true

    log_info "MarbleRun manifest configured"
}

# =============================================================================
# Deploy to Kubernetes
# =============================================================================
deploy_k8s() {
    log_step "Deploying to Kubernetes (environment: $ENVIRONMENT)..."

    cd "$PROJECT_ROOT"

    local overlay_path="k8s/overlays/$ENVIRONMENT"

    # Check if overlay exists
    if [ ! -d "$overlay_path" ]; then
        log_warn "Overlay not found: $overlay_path, using simulation"
        overlay_path="k8s/overlays/simulation"
    fi

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would apply: kubectl apply -k $overlay_path"
        return 0
    fi

    # Apply Kubernetes manifests
    log_info "Applying Kubernetes manifests from $overlay_path..."
    kubectl apply -k "$overlay_path" || {
        log_error "Failed to apply Kubernetes manifests"
        exit 1
    }

    # Wait for deployments
    log_info "Waiting for deployments to be ready (timeout: ${WAIT_TIMEOUT}s)..."
    kubectl -n service-layer wait --for=condition=available \
        --timeout="${WAIT_TIMEOUT}s" deployment --all || {
        log_warn "Some deployments may not be ready yet"
        kubectl -n service-layer get pods
        return 1
    }

    log_info "Deployment complete"
}

# =============================================================================
# Rolling Update
# =============================================================================
rolling_update() {
    log_step "Performing rolling update..."

    cd "$PROJECT_ROOT"

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would perform rolling update"
        return 0
    fi

    for service in "${SERVICES[@]}"; do
        log_info "Updating $service..."

        # Restart deployment to trigger rolling update
        kubectl -n service-layer rollout restart deployment "$service" || {
            log_warn "Failed to restart $service deployment"
            continue
        }

        # Wait for rollout to complete
        kubectl -n service-layer rollout status deployment "$service" --timeout="${WAIT_TIMEOUT}s" || {
            log_error "Rolling update failed for $service"
            exit 1
        }

        log_info "$service updated successfully"
    done

    log_info "Rolling update complete"
}

# =============================================================================
# Show Status
# =============================================================================
show_status() {
    log_step "Deployment Status (environment: $ENVIRONMENT)"

    echo ""
    echo "=== Kubernetes Nodes ==="
    kubectl get nodes

    echo ""
    echo "=== MarbleRun Status ==="
    kubectl -n marblerun get pods 2>/dev/null || echo "MarbleRun not installed"

    echo ""
    echo "=== Service Layer Pods ==="
    kubectl -n service-layer get pods

    echo ""
    echo "=== Service Layer Services ==="
    kubectl -n service-layer get svc

    echo ""
    echo "=== Service Layer Deployments ==="
    kubectl -n service-layer get deployments

    echo ""
    log_info "Gateway accessible at: http://localhost:30080"
    log_info "Or via: kubectl -n service-layer port-forward svc/gateway 8080:8080"
}

# =============================================================================
# Cleanup
# =============================================================================
cleanup() {
    log_step "Cleaning up..."

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would delete namespace: service-layer"
        return 0
    fi

    kubectl delete namespace service-layer --ignore-not-found=true

    log_info "Cleanup complete"
}

# =============================================================================
# Main Execution
# =============================================================================
main() {
    export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

    parse_args "$@"

    echo "=============================================="
    echo "  Service Layer Kubernetes Deployment"
    echo "  Environment: $ENVIRONMENT"
    echo "  Command: $COMMAND"
    if [ "$DRY_RUN" == "true" ]; then
        echo "  Mode: DRY RUN"
    fi
    echo "=============================================="
    echo ""

    case "$COMMAND" in
        build)
            preflight_checks
            run_tests
            build_images
            ;;
        push)
            preflight_checks
            push_images
            ;;
        deploy)
            preflight_checks
            import_images_k3s
            setup_marblerun_manifest
            deploy_k8s
            show_status
            ;;
        update)
            preflight_checks
            rolling_update
            show_status
            ;;
        status)
            show_status
            ;;
        cleanup)
            cleanup
            ;;
        all)
            preflight_checks
            run_tests
            build_images
            push_images
            import_images_k3s
            setup_marblerun_manifest
            deploy_k8s
            show_status
            ;;
        *)
            log_error "Unknown command: $COMMAND"
            usage
            exit 1
            ;;
    esac

    echo ""
    log_info "Operation completed successfully"
}

main "$@"
