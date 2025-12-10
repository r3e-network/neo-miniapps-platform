#!/bin/bash
#
# Installation script for MarbleRun, EGo, and Kubernetes (k3s)
# Target: Ubuntu 24.04 LTS
#
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running as root for certain operations
check_sudo() {
    if [ "$EUID" -ne 0 ]; then
        log_warn "Some operations require sudo. You may be prompted for password."
    fi
}

# ============================================================================
# 1. Install Prerequisites
# ============================================================================
install_prerequisites() {
    log_info "Installing prerequisites..."
    sudo apt-get update
    sudo apt-get install -y \
        build-essential \
        libssl-dev \
        curl \
        wget \
        gnupg \
        apt-transport-https \
        ca-certificates \
        software-properties-common
    log_info "Prerequisites installed."
}

# ============================================================================
# 2. Install Intel SGX SDK and PSW (for SGX support)
# ============================================================================
install_sgx_sdk() {
    log_info "Setting up Intel SGX repository..."

    # Add Intel SGX repository
    echo 'deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx-keyring.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu noble main' | \
        sudo tee /etc/apt/sources.list.d/intel-sgx.list

    # Import Intel SGX key
    wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
        gpg --dearmor | sudo tee /usr/share/keyrings/intel-sgx-keyring.gpg > /dev/null

    sudo apt-get update

    log_info "Installing Intel SGX DCAP packages..."
    sudo apt-get install -y \
        libsgx-epid \
        libsgx-quote-ex \
        libsgx-dcap-ql \
        libsgx-dcap-default-qpl \
        sgx-aesm-service \
        libsgx-aesm-launch-plugin \
        libsgx-aesm-epid-plugin \
        libsgx-aesm-quote-ex-plugin \
        libsgx-aesm-ecdsa-plugin \
        libsgx-dcap-quote-verify-dev || {
            log_warn "Some SGX packages may not be available. Continuing..."
        }

    log_info "Intel SGX SDK setup complete."
}

# ============================================================================
# 3. Install EGo Runtime
# ============================================================================
install_ego() {
    log_info "Installing EGo runtime..."

    # Method 1: Snap (recommended)
    if command -v snap &> /dev/null; then
        log_info "Installing EGo via snap..."
        sudo snap install ego-dev --classic
        log_info "EGo installed via snap."
    else
        # Method 2: DEB package for Ubuntu 24.04
        log_info "Snap not available, installing EGo via DEB package..."

        EGO_VERSION="1.8.0"
        wget "https://github.com/edgelesssys/ego/releases/download/v${EGO_VERSION}/ego_${EGO_VERSION}_amd64_ubuntu-24.04.deb" \
            -O /tmp/ego.deb
        sudo dpkg -i /tmp/ego.deb || sudo apt-get install -f -y
        rm /tmp/ego.deb
        log_info "EGo installed via DEB package."
    fi

    # Verify installation
    if command -v ego &> /dev/null; then
        log_info "EGo version: $(ego version 2>/dev/null || echo 'installed')"
    else
        # Check snap path
        if [ -f /snap/bin/ego ]; then
            log_info "EGo installed at /snap/bin/ego"
            echo 'export PATH=$PATH:/snap/bin' >> ~/.bashrc
        fi
    fi
}

# ============================================================================
# 4. Install MarbleRun CLI
# ============================================================================
install_marblerun() {
    log_info "Installing MarbleRun CLI..."

    # Download and install MarbleRun CLI
    curl -fsSL https://github.com/edgelesssys/marblerun/releases/latest/download/marblerun-linux-amd64 \
        -o /tmp/marblerun

    chmod +x /tmp/marblerun
    sudo mv /tmp/marblerun /usr/local/bin/marblerun

    # Verify installation
    if command -v marblerun &> /dev/null; then
        log_info "MarbleRun CLI installed: $(marblerun version 2>/dev/null || echo 'installed')"
    else
        log_error "MarbleRun CLI installation failed"
        return 1
    fi
}

# ============================================================================
# 5. Install Kubernetes (k3s)
# ============================================================================
install_k3s() {
    log_info "Installing k3s (lightweight Kubernetes)..."

    # Install k3s
    curl -sfL https://get.k3s.io | sh -

    # Wait for k3s to be ready
    log_info "Waiting for k3s to be ready..."
    sleep 10

    # Setup kubectl for current user
    mkdir -p ~/.kube
    sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
    sudo chown $(id -u):$(id -g) ~/.kube/config
    chmod 600 ~/.kube/config

    # Add KUBECONFIG to bashrc
    if ! grep -q "KUBECONFIG" ~/.bashrc; then
        echo 'export KUBECONFIG=~/.kube/config' >> ~/.bashrc
    fi
    export KUBECONFIG=~/.kube/config

    # Verify installation
    log_info "Verifying k3s installation..."
    kubectl get nodes

    log_info "k3s installed successfully."
}

