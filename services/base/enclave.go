// Package base provides base components for all services.
package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/platform/supabase"
)

// Enclave is the base interface for service enclave operations.
// Enclave operations run inside the TEE and have access to secrets.
type Enclave interface {
	// Initialize initializes the enclave
	Initialize(ctx context.Context) error

	// Shutdown shuts down the enclave
	Shutdown(ctx context.Context) error

	// Health checks enclave health
	Health(ctx context.Context) error
}

// BaseEnclave provides common enclave functionality.
type BaseEnclave struct {
	mu sync.RWMutex

	serviceID string
	os        os.ServiceOS
	logger    os.Logger
	ready     bool
}

// NewBaseEnclave creates a new BaseEnclave.
func NewBaseEnclave(serviceID string, serviceOS os.ServiceOS) *BaseEnclave {
	return &BaseEnclave{
		serviceID: serviceID,
		os:        serviceOS,
		logger:    serviceOS.Logger(),
	}
}

// Initialize initializes the base enclave.
func (e *BaseEnclave) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ready {
		return nil
	}

	e.logger.Info("enclave initializing", "service", e.serviceID)
	e.ready = true
	e.logger.Info("enclave initialized", "service", e.serviceID)

	return nil
}

// Shutdown shuts down the base enclave.
func (e *BaseEnclave) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.ready {
		return nil
	}

	e.logger.Info("enclave shutting down", "service", e.serviceID)
	e.ready = false
	e.logger.Info("enclave shut down", "service", e.serviceID)

	return nil
}

// Health checks if the enclave is healthy.
func (e *BaseEnclave) Health(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.ready {
		return fmt.Errorf("enclave not ready")
	}
	return nil
}

// OS returns the ServiceOS.
func (e *BaseEnclave) OS() os.ServiceOS {
	return e.os
}

// Logger returns the logger.
func (e *BaseEnclave) Logger() os.Logger {
	return e.logger
}

// ServiceID returns the service ID.
func (e *BaseEnclave) ServiceID() string {
	return e.serviceID
}

// IsReady returns whether the enclave is ready.
func (e *BaseEnclave) IsReady() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ready
}

// =============================================================================
// Enclave Operations Helpers
// =============================================================================

// UseSecret executes a function with access to a secret.
func (e *BaseEnclave) UseSecret(ctx context.Context, name string, fn func(secret []byte) error) error {
	if !e.IsReady() {
		return fmt.Errorf("enclave not ready")
	}
	return e.os.Secrets().Use(ctx, name, fn)
}

// UseSecrets executes a function with access to multiple secrets.
func (e *BaseEnclave) UseSecrets(ctx context.Context, names []string, fn func(secrets map[string][]byte) error) error {
	if !e.IsReady() {
		return fmt.Errorf("enclave not ready")
	}
	return e.os.Secrets().UseMultiple(ctx, names, fn)
}

// DeriveKey derives a key for this service.
func (e *BaseEnclave) DeriveKey(ctx context.Context, subPath string) (os.KeyHandle, error) {
	if !e.IsReady() {
		return "", fmt.Errorf("enclave not ready")
	}
	return e.os.Keys().DeriveKey(ctx, subPath)
}

// Sign signs data with a key.
func (e *BaseEnclave) Sign(ctx context.Context, handle os.KeyHandle, data []byte) ([]byte, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}
	return e.os.Keys().Sign(ctx, handle, data)
}

// FetchWithSecret performs an HTTP request with secret-based auth.
func (e *BaseEnclave) FetchWithSecret(ctx context.Context, req os.HTTPRequest, secretName string, authType os.AuthType) (*os.HTTPResponse, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}
	return e.os.Network().FetchWithSecret(ctx, req, secretName, authType)
}

// Execute runs code inside the enclave.
func (e *BaseEnclave) Execute(ctx context.Context, req os.ComputeRequest) (*os.ComputeResult, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}
	return e.os.Compute().Execute(ctx, req)
}

// ExecuteWithSecrets runs code with access to secrets.
func (e *BaseEnclave) ExecuteWithSecrets(ctx context.Context, req os.ComputeRequest, secretNames []string) (*os.ComputeResult, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}
	return e.os.Compute().ExecuteWithSecrets(ctx, req, secretNames)
}

// GenerateQuote generates an attestation quote.
func (e *BaseEnclave) GenerateQuote(ctx context.Context, userData []byte) (*os.Quote, error) {
	if !e.IsReady() {
		return nil, fmt.Errorf("enclave not ready")
	}
	return e.os.Attestation().GenerateQuote(ctx, userData)
}

// UseStorage executes a callback with a stored value without exporting it to the caller.
func (e *BaseEnclave) UseStorage(ctx context.Context, key string, fn func(value []byte) error) error {
	if !e.IsReady() {
		return fmt.Errorf("enclave not ready")
	}
	return e.os.Storage().Use(ctx, key, fn)
}

// StorageExists checks if a storage key exists in the enclave-backed store.
func (e *BaseEnclave) StorageExists(ctx context.Context, key string) (bool, error) {
	if !e.IsReady() {
		return false, fmt.Errorf("enclave not ready")
	}
	return e.os.Storage().Exists(ctx, key)
}

// LoadConfigJSON loads a sealed JSON config (if present) into dst.
// Returns (loaded, nil) if present, (false, nil) if missing or capability is denied.
func (e *BaseEnclave) LoadConfigJSON(ctx context.Context, key string, dst any) (bool, error) {
	if !e.IsReady() {
		return false, fmt.Errorf("enclave not ready")
	}

	exists, err := e.StorageExists(ctx, key)
	if err != nil {
		var osErr *os.OSError
		if errors.As(err, &osErr) && osErr.Code == os.ErrCodeCapabilityDenied {
			return false, nil
		}
		return false, err
	}
	if !exists {
		return false, nil
	}

	if err := e.UseStorage(ctx, key, func(value []byte) error {
		return json.Unmarshal(value, dst)
	}); err != nil {
		return false, err
	}

	return true, nil
}

// SupabaseClient loads a sealed Supabase config and returns a ready client.
// If the config is missing or capability is denied, (nil, false, nil) is returned.
func (e *BaseEnclave) SupabaseClient(ctx context.Context, storageKey string) (*supabase.Client, bool, error) {
	var cfg supabase.Config
	loaded, err := e.LoadConfigJSON(ctx, storageKey, &cfg)
	if err != nil || !loaded {
		return nil, loaded, err
	}
	client, err := supabase.New(e.os, supabase.Config{
		ProjectURL:     cfg.ProjectURL,
		APIKeySecret:   cfg.APIKeySecret,
		APIKey:         cfg.APIKey,
		DefaultHeaders: cfg.DefaultHeaders,
		AllowedHosts:   cfg.AllowedHosts,
	})
	if err != nil {
		return nil, true, err
	}
	return client, true, nil
}
