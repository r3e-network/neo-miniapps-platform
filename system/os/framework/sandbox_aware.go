// Package framework provides sandbox-aware service infrastructure.
//
// This file extends the framework with sandbox integration, allowing services
// to operate within Android-style security sandboxes with capability-based
// access control.
package framework

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// =============================================================================
// Sandbox-Aware Interfaces
// =============================================================================

// SandboxContext provides sandbox security context to services.
// Services can use this to check capabilities and access sandbox features.
type SandboxContext interface {
	// Identity returns the service's security identity.
	Identity() *sandbox.ServiceIdentity

	// CheckCapability verifies the service has a specific capability.
	// Returns an error if the capability is not granted.
	CheckCapability(ctx context.Context, cap sandbox.Capability) error

	// HasCapability returns true if the service has the capability.
	HasCapability(cap sandbox.Capability) bool

	// SecurityLevel returns the service's security level.
	SecurityLevel() sandbox.SecurityLevel

	// ServiceID returns the service's unique identifier.
	ServiceID() string
}

// SandboxAware is implemented by services that need sandbox context.
// Services implementing this interface will receive sandbox context
// during initialization.
type SandboxAware interface {
	// SetSandboxContext provides the sandbox context to the service.
	SetSandboxContext(ctx SandboxContext)
}

// IPCAware is implemented by services that support inter-process communication.
// Services implementing this interface can make secure calls to other services.
type IPCAware interface {
	// SetIPCProxy provides the IPC proxy for inter-service calls.
	SetIPCProxy(proxy *sandbox.IPCProxy)
}

// IsolatedStorageAware is implemented by services that need isolated storage.
type IsolatedStorageAware interface {
	// SetIsolatedStorage provides sandbox-isolated storage to the service.
	SetIsolatedStorage(storage *sandbox.IsolatedStorage)
}

// =============================================================================
// Sandbox Context Implementation
// =============================================================================

// sandboxContextImpl wraps a ServiceSandbox to implement SandboxContext.
type sandboxContextImpl struct {
	sandbox *sandbox.ServiceSandbox
}

// NewSandboxContext creates a SandboxContext from a ServiceSandbox.
func NewSandboxContext(sb *sandbox.ServiceSandbox) SandboxContext {
	if sb == nil {
		return &nilSandboxContext{}
	}
	return &sandboxContextImpl{sandbox: sb}
}

func (c *sandboxContextImpl) Identity() *sandbox.ServiceIdentity {
	return c.sandbox.Identity
}

func (c *sandboxContextImpl) CheckCapability(ctx context.Context, cap sandbox.Capability) error {
	return c.sandbox.Context.CheckCapability(ctx, cap)
}

func (c *sandboxContextImpl) HasCapability(cap sandbox.Capability) bool {
	return c.sandbox.Caps.Has(cap)
}

func (c *sandboxContextImpl) SecurityLevel() sandbox.SecurityLevel {
	return c.sandbox.Identity.SecurityLevel
}

func (c *sandboxContextImpl) ServiceID() string {
	return c.sandbox.Identity.ServiceID
}

// =============================================================================
// Nil Sandbox Context (for non-sandboxed services)
// =============================================================================

// nilSandboxContext provides a no-op implementation for non-sandboxed services.
type nilSandboxContext struct{}

func (c *nilSandboxContext) Identity() *sandbox.ServiceIdentity {
	return &sandbox.ServiceIdentity{
		ServiceID:     "unknown",
		SecurityLevel: sandbox.SecurityLevelNormal,
	}
}

func (c *nilSandboxContext) CheckCapability(ctx context.Context, cap sandbox.Capability) error {
	// Non-sandboxed services have all capabilities (backward compatibility)
	return nil
}

func (c *nilSandboxContext) HasCapability(cap sandbox.Capability) bool {
	// Non-sandboxed services have all capabilities (backward compatibility)
	return true
}

func (c *nilSandboxContext) SecurityLevel() sandbox.SecurityLevel {
	return sandbox.SecurityLevelNormal
}

func (c *nilSandboxContext) ServiceID() string {
	return "unknown"
}

// NilSandboxContext returns a no-op sandbox context for non-sandboxed services.
func NilSandboxContext() SandboxContext {
	return &nilSandboxContext{}
}

// =============================================================================
// Extended Environment with Sandbox Support
// =============================================================================

// SandboxedEnvironment extends Environment with sandbox-specific features.
type SandboxedEnvironment struct {
	Environment

	// Sandbox context for security checks
	SandboxCtx SandboxContext

	// IPC proxy for inter-service communication
	IPC *sandbox.IPCProxy

	// Isolated storage for this service
	IsolatedStorage *sandbox.IsolatedStorage
}

