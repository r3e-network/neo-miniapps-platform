package framework

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/system/core"
	service "github.com/R3E-Network/service_layer/system/framework/core"
	"github.com/R3E-Network/service_layer/system/framework/lifecycle"
)

// APISurface is an alias for engine.APISurface for convenience.
type APISurface = engine.APISurface

// Descriptor is an alias for service.Descriptor for convenience.
type Descriptor = service.Descriptor

// ServiceBuilder provides a fluent API for constructing services.
// It reduces boilerplate and enforces consistent service structure.
type ServiceBuilder struct {
	name        string
	domain      string
	description string
	layer       string
	manifest    *Manifest
	hooks       *lifecycle.Hooks
	readyCheck  func(context.Context) error
	bus         BusClient

	// Lifecycle functions
	startFn func(context.Context) error
	stopFn  func(context.Context) error

	// Errors accumulated during building
	errs []error
}

// NewService creates a new ServiceBuilder with the given name and domain.
func NewService(name, domain string) *ServiceBuilder {
	return &ServiceBuilder{
		name:   name,
		domain: domain,
		layer:  "service",
		hooks:  lifecycle.NewHooks(),
		manifest: &Manifest{
			Name:   name,
			Domain: domain,
			Layer:  "service",
		},
	}
}

// WithDescription sets the service description.
func (b *ServiceBuilder) WithDescription(desc string) *ServiceBuilder {
	b.description = desc
	b.manifest.Description = desc
	return b
}

// WithLayer sets the service layer (service, runner, infra).
func (b *ServiceBuilder) WithLayer(layer string) *ServiceBuilder {
	b.layer = layer
	b.manifest.Layer = layer
	return b
}

// WithManifest sets a complete manifest (replaces the auto-generated one).
func (b *ServiceBuilder) WithManifest(m *Manifest) *ServiceBuilder {
	if m != nil {
		b.manifest = m
		// Sync name/domain if manifest has them
		if m.Name != "" {
			b.name = m.Name
		}
		if m.Domain != "" {
			b.domain = m.Domain
		}
	}
	return b
}

// WithCapabilities adds capabilities to the service manifest.
func (b *ServiceBuilder) WithCapabilities(caps ...string) *ServiceBuilder {
	b.manifest.Capabilities = append(b.manifest.Capabilities, caps...)
	return b
}

// DependsOn declares service dependencies.
func (b *ServiceBuilder) DependsOn(deps ...string) *ServiceBuilder {
	b.manifest.DependsOn = append(b.manifest.DependsOn, deps...)
	return b
}

// RequiresAPI declares required API surfaces.
func (b *ServiceBuilder) RequiresAPI(apis ...string) *ServiceBuilder {
	for _, api := range apis {
		b.manifest.RequiresAPIs = append(b.manifest.RequiresAPIs, APISurface(api))
	}
	return b
}

// WithQuotas sets service quotas.
func (b *ServiceBuilder) WithQuotas(quotas map[string]string) *ServiceBuilder {
	b.manifest.Quotas = quotas
	return b
}

// WithQuota sets a single quota key-value pair.
func (b *ServiceBuilder) WithQuota(key, value string) *ServiceBuilder {
	b.manifest.SetQuota(key, value)
	return b
}

// WithVersion sets the service version.
func (b *ServiceBuilder) WithVersion(version string) *ServiceBuilder {
	b.manifest.Version = version
	return b
}

// WithTags sets service tags for filtering and metadata.
func (b *ServiceBuilder) WithTags(tags map[string]string) *ServiceBuilder {
	b.manifest.Tags = tags
	return b
}

// WithTag sets a single tag key-value pair.
func (b *ServiceBuilder) WithTag(key, value string) *ServiceBuilder {
	b.manifest.SetTag(key, value)
	return b
}

// Enabled sets whether the service is enabled.
func (b *ServiceBuilder) Enabled(enabled bool) *ServiceBuilder {
	b.manifest.SetEnabled(enabled)
	return b
}

