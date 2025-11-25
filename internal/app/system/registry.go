package system

import (
	"context"
	"fmt"
	"sync"

	core "github.com/R3E-Network/service_layer/internal/services/core"
)

// ServiceFactory creates a service instance given dependencies.
type ServiceFactory func(deps ServiceDeps) (Service, error)

// ServiceDeps provides dependencies for service creation.
// Implementations should embed this and add domain-specific fields.
type ServiceDeps interface {
	Logger() any
	Stores() any
}

// ServiceEntry represents a registered service factory with metadata.
type ServiceEntry struct {
	Name        string
	Domain      string
	Factory     ServiceFactory
	Priority    int  // Lower values start first
	Required    bool // If true, failure to create/start is fatal
	AutoStart   bool // If true, automatically start with manager
	Descriptor  *core.Descriptor
	DependsOn   []string
}

// ServiceRegistry maintains a collection of service factories that can be
// instantiated and registered with a Manager. This allows services to
// self-register via init() functions, eliminating hardcoded service lists.
type ServiceRegistry struct {
	mu      sync.RWMutex
	entries map[string]*ServiceEntry
	order   []string // maintains registration order
}

// GlobalRegistry is the default registry used for service auto-registration.
var GlobalRegistry = NewServiceRegistry()

// NewServiceRegistry creates a new empty service registry.
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		entries: make(map[string]*ServiceEntry),
		order:   make([]string, 0),
	}
}

// Register adds a service factory to the registry.
// Services are identified by name; duplicate registration returns an error.
func (r *ServiceRegistry) Register(entry ServiceEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("service entry requires a name")
	}
	if entry.Factory == nil {
		return fmt.Errorf("service %q requires a factory function", entry.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.Name]; exists {
		return fmt.Errorf("service %q already registered", entry.Name)
	}

	r.entries[entry.Name] = &entry
	r.order = append(r.order, entry.Name)
	return nil
}

// MustRegister is like Register but panics on error.
// Useful for init() functions.
func (r *ServiceRegistry) MustRegister(entry ServiceEntry) {
	if err := r.Register(entry); err != nil {
		panic(fmt.Sprintf("service registration failed: %v", err))
	}
}

// Get retrieves a service entry by name.
func (r *ServiceRegistry) Get(name string) (*ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	return entry, ok
}

// List returns all registered service entries in registration order.
func (r *ServiceRegistry) List() []*ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ServiceEntry, 0, len(r.order))
	for _, name := range r.order {
		if entry, ok := r.entries[name]; ok {
			result = append(result, entry)
		}
	}
	return result
}

// ListByPriority returns entries sorted by priority (lower first).
func (r *ServiceRegistry) ListByPriority() []*ServiceEntry {
	entries := r.List()
	// Simple insertion sort - typically small number of services
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j-1].Priority > entries[j].Priority; j-- {
			entries[j-1], entries[j] = entries[j], entries[j-1]
		}
	}
	return entries
}

// Names returns the names of all registered services.
func (r *ServiceRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Count returns the number of registered services.
func (r *ServiceRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Unregister removes a service from the registry.
func (r *ServiceRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.entries[name]; !ok {
		return fmt.Errorf("service %q not found", name)
	}

	delete(r.entries, name)

	// Remove from order slice
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

// CreateServices instantiates all registered services using the provided
// dependencies and returns them in priority order.
func (r *ServiceRegistry) CreateServices(deps ServiceDeps) ([]Service, error) {
	entries := r.ListByPriority()
	services := make([]Service, 0, len(entries))

	for _, entry := range entries {
		if !entry.AutoStart {
			continue
		}

		svc, err := entry.Factory(deps)
		if err != nil {
			if entry.Required {
				return nil, fmt.Errorf("create required service %q: %w", entry.Name, err)
			}
			// Log warning but continue for optional services
			continue
		}

		services = append(services, svc)
	}

	return services, nil
}

// RegisterWithManager creates all auto-start services and registers them
// with the provided manager.
func (r *ServiceRegistry) RegisterWithManager(mgr *Manager, deps ServiceDeps) error {
	services, err := r.CreateServices(deps)
	if err != nil {
		return err
	}

	for _, svc := range services {
		if err := mgr.Register(svc); err != nil {
			return fmt.Errorf("register service %q: %w", svc.Name(), err)
		}
	}

	return nil
}

// CollectDescriptors returns descriptors for all registered services
// that provide them.
func (r *ServiceRegistry) CollectDescriptors() []core.Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]core.Descriptor, 0)
	for _, name := range r.order {
		entry := r.entries[name]
		if entry.Descriptor != nil {
			descriptors = append(descriptors, *entry.Descriptor)
		}
	}

	return SortDescriptors(descriptors)
}

// Clear removes all entries from the registry. Useful for testing.
func (r *ServiceRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = make(map[string]*ServiceEntry)
	r.order = make([]string, 0)
}

