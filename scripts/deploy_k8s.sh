#!/bin/bash
#
# Service Layer Kubernetes Deployment Script
# Deploys all services to k3s with MarbleRun (simulation mode)
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
# Build Docker Images
# =============================================================================
build_images() {
    log_step "Building Docker images..."

    cd "$PROJECT_ROOT"

    for service in "${SERVICES[@]}"; do
        log_info "Building $service..."

        if [ "$service" == "gateway" ]; then
            docker build -t "service-layer/$service:latest" \
                -f docker/Dockerfile.gateway .
        else
            docker build -t "service-layer/$service:latest" \
                --build-arg SERVICE="$service" \
                -f docker/Dockerfile.service .
        fi

        log_info "$service image built successfully"
    done

    log_info "All images built successfully"
}

# =============================================================================
# Import Images to k3s
# =============================================================================
import_images_k3s() {
    log_step "Importing images to k3s..."

    for service in "${SERVICES[@]}"; do
        log_info "Importing $service to k3s..."
        docker save "service-layer/$service:latest" | sudo k3s ctr images import -
    done

    log_info "All images imported to k3s"
}

# =============================================================================
# Setup MarbleRun Manifest
# =============================================================================
setup_marblerun_manifest() {
    log_step "Setting up MarbleRun manifest..."

    # Check if MarbleRun is ready
    marblerun check --timeout 60s || {
        log_error "MarbleRun is not ready. Please ensure MarbleRun is installed."
        exit 1
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
    log_step "Deploying to Kubernetes..."

    cd "$PROJECT_ROOT"

    # Apply base configuration
    log_info "Applying Kubernetes manifests..."
    kubectl apply -k k8s/base

    # Wait for deployments
    log_info "Waiting for deployments to be ready..."
    kubectl -n service-layer wait --for=condition=available \
        --timeout=300s deployment --all || {
        log_warn "Some deployments may not be ready yet"
        kubectl -n service-layer get pods
    }

    log_info "Deployment complete"
}

# =============================================================================
# Show Status
# =============================================================================
show_status() {
    log_step "Deployment Status"

    echo ""
    echo "=== Kubernetes Nodes ==="
    kubectl get nodes

    echo ""
    echo "=== MarbleRun Status ==="
    kubectl -n marblerun get pods

    echo ""
    echo "=== Service Layer Pods ==="
    kubectl -n service-layer get pods

    echo ""
    echo "=== Service Layer Services ==="
    kubectl -n service-layer get svc

    echo ""
    log_info "Gateway accessible at: http://localhost:30080"
    log_info "Or via: kubectl -n service-layer port-forward svc/gateway 8080:8080"
}

# =============================================================================
# Cleanup
# =============================================================================
cleanup() {
    log_step "Cleaning up..."

    kubectl delete namespace service-layer --ignore-not-found=true

    log_info "Cleanup complete"
}

# =============================================================================
# Main
# =============================================================================
usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  build       Build all Docker images"
    echo "  import      Import images to k3s"
    echo "  manifest    Setup MarbleRun manifest"
    echo "  deploy      Deploy to Kubernetes"
    echo "  status      Show deployment status"
    echo "  cleanup     Remove all deployments"
    echo "  all         Build, import, and deploy (default)"
    echo ""
}

main() {
    export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

    case "${1:-all}" in
        build)
            build_images
            ;;
        import)
            import_images_k3s
            ;;
        manifest)
            setup_marblerun_manifest
            ;;
        deploy)
            deploy_k8s
            ;;
        status)
            show_status
            ;;
        cleanup)
            cleanup
            ;;
        all)
            build_images
            import_images_k3s
            setup_marblerun_manifest
            deploy_k8s
            show_status
            ;;
        -h|--help)
            usage
            ;;
        *)
            log_error "Unknown command: $1"
            usage
            exit 1
            ;;
    esac
}

main "$@"
