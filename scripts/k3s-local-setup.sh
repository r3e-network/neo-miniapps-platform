#!/usr/bin/env bash
#
# k3s Local Development Stack Setup Script (DEVSTACK-1)
# Purpose: Complete local k3s setup with MarbleRun, cert-manager, and ingress
# Idempotent: Can be run multiple times safely
#

set -euo pipefail

# ==================== Configuration ====================
K3S_VERSION="${K3S_VERSION:-v1.28.5+k3s1}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.14.0}"
INSTALL_TIMEOUT="${INSTALL_TIMEOUT:-300}"
LOG_FILE="/tmp/k3s-local-setup.log"
STATE_FILE="/tmp/k3s-local-setup-state"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ==================== Logging ====================
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARN:${NC} $*" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR:${NC} $*" | tee -a "$LOG_FILE"
    exit 1
}

step() {
    echo -e "${BLUE}[STEP]${NC} $*" | tee -a "$LOG_FILE"
}

# ==================== State Management ====================
save_state() {
    local step="$1"
    echo "$step" > "$STATE_FILE"
    log "✓ Checkpoint: $step"
}

get_state() {
    if [[ -f "$STATE_FILE" ]]; then
        cat "$STATE_FILE"
    else
        echo "none"
    fi
}

# ==================== Pre-flight Checks ====================
check_dependencies() {
    step "Checking dependencies..."

    local missing_deps=()

    if ! command -v kubectl &> /dev/null; then
        missing_deps+=("kubectl")
    fi

    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        error "Missing dependencies: ${missing_deps[*]}"
    fi

    log "✓ All dependencies present"
}

check_resources() {
    step "Checking system resources..."

    local cpu_cores=$(nproc)
    local mem_gb=$(free -g | awk '/^Mem:/{print $2}')
    local disk_gb=$(df -BG "$HOME" | awk 'NR==2 {print $4}' | sed 's/G//')

    log "CPU: ${cpu_cores} cores, Memory: ${mem_gb}GB, Disk: ${disk_gb}GB available"

    if [[ $cpu_cores -lt 4 ]]; then
        warn "CPU cores < 4 (current: $cpu_cores), may be slow"
    fi

    if [[ $mem_gb -lt 8 ]]; then
        warn "Memory < 8GB (current: ${mem_gb}GB), may be insufficient"
    fi

    if [[ $disk_gb -lt 20 ]]; then
        warn "Disk space < 20GB (current: ${disk_gb}GB)"
    fi
}

# ==================== k3s Installation ====================
install_k3s() {
    step "Installing k3s..."

    local current_state=$(get_state)
    if [[ "$current_state" == "k3s_installed" ]] || command -v k3s &> /dev/null; then
        if systemctl is-active --quiet k3s 2>/dev/null || k3s kubectl get nodes &> /dev/null; then
            log "k3s already installed and running"
            save_state "k3s_installed"
            return 0
        fi
    fi

    log "Installing k3s ${K3S_VERSION}..."

    # Install k3s with Traefik enabled (default ingress controller)
    # Disable servicelb (use ClusterIP for local dev)
    curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - \
        --write-kubeconfig-mode 644 \
        --disable servicelb \
        || error "k3s installation failed"

    save_state "k3s_installed"
    log "✓ k3s installed successfully"
}

wait_for_k3s() {
    step "Waiting for k3s to be ready..."

    local timeout=$INSTALL_TIMEOUT
    local elapsed=0

    while ! kubectl get nodes &> /dev/null; do
        if [[ $elapsed -ge $timeout ]]; then
            error "k3s failed to start within ${timeout}s"
        fi
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done

    echo ""
    log "✓ k3s API server ready"

    # Wait for nodes to be Ready
    kubectl wait --for=condition=Ready nodes --all --timeout=120s \
        || error "Nodes failed to become ready"

    save_state "k3s_ready"
    log "✓ All nodes ready"
}