// ServiceBuilder provides a fluent API for building ServiceEntry.
type ServiceBuilder struct {
	entry ServiceEntry
}

// NewServiceBuilder creates a new service entry builder.
func NewServiceBuilder(name string) *ServiceBuilder {
	return &ServiceBuilder{
		entry: ServiceEntry{
			Name:      name,
			AutoStart: true,
			Required:  true,
		},
	}
}

// Domain sets the service domain.
func (b *ServiceBuilder) Domain(domain string) *ServiceBuilder {
	b.entry.Domain = domain
	return b
}

// Factory sets the service factory function.
func (b *ServiceBuilder) Factory(fn ServiceFactory) *ServiceBuilder {
	b.entry.Factory = fn
	return b
}

// Priority sets the start priority (lower starts first).
func (b *ServiceBuilder) Priority(p int) *ServiceBuilder {
	b.entry.Priority = p
	return b
}

// Required marks the service as required for startup.
func (b *ServiceBuilder) Required(r bool) *ServiceBuilder {
	b.entry.Required = r
	return b
}

// AutoStart sets whether the service starts automatically.
func (b *ServiceBuilder) AutoStart(a bool) *ServiceBuilder {
	b.entry.AutoStart = a
	return b
}

// DependsOn sets service dependencies.
func (b *ServiceBuilder) DependsOn(deps ...string) *ServiceBuilder {
	b.entry.DependsOn = deps
	return b
}

// WithDescriptor sets the service descriptor.
func (b *ServiceBuilder) WithDescriptor(d core.Descriptor) *ServiceBuilder {
	b.entry.Descriptor = &d
	return b
}

// Build returns the constructed ServiceEntry.
func (b *ServiceBuilder) Build() ServiceEntry {
	return b.entry
}

// Register builds and registers the entry with GlobalRegistry.
func (b *ServiceBuilder) Register() error {
	return GlobalRegistry.Register(b.Build())
}

// MustRegister builds and registers, panicking on error.
func (b *ServiceBuilder) MustRegister() {
	GlobalRegistry.MustRegister(b.Build())
}

// ServiceLookup provides read-only access to instantiated services.
type ServiceLookup interface {
	GetService(name string) (Service, bool)
	GetServiceAs(name string, target any) bool
	ListServices() []Service
}

// ServiceContainer holds instantiated services and provides lookup.
type ServiceContainer struct {
	mu       sync.RWMutex
	services map[string]Service
	ordered  []Service
}

// NewServiceContainer creates an empty service container.
func NewServiceContainer() *ServiceContainer {
	return &ServiceContainer{
		services: make(map[string]Service),
		ordered:  make([]Service, 0),
	}
}

// Add registers a service instance in the container.
func (c *ServiceContainer) Add(svc Service) error {
	if svc == nil {
		return fmt.Errorf("cannot add nil service")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	name := svc.Name()
	if _, exists := c.services[name]; exists {
		return fmt.Errorf("service %q already exists", name)
	}

	c.services[name] = svc
	c.ordered = append(c.ordered, svc)
	return nil
}

// GetService retrieves a service by name.
func (c *ServiceContainer) GetService(name string) (Service, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	svc, ok := c.services[name]
	return svc, ok
}

// GetServiceAs retrieves and type-asserts a service.
// Returns false if service not found or type assertion fails.
func (c *ServiceContainer) GetServiceAs(name string, target any) bool {
	svc, ok := c.GetService(name)
	if !ok {
		return false
	}

	// Use type switch for common service types
	switch t := target.(type) {
	case *Service:
		*t = svc
		return true
	case *LifecycleService:
		if ls, ok := svc.(LifecycleService); ok {
			*t = ls
			return true
		}
	}
	return false
}

// ListServices returns all services in add order.
func (c *ServiceContainer) ListServices() []Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Service, len(c.ordered))
	copy(result, c.ordered)
	return result
}

// StartAll starts all services in order, rolling back on failure.
func (c *ServiceContainer) StartAll(ctx context.Context) error {
	c.mu.RLock()
	services := append([]Service(nil), c.ordered...)
	c.mu.RUnlock()

	for idx, svc := range services {
		if err := svc.Start(ctx); err != nil {
			// Rollback already started
			for i := idx - 1; i >= 0; i-- {
				_ = services[i].Stop(ctx)
			}
			return fmt.Errorf("start %s: %w", svc.Name(), err)
		}
	}
	return nil
}

// StopAll stops all services in reverse order.
func (c *ServiceContainer) StopAll(ctx context.Context) error {
	c.mu.RLock()
	services := append([]Service(nil), c.ordered...)
	c.mu.RUnlock()

	var firstErr error
	for i := len(services) - 1; i >= 0; i-- {
		if err := services[i].Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stop %s: %w", services[i].Name(), err)
		}
	}
	return firstErr
}
