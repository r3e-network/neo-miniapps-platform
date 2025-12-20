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
SIGNING_KEY=""
SIGNING_KEY_DIR=""
SKIP_SIGNER_CHECK=false
OVERLAY_PATH=""

# Services to build and deploy
SERVICES=(
    "neofeeds"
    "neoflow"
    "neoaccounts"
    "neocompute"
    "neovrf"
    "neooracle"
    "txproxy"
    "globalsigner"
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
  --overlay <path>      Override the kustomize overlay path (e.g. k8s/overlays/production-hardened)
  --registry <url>      Docker registry URL (e.g., docker.io/myorg). Images will be pushed as: <registry>/service-layer/<service>:<tag>
  --push                Push images to registry after build
  --skip-build          Skip Docker image build
  --skip-tests          Skip running tests before deployment
  --rolling-update      Perform rolling update instead of recreate
  --timeout <seconds>   Wait timeout for deployments (default: 300)
  --signing-key <path>  Enclave signing key (PEM). Required for prod builds unless images are already available.
  --signing-key-dir <dir>
                        Per-service signing keys named <service>.pem or <service>-private.pem (recommended for prod).
  --skip-signer-check   Skip comparing key-derived SignerIDs against manifests/manifest.json (not recommended).
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

  # Deploy hardened production overlay
  $0 --env prod --overlay k8s/overlays/production-hardened deploy

EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --env)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --overlay)
                OVERLAY_PATH="$2"
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
            --signing-key)
                SIGNING_KEY="$2"
                shift 2
                ;;
            --signing-key-dir)
                SIGNING_KEY_DIR="$2"
                shift 2
                ;;
            --skip-signer-check)
                SKIP_SIGNER_CHECK=true
                shift
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
        IMAGE_PREFIX="$REGISTRY/service-layer/"
    else
        IMAGE_PREFIX="service-layer/"
    fi
}

# =============================================================================
# Helpers
# =============================================================================
default_service_binary() {
    # Some packages keep legacy binary names for compatibility.
    case "$1" in
        neoaccounts) echo "accountpool" ;;
        *) echo "$1" ;;
    esac
}

resolve_overlay_path() {
    if [[ -n "$OVERLAY_PATH" ]]; then
        echo "$OVERLAY_PATH"
        return 0
    fi
    case "$ENVIRONMENT" in
        dev) echo "k8s/overlays/simulation" ;;
        test) echo "k8s/overlays/test" ;;
        prod) echo "k8s/overlays/production" ;;
        *) echo "k8s/overlays/simulation" ;;
    esac
}

resolve_signing_key() {
    local pkg="$1"
    if [[ -n "$SIGNING_KEY" ]]; then
        echo "$SIGNING_KEY"
        return 0
    fi
    if [[ -n "$SIGNING_KEY_DIR" ]]; then
        local candidates=(
            "${SIGNING_KEY_DIR}/${pkg}.pem"
            "${SIGNING_KEY_DIR}/${pkg}-private.pem"
            "${SIGNING_KEY_DIR}/${pkg}.key"
            "${SIGNING_KEY_DIR}/${pkg}-private.key"
        )
        local candidate
        for candidate in "${candidates[@]}"; do
            if [[ -f "$candidate" ]]; then
                echo "$candidate"
                return 0
            fi
        done
    fi
    return 1
}

ego_image() {
    echo "ghcr.io/edgelesssys/ego-dev:v${EGO_VERSION:-1.8.0}"
}

ego_signerid() {
    local key_path="$1"
    if command -v ego &> /dev/null; then
        ego signerid "$key_path"
        return $?
    fi
    docker run --rm -v "${key_path}:/signing-key:ro" "$(ego_image)" ego signerid /signing-key
}

ego_signerid_from_private_key() {
    local key_path="$1"

    local signer
    signer="$(ego_signerid "$key_path" 2>/dev/null | tr -d '\r\n' || true)"
    if [[ -n "$signer" ]]; then
        echo "$signer"
        return 0
    fi

    if command -v openssl &> /dev/null; then
        local tmp_pub
        tmp_pub="$(mktemp -t service-layer-signingkey.pub.XXXXXX.pem)"
        if openssl rsa -in "$key_path" -pubout -out "$tmp_pub" &>/dev/null; then
            signer="$(ego_signerid "$tmp_pub" 2>/dev/null | tr -d '\r\n' || true)"
        fi
        rm -f "$tmp_pub"
        if [[ -n "$signer" ]]; then
            echo "$signer"
            return 0
        fi
    fi

    return 1
}