# ============================================================================
# 6. Install Helm (for MarbleRun deployment)
# ============================================================================
install_helm() {
    log_info "Installing Helm..."

    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

    if command -v helm &> /dev/null; then
        log_info "Helm installed: $(helm version --short)"
    fi
}

# ============================================================================
# 7. Deploy MarbleRun to Kubernetes
# ============================================================================
deploy_marblerun() {
    log_info "Deploying MarbleRun to Kubernetes..."

    # Check if SGX is available
    if [ -e /dev/sgx_enclave ] || [ -e /dev/sgx/enclave ]; then
        log_info "SGX detected, deploying in SGX mode..."
        marblerun install
    else
        log_warn "SGX not detected, deploying in SIMULATION mode..."
        log_warn "⚠️  SIMULATION MODE IS NOT SECURE - FOR DEVELOPMENT ONLY"
        marblerun install --simulation
    fi

    # Wait for MarbleRun to be ready
    log_info "Waiting for MarbleRun components..."
    marblerun check --timeout 120s || {
        log_warn "MarbleRun check timed out, checking pod status..."
        kubectl get pods -n marblerun
    }
}

# ============================================================================
# Main Installation Flow
# ============================================================================
main() {
    echo "=============================================="
    echo "  MarbleRun + EGo + Kubernetes Installation"
    echo "  Target: Ubuntu 24.04 LTS"
    echo "=============================================="
    echo ""

    check_sudo

    # Parse arguments
    SKIP_SGX=false
    SKIP_K8S=false
    DEPLOY_MARBLERUN=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-sgx) SKIP_SGX=true; shift ;;
            --skip-k8s) SKIP_K8S=true; shift ;;
            --deploy-marblerun) DEPLOY_MARBLERUN=true; shift ;;
            --all) DEPLOY_MARBLERUN=true; shift ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --skip-sgx          Skip Intel SGX SDK installation"
                echo "  --skip-k8s          Skip Kubernetes (k3s) installation"
                echo "  --deploy-marblerun  Deploy MarbleRun to Kubernetes after install"
                echo "  --all               Install everything and deploy MarbleRun"
                echo "  -h, --help          Show this help message"
                exit 0
                ;;
            *) log_error "Unknown option: $1"; exit 1 ;;
        esac
    done

    # Step 1: Prerequisites
    install_prerequisites

    # Step 2: Intel SGX SDK
    if [ "$SKIP_SGX" = false ]; then
        install_sgx_sdk
    else
        log_info "Skipping SGX SDK installation."
    fi

    # Step 3: EGo Runtime
    install_ego

    # Step 4: MarbleRun CLI
    install_marblerun

    # Step 5: Kubernetes (k3s)
    if [ "$SKIP_K8S" = false ]; then
        install_k3s
        install_helm
    else
        log_info "Skipping Kubernetes installation."
    fi

    # Step 6: Deploy MarbleRun (optional)
    if [ "$DEPLOY_MARBLERUN" = true ] && [ "$SKIP_K8S" = false ]; then
        deploy_marblerun
    fi

    echo ""
    echo "=============================================="
    echo "  Installation Complete!"
    echo "=============================================="
    echo ""
    log_info "Installed components:"
    echo "  - EGo runtime (SGX development framework)"
    echo "  - MarbleRun CLI (confidential computing orchestration)"
    if [ "$SKIP_K8S" = false ]; then
        echo "  - k3s (lightweight Kubernetes)"
        echo "  - Helm (Kubernetes package manager)"
    fi
    echo ""
    log_warn "IMPORTANT: Reload your shell or run: source ~/.bashrc"
    echo ""

    # Check SGX status
    if [ ! -e /dev/sgx_enclave ] && [ ! -e /dev/sgx/enclave ]; then
        log_warn "SGX devices not detected. To enable SGX:"
        echo "  1. Enable SGX in BIOS (Intel Software Guard Extensions)"
        echo "  2. Reboot the system"
        echo "  3. Verify with: ls /dev/sgx*"
        echo ""
        echo "  For development without SGX, use --simulation flag with MarbleRun"
    fi

    echo ""
    log_info "Next steps:"
    echo "  1. source ~/.bashrc"
    echo "  2. kubectl get nodes  # Verify Kubernetes"
    echo "  3. ego --help         # Verify EGo"
    echo "  4. marblerun --help   # Verify MarbleRun"
    if [ "$DEPLOY_MARBLERUN" = false ] && [ "$SKIP_K8S" = false ]; then
        echo "  5. marblerun install --simulation  # Deploy MarbleRun (dev mode)"
    fi
}

main "$@"
