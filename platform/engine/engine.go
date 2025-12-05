// Package engine provides the Service Engine for the Service Layer.
// The Engine is like Android's ActivityManager + ServiceManager combined.
// It manages service lifecycle, orchestration, and provides a unified API.
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/bootstrap"
	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// State represents the engine state.
type State string

const (
	StateCreated  State = "created"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
)

// ServiceFactory creates a service from a ServiceOS context.
type ServiceFactory func(os.ServiceOS) (base.Service, error)

// ServiceDefinition defines a service to be registered.
type ServiceDefinition struct {
	Manifest *os.LegacyManifest
	Factory  ServiceFactory
}

// Config holds engine configuration.
type Config struct {
	// TEE configuration
	EnclaveID      string
	Mode           string // "simulation" or "hardware"
	SealingKeyPath string
	StoragePath    string
	AllowedHosts   []string
	PinnedCerts    map[string]string
	Debug          bool

	// Supabase configuration
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseServiceKey string
	SupabaseJWTSecret  string

	// Engine configuration
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		EnclaveID:       "service-layer-enclave",
		Mode:            "simulation",
		SealingKeyPath:  "./sealing.key",
		StoragePath:     "./sealed_store",
		StartupTimeout:  30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// Engine is the core service orchestrator.
// It manages the TEE foundation, service registry, and lifecycle.
type Engine struct {
	mu sync.RWMutex

	config     Config
	state      State
	foundation *bootstrap.Foundation
	registry   *base.Registry

	// Service definitions (registered before Start)
	definitions []ServiceDefinition

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Callbacks
	onStateChange func(State)
}

// New creates a new Engine.
func New(cfg Config) *Engine {
	return &Engine{
		config:      cfg,
		state:       StateCreated,
		definitions: make([]ServiceDefinition, 0),
	}
}

// State returns the current engine state.
func (e *Engine) State() State {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Foundation returns the TEE foundation.
func (e *Engine) Foundation() *bootstrap.Foundation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.foundation
}

// Registry returns the service registry.
func (e *Engine) Registry() *base.Registry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.registry
}

// OnStateChange sets a callback for state changes.
func (e *Engine) OnStateChange(fn func(State)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStateChange = fn
}

// setState updates the engine state and notifies listeners.
func (e *Engine) setState(state State) {
	e.mu.Lock()
	e.state = state
	callback := e.onStateChange
	e.mu.Unlock()

	if callback != nil {
		callback(state)
	}
}

// RegisterService registers a service definition.
// Must be called before Start().
func (e *Engine) RegisterService(def ServiceDefinition) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != StateCreated {
		return fmt.Errorf("cannot register services after engine has started")
	}

	if def.Manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	if def.Factory == nil {
		return fmt.Errorf("factory is required")
	}

	e.definitions = append(e.definitions, def)
	return nil
}

// RegisterServices registers multiple service definitions.
func (e *Engine) RegisterServices(defs []ServiceDefinition) error {
	for _, def := range defs {
		if err := e.RegisterService(def); err != nil {
			return err
		}
	}
	return nil
}

