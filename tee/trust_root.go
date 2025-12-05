// Package tee provides the Trust Root for the Service Layer.
// TEE (Trusted Execution Environment) is the foundation of all secure operations.
// All sensitive data (secrets, keys, credentials) NEVER leave the enclave in plaintext.
package tee

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/tee/attestation"
	"github.com/R3E-Network/service_layer/tee/bridge"
	"github.com/R3E-Network/service_layer/tee/compute"
	"github.com/R3E-Network/service_layer/tee/enclave"
	"github.com/R3E-Network/service_layer/tee/keys"
	"github.com/R3E-Network/service_layer/tee/neo"
	"github.com/R3E-Network/service_layer/tee/network"
	"github.com/R3E-Network/service_layer/tee/types"
	"github.com/R3E-Network/service_layer/tee/vault"
)

// Config holds TrustRoot configuration.
type Config struct {
	// EnclaveID is the unique identifier for this enclave
	EnclaveID string

	// Mode specifies simulation or hardware mode
	Mode types.EnclaveMode

	// SealingKeyPath is the path to the sealing key (simulation mode only)
	SealingKeyPath string

	// StoragePath is the path for sealed storage (secrets, state)
	StoragePath string

	// AllowedHosts restricts outbound network destinations (optional)
	AllowedHosts []string

	// PinnedCerts maps host -> hex-encoded SHA256 fingerprint for certificate pinning (optional)
	PinnedCerts map[string]string

	// DebugMode enables debug logging
	DebugMode bool
}

// TrustRoot implements types.TrustRoot.
// It is the foundation of all secure operations in the Service Layer.
type TrustRoot struct {
	mu sync.RWMutex

	config  Config
	runtime enclave.Runtime

	// Core components
	vaultImpl       *vault.Vault
	networkImpl     *network.Client
	keysImpl        *keys.Manager
	computeImpl     *compute.Engine
	attestationImpl *attestation.Attestor
	neoImpl         *neo.Signer // Neo N3 blockchain operations

	ready bool
}

// New creates a new TrustRoot.
func New(cfg Config) (*TrustRoot, error) {
	if cfg.EnclaveID == "" {
		return nil, fmt.Errorf("enclave_id is required")
	}

	// Convert mode
	var enclaveMode enclave.Mode
	switch cfg.Mode {
	case types.EnclaveModeHardware:
		enclaveMode = enclave.ModeHardware
	default:
		enclaveMode = enclave.ModeSimulation
	}

	// Create enclave runtime
	runtime, err := enclave.New(enclave.Config{
		Mode:           enclaveMode,
		EnclaveID:      cfg.EnclaveID,
		SealingKeyPath: cfg.SealingKeyPath,
		DebugMode:      cfg.DebugMode,
	})
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}

	return &TrustRoot{
		config:  cfg,
		runtime: runtime,
	}, nil
}

