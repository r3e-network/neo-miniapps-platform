package service

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
)

// ServiceRegistry is the central registry for all services.
// Services self-register via init() functions, making the engine completely generic.
var ServiceRegistry = newRegistry()

// Registry holds all registered service factories and route handlers.
type Registry struct {
	mu             sync.RWMutex
	factories      map[string]ServiceFactory
	routeHandlers  map[string]RouteHandler
	actionHandlers map[string]ActionHandler
	order          []string // registration order for deterministic startup
}

func newRegistry() *Registry {
	return &Registry{
		factories:      make(map[string]ServiceFactory),
		routeHandlers:  make(map[string]RouteHandler),
		actionHandlers: make(map[string]ActionHandler),
	}
}

// ServiceFactory creates a service instance with its dependencies.
// Each service package implements this to handle its own initialization.
type ServiceFactory func(deps ServiceDependencies) (Service, error)

// ServiceDependencies provides common dependencies for service initialization.
// Services request only what they need from this interface.
type ServiceDependencies interface {
	// Database returns the database connection.
	Database() *sql.DB

	// Logger returns a logger for the service.
	Logger(name string) Logger

	// AccountChecker returns the account validation interface.
	// Returns nil if accounts service is not yet initialized.
	AccountChecker() AccountChecker

	// LookupService returns another service by name.
	// Used for inter-service dependencies.
	LookupService(name string) Service

	// Config returns service-specific configuration.
	Config(key string) string

	// HTTPClient returns a shared HTTP client.
	HTTPClient() *http.Client
}

// Service is the minimal interface all services must implement.
type Service interface {
	// Name returns the unique service identifier.
	Name() string

	// Start initializes the service.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service.
	Stop(ctx context.Context) error

	// Ready returns nil when the service is ready to handle requests.
	Ready(ctx context.Context) error
}

// RouteHandler registers HTTP routes for a service.
// Services implement this to expose their own API endpoints.
type RouteHandler interface {
	// RegisterRoutes adds the service's HTTP routes to the mux.
	// The basePath is typically "/accounts/{id}/<service-name>".
	RegisterRoutes(mux *http.ServeMux, basePath string)
}

// ActionHandler processes devpack actions for a service.
// Services implement this to handle function actions.
type ActionHandler interface {
	// ActionTypes returns the action types this handler supports.
	ActionTypes() []string

	// HandleAction processes a single action.
	HandleAction(ctx context.Context, accountID, actionType string, params map[string]any) (map[string]any, error)
}

// DescriptorProvider returns service metadata for introspection.
type DescriptorProvider interface {
	Descriptor() Descriptor
}

// Register adds a service factory to the registry.
// Called from service package init() functions.
func (r *Registry) Register(name string, factory ServiceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		panic("service already registered: " + name)
	}
	r.factories[name] = factory
	r.order = append(r.order, name)
}

// RegisterRouteHandler adds an HTTP route handler for a service.
func (r *Registry) RegisterRouteHandler(name string, handler RouteHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routeHandlers[name] = handler
}

// RegisterActionHandler adds an action handler for a service.
func (r *Registry) RegisterActionHandler(name string, handler ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actionHandlers[name] = handler
}

// Factories returns all registered service factories in registration order.
func (r *Registry) Factories() []NamedFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]NamedFactory, 0, len(r.order))
	for _, name := range r.order {
		if factory, ok := r.factories[name]; ok {
			result = append(result, NamedFactory{Name: name, Factory: factory})
		}
	}
	return result
}

// RouteHandlers returns all registered route handlers.
func (r *Registry) RouteHandlers() map[string]RouteHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]RouteHandler, len(r.routeHandlers))
	for k, v := range r.routeHandlers {
		result[k] = v
	}
	return result
}

// ActionHandlers returns all registered action handlers.
func (r *Registry) ActionHandlers() map[string]ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]ActionHandler, len(r.actionHandlers))
	for k, v := range r.actionHandlers {
		result[k] = v
	}
	return result
}

// NamedFactory pairs a service name with its factory.
type NamedFactory struct {
	Name    string
	Factory ServiceFactory
}

// RegisterService is a convenience function for service packages.
func RegisterService(name string, factory ServiceFactory) {
	ServiceRegistry.Register(name, factory)
}

// RegisterRoutes is a convenience function for service packages.
func RegisterRoutes(name string, handler RouteHandler) {
	ServiceRegistry.RegisterRouteHandler(name, handler)
}

// RegisterActions is a convenience function for service packages.
func RegisterActions(name string, handler ActionHandler) {
	ServiceRegistry.RegisterActionHandler(name, handler)
}

// Logger is a minimal logging interface for services.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
	WithField(key string, value any) Logger
	WithError(err error) Logger
}
