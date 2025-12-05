// Package base provides base components for all services.
// Services extend these base types to implement their specific functionality.
package base

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
)

// ServiceState represents the state of a service.
type ServiceState string

const (
	StateCreated  ServiceState = "created"
	StateStarting ServiceState = "starting"
	StateRunning  ServiceState = "running"
	StateStopping ServiceState = "stopping"
	StateStopped  ServiceState = "stopped"
	StateFailed   ServiceState = "failed"
)

// Service is the base interface for all services.
type Service interface {
	// Identity
	ID() string
	Name() string
	Version() string

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	State() ServiceState

	// Health
	Health(ctx context.Context) error
}

// =============================================================================
// Component Interfaces - for unified lifecycle management
// =============================================================================

// Component is the base interface for service components (enclave, store, etc.)
type Component interface {
	// Initialize initializes the component
	Initialize(ctx context.Context) error
	// Shutdown shuts down the component
	Shutdown(ctx context.Context) error
	// Health checks component health
	Health(ctx context.Context) error
}

// Store is the interface for service data stores.
type Store interface {
	Component
	// Close closes the store (alias for Shutdown for backward compatibility)
	Close(ctx context.Context) error
}

// =============================================================================
// Lifecycle Hooks - for customizing service behavior
// =============================================================================

// LifecycleHooks allows services to customize lifecycle behavior.
type LifecycleHooks struct {
	// OnBeforeStart is called before starting components
	OnBeforeStart func(ctx context.Context) error
	// OnAfterStart is called after all components are started
	OnAfterStart func(ctx context.Context) error
	// OnBeforeStop is called before stopping components
	OnBeforeStop func(ctx context.Context) error
	// OnAfterStop is called after all components are stopped
	OnAfterStop func(ctx context.Context) error
}

// =============================================================================
// BaseService - enhanced with component management
// =============================================================================

// BaseService provides common functionality for all services.
type BaseService struct {
	mu sync.RWMutex

	id      string
	name    string
	version string
	state   ServiceState

	os     os.ServiceOS
	logger os.Logger

	// Component management
	enclave Component
	store   Store
	hooks   LifecycleHooks

	// Config storage key for sealed config hydration
	configKey string
}

// NewBaseService creates a new BaseService.
func NewBaseService(id, name, version string, serviceOS os.ServiceOS) *BaseService {
	return &BaseService{
		id:        id,
		name:      name,
		version:   version,
		state:     StateCreated,
		os:        serviceOS,
		logger:    serviceOS.Logger(),
		configKey: id + "/config", // default config key
	}
}

// SetEnclave sets the enclave component for lifecycle management.
func (s *BaseService) SetEnclave(enclave Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enclave = enclave
}

// SetStore sets the store component for lifecycle management.
func (s *BaseService) SetStore(store Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
}

// SetHooks sets lifecycle hooks.
func (s *BaseService) SetHooks(hooks LifecycleHooks) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = hooks
}

// SetConfigKey sets the sealed config storage key.
func (s *BaseService) SetConfigKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configKey = key
}

// GetConfigKey returns the sealed config storage key.
func (s *BaseService) GetConfigKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configKey
}

// ID returns the service ID.
func (s *BaseService) ID() string {
	return s.id
}

// Name returns the service name.
func (s *BaseService) Name() string {
	return s.name
}

// Version returns the service version.
func (s *BaseService) Version() string {
	return s.version
}