// MergeManifest merges another manifest into the builder's manifest.
// Useful for combining base manifests with overrides.
func (b *ServiceBuilder) MergeManifest(other *Manifest) *ServiceBuilder {
	if other != nil {
		b.manifest.Merge(other)
		// Sync name/domain if they changed
		if b.manifest.Name != "" {
			b.name = b.manifest.Name
		}
		if b.manifest.Domain != "" {
			b.domain = b.manifest.Domain
		}
	}
	return b
}

// WithValidator adds a custom manifest validator.
func (b *ServiceBuilder) WithValidator(v ManifestValidator) *ServiceBuilder {
	if v != nil {
		if err := v.ValidateManifest(b.manifest); err != nil {
			b.errs = append(b.errs, err)
		}
	}
	return b
}

// WithValidatorFunc adds a custom manifest validation function.
func (b *ServiceBuilder) WithValidatorFunc(fn func(*Manifest) error) *ServiceBuilder {
	if fn != nil {
		if err := fn(b.manifest); err != nil {
			b.errs = append(b.errs, err)
		}
	}
	return b
}

// OnPreStart adds a pre-start hook.
func (b *ServiceBuilder) OnPreStart(fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPreStart(fn)
	return b
}

// OnPreStartNamed adds a named pre-start hook.
func (b *ServiceBuilder) OnPreStartNamed(name string, fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPreStartNamed(name, fn)
	return b
}

// OnPostStart adds a post-start hook.
func (b *ServiceBuilder) OnPostStart(fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPostStart(fn)
	return b
}

// OnPostStartNamed adds a named post-start hook.
func (b *ServiceBuilder) OnPostStartNamed(name string, fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPostStartNamed(name, fn)
	return b
}

// OnPreStop adds a pre-stop hook.
func (b *ServiceBuilder) OnPreStop(fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPreStop(fn)
	return b
}

// OnPreStopNamed adds a named pre-stop hook.
func (b *ServiceBuilder) OnPreStopNamed(name string, fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPreStopNamed(name, fn)
	return b
}

// OnPostStop adds a post-stop hook.
func (b *ServiceBuilder) OnPostStop(fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPostStop(fn)
	return b
}

// OnPostStopNamed adds a named post-stop hook.
func (b *ServiceBuilder) OnPostStopNamed(name string, fn func(context.Context) error) *ServiceBuilder {
	b.hooks.OnPostStopNamed(name, fn)
	return b
}

// OnStart sets the main start function (runs after pre-start hooks).
func (b *ServiceBuilder) OnStart(fn func(context.Context) error) *ServiceBuilder {
	b.startFn = fn
	return b
}

// OnStop sets the main stop function (runs after pre-stop hooks).
func (b *ServiceBuilder) OnStop(fn func(context.Context) error) *ServiceBuilder {
	b.stopFn = fn
	return b
}

// WithReadyCheck sets a custom readiness check function.
func (b *ServiceBuilder) WithReadyCheck(fn func(context.Context) error) *ServiceBuilder {
	b.readyCheck = fn
	return b
}

// WithBus sets the bus client for the service.
func (b *ServiceBuilder) WithBus(bus BusClient) *ServiceBuilder {
	b.bus = bus
	return b
}

// Build creates the service. Returns an error if validation fails.
func (b *ServiceBuilder) Build() (*BuiltService, error) {
	// Validate required fields
	if b.name == "" {
		return nil, fmt.Errorf("%w: service name required", service.ErrInvalidManifest)
	}
	if b.domain == "" {
		return nil, fmt.Errorf("%w: service domain required", service.ErrInvalidManifest)
	}

	// Normalize and validate manifest
	b.manifest.Name = b.name
	b.manifest.Domain = b.domain
	b.manifest.Normalize()

	if err := b.manifest.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrInvalidManifest, err)
	}

	// Check for accumulated errors
	if len(b.errs) > 0 {
		return nil, fmt.Errorf("builder errors: %v", b.errs)
	}

	svc := &BuiltService{
		ServiceBase: *NewServiceBase(b.name, b.domain),
		manifest:    b.manifest,
		hooks:       b.hooks,
		startFn:     b.startFn,
		stopFn:      b.stopFn,
		readyCheck:  b.readyCheck,
		bus:         b.bus,
		shutdown:    lifecycle.NewGracefulShutdown(),
	}

	return svc, nil
}