// Start initializes and starts the engine.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.state != StateCreated {
		e.mu.Unlock()
		return fmt.Errorf("engine already started")
	}
	e.mu.Unlock()

	e.setState(StateStarting)

	// Create context
	e.ctx, e.cancel = context.WithCancel(ctx)

	// Initialize TEE foundation
	log.Printf("[Engine] Initializing TEE Trust Root...")
	foundation, err := bootstrap.NewFoundation(bootstrap.Config{
		EnclaveID:      e.config.EnclaveID,
		Mode:           e.config.Mode,
		SealingKeyPath: e.config.SealingKeyPath,
		StoragePath:    e.config.StoragePath,
		AllowedHosts:   e.config.AllowedHosts,
		PinnedCerts:    e.config.PinnedCerts,
		Debug:          e.config.Debug,
	})
	if err != nil {
		e.setState(StateStopped)
		return fmt.Errorf("create foundation: %w", err)
	}

	if err := foundation.Start(e.ctx); err != nil {
		e.setState(StateStopped)
		return fmt.Errorf("start foundation: %w", err)
	}

	e.mu.Lock()
	e.foundation = foundation
	e.mu.Unlock()

	log.Printf("[Engine] TEE Trust Root initialized (mode: %s)", foundation.TrustRoot().Mode())

	// Initialize Supabase if configured
	if e.config.SupabaseURL != "" {
		log.Printf("[Engine] Initializing Supabase integration...")
		if err := foundation.InitSupabase(e.ctx, bootstrap.Config{
			SupabaseURL:        e.config.SupabaseURL,
			SupabaseAnonKey:    e.config.SupabaseAnonKey,
			SupabaseServiceKey: e.config.SupabaseServiceKey,
			SupabaseJWTSecret:  e.config.SupabaseJWTSecret,
		}); err != nil {
			e.Stop(context.Background())
			return fmt.Errorf("init supabase: %w", err)
		}
		log.Printf("[Engine] Supabase initialized")
	}

	// Create service registry
	registry := base.NewRegistry()

	e.mu.Lock()
	e.registry = registry
	definitions := e.definitions
	e.mu.Unlock()

	// Register and create services
	log.Printf("[Engine] Registering %d services...", len(definitions))
	for _, def := range definitions {
		serviceOS, err := foundation.ServiceOS(def.Manifest, nil)
		if err != nil {
			log.Printf("[Engine] Failed to create ServiceOS for %s: %v", def.Manifest.ServiceID, err)
			continue
		}

		svc, err := def.Factory(serviceOS)
		if err != nil {
			log.Printf("[Engine] Failed to create service %s: %v", def.Manifest.ServiceID, err)
			continue
		}

		if err := registry.Register(svc); err != nil {
			log.Printf("[Engine] Failed to register service %s: %v", def.Manifest.ServiceID, err)
			continue
		}

		log.Printf("[Engine] Registered service: %s", def.Manifest.ServiceID)
	}

	// Start all services
	log.Printf("[Engine] Starting services...")
	if err := registry.StartAll(e.ctx); err != nil {
		log.Printf("[Engine] Warning: some services failed to start: %v", err)
	}

	// Log started services
	services := registry.List()
	runningCount := 0
	for _, svc := range services {
		if svc.State() == base.StateRunning {
			runningCount++
		}
	}
	log.Printf("[Engine] Started %d/%d services", runningCount, len(services))

	e.setState(StateRunning)
	return nil
}

// Stop gracefully stops the engine.
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	if e.state != StateRunning && e.state != StateStarting {
		e.mu.Unlock()
		return nil
	}
	registry := e.registry
	foundation := e.foundation
	cancel := e.cancel
	e.mu.Unlock()

	e.setState(StateStopping)

	// Cancel context
	if cancel != nil {
		cancel()
	}

	// Stop services
	if registry != nil {
		log.Printf("[Engine] Stopping services...")
		if err := registry.StopAll(ctx); err != nil {
			log.Printf("[Engine] Warning: error stopping services: %v", err)
		}
	}

	// Stop foundation
	if foundation != nil {
		log.Printf("[Engine] Stopping TEE Trust Root...")
		if err := foundation.Stop(ctx); err != nil {
			log.Printf("[Engine] Warning: error stopping foundation: %v", err)
		}
	}

	e.setState(StateStopped)
	log.Printf("[Engine] Stopped")
	return nil
}

// GetService returns a service by ID.
func (e *Engine) GetService(id string) (base.Service, bool) {
	e.mu.RLock()
	registry := e.registry
	e.mu.RUnlock()

	if registry == nil {
		return nil, false
	}
	return registry.Get(id)
}

// ListServices returns all registered services.
func (e *Engine) ListServices() []base.Service {
	e.mu.RLock()
	registry := e.registry
	e.mu.RUnlock()

	if registry == nil {
		return nil
	}
	return registry.List()
}

// Health checks the engine health.
func (e *Engine) Health(ctx context.Context) error {
	e.mu.RLock()
	state := e.state
	foundation := e.foundation
	e.mu.RUnlock()

	if state != StateRunning {
		return fmt.Errorf("engine not running: %s", state)
	}

	if foundation == nil {
		return fmt.Errorf("foundation not initialized")
	}

	return foundation.TrustRoot().Health(ctx)
}

// Stats returns engine statistics.
func (e *Engine) Stats() EngineStats {
	e.mu.RLock()
	registry := e.registry
	state := e.state
	e.mu.RUnlock()

	stats := EngineStats{
		State: state,
	}

	if registry != nil {
		services := registry.List()
		stats.TotalServices = len(services)
		for _, svc := range services {
			if svc.State() == base.StateRunning {
				stats.RunningServices++
			}
		}
	}

	return stats
}

// EngineStats holds engine statistics.
type EngineStats struct {
	State           State
	TotalServices   int
	RunningServices int
}
