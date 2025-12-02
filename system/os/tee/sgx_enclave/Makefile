# SGX Enclave Build System
#
# This Makefile builds the SGX enclave and bridge library.
#
# Prerequisites:
#   - Intel SGX SDK installed (https://github.com/intel/linux-sgx)
#   - Rust with nightly toolchain
#   - Apache Teaclave SGX SDK
#
# Build modes:
#   - Hardware mode: make SGX_MODE=HW
#   - Simulation mode: make SGX_MODE=SIM (default)
#
# Usage:
#   make              # Build in simulation mode
#   make SGX_MODE=HW  # Build in hardware mode
#   make clean        # Clean build artifacts
#   make test         # Run tests

# =============================================================================
# Configuration
# =============================================================================

SGX_SDK ?= /opt/intel/sgxsdk
SGX_MODE ?= SIM
SGX_ARCH ?= x64
SGX_DEBUG ?= 1

# Rust configuration
RUST_SGX_SDK ?= $(HOME)/.cargo/git/checkouts/incubator-teaclave-sgx-sdk-*
CARGO ?= cargo

# Output directories
BUILD_DIR := target
ENCLAVE_DIR := enclave
BRIDGE_DIR := bridge
APP_DIR := app

# Enclave configuration
ENCLAVE_NAME := tee_enclave
ENCLAVE_CONFIG := $(ENCLAVE_DIR)/Enclave.config.xml
ENCLAVE_EDL := $(ENCLAVE_DIR)/Enclave.edl
ENCLAVE_KEY := $(ENCLAVE_DIR)/Enclave_private.pem

# =============================================================================
# SGX SDK Paths
# =============================================================================

ifeq ($(SGX_ARCH), x86)
	SGX_COMMON_FLAGS := -m32
	SGX_LIBRARY_PATH := $(SGX_SDK)/lib
	SGX_ENCLAVE_SIGNER := $(SGX_SDK)/bin/x86/sgx_sign
	SGX_EDGER8R := $(SGX_SDK)/bin/x86/sgx_edger8r
else
	SGX_COMMON_FLAGS := -m64
	SGX_LIBRARY_PATH := $(SGX_SDK)/lib64
	SGX_ENCLAVE_SIGNER := $(SGX_SDK)/bin/x64/sgx_sign
	SGX_EDGER8R := $(SGX_SDK)/bin/x64/sgx_edger8r
endif

ifeq ($(SGX_DEBUG), 1)
	SGX_COMMON_FLAGS += -O0 -g
	CARGO_FLAGS :=
	RUST_TARGET_DIR := $(BUILD_DIR)/debug
else
	SGX_COMMON_FLAGS += -O2
	CARGO_FLAGS := --release
	RUST_TARGET_DIR := $(BUILD_DIR)/release
endif

ifeq ($(SGX_MODE), HW)
	TRTS_LIB := sgx_trts
	SERVICE_LIB := sgx_tservice
	CRYPTO_LIB := sgx_tcrypto
	URTS_LIB := sgx_urts
	UAE_SERVICE_LIB := sgx_uae_service
	RUST_SGX_FEATURE :=
else
	TRTS_LIB := sgx_trts_sim
	SERVICE_LIB := sgx_tservice_sim
	CRYPTO_LIB := sgx_tcrypto
	URTS_LIB := sgx_urts_sim
	UAE_SERVICE_LIB := sgx_uae_service_sim
	RUST_SGX_FEATURE := --features sim
endif

# =============================================================================
# Compiler Flags
# =============================================================================

ENCLAVE_CFLAGS := $(SGX_COMMON_FLAGS) \
	-nostdinc \
	-fvisibility=hidden \
	-fpie \
	-ffunction-sections \
	-fdata-sections \
	-fstack-protector-strong

ENCLAVE_CXXFLAGS := $(ENCLAVE_CFLAGS) \
	-nostdinc++

ENCLAVE_LDFLAGS := \
	-Wl,--no-undefined \
	-nostdlib \
	-nodefaultlibs \
	-nostartfiles \
	-L$(SGX_LIBRARY_PATH) \
	-Wl,--whole-archive -l$(TRTS_LIB) -Wl,--no-whole-archive \
	-Wl,--start-group \
	-lsgx_tstdc \
	-lsgx_tcxx \
	-l$(CRYPTO_LIB) \
	-l$(SERVICE_LIB) \
	-Wl,--end-group \
	-Wl,-Bstatic \
	-Wl,-Bsymbolic \
	-Wl,--no-undefined \
	-Wl,-pie,-eenclave_entry \
	-Wl,--export-dynamic \
	-Wl,--defsym,__ImageBase=0 \
	-Wl,--gc-sections

APP_CFLAGS := $(SGX_COMMON_FLAGS) \
	-fPIC \
	-Wno-attributes \
	-I$(SGX_SDK)/include

