// Package registry provides enclave authenticity verification and service registration.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                     Enclave Registry System                              │
//	├─────────────────────────────────────────────────────────────────────────┤
//	│                                                                          │
//	│  1. Master Account Generation                                            │
//	│     - Enclave generates master keypair on first boot                     │
//	│     - Master public key registered to ServiceLayer contract              │
//	│     - Master key used to sign service registrations                      │
//	│                                                                          │
//	│  2. Service Enclave Registration                                         │
//	│     - Service script loaded into enclave engine                          │
//	│     - Script content hashed (SHA256)                                     │
//	│     - (ServiceName, ScriptHash) registered to contract                   │
//	│     - Registry remembers registered services                             │
//	│                                                                          │
//	│  3. Execution Verification                                               │
//	│     - Before execution, script is hashed                                 │
//	│     - Hash compared against registered hash                              │
//	│     - Execution proceeds only if hashes match                            │
//	│                                                                          │
//	│  4. Service Update                                                       │
//	│     - Explicit update call required                                      │
//	│     - New script hash computed and registered                            │
//	│     - Old hash invalidated                                               │
//	│                                                                          │
//	└─────────────────────────────────────────────────────────────────────────┘
package registry

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotInitialized       = errors.New("enclave registry not initialized")
	ErrMasterKeyNotSet      = errors.New("master key not set")
	ErrServiceNotRegistered = errors.New("service not registered")
	ErrScriptHashMismatch   = errors.New("script hash does not match registered hash")
	ErrServiceAlreadyExists = errors.New("service already registered")
	ErrInvalidSignature     = errors.New("invalid signature")
)

