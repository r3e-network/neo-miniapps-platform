// Package framework provides sandbox-aware service engine infrastructure.
//
// This file extends ServiceEngine with Android-style sandbox integration,
// providing capability-based access control and isolated storage.
package framework

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// =============================================================================
// Sandboxed Service Engine
// =============================================================================

// SandboxedServiceConfig extends ServiceConfig with sandbox settings.
type SandboxedServiceConfig struct {
	ServiceConfig

	// Sandbox configuration
	SecurityLevel         sandbox.SecurityLevel
	RequestedCapabilities []sandbox.Capability
	StorageQuota          int64
}

// SandboxedServiceEngine extends ServiceEngine with sandbox support.
// It provides capability-based access control and isolated storage.
//
// Example usage:
//
//	type MyService struct {
//	    *framework.SandboxedServiceEngine
//	    store Store
//	}
//
//	func New(accounts AccountChecker, store Store, log *logger.Logger) *MyService {
//	    return &MyService{
//	        SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
//	            ServiceConfig: framework.ServiceConfig{
//	                Name:        "myservice",
//	                Description: "My service description",
//	                Accounts:    accounts,
//	                Logger:      log,
//	            },
//	            SecurityLevel: sandbox.SecurityLevelNormal,
//	            RequestedCapabilities: []sandbox.Capability{
//	                sandbox.CapStorageRead,
//	                sandbox.CapStorageWrite,
//	            },
//	        }),
//	        store: store,
//	    }
//	}
type SandboxedServiceEngine struct {
	*ServiceEngine

	// Sandbox components
	sandboxCtx      SandboxContext
	ipc             *sandbox.IPCProxy
	isolatedStorage *sandbox.IsolatedStorage

	// Sandbox configuration
	securityLevel         sandbox.SecurityLevel
	requestedCapabilities []sandbox.Capability
	storageQuota          int64
}

// NewSandboxedServiceEngine creates a sandboxed service engine.
func NewSandboxedServiceEngine(cfg SandboxedServiceConfig) *SandboxedServiceEngine {
	eng := &SandboxedServiceEngine{
		ServiceEngine:         NewServiceEngine(cfg.ServiceConfig),
		sandboxCtx:            NilSandboxContext(),
		securityLevel:         cfg.SecurityLevel,
		requestedCapabilities: cfg.RequestedCapabilities,
		storageQuota:          cfg.StorageQuota,
	}

	// Set default security level if not specified (0 is SecurityLevelUntrusted)
	// We use SecurityLevelNormal as the default for services
	if cfg.SecurityLevel == 0 {
		eng.securityLevel = sandbox.SecurityLevelNormal
	}

	return eng
}

// =============================================================================
// SandboxAware Interface Implementation
// =============================================================================

// SetSandboxContext implements SandboxAware.
func (e *SandboxedServiceEngine) SetSandboxContext(ctx SandboxContext) {
	if ctx != nil {
		e.sandboxCtx = ctx
	} else {
		e.sandboxCtx = NilSandboxContext()
	}
}

// SetIPCProxy implements IPCAware.
func (e *SandboxedServiceEngine) SetIPCProxy(proxy *sandbox.IPCProxy) {
	e.ipc = proxy
}

// SetIsolatedStorage implements IsolatedStorageAware.
func (e *SandboxedServiceEngine) SetIsolatedStorage(storage *sandbox.IsolatedStorage) {
	e.isolatedStorage = storage
}

// =============================================================================
// Sandbox Accessors
// =============================================================================

// SandboxContext returns the sandbox context.
func (e *SandboxedServiceEngine) SandboxContext() SandboxContext {
	if e.sandboxCtx == nil {
		return NilSandboxContext()
	}
	return e.sandboxCtx
}

// IPC returns the IPC proxy for inter-service calls.
func (e *SandboxedServiceEngine) IPC() *sandbox.IPCProxy {
	return e.ipc
}

// IsolatedStorage returns the isolated storage.
func (e *SandboxedServiceEngine) IsolatedStorage() *sandbox.IsolatedStorage {
	return e.isolatedStorage
}