// Start initializes and starts the TrustRoot.
func (t *TrustRoot) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ready {
		return nil
	}

	// Initialize runtime
	if err := t.runtime.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize runtime: %w", err)
	}

	// Configure sealed storage bridge if requested
	var storage vault.Storage
	if t.config.StoragePath != "" {
		bridgeStorage, err := bridge.NewStorage(bridge.StorageConfig{
			BasePath: t.config.StoragePath,
		})
		if err != nil {
			return fmt.Errorf("create storage bridge: %w", err)
		}
		storage = bridgeStorage
	}

	// Initialize vault
	vaultImpl, err := vault.New(vault.Config{
		Runtime: t.runtime,
		Storage: storage,
	})
	if err != nil {
		return fmt.Errorf("create vault: %w", err)
	}
	t.vaultImpl = vaultImpl

	// Initialize key manager
	keysImpl := keys.New(t.runtime)
	if err := keysImpl.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize keys: %w", err)
	}
	t.keysImpl = keysImpl

	// Initialize network client
	pins, err := decodePins(t.config.PinnedCerts)
	if err != nil {
		return fmt.Errorf("decode pinned certs: %w", err)
	}

	networkImpl, err := network.New(network.Config{
		Runtime:      t.runtime,
		Vault:        t.vaultImpl,
		AllowedHosts: t.config.AllowedHosts,
		PinnedCerts:  pins,
	})
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	t.networkImpl = networkImpl

	// Initialize compute engine
	computeImpl, err := compute.New(compute.Config{
		Runtime:  t.runtime,
		Vault:    t.vaultImpl,
		Executor: &compute.DefaultExecutor{},
	})
	if err != nil {
		return fmt.Errorf("create compute: %w", err)
	}
	t.computeImpl = computeImpl

	// Initialize attestor
	attestorImpl, err := attestation.New(attestation.Config{
		Runtime:   t.runtime,
		EnclaveID: t.config.EnclaveID,
	})
	if err != nil {
		return fmt.Errorf("create attestor: %w", err)
	}
	t.attestationImpl = attestorImpl

	// Initialize Neo signer
	neoImpl, err := neo.New(neo.Config{
		Runtime: t.runtime,
	})
	if err != nil {
		return fmt.Errorf("create neo signer: %w", err)
	}
	t.neoImpl = neoImpl

	t.ready = true
	return nil
}

// Stop shuts down the TrustRoot.
func (t *TrustRoot) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return nil
	}

	// Zero sensitive data
	if t.keysImpl != nil {
		t.keysImpl.Zero()
	}

	// Zero Neo signer data
	if t.neoImpl != nil {
		t.neoImpl.Zero()
	}

	// Shutdown runtime
	if err := t.runtime.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown runtime: %w", err)
	}

	t.ready = false
	return nil
}

// Health checks if the TrustRoot is healthy.
func (t *TrustRoot) Health(ctx context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.ready {
		return types.ErrEnclaveNotReady
	}

	return t.runtime.Health(ctx)
}

// Mode returns the enclave mode.
func (t *TrustRoot) Mode() types.EnclaveMode {
	return t.config.Mode
}

// Vault returns the SecureVault.
func (t *TrustRoot) Vault() types.SecureVault {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.vaultImpl
}

// Network returns the SecureNetwork.
func (t *TrustRoot) Network() types.SecureNetwork {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.networkImpl
}

// Keys returns the KeyManager.
func (t *TrustRoot) Keys() types.KeyManager {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.keysImpl
}

// Compute returns the ConfidentialCompute.
func (t *TrustRoot) Compute() types.ConfidentialCompute {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.computeImpl
}

// Attestation returns the Attestor.
func (t *TrustRoot) Attestation() types.Attestor {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.attestationImpl
}

// Neo returns the NeoSigner.
func (t *TrustRoot) Neo() types.NeoSigner {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.neoImpl
}

// Runtime returns the enclave runtime (for internal use).
func (t *TrustRoot) Runtime() enclave.Runtime {
	return t.runtime
}

// =============================================================================
// Factory Functions
// =============================================================================

// NewSimulation creates a TrustRoot in simulation mode.
func NewSimulation(enclaveID string) (*TrustRoot, error) {
	return New(Config{
		EnclaveID: enclaveID,
		Mode:      types.EnclaveModeSimulation,
		DebugMode: true,
	})
}

// NewHardware creates a TrustRoot in hardware mode.
func NewHardware(enclaveID string) (*TrustRoot, error) {
	return New(Config{
		EnclaveID: enclaveID,
		Mode:      types.EnclaveModeHardware,
	})
}

// decodePins converts hex-encoded pins to raw bytes.
func decodePins(src map[string]string) (map[string][]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}
	dst := make(map[string][]byte, len(src))
	for host, hexVal := range src {
		if host == "" || hexVal == "" {
			continue
		}
		b, err := hex.DecodeString(hexVal)
		if err != nil {
			return nil, fmt.Errorf("decode pin for host %s: %w", host, err)
		}
		dst[host] = b
	}
	return dst, nil
}