// NewSandboxedEnvironment creates a SandboxedEnvironment from a ServiceSandbox.
func NewSandboxedEnvironment(env Environment, sb *sandbox.ServiceSandbox) SandboxedEnvironment {
	senv := SandboxedEnvironment{
		Environment: env,
		SandboxCtx:  NewSandboxContext(sb),
	}

	if sb != nil {
		senv.IPC = sb.IPC
		senv.IsolatedStorage = sb.Storage
	}

	return senv
}

// =============================================================================
// Sandbox-Aware Service Base
// =============================================================================

// SandboxedServiceBase extends ServiceBase with sandbox support.
// Embed this in services that need sandbox features.
type SandboxedServiceBase struct {
	ServiceBase

	sandboxCtx      SandboxContext
	ipc             *sandbox.IPCProxy
	isolatedStorage *sandbox.IsolatedStorage
}

// NewSandboxedServiceBase creates a new SandboxedServiceBase.
func NewSandboxedServiceBase(name, domain string) *SandboxedServiceBase {
	return &SandboxedServiceBase{
		ServiceBase: *NewServiceBase(name, domain),
		sandboxCtx:  NilSandboxContext(),
	}
}

// SetSandboxContext implements SandboxAware.
func (b *SandboxedServiceBase) SetSandboxContext(ctx SandboxContext) {
	b.sandboxCtx = ctx
}

// SetIPCProxy implements IPCAware.
func (b *SandboxedServiceBase) SetIPCProxy(proxy *sandbox.IPCProxy) {
	b.ipc = proxy
}

// SetIsolatedStorage implements IsolatedStorageAware.
func (b *SandboxedServiceBase) SetIsolatedStorage(storage *sandbox.IsolatedStorage) {
	b.isolatedStorage = storage
}

// SandboxContext returns the sandbox context.
func (b *SandboxedServiceBase) SandboxContext() SandboxContext {
	if b.sandboxCtx == nil {
		return NilSandboxContext()
	}
	return b.sandboxCtx
}

// IPC returns the IPC proxy for inter-service calls.
func (b *SandboxedServiceBase) IPC() *sandbox.IPCProxy {
	return b.ipc
}

// IsolatedStorage returns the isolated storage.
func (b *SandboxedServiceBase) IsolatedStorage() *sandbox.IsolatedStorage {
	return b.isolatedStorage
}

// CheckCapability verifies the service has a specific capability.
func (b *SandboxedServiceBase) CheckCapability(ctx context.Context, cap sandbox.Capability) error {
	return b.SandboxContext().CheckCapability(ctx, cap)
}

// HasCapability returns true if the service has the capability.
func (b *SandboxedServiceBase) HasCapability(cap sandbox.Capability) bool {
	return b.SandboxContext().HasCapability(cap)
}

// RequireCapability checks capability and returns a standardized error.
func (b *SandboxedServiceBase) RequireCapability(ctx context.Context, cap sandbox.Capability) error {
	if err := b.CheckCapability(ctx, cap); err != nil {
		return &sandbox.CapabilityDeniedError{
			ServiceID:  b.SandboxContext().ServiceID(),
			Capability: cap,
		}
	}
	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// InjectSandboxContext injects sandbox context into a service if it implements SandboxAware.
func InjectSandboxContext(service any, sb *sandbox.ServiceSandbox) {
	ctx := NewSandboxContext(sb)

	if aware, ok := service.(SandboxAware); ok {
		aware.SetSandboxContext(ctx)
	}

	if sb != nil {
		if ipcAware, ok := service.(IPCAware); ok {
			ipcAware.SetIPCProxy(sb.IPC)
		}

		if storageAware, ok := service.(IsolatedStorageAware); ok {
			storageAware.SetIsolatedStorage(sb.Storage)
		}
	}
}

// InjectSandboxedEnvironment injects a sandboxed environment into a service.
func InjectSandboxedEnvironment(service any, env SandboxedEnvironment) {
	// First inject the base environment
	if aware, ok := service.(EnvironmentAware); ok {
		aware.SetEnvironment(env.Environment)
	}

	// Then inject sandbox-specific components
	if aware, ok := service.(SandboxAware); ok {
		aware.SetSandboxContext(env.SandboxCtx)
	}

	if ipcAware, ok := service.(IPCAware); ok && env.IPC != nil {
		ipcAware.SetIPCProxy(env.IPC)
	}

	if storageAware, ok := service.(IsolatedStorageAware); ok && env.IsolatedStorage != nil {
		storageAware.SetIsolatedStorage(env.IsolatedStorage)
	}
}