// SecurityLevel returns the configured security level.
func (e *SandboxedServiceEngine) SecurityLevel() sandbox.SecurityLevel {
	return e.securityLevel
}

// RequestedCapabilities returns the capabilities requested by this service.
func (e *SandboxedServiceEngine) RequestedCapabilities() []sandbox.Capability {
	return e.requestedCapabilities
}

// StorageQuota returns the configured storage quota.
func (e *SandboxedServiceEngine) StorageQuota() int64 {
	return e.storageQuota
}

// =============================================================================
// Capability Checking Methods
// =============================================================================

// CheckCapability verifies the service has a specific capability.
// Returns an error if the capability is not granted.
func (e *SandboxedServiceEngine) CheckCapability(ctx context.Context, cap sandbox.Capability) error {
	return e.SandboxContext().CheckCapability(ctx, cap)
}

// HasCapability returns true if the service has the capability.
func (e *SandboxedServiceEngine) HasCapability(cap sandbox.Capability) bool {
	return e.SandboxContext().HasCapability(cap)
}

// RequireCapability checks capability and returns a standardized error.
// Use this at the start of methods that require specific capabilities.
func (e *SandboxedServiceEngine) RequireCapability(ctx context.Context, cap sandbox.Capability) error {
	if err := e.CheckCapability(ctx, cap); err != nil {
		return &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: cap,
		}
	}
	return nil
}

// RequireCapabilities checks multiple capabilities at once.
func (e *SandboxedServiceEngine) RequireCapabilities(ctx context.Context, caps ...sandbox.Capability) error {
	for _, cap := range caps {
		if err := e.RequireCapability(ctx, cap); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// Sandboxed Operation Methods
// =============================================================================

// BeginSandboxedOperation starts an observed operation with capability checking.
// This extends BeginOperation with sandbox capability verification.
func (e *SandboxedServiceEngine) BeginSandboxedOperation(
	ctx context.Context,
	accountID, resource, operation string,
	requiredCaps ...sandbox.Capability,
) (*OperationContext, error) {
	// First check capabilities
	for _, cap := range requiredCaps {
		if err := e.RequireCapability(ctx, cap); err != nil {
			return nil, err
		}
	}

	// Then proceed with normal operation
	return e.BeginOperation(ctx, accountID, resource, operation)
}

// RunSandboxedOperation executes an operation with capability checking.
func (e *SandboxedServiceEngine) RunSandboxedOperation(
	ctx context.Context,
	accountID, resource, operation string,
	requiredCaps []sandbox.Capability,
	op func(context.Context) error,
) error {
	opCtx, err := e.BeginSandboxedOperation(ctx, accountID, resource, operation, requiredCaps...)
	if err != nil {
		return err
	}
	err = op(opCtx.Ctx)
	opCtx.Finish(err)
	return err
}

// =============================================================================
// Isolated Storage Methods
// =============================================================================

// StorageGet retrieves a value from isolated storage.
// Requires CapStorageRead capability.
func (e *SandboxedServiceEngine) StorageGet(ctx context.Context, key string) ([]byte, error) {
	if err := e.RequireCapability(ctx, sandbox.CapStorageRead); err != nil {
		return nil, err
	}
	if e.isolatedStorage == nil {
		return nil, &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: sandbox.CapStorageRead,
		}
	}
	return e.isolatedStorage.Get(ctx, key)
}

// StorageSet stores a value in isolated storage.
// Requires CapStorageWrite capability.
func (e *SandboxedServiceEngine) StorageSet(ctx context.Context, key string, value []byte) error {
	if err := e.RequireCapability(ctx, sandbox.CapStorageWrite); err != nil {
		return err
	}
	if e.isolatedStorage == nil {
		return &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: sandbox.CapStorageWrite,
		}
	}
	return e.isolatedStorage.Set(ctx, key, value)
}

// StorageDelete removes a value from isolated storage.
// Requires CapStorageDelete capability.
func (e *SandboxedServiceEngine) StorageDelete(ctx context.Context, key string) error {
	if err := e.RequireCapability(ctx, sandbox.CapStorageDelete); err != nil {
		return err
	}
	if e.isolatedStorage == nil {
		return &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: sandbox.CapStorageDelete,
		}
	}
	return e.isolatedStorage.Delete(ctx, key)
}

