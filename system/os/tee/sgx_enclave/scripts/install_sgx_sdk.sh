#!/bin/bash
# Intel SGX SDK Installation Script
#
# This script installs the Intel SGX SDK for simulation mode development.
# Run with: sudo ./install_sgx_sdk.sh
#
# Prerequisites:
#   - Ubuntu 22.04 or 24.04
#   - sudo access

set -e

echo "=========================================="
echo "Intel SGX SDK Installation Script"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo $0"
    exit 1
fi

# Detect Ubuntu version
UBUNTU_VERSION=$(lsb_release -rs)
echo "Detected Ubuntu version: $UBUNTU_VERSION"

# Set repository based on version
case $UBUNTU_VERSION in
    22.04)
        REPO_CODENAME="jammy"
        ;;
    24.04)
        REPO_CODENAME="noble"
        ;;
    *)
        echo "Warning: Ubuntu $UBUNTU_VERSION not officially supported, using jammy"
        REPO_CODENAME="jammy"
        ;;
esac

echo ""
echo "Step 1: Adding Intel SGX repository..."
echo "======================================="

# Add Intel SGX repository key
wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor -o /usr/share/keyrings/intel-sgx-deb.gpg

# Add repository
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx-deb.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu $REPO_CODENAME main" | \
    tee /etc/apt/sources.list.d/intel-sgx.list

# Update package list
apt-get update

echo ""
echo "Step 2: Installing SGX SDK packages..."
echo "======================================="

# Install SGX SDK and simulation libraries
apt-get install -y \
    libsgx-enclave-common-dev \
    libsgx-dcap-quote-verify-dev \
    libsgx-urts \
    libsgx-launch \
    libsgx-epid \
    libsgx-quote-ex \
    sgx-aesm-service || true

# Install simulation mode libraries (for development without SGX hardware)
apt-get install -y \
    libsgx-urts-sim \
    libsgx-uae-service-sim || echo "Simulation libraries may not be available"

echo ""
echo "Step 3: Installing build dependencies..."
echo "========================================"

apt-get install -y \
    build-essential \
    ocaml \
    ocamlbuild \
    automake \
    autoconf \
    libtool \
    wget \
    python3 \
    libssl-dev \
    git \
    cmake \
    perl \
    libcurl4-openssl-dev \
    protobuf-compiler \
    libprotobuf-dev \
    debhelper \
    reprepro \
    unzip \
    pkgconf

echo ""
echo "Step 4: Downloading and installing SGX SDK binary..."
echo "====================================================="

SDK_VERSION="2.24"
SDK_FILE="sgx_linux_x64_sdk_${SDK_VERSION}.100.3.bin"
SDK_URL="https://download.01.org/intel-sgx/sgx-linux/${SDK_VERSION}/distro/ubuntu22.04-server/${SDK_FILE}"

cd /tmp
if [ ! -f "$SDK_FILE" ]; then
    echo "Downloading SGX SDK..."
    wget -q --show-progress "$SDK_URL" -O "$SDK_FILE" || {
        echo "Failed to download from primary URL, trying alternative..."
        # Try alternative download
        wget -q --show-progress "https://download.01.org/intel-sgx/latest/linux-latest/distro/ubuntu22.04-server/${SDK_FILE}" -O "$SDK_FILE" || {
            echo "Warning: Could not download SDK binary. You may need to download manually."
        }
    }
fi

if [ -f "$SDK_FILE" ]; then
    chmod +x "$SDK_FILE"

    # Install SDK to /opt/intel
    echo "Installing SDK to /opt/intel/sgxsdk..."
    mkdir -p /opt/intel
    echo "yes" | ./"$SDK_FILE" --prefix=/opt/intel

    echo ""
    echo "Step 5: Setting up environment..."
    echo "=================================="

    # Create environment setup script
    cat > /etc/profile.d/sgx-sdk.sh << 'EOF'
# Intel SGX SDK Environment
if [ -d "/opt/intel/sgxsdk" ]; then
    export SGX_SDK=/opt/intel/sgxsdk
    export PATH=$PATH:$SGX_SDK/bin:$SGX_SDK/bin/x64
    export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:$SGX_SDK/pkgconfig
    export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$SGX_SDK/sdk_libs
fi
EOF

    chmod +x /etc/profile.d/sgx-sdk.sh
    source /etc/profile.d/sgx-sdk.sh
fi

echo ""
echo "=========================================="
echo "Installation Complete!"
echo "=========================================="
echo ""
echo "To use the SGX SDK, run:"
echo "  source /opt/intel/sgxsdk/environment"
echo ""
echo "Or add to your ~/.bashrc:"
echo "  source /opt/intel/sgxsdk/environment"
echo ""
echo "To verify installation:"
echo "  ls /opt/intel/sgxsdk"
echo ""
echo "To build the SGX enclave in simulation mode:"
echo "  cd system/tee/sgx_enclave"
echo "  make SGX_MODE=SIM"
echo ""