APP_LDFLAGS := \
	-L$(SGX_LIBRARY_PATH) \
	-l$(URTS_LIB) \
	-l$(UAE_SERVICE_LIB) \
	-lpthread \
	-ldl

# =============================================================================
# Targets
# =============================================================================

.PHONY: all clean enclave bridge test install

all: enclave bridge

# Build the Rust SGX enclave
enclave:
	@echo "Building SGX enclave ($(SGX_MODE) mode)..."
	@mkdir -p $(RUST_TARGET_DIR)
	cd $(ENCLAVE_DIR) && $(CARGO) build $(CARGO_FLAGS) $(RUST_SGX_FEATURE)
	@echo "Enclave built successfully"

# Build the C bridge library
bridge: enclave
	@echo "Building SGX bridge library..."
	@mkdir -p $(RUST_TARGET_DIR)
	cd $(BRIDGE_DIR) && $(CARGO) build $(CARGO_FLAGS)
	@echo "Bridge library built successfully"

# Sign the enclave (for hardware mode)
sign: enclave
ifeq ($(SGX_MODE), HW)
	@echo "Signing enclave..."
	@if [ ! -f $(ENCLAVE_KEY) ]; then \
		openssl genrsa -out $(ENCLAVE_KEY) -3 3072; \
	fi
	$(SGX_ENCLAVE_SIGNER) sign \
		-key $(ENCLAVE_KEY) \
		-enclave $(RUST_TARGET_DIR)/lib$(ENCLAVE_NAME).so \
		-out $(RUST_TARGET_DIR)/$(ENCLAVE_NAME).signed.so \
		-config $(ENCLAVE_CONFIG)
	@echo "Enclave signed successfully"
else
	@echo "Skipping signing in simulation mode"
	@cp $(RUST_TARGET_DIR)/lib$(ENCLAVE_NAME).so $(RUST_TARGET_DIR)/$(ENCLAVE_NAME).signed.so 2>/dev/null || true
endif

# Run tests
test:
	@echo "Running enclave tests..."
	cd $(ENCLAVE_DIR) && $(CARGO) test $(RUST_SGX_FEATURE)
	cd $(BRIDGE_DIR) && $(CARGO) test

# Install libraries to system
install: bridge sign
	@echo "Installing SGX libraries..."
	@mkdir -p /usr/local/lib/sgx
	@cp $(RUST_TARGET_DIR)/libsgx_bridge.so /usr/local/lib/sgx/
	@cp $(RUST_TARGET_DIR)/$(ENCLAVE_NAME).signed.so /usr/local/lib/sgx/
	@ldconfig
	@echo "Installation complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd $(ENCLAVE_DIR) && $(CARGO) clean
	cd $(BRIDGE_DIR) && $(CARGO) clean
	cd $(APP_DIR) && $(CARGO) clean 2>/dev/null || true
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Generate EDL files (if using Intel SGX SDK directly)
edl:
	@echo "Generating EDL trusted/untrusted files..."
	$(SGX_EDGER8R) --trusted $(ENCLAVE_EDL) --search-path $(SGX_SDK)/include
	$(SGX_EDGER8R) --untrusted $(ENCLAVE_EDL) --search-path $(SGX_SDK)/include

# Check SGX environment
check:
	@echo "Checking SGX environment..."
	@echo "SGX_SDK: $(SGX_SDK)"
	@echo "SGX_MODE: $(SGX_MODE)"
	@echo "SGX_ARCH: $(SGX_ARCH)"
	@if [ -d "$(SGX_SDK)" ]; then \
		echo "SGX SDK: Found"; \
	else \
		echo "SGX SDK: NOT FOUND - Please install Intel SGX SDK"; \
	fi
	@if command -v rustc >/dev/null 2>&1; then \
		echo "Rust: $$(rustc --version)"; \
	else \
		echo "Rust: NOT FOUND"; \
	fi
	@if [ -f "/dev/isgx" ] || [ -f "/dev/sgx_enclave" ]; then \
		echo "SGX Device: Found (hardware mode available)"; \
	else \
		echo "SGX Device: NOT FOUND (simulation mode only)"; \
	fi

# Help
help:
	@echo "SGX Enclave Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build enclave and bridge (simulation mode)"
	@echo "  make SGX_MODE=HW  Build for hardware mode"
	@echo "  make sign         Sign the enclave (hardware mode)"
	@echo "  make test         Run tests"
	@echo "  make install      Install libraries"
	@echo "  make clean        Clean build artifacts"
	@echo "  make check        Check SGX environment"
	@echo ""
	@echo "Environment variables:"
	@echo "  SGX_SDK           Path to Intel SGX SDK (default: /opt/intel/sgxsdk)"
	@echo "  SGX_MODE          HW or SIM (default: SIM)"
	@echo "  SGX_DEBUG         1 or 0 (default: 1)"