// ServiceEnclave represents a registered service enclave.
type ServiceEnclave struct {
	ServiceID    string    `json:"service_id"`
	ServiceName  string    `json:"service_name"`
	ScriptHash   string    `json:"script_hash"`   // SHA256 hash of script content
	Version      uint64    `json:"version"`       // Incremented on each update
	RegisteredAt time.Time `json:"registered_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Active       bool      `json:"active"`
}

// MasterAccount represents the enclave's master identity.
type MasterAccount struct {
	PublicKey    []byte    `json:"public_key"`
	KeyID        string    `json:"key_id"`
	CreatedAt    time.Time `json:"created_at"`
	RegisteredAt time.Time `json:"registered_at"` // When registered to contract
	Registered   bool      `json:"registered"`
}

// RegistryConfig holds configuration for the enclave registry.
type RegistryConfig struct {
	// ContractClient for interacting with ServiceLayer contract
	ContractClient ContractClient

	// SealKey for encrypting master key at rest
	SealKey []byte

	// AutoRegister automatically registers master account on init
	AutoRegister bool
}

// ContractClient interface for interacting with the blockchain.
type ContractClient interface {
	// RegisterMasterAccount registers the enclave master account
	RegisterMasterAccount(ctx context.Context, publicKey []byte, signature []byte) error

	// RegisterServiceEnclave registers a service enclave hash
	RegisterServiceEnclave(ctx context.Context, serviceID, scriptHash string, signature []byte) error

	// UpdateServiceEnclave updates a service enclave hash
	UpdateServiceEnclave(ctx context.Context, serviceID, newScriptHash string, version uint64, signature []byte) error

	// GetServiceEnclave retrieves registered service info from contract
	GetServiceEnclave(ctx context.Context, serviceID string) (*ServiceEnclave, error)

	// VerifyMasterAccount verifies master account is registered
	VerifyMasterAccount(ctx context.Context, publicKey []byte) (bool, error)
}

// Registry manages enclave authenticity verification.
type Registry struct {
	mu sync.RWMutex

	// Master account
	masterKey     *ecdsa.PrivateKey
	masterAccount *MasterAccount

	// Registered services (in-memory cache)
	services map[string]*ServiceEnclave

	// Configuration
	config *RegistryConfig

	// State
	initialized bool
}

// NewRegistry creates a new enclave registry.
func NewRegistry(cfg *RegistryConfig) *Registry {
	return &Registry{
		services: make(map[string]*ServiceEnclave),
		config:   cfg,
	}
}

// Initialize initializes the registry and generates/loads master account.
func (r *Registry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	// Generate master key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate master key: %w", err)
	}

	r.masterKey = privateKey

	// Create master account
	pubKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve,
		privateKey.PublicKey.X, privateKey.PublicKey.Y)

	keyID := sha256.Sum256(pubKeyBytes)

	r.masterAccount = &MasterAccount{
		PublicKey:  pubKeyBytes,
		KeyID:      hex.EncodeToString(keyID[:]),
		CreatedAt:  time.Now(),
		Registered: false,
	}

	// Auto-register if configured
	if r.config.AutoRegister && r.config.ContractClient != nil {
		if err := r.registerMasterAccountLocked(ctx); err != nil {
			return fmt.Errorf("failed to auto-register master account: %w", err)
		}
	}

	r.initialized = true
	return nil
}

// RegisterMasterAccount registers the master account to the contract.
func (r *Registry) RegisterMasterAccount(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.registerMasterAccountLocked(ctx)
}

func (r *Registry) registerMasterAccountLocked(ctx context.Context) error {
	if r.masterKey == nil {
		return ErrMasterKeyNotSet
	}

	if r.masterAccount.Registered {
		return nil // Already registered
	}

	if r.config.ContractClient == nil {
		return errors.New("contract client not configured")
	}

	// Sign the public key for registration
	signature, err := r.signData(r.masterAccount.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to sign registration: %w", err)
	}

	// Register to contract
	if err := r.config.ContractClient.RegisterMasterAccount(ctx, r.masterAccount.PublicKey, signature); err != nil {
		return fmt.Errorf("failed to register master account: %w", err)
	}

	r.masterAccount.Registered = true
	r.masterAccount.RegisteredAt = time.Now()

	return nil
}

// RegisterServiceEnclave registers a new service enclave.
func (r *Registry) RegisterServiceEnclave(ctx context.Context, serviceID, serviceName string, scriptContent []byte) (*ServiceEnclave, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil, ErrNotInitialized
	}

	if !r.masterAccount.Registered {
		return nil, ErrMasterKeyNotSet
	}

	// Check if already registered
	if _, exists := r.services[serviceID]; exists {
		return nil, ErrServiceAlreadyExists
	}

	// Compute script hash
	scriptHash := computeScriptHash(scriptContent)

	// Create service enclave record
	service := &ServiceEnclave{
		ServiceID:    serviceID,
		ServiceName:  serviceName,
		ScriptHash:   scriptHash,
		Version:      1,
		RegisteredAt: time.Now(),
		UpdatedAt:    time.Now(),
		Active:       true,
	}

	// Sign the registration
	registrationData := []byte(serviceID + scriptHash)
	signature, err := r.signData(registrationData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign service registration: %w", err)
	}

	// Register to contract
	if r.config.ContractClient != nil {
		if err := r.config.ContractClient.RegisterServiceEnclave(ctx, serviceID, scriptHash, signature); err != nil {
			return nil, fmt.Errorf("failed to register service to contract: %w", err)
		}
	}

	// Store in local cache
	r.services[serviceID] = service

	return service, nil
}

// UpdateServiceEnclave updates an existing service enclave script.
func (r *Registry) UpdateServiceEnclave(ctx context.Context, serviceID string, newScriptContent []byte) (*ServiceEnclave, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil, ErrNotInitialized
	}

	// Get existing service
	service, exists := r.services[serviceID]
	if !exists {
		return nil, ErrServiceNotRegistered
	}

	// Compute new script hash
	newScriptHash := computeScriptHash(newScriptContent)

	// If hash is the same, no update needed
	if newScriptHash == service.ScriptHash {
		return service, nil
	}

	// Increment version
	newVersion := service.Version + 1

	// Sign the update
	updateData := []byte(fmt.Sprintf("%s%s%d", serviceID, newScriptHash, newVersion))
	signature, err := r.signData(updateData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign service update: %w", err)
	}

	// Update on contract
	if r.config.ContractClient != nil {
		if err := r.config.ContractClient.UpdateServiceEnclave(ctx, serviceID, newScriptHash, newVersion, signature); err != nil {
			return nil, fmt.Errorf("failed to update service on contract: %w", err)
		}
	}

	// Update local cache
	service.ScriptHash = newScriptHash
	service.Version = newVersion
	service.UpdatedAt = time.Now()

	return service, nil
}

// VerifyScript verifies a script's authenticity before execution.
func (r *Registry) VerifyScript(serviceID string, scriptContent []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.initialized {
		return ErrNotInitialized
	}

	// Get registered service
	service, exists := r.services[serviceID]
	if !exists {
		return ErrServiceNotRegistered
	}

	if !service.Active {
		return errors.New("service is not active")
	}

	// Compute script hash
	scriptHash := computeScriptHash(scriptContent)

	// Compare with registered hash
	if scriptHash != service.ScriptHash {
		return fmt.Errorf("%w: expected %s, got %s", ErrScriptHashMismatch, service.ScriptHash, scriptHash)
	}

	return nil
}

// GetServiceEnclave returns a registered service enclave.
func (r *Registry) GetServiceEnclave(serviceID string) (*ServiceEnclave, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[serviceID]
	if !exists {
		return nil, ErrServiceNotRegistered
	}

	// Return a copy
	copy := *service
	return &copy, nil
}

// GetMasterAccount returns the master account info.
func (r *Registry) GetMasterAccount() (*MasterAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.masterAccount == nil {
		return nil, ErrMasterKeyNotSet
	}

	// Return a copy
	copy := *r.masterAccount
	return &copy, nil
}

// GetMasterPublicKey returns the master public key.
func (r *Registry) GetMasterPublicKey() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.masterAccount == nil {
		return nil, ErrMasterKeyNotSet
	}

	return r.masterAccount.PublicKey, nil
}

// ListServices returns all registered services.
func (r *Registry) ListServices() []*ServiceEnclave {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*ServiceEnclave, 0, len(r.services))
	for _, s := range r.services {
		copy := *s
		services = append(services, &copy)
	}
	return services
}

// DeactivateService deactivates a service (prevents execution).
func (r *Registry) DeactivateService(serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[serviceID]
	if !exists {
		return ErrServiceNotRegistered
	}

	service.Active = false
	return nil
}

// ActivateService activates a service.
func (r *Registry) ActivateService(serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[serviceID]
	if !exists {
		return ErrServiceNotRegistered
	}

	service.Active = true
	return nil
}

// SignData signs data with the master key.
func (r *Registry) SignData(data []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.signData(data)
}

func (r *Registry) signData(data []byte) ([]byte, error) {
	if r.masterKey == nil {
		return nil, ErrMasterKeyNotSet
	}

	hash := sha256.Sum256(data)
	signature, err := ecdsa.SignASN1(rand.Reader, r.masterKey, hash[:])
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// IsInitialized returns whether the registry is initialized.
func (r *Registry) IsInitialized() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.initialized
}

// computeScriptHash computes SHA256 hash of script content.
func computeScriptHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// ComputeScriptHash is a public helper to compute script hash.
func ComputeScriptHash(content []byte) string {
	return computeScriptHash(content)
}