// StorageList lists keys in isolated storage with optional prefix.
// Requires CapStorageRead capability.
func (e *SandboxedServiceEngine) StorageList(ctx context.Context, prefix string) ([]string, error) {
	if err := e.RequireCapability(ctx, sandbox.CapStorageRead); err != nil {
		return nil, err
	}
	if e.isolatedStorage == nil {
		return nil, &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: sandbox.CapStorageRead,
		}
	}
	return e.isolatedStorage.List(ctx, prefix)
}

// =============================================================================
// IPC Methods
// =============================================================================

// CallService makes a secure IPC call to another service.
// Requires CapServiceCall capability.
func (e *SandboxedServiceEngine) CallService(ctx context.Context, targetService, method string, args any) (any, error) {
	if err := e.RequireCapability(ctx, sandbox.CapServiceCall); err != nil {
		return nil, err
	}
	if e.ipc == nil {
		return nil, &sandbox.CapabilityDeniedError{
			ServiceID:  e.SandboxContext().ServiceID(),
			Capability: sandbox.CapServiceCall,
		}
	}
	return e.ipc.Call(ctx, targetService, method, args)
}

// =============================================================================
// Sandboxed Bus Methods (Override parent methods with capability checks)
// =============================================================================

// PublishEventSecure publishes an event with capability checking.
// Requires CapBusPublish capability.
func (e *SandboxedServiceEngine) PublishEventSecure(ctx context.Context, event string, payload any) error {
	if err := e.RequireCapability(ctx, sandbox.CapBusPublish); err != nil {
		return err
	}
	return e.ServiceEngine.PublishEvent(ctx, event, payload)
}

// PushDataSecure pushes data with capability checking.
// Requires CapBusPublish capability.
func (e *SandboxedServiceEngine) PushDataSecure(ctx context.Context, topic string, payload any) error {
	if err := e.RequireCapability(ctx, sandbox.CapBusPublish); err != nil {
		return err
	}
	return e.ServiceEngine.PushData(ctx, topic, payload)
}

// =============================================================================
// Factory Functions
// =============================================================================

// NewSandboxedServiceEngineSimple creates a sandboxed service engine with minimal config.
func NewSandboxedServiceEngineSimple(name, description string, log *logger.Logger) *SandboxedServiceEngine {
	return NewSandboxedServiceEngine(SandboxedServiceConfig{
		ServiceConfig: ServiceConfig{
			Name:        name,
			Description: description,
			Logger:      log,
		},
		SecurityLevel: sandbox.SecurityLevelNormal,
		RequestedCapabilities: []sandbox.Capability{
			sandbox.CapStorageRead,
			sandbox.CapStorageWrite,
			sandbox.CapBusPublish,
			sandbox.CapBusSubscribe,
		},
	})
}

// =============================================================================
// Sandbox Integration Helper
// =============================================================================

// InjectSandbox injects sandbox components into a SandboxedServiceEngine.
// This is called by the runtime when initializing services.
func (e *SandboxedServiceEngine) InjectSandbox(sb *sandbox.ServiceSandbox) {
	if sb == nil {
		e.sandboxCtx = NilSandboxContext()
		e.ipc = nil
		e.isolatedStorage = nil
		return
	}

	e.sandboxCtx = NewSandboxContext(sb)
	e.ipc = sb.IPC
	e.isolatedStorage = sb.Storage
}

// CreateSandboxRequest returns a sandbox creation request for this service.
// Used by the runtime to create the sandbox for this service.
func (e *SandboxedServiceEngine) CreateSandboxRequest() sandbox.CreateSandboxRequest {
	return sandbox.CreateSandboxRequest{
		ServiceID:             e.Name(),
		PackageID:             e.Domain(),
		SecurityLevel:         e.securityLevel,
		RequestedCapabilities: e.requestedCapabilities,
		StorageQuota:          e.storageQuota,
	}
}