// State returns the current service state.
func (s *BaseService) State() ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState sets the service state.
func (s *BaseService) SetState(state ServiceState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// OS returns the ServiceOS.
func (s *BaseService) OS() os.ServiceOS {
	return s.os
}

// Logger returns the logger.
func (s *BaseService) Logger() os.Logger {
	return s.logger
}

// Start starts the base service with component lifecycle management.
// If enclave and store are set, they will be initialized automatically.
func (s *BaseService) Start(ctx context.Context) error {
	s.SetState(StateStarting)
	s.logger.Info("service starting", "id", s.id)

	// Get components and hooks under lock
	s.mu.RLock()
	enclave := s.enclave
	store := s.store
	hooks := s.hooks
	s.mu.RUnlock()

	// OnBeforeStart hook
	if hooks.OnBeforeStart != nil {
		if err := hooks.OnBeforeStart(ctx); err != nil {
			s.SetState(StateFailed)
			return fmt.Errorf("before start hook: %w", err)
		}
	}

	// Initialize enclave
	if enclave != nil {
		if err := enclave.Initialize(ctx); err != nil {
			s.SetState(StateFailed)
			return fmt.Errorf("initialize enclave: %w", err)
		}
	}

	// Initialize store
	if store != nil {
		if err := store.Initialize(ctx); err != nil {
			s.SetState(StateFailed)
			return fmt.Errorf("initialize store: %w", err)
		}
	}

	// OnAfterStart hook
	if hooks.OnAfterStart != nil {
		if err := hooks.OnAfterStart(ctx); err != nil {
			s.SetState(StateFailed)
			return fmt.Errorf("after start hook: %w", err)
		}
	}

	s.SetState(StateRunning)
	s.logger.Info("service started", "id", s.id)
	return nil
}

// Stop stops the base service with component lifecycle management.
// Components are stopped in reverse order (store first, then enclave).
func (s *BaseService) Stop(ctx context.Context) error {
	s.SetState(StateStopping)
	s.logger.Info("service stopping", "id", s.id)

	// Get components and hooks under lock
	s.mu.RLock()
	enclave := s.enclave
	store := s.store
	hooks := s.hooks
	s.mu.RUnlock()

	// OnBeforeStop hook
	if hooks.OnBeforeStop != nil {
		if err := hooks.OnBeforeStop(ctx); err != nil {
			s.logger.Error("before stop hook failed", "error", err)
		}
	}

	// Shutdown store first (reverse order)
	if store != nil {
		if err := store.Close(ctx); err != nil {
			s.logger.Error("close store failed", "error", err)
		}
	}

	// Shutdown enclave
	if enclave != nil {
		if err := enclave.Shutdown(ctx); err != nil {
			s.logger.Error("shutdown enclave failed", "error", err)
		}
	}

	// OnAfterStop hook
	if hooks.OnAfterStop != nil {
		if err := hooks.OnAfterStop(ctx); err != nil {
			s.logger.Error("after stop hook failed", "error", err)
		}
	}

	s.SetState(StateStopped)
	s.logger.Info("service stopped", "id", s.id)
	return nil
}

// Health checks if the service and all components are healthy.
func (s *BaseService) Health(ctx context.Context) error {
	state := s.State()
	if state != StateRunning {
		return fmt.Errorf("service not running: %s", state)
	}

	// Get components under lock
	s.mu.RLock()
	enclave := s.enclave
	store := s.store
	s.mu.RUnlock()

	// Check enclave health
	if enclave != nil {
		if err := enclave.Health(ctx); err != nil {
			return fmt.Errorf("enclave unhealthy: %w", err)
		}
	}

	// Check store health
	if store != nil {
		if err := store.Health(ctx); err != nil {
			return fmt.Errorf("store unhealthy: %w", err)
		}
	}

	return nil
}

// =============================================================================
// Helper Functions - commonly used across services
// =============================================================================

// IsCapabilityDenied checks if an error is a capability denied error.
func IsCapabilityDenied(err error) bool {
	var osErr *os.OSError
	return errors.As(err, &osErr) && osErr.Code == os.ErrCodeCapabilityDenied
}

// RegisterMetrics registers common service metrics if metrics capability is available.
func (s *BaseService) RegisterMetrics(prefix string, customMetrics func(metrics os.MetricsAPI)) {
	if !s.os.HasCapability(os.CapMetrics) {
		return
	}

	metrics := s.os.Metrics()

	// Register common metrics
	metrics.RegisterCounter(prefix+"_requests_total", "Total number of requests")
	metrics.RegisterCounter(prefix+"_errors_total", "Total number of errors")
	metrics.RegisterHistogram(prefix+"_duration_seconds", "Request duration", []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10})

	// Register custom metrics
	if customMetrics != nil {
		customMetrics(metrics)
	}
}

// StartContractListener starts listening for contract requests if capability is available.
func (s *BaseService) StartContractListener(ctx context.Context, handler os.ServiceRequestHandler) {
	if !s.os.HasCapability(os.CapContract) {
		return
	}

	contract := s.os.Contract()
	_, err := contract.SubscribeRequests(ctx, handler)
	if err != nil {
		s.logger.Warn("failed to subscribe to contract requests", "error", err)
		return
	}

	s.logger.Info("contract request listener started")
}

// SendContractCallback sends a successful callback to the contract.
func (s *BaseService) SendContractCallback(ctx context.Context, requestID string, result, signature []byte) error {
	if !s.os.HasCapability(os.CapContractWrite) {
		return fmt.Errorf("contract write capability not available")
	}

	response := &os.ServiceResponse{
		RequestID: requestID,
		Success:   true,
		Result:    result,
		Signature: signature,
	}

	if err := s.os.Contract().SendCallback(ctx, response); err != nil {
		s.logger.Error("failed to send contract callback", "error", err, "request_id", requestID)
		return err
	}

	s.logger.Info("contract callback sent", "request_id", requestID)
	return nil
}

// SendContractError sends an error callback to the contract.
func (s *BaseService) SendContractError(ctx context.Context, requestID, errMsg string) error {
	if !s.os.HasCapability(os.CapContractWrite) {
		return fmt.Errorf("contract write capability not available")
	}

	response := &os.ServiceResponse{
		RequestID: requestID,
		Success:   false,
		Error:     errMsg,
	}

	if err := s.os.Contract().SendCallback(ctx, response); err != nil {
		s.logger.Error("failed to send error callback", "error", err, "request_id", requestID)
		return err
	}

	s.logger.Warn("contract error callback sent", "request_id", requestID, "error", errMsg)
	return nil
}

// =============================================================================
// Service Registry
// =============================================================================

// Registry manages service instances.
type Registry struct {
	mu       sync.RWMutex
	services map[string]Service
}

// NewRegistry creates a new service registry.
func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]Service),
	}
}

// Register registers a service.
func (r *Registry) Register(svc Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[svc.ID()]; exists {
		return fmt.Errorf("service already registered: %s", svc.ID())
	}

	r.services[svc.ID()] = svc
	return nil
}

// Unregister unregisters a service.
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.services, id)
	return nil
}

// Get returns a service by ID.
func (r *Registry) Get(id string) (Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.services[id]
	return svc, ok
}

// List returns all registered services.
func (r *Registry) List() []Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]Service, 0, len(r.services))
	for _, svc := range r.services {
		services = append(services, svc)
	}
	return services
}

// StartAll starts all registered services.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	services := make([]Service, 0, len(r.services))
	for _, svc := range r.services {
		services = append(services, svc)
	}
	r.mu.RUnlock()

	for _, svc := range services {
		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("start service %s: %w", svc.ID(), err)
		}
	}
	return nil
}

// StopAll stops all registered services.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	services := make([]Service, 0, len(r.services))
	for _, svc := range r.services {
		services = append(services, svc)
	}
	r.mu.RUnlock()

	// Stop in reverse order
	for i := len(services) - 1; i >= 0; i-- {
		if err := services[i].Stop(ctx); err != nil {
			return fmt.Errorf("stop service %s: %w", services[i].ID(), err)
		}
	}
	return nil
}
