// Package engine provides the Service Engine (OS Core) for orchestrating service modules.
// It handles module registration, lifecycle management, health monitoring, and inter-service communication.
//
// The engine is structured into several components:
//   - Registry: Module registration and lookup
//   - LifecycleManager: Start/stop orchestration
//   - HealthMonitor: Health and readiness tracking
//   - DependencyManager: Dependency resolution and ordering
//   - Bus: Event publishing, data pushing, compute invocation
//   - BusPermissionManager: Bus permission control
//   - MetadataManager: Module notes, capabilities, quotas
package engine

import (
	"context"
	"log"
	"strings"
)

// Engine is the lightweight core orchestrator. It holds a registry of modules and drives lifecycle.
// This is the main facade that composes all engine subsystems.
type Engine struct {
	// Core subsystems
	registry  *Registry
	lifecycle *LifecycleManager
	health    *HealthMonitor
	deps      *DependencyManager
	bus       *Bus
	perms     *BusPermissionManager
	metadata  *MetadataManager

	// Configuration
	log *log.Logger
}

// New returns an empty Engine ready to accept modules.
func New(opts ...Option) *Engine {
	e := &Engine{
		registry: NewRegistry(),
		health:   NewHealthMonitor(),
		deps:     NewDependencyManager(),
		perms:    NewBusPermissionManager(),
		metadata: NewMetadataManager(),
		log:      log.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Wire up subsystems
	e.registry.SetHealthMonitor(e.health)
	e.bus = NewBus(e.registry, e.perms)
	e.lifecycle = NewLifecycleManager(e.registry, e.deps, e.health, e.log)

	return e
}

// =============================================================================
// Module Registration (delegates to Registry)
// =============================================================================

// Register adds a service module to the engine. Names must be unique.
func (e *Engine) Register(module ServiceModule) error {
	return e.registry.Register(module)
}

// Unregister removes a module from the engine and cleans up all associated data.
// This includes health data, metadata, and bus permissions to prevent memory leaks.
func (e *Engine) Unregister(name string) error {
	// Clean up metadata
	if e.metadata != nil {
		e.metadata.RemoveModule(name)
	}

	// Clean up permissions
	if e.perms != nil {
		e.perms.RemovePermissions(name)
	}

	// Clean up dependencies
	if e.deps != nil {
		e.deps.RemoveDeps(name)
	}

	// Unregister from registry (this also cleans up health via registry.health)
	return e.registry.Unregister(name)
}

// Modules returns the registered module names (sorted).
func (e *Engine) Modules() []string {
	return e.registry.Modules()
}

// Lookup returns a module by name, if registered.
func (e *Engine) Lookup(name string) ServiceModule {
	return e.registry.Lookup(name)
}

// ModulesByDomain returns modules matching the provided domain.
func (e *Engine) ModulesByDomain(domain string) []ServiceModule {
	return e.registry.ModulesByDomain(domain)
}

// =============================================================================
// Engine Type Accessors (delegates to Registry)
// =============================================================================

// AccountEngines returns registered account engines.
func (e *Engine) AccountEngines() []AccountEngine {
	return e.registry.AccountEngines()
}

// StoreEngines returns registered store engines.
func (e *Engine) StoreEngines() []StoreEngine {
	return e.registry.StoreEngines()
}

// ComputeEngines returns registered compute engines.
func (e *Engine) ComputeEngines() []ComputeEngine {
	return e.registry.ComputeEnginesWithPerms(e.perms.GetPermissions)
}

// DataEngines returns registered data engines.
func (e *Engine) DataEngines() []DataEngine {
	return e.registry.DataEnginesWithPerms(e.perms.GetPermissions)
}

// EventEngines returns registered event engines.
func (e *Engine) EventEngines() []EventEngine {
	return e.registry.EventEnginesWithPerms(e.perms.GetPermissions)
}

// LedgerEngines returns registered ledger engines.
func (e *Engine) LedgerEngines() []LedgerEngine {
	return e.registry.LedgerEngines()
}

// IndexerEngines returns registered indexer engines.
func (e *Engine) IndexerEngines() []IndexerEngine {
	return e.registry.IndexerEngines()
}

// RPCEngines returns registered chain RPC hubs.
func (e *Engine) RPCEngines() []RPCEngine {
	return e.registry.RPCEngines()
}

// DataSourceEngines returns registered data source hubs.
func (e *Engine) DataSourceEngines() []DataSourceEngine {
	return e.registry.DataSourceEngines()
}

// ContractsEngines returns registered contract managers.
func (e *Engine) ContractsEngines() []ContractsEngine {
	return e.registry.ContractsEngines()
}

// ServiceBankEngines returns registered service-owned GAS controllers.
func (e *Engine) ServiceBankEngines() []ServiceBankEngine {
	return e.registry.ServiceBankEngines()
}

// CryptoEngines returns registered crypto helpers.
func (e *Engine) CryptoEngines() []CryptoEngine {
	return e.registry.CryptoEngines()
}

// =============================================================================
// Lifecycle Management (delegates to LifecycleManager)
// =============================================================================

// Start walks registered modules in registration order.
func (e *Engine) Start(ctx context.Context) error {
	return e.lifecycle.Start(ctx)
}

// Stop walks registered modules in reverse registration order.
func (e *Engine) Stop(ctx context.Context) error {
	return e.lifecycle.Stop(ctx)
}

// MarkStarted records a started status for the given module names.
func (e *Engine) MarkStarted(names ...string) {
	e.lifecycle.MarkStarted(names...)
}

// MarkStopped records a stopped status for the given module names.
func (e *Engine) MarkStopped(names ...string) {
	e.lifecycle.MarkStopped(names...)
}

// MarkReady updates readiness for the provided modules.
func (e *Engine) MarkReady(status, errMsg string, names ...string) {
	e.lifecycle.MarkReady(status, errMsg, names...)
}

// ProbeReadiness runs lightweight readiness checks for modules that implement ReadyChecker.
func (e *Engine) ProbeReadiness(ctx context.Context) {
	e.lifecycle.ProbeReadiness(ctx)
}

// =============================================================================
// Health Monitoring (delegates to HealthMonitor)
// =============================================================================

// ModulesHealth returns the latest known lifecycle state per module (ordered).
func (e *Engine) ModulesHealth() []ModuleHealth {
	return e.health.ModulesHealth(e.registry.Modules())
}

// =============================================================================
// Bus Operations (delegates to Bus)
// =============================================================================

// SubscribeEvent registers a handler for an event across all event engines and the in-process bus.
func (e *Engine) SubscribeEvent(ctx context.Context, event string, handler EventHandler) error {
	return e.bus.SubscribeEvent(ctx, event, handler)
}

// PublishEvent fan-outs an event to all registered EventEngines plus local subscribers.
func (e *Engine) PublishEvent(ctx context.Context, event string, payload any) error {
	return e.bus.PublishEvent(ctx, event, payload)
}

// PushData dispatches a payload to every registered DataEngine.
func (e *Engine) PushData(ctx context.Context, topic string, payload any) error {
	return e.bus.PushData(ctx, topic, payload)
}

// InvokeComputeAll invokes every registered ComputeEngine with the provided payload.
func (e *Engine) InvokeComputeAll(ctx context.Context, payload any) ([]InvokeResult, error) {
	return e.bus.InvokeComputeAll(ctx, payload)
}

// =============================================================================
// Dependency Management (delegates to DependencyManager)
// =============================================================================

// SetModuleDeps records dependencies for a module.
func (e *Engine) SetModuleDeps(name string, deps ...string) {
	e.deps.SetDeps(name, deps...)
}

// =============================================================================
// Permission Management (delegates to PermissionManager)
// =============================================================================

// SetBusPermissions overrides bus permissions for a module.
func (e *Engine) SetBusPermissions(name string, perms BusPermissions) {
	e.perms.SetPermissions(name, perms)
}

// =============================================================================
// Metadata Management (delegates to MetadataManager)
// =============================================================================

// AddModuleNote attaches a note to a module for observability.
func (e *Engine) AddModuleNote(name, note string) {
	e.metadata.AddNote(name, note)
}

// SetModuleCapabilities records declared capabilities for a module.
func (e *Engine) SetModuleCapabilities(name string, caps ...string) {
	e.metadata.SetCapabilities(name, caps...)
}

// SetModuleQuotas records declared quotas for a module.
func (e *Engine) SetModuleQuotas(name string, quotas map[string]string) {
	e.metadata.SetQuotas(name, quotas)
}

// SetModuleRequiredAPIs records declared required API surfaces for a module.
func (e *Engine) SetModuleRequiredAPIs(name string, surfaces ...APISurface) {
	e.metadata.SetRequiredAPIs(name, surfaces...)
}

// SetModuleLabel records a human-friendly label for a module.
func (e *Engine) SetModuleLabel(name, label string) {
	e.metadata.SetLabel(name, label)
}

// SetModuleLayer records an optional layer hint for a module.
func (e *Engine) SetModuleLayer(name, layer string) {
	e.metadata.SetLayer(name, layer)
}

// =============================================================================
// Module Info (aggregated from multiple subsystems)
// =============================================================================

// ModulesInfo returns rich module metadata honouring the explicit ordering.
func (e *Engine) ModulesInfo() []ModuleInfo {
	names := e.registry.Modules()

	// Build available API surfaces map
	available := make(map[string]bool)
	moduleAPIs := make(map[string][]APIDescriptor)

	for _, name := range names {
		mod := e.registry.Lookup(name)
		if mod == nil {
			continue
		}
		perms := e.perms.GetPermissions(name)
		apis := standardAPIs(mod, perms)
		if described, ok := mod.(APIDescriber); ok {
			apis = mergeAPIs(apis, described.APIs())
		}
		moduleAPIs[name] = apis
		for _, api := range apis {
			surf := strings.TrimSpace(string(api.Surface))
			if surf == "" {
				continue
			}
			available[strings.ToLower(surf)] = true
		}
	}

	// Build module info
	out := make([]ModuleInfo, 0, len(names))
	for _, name := range names {
		mod := e.registry.Lookup(name)
		if mod == nil {
			continue
		}

		perms := e.perms.GetPermissions(name)
		deps := e.deps.GetDeps(name)
		info := e.metadata.BuildModuleInfo(mod, perms, deps, available)
		info.APIs = moduleAPIs[name]

		out = append(out, info)
	}

	return out
}

// MissingRequiredAPIs reports modules and the API surfaces they require that
// are not currently provided by any module.
func (e *Engine) MissingRequiredAPIs() map[string][]string {
	names := e.registry.Modules()

	// Build available API surfaces
	available := make(map[string]bool)
	for _, name := range names {
		mod := e.registry.Lookup(name)
		if mod == nil {
			continue
		}
		perms := e.perms.GetPermissions(name)
		apis := standardAPIs(mod, perms)
		if described, ok := mod.(APIDescriber); ok {
			apis = mergeAPIs(apis, described.APIs())
		}
		for _, api := range apis {
			surf := strings.TrimSpace(string(api.Surface))
			if surf == "" {
				continue
			}
			available[strings.ToLower(surf)] = true
		}
	}

	// Check for missing required APIs
	missing := make(map[string][]string)
	for _, name := range names {
		reqs := e.metadata.GetRequiredAPIs(name)
		if len(reqs) == 0 {
			continue
		}
		for _, req := range reqs {
			surf := strings.TrimSpace(strings.ToLower(string(req)))
			if surf == "" {
				continue
			}
			if !available[surf] {
				missing[name] = append(missing[name], surf)
			}
		}
	}

	return missing
}

// =============================================================================
// Utility Methods
// =============================================================================

// Logger returns the engine logger (may be nil).
func (e *Engine) Logger() *log.Logger {
	if e == nil {
		return nil
	}
	return e.log
}

// Registry returns the underlying registry for advanced use cases.
func (e *Engine) Registry() *Registry {
	return e.registry
}

// Health returns the underlying health monitor for advanced use cases.
func (e *Engine) Health() *HealthMonitor {
	return e.health
}

// Dependencies returns the underlying dependency manager for advanced use cases.
func (e *Engine) Dependencies() *DependencyManager {
	return e.deps
}

// Permissions returns the underlying bus permission manager for advanced use cases.
func (e *Engine) Permissions() *BusPermissionManager {
	return e.perms
}

// Metadata returns the underlying metadata manager for advanced use cases.
func (e *Engine) Metadata() *MetadataManager {
	return e.metadata
}

// Bus returns the underlying bus for advanced use cases.
func (e *Engine) Bus() *Bus {
	return e.bus
}