verify_signerids() {
    if [[ "$SKIP_SIGNER_CHECK" == "true" ]]; then
        log_warn "Skipping signer ID checks (--skip-signer-check)"
        return 0
    fi

    local manifest="$PROJECT_ROOT/manifests/manifest.json"
    if [[ ! -f "$manifest" ]]; then
        log_error "Manifest not found: $manifest"
        exit 1
    fi

    for pkg in "${SERVICES[@]}"; do
        local key_path
        if ! key_path="$(resolve_signing_key "$pkg")"; then
            log_error "Missing signing key for ${pkg}. Provide --signing-key or --signing-key-dir."
            exit 1
        fi
        if [[ ! -r "$key_path" ]]; then
            log_error "Signing key not readable: $key_path"
            exit 1
        fi

        local expected actual
        expected="$(jq -r --arg pkg "$pkg" '.Packages[$pkg].SignerID' "$manifest" 2>/dev/null || true)"
        if [[ -z "$expected" || "$expected" == "null" ]]; then
            log_error "Manifest does not define Packages.${pkg}.SignerID"
            exit 1
        fi

        actual="$(ego_signerid_from_private_key "$key_path" || true)"
        if [[ -z "$actual" ]]; then
            log_error "Unable to compute SignerID from signing key: $key_path"
            log_error "Install the ego CLI or ensure Docker can pull $(ego_image), or use --skip-signer-check."
            exit 1
        fi

        if [[ "$expected" != "$actual" ]]; then
            log_error "SignerID mismatch for '${pkg}': manifest expects ${expected}, signing key yields ${actual}"
            exit 1
        fi
    done

    log_info "SignerID checks passed"
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

    # Production builds require stable enclave signing keys that match the manifest.
    if [[ "$ENVIRONMENT" == "prod" ]] && [[ "$SKIP_BUILD" != "true" ]]; then
        if [[ -z "$SIGNING_KEY" && -z "$SIGNING_KEY_DIR" ]]; then
            log_error "Production builds require enclave signing keys."
            log_error "Provide --signing-key-dir <dir> (recommended) or --signing-key <path>, or use --skip-build and ensure images exist."
            exit 1
        fi
        if [[ "$SKIP_SIGNER_CHECK" != "true" ]] && ! command -v jq &> /dev/null; then
            log_error "jq is required for signer ID checks in prod. Install jq or use --skip-signer-check."
            exit 1
        fi
    fi

    # Check cert-manager ClusterIssuer email configuration
    local cert_issuer_file="$PROJECT_ROOT/k8s/platform/cert-manager/cluster-issuer.yaml"
    if [[ -f "$cert_issuer_file" ]]; then
        if grep -q "email:.*@example\.com" "$cert_issuer_file"; then
            log_error "cert-manager ClusterIssuer still contains the default example email (@example.com)"
            log_error "Please update the email addresses in: $cert_issuer_file"
            log_error "The email is required for Let's Encrypt ACME registration"
            exit 1
        fi
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

    if [[ "$ENVIRONMENT" == "prod" ]]; then
        export DOCKER_BUILDKIT=1
        verify_signerids
    fi

    for service in "${SERVICES[@]}"; do
        log_info "Building $service..."

        local image_name="${IMAGE_PREFIX}${service}:${ENVIRONMENT}"
        local dockerfile="docker/Dockerfile.service"

        if [ "$DRY_RUN" == "true" ]; then
            log_info "[DRY RUN] Would build: $image_name"
            continue
        fi

        local service_binary
        service_binary="$(default_service_binary "$service")"
        if [[ "$ENVIRONMENT" == "prod" ]]; then
            local key_path
            if ! key_path="$(resolve_signing_key "$service")"; then
                log_error "Missing signing key for ${service}. Provide --signing-key or --signing-key-dir."
                exit 1
            fi
            docker build -t "$image_name" \
                --secret id=ego_private_key,src="$key_path" \
                --build-arg EGO_STRICT_SIGNING=1 \
                --build-arg SERVICE="$service_binary" \
                -f "$dockerfile" . || {
                log_error "Failed to build $service"
                exit 1
            }
        else
            docker build -t "$image_name" \
                --build-arg SERVICE="$service_binary" \
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

    # Port forward to coordinator
    log_info "Setting up port forwarding to MarbleRun Coordinator..."
    local coordinator_svc="coordinator-client-api"
    if ! kubectl -n marblerun get svc "$coordinator_svc" &>/dev/null; then
        coordinator_svc="marblerun-coordinator-client-api"
    fi
    if ! kubectl -n marblerun get svc "$coordinator_svc" &>/dev/null; then
        log_warn "Coordinator client service not found in namespace 'marblerun'. Skipping manifest setup."
        return 0
    fi

    kubectl -n marblerun port-forward "svc/${coordinator_svc}" 4433:4433 &
    PF_PID=$!
    sleep 3

    # Set the manifest
    log_info "Setting MarbleRun manifest..."
    local flags=()
    if [[ "$ENVIRONMENT" != "prod" ]]; then
        flags+=(--insecure)
    fi
    marblerun manifest set "$PROJECT_ROOT/manifests/manifest.json" "localhost:4433" "${flags[@]}" || {
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

    local overlay_path
    overlay_path="$(resolve_overlay_path)"
    if [ ! -d "$overlay_path" ]; then
        log_error "Overlay not found: $overlay_path"
        exit 1
    fi

    if [ "$DRY_RUN" == "true" ]; then
        log_info "[DRY RUN] Would apply: $overlay_path"
        log_info "[DRY RUN] Would set images to: ${IMAGE_PREFIX}<service>:${ENVIRONMENT}"
        return 0
    fi

    # Apply Kubernetes manifests with image overrides to match the environment tag.
    log_info "Applying Kubernetes manifests from $overlay_path..."
    kubectl kustomize "$overlay_path" | \
        sed -E \
            -e "s#(^[[:space:]]*image:[[:space:]]*)service-layer/#\\1${IMAGE_PREFIX}#" \
            -e "s#(^[[:space:]]*image:[[:space:]].*):latest#\\1:${ENVIRONMENT}#" | \
        kubectl apply -f - || {
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
    if [[ "$ENVIRONMENT" == "prod" ]]; then
        log_info "Public gateway is Supabase Edge (outside this cluster)."
        log_info "To debug internal services, port-forward individual services, e.g.:"
        log_info "  kubectl -n service-layer port-forward svc/neofeeds 8083:8083"
    else
        log_info "Public gateway is Supabase Edge (outside this cluster)."
        log_info "To debug internal services, port-forward individual services, e.g.:"
        log_info "  kubectl -n service-layer port-forward svc/neofeeds 8083:8083"
    fi
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