# ==================== kubeconfig Setup ====================
setup_kubeconfig() {
    step "Setting up kubeconfig..."

    local kubeconfig_dir="$HOME/.kube"
    local kubeconfig_file="$kubeconfig_dir/config"

    mkdir -p "$kubeconfig_dir"

    if [[ ! -f "$kubeconfig_file" ]] || ! grep -q "k3s" "$kubeconfig_file" 2>/dev/null; then
        sudo cp /etc/rancher/k3s/k3s.yaml "$kubeconfig_file"
        sudo chown $(id -u):$(id -g) "$kubeconfig_file"
        chmod 600 "$kubeconfig_file"
        log "✓ kubeconfig copied to $kubeconfig_file"
    else
        log "kubeconfig already configured"
    fi

    export KUBECONFIG="$kubeconfig_file"

    # Verify kubectl access
    if kubectl get nodes &> /dev/null; then
        log "✓ kubectl access verified"
    else
        error "kubectl cannot access cluster"
    fi
}

# ==================== Namespaces ====================
create_namespaces() {
    step "Creating namespaces..."

    if kubectl get namespace marblerun &> /dev/null; then
        log "Namespaces already exist"
        return 0
    fi

    kubectl apply -f "$PROJECT_ROOT/k8s/namespaces.yaml" \
        || error "Failed to create namespaces"

    save_state "namespaces_created"
    log "✓ Namespaces created"
}

# ==================== cert-manager Installation ====================
install_cert_manager() {
    step "Installing cert-manager..."

    if kubectl get namespace cert-manager &> /dev/null; then
        log "cert-manager already installed"
        return 0
    fi

    log "Applying cert-manager CRDs and deployment..."
    kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml" \
        || error "Failed to install cert-manager"

    log "Waiting for cert-manager to be ready..."
    kubectl -n cert-manager wait --for=condition=available \
        --timeout=180s deployment --all \
        || error "cert-manager failed to become ready"

    save_state "cert_manager_installed"
    log "✓ cert-manager installed successfully"
}

# ==================== Ingress Configuration ====================
setup_ingress() {
    step "Setting up ingress configuration..."

    # Wait a bit for cert-manager to be fully operational
    sleep 10

    log "Applying self-signed issuer and wildcard certificate..."
    kubectl apply -k "$PROJECT_ROOT/k8s/ingress/" \
        || error "Failed to apply ingress configuration"

    log "Waiting for certificate to be ready..."
    local timeout=60
    local elapsed=0
    while ! kubectl -n cert-manager get certificate wildcard-localhost -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; do
        if [[ $elapsed -ge $timeout ]]; then
            warn "Certificate not ready after ${timeout}s, but continuing..."
            break
        fi
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    echo ""

    save_state "ingress_configured"
    log "✓ Ingress configuration applied"
}

# ==================== MarbleRun Installation ====================
install_marblerun() {
    step "Installing MarbleRun coordinator (simulation mode)..."

    if kubectl -n marblerun get deployment coordinator &> /dev/null; then
        log "MarbleRun coordinator already installed"
        return 0
    fi

    log "Applying MarbleRun coordinator manifests..."
    kubectl apply -k "$PROJECT_ROOT/k8s/marblerun/overlays/simulation/" \
        || error "Failed to install MarbleRun coordinator"

    log "Waiting for MarbleRun coordinator to be ready..."
    kubectl -n marblerun wait --for=condition=available \
        --timeout=180s deployment coordinator \
        || warn "MarbleRun coordinator may not be fully ready yet"

    save_state "marblerun_installed"
    log "✓ MarbleRun coordinator installed successfully"
}