// MustBuild creates the service or panics on error.
// Use in init functions or tests where errors are not expected.
func (b *ServiceBuilder) MustBuild() *BuiltService {
	svc, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build service %q: %v", b.name, err))
	}
	return svc
}

// BuiltService is a service created by ServiceBuilder.
// It implements the ServiceModule interface expected by the engine.
type BuiltService struct {
	ServiceBase

	manifest   *Manifest
	hooks      *lifecycle.Hooks
	startFn    func(context.Context) error
	stopFn     func(context.Context) error
	readyCheck func(context.Context) error
	bus        BusClient
	shutdown   *lifecycle.GracefulShutdown

	started bool
}

// Manifest returns the service manifest.
func (s *BuiltService) Manifest() *Manifest {
	return s.manifest
}

// Start starts the service with proper hook execution.
func (s *BuiltService) Start(ctx context.Context) error {
	if s.started {
		return service.ErrServiceAlreadyStarted
	}

	// Run pre-start hooks
	if err := s.hooks.RunPreStart(ctx); err != nil {
		return service.NewHookError(s.Name(), "PreStart", err)
	}

	// Run main start function if provided
	if s.startFn != nil {
		if err := s.startFn(ctx); err != nil {
			return service.WrapServiceError(s.Name(), "start", err)
		}
	}

	// Mark as ready and started
	s.MarkReady(true)
	s.started = true

	// Run post-start hooks
	if err := s.hooks.RunPostStart(ctx); err != nil {
		// Still started, but log the hook error
		return service.NewHookError(s.Name(), "PostStart", err)
	}

	return nil
}

// Stop stops the service with proper hook execution.
func (s *BuiltService) Stop(ctx context.Context) error {
	if !s.started {
		return nil // Already stopped, not an error
	}

	// Initiate graceful shutdown
	s.shutdown.Shutdown()

	// Run pre-stop hooks
	if err := s.hooks.RunPreStop(ctx); err != nil {
		return service.NewHookError(s.Name(), "PreStop", err)
	}

	// Mark as not ready
	s.MarkReady(false)

	// Run main stop function if provided
	if s.stopFn != nil {
		if err := s.stopFn(ctx); err != nil {
			return service.WrapServiceError(s.Name(), "stop", err)
		}
	}

	s.started = false

	// Run post-stop hooks (in reverse order)
	if err := s.hooks.RunPostStop(ctx); err != nil {
		return service.NewHookError(s.Name(), "PostStop", err)
	}

	return nil
}

// Ready checks if the service is ready.
func (s *BuiltService) Ready(ctx context.Context) error {
	// Check base readiness first
	if err := s.ServiceBase.Ready(ctx); err != nil {
		return err
	}

	// Run custom ready check if provided
	if s.readyCheck != nil {
		return s.readyCheck(ctx)
	}

	return nil
}

// Bus returns the service's bus client.
func (s *BuiltService) Bus() BusClient {
	return s.bus
}

// Hooks returns the service's lifecycle hooks.
func (s *BuiltService) Hooks() *lifecycle.Hooks {
	return s.hooks
}

// Shutdown returns the graceful shutdown coordinator.
func (s *BuiltService) Shutdown() *lifecycle.GracefulShutdown {
	return s.shutdown
}

// IsStarted returns true if the service has been started.
func (s *BuiltService) IsStarted() bool {
	return s.started
}

// Descriptor returns the service descriptor for engine integration.
func (s *BuiltService) Descriptor() Descriptor {
	if s.manifest == nil {
		return Descriptor{}
	}
	return s.manifest.ToDescriptor()
}

// IsEnabled returns whether the service is enabled.
func (s *BuiltService) IsEnabled() bool {
	if s.manifest == nil {
		return true
	}
	return s.manifest.IsEnabled()
}

// Version returns the service version.
func (s *BuiltService) Version() string {
	if s.manifest == nil {
		return ""
	}
	return s.manifest.Version
}

// Description returns the service description.
func (s *BuiltService) Description() string {
	if s.manifest == nil {
		return ""
	}
	return s.manifest.Description
}