# ==================== Verification ====================
verify_installation() {
    step "Verifying installation..."

    echo ""
    log "========== Cluster Information =========="
    kubectl cluster-info

    echo ""
    log "========== Nodes =========="
    kubectl get nodes -o wide

    echo ""
    log "========== Namespaces =========="
    kubectl get namespaces

    echo ""
    log "========== cert-manager Status =========="
    kubectl -n cert-manager get pods

    echo ""
    log "========== Certificates =========="
    kubectl -n cert-manager get certificates

    echo ""
    log "========== MarbleRun Status =========="
    kubectl -n marblerun get pods

    echo ""
    log "========== Traefik Ingress Controller =========="
    kubectl -n kube-system get pods -l app.kubernetes.io/name=traefik

    log "✓ Installation verification complete"
}

# ==================== Status Check ====================
check_status() {
    echo ""
    echo "=========================================="
    echo "  k3s Local Dev Stack Status"
    echo "=========================================="
    echo ""

    if ! command -v k3s &> /dev/null; then
        echo "❌ k3s: Not installed"
        return 1
    fi

    if systemctl is-active --quiet k3s 2>/dev/null || k3s kubectl get nodes &> /dev/null; then
        echo "✓ k3s: Running"
    else
        echo "❌ k3s: Installed but not running"
        return 1
    fi

    if kubectl get namespace cert-manager &> /dev/null; then
        echo "✓ cert-manager: Installed"
    else
        echo "❌ cert-manager: Not installed"
    fi

    if kubectl get namespace marblerun &> /dev/null; then
        echo "✓ MarbleRun: Installed"
    else
        echo "❌ MarbleRun: Not installed"
    fi

    if kubectl -n kube-system get pods -l app.kubernetes.io/name=traefik &> /dev/null; then
        echo "✓ Traefik: Running"
    else
        echo "❌ Traefik: Not running"
    fi

    echo ""
    echo "For detailed status, run: kubectl get pods -A"
    echo ""
}

# ==================== Cleanup ====================
cleanup() {
    step "Cleaning up k3s local dev stack..."

    if command -v k3s-uninstall.sh &> /dev/null; then
        log "Uninstalling k3s..."
        sudo k3s-uninstall.sh || warn "k3s uninstall failed"
    else
        warn "k3s-uninstall.sh not found, k3s may not be installed"
    fi

    rm -f "$STATE_FILE"
    log "✓ Cleanup complete"
}

# ==================== Usage ====================
usage() {
    cat << EOF
Usage: $0 [COMMAND]

Commands:
  install     Install complete k3s local dev stack (default)
  status      Check status of dev stack components
  cleanup     Remove k3s and all components
  --check     Same as 'status'
  -h, --help  Show this help message

Examples:
  # Install complete dev stack
  $0 install

  # Check status
  $0 status

  # Remove everything
  $0 cleanup

Environment Variables:
  K3S_VERSION              k3s version (default: v1.28.5+k3s1)
  CERT_MANAGER_VERSION     cert-manager version (default: v1.14.0)
  INSTALL_TIMEOUT          Timeout in seconds (default: 300)

EOF
}

# ==================== Main ====================
main() {
    local command="${1:-install}"

    case "$command" in
        install)
            log "=========================================="
            log "k3s Local Dev Stack Setup (DEVSTACK-1)"
            log "=========================================="

            check_dependencies
            check_resources

            install_k3s
            wait_for_k3s
            setup_kubeconfig

            create_namespaces
            install_cert_manager
            setup_ingress
            install_marblerun

            verify_installation

            log ""
            log "=========================================="
            log "✓ k3s Local Dev Stack Setup Complete!"
            log "=========================================="
            log ""
            log "Next steps:"
            log "  1. Deploy services: make dev-stack-up"
            log "  2. Check status: make dev-stack-status"
            log "  3. View documentation: docs/LOCAL_DEV.md"
            log ""
            log "Coordinator: kubectl -n marblerun port-forward svc/coordinator-client-api 4433:4433"
            log ""
            ;;
        status|--check)
            check_status
            ;;
        cleanup)
            cleanup
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

main "$@"
