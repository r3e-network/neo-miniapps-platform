package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry manages service module registration and lookup.
type Registry struct {
	mu       sync.RWMutex
	modules  map[string]ServiceModule
	order    []string // registration order
	ordering []string // explicit startup order
	health   *HealthMonitor
}

// NewRegistry creates a new module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]ServiceModule),
	}
}

// SetHealthMonitor attaches a health monitor to update on registration.
func (r *Registry) SetHealthMonitor(h *HealthMonitor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.health = h
}

// SetOrdering sets an explicit startup order (by module name).
// Unlisted modules start after, in registration order.
func (r *Registry) SetOrdering(modules ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ordering = append([]string{}, modules...)
}

// Register adds a service module to the registry. Names must be unique.
func (r *Registry) Register(module ServiceModule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if module == nil {
		return fmt.Errorf("module is nil")
	}
	name := module.Name()
	if name == "" {
		return fmt.Errorf("module name required")
	}
	if _, exists := r.modules[name]; exists {
		return fmt.Errorf("module %q already registered", name)
	}

	r.modules[name] = module
	r.order = append(r.order, name)

	// Update health if monitor is attached
	if r.health != nil {
		now := time.Now().UTC()
		r.health.setHealthLocked(name, ModuleHealth{
			Name:      name,
			Domain:    module.Domain(),
			Status:    StatusRegistered,
			UpdatedAt: now,
		})
	}

	return nil
}

// Unregister removes a module from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[name]; !exists {
		return fmt.Errorf("module %q not found", name)
	}

	delete(r.modules, name)

	// Remove from order slice
	newOrder := make([]string, 0, len(r.order)-1)
	for _, n := range r.order {
		if n != name {
			newOrder = append(newOrder, n)
		}
	}
	r.order = newOrder

	// Clean up health data to prevent memory leaks
	if r.health != nil {
		r.health.Delete(name)
	}

	return nil
}

// Lookup returns a module by name, if registered.
func (r *Registry) Lookup(name string) ServiceModule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.modules[name]
}

// Modules returns the registered module names (ordered).
func (r *Registry) Modules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.orderedModulesLocked()
}

// ModuleCount returns the number of registered modules.
func (r *Registry) ModuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// ModulesByDomain returns modules matching the provided domain.
func (r *Registry) ModulesByDomain(domain string) []ServiceModule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []ServiceModule
	for _, name := range r.orderedModulesLocked() {
		if mod := r.modules[name]; mod != nil && mod.Domain() == domain {
			out = append(out, mod)
		}
	}
	return out
}

// ModulesByNames returns modules for the given names in order.
func (r *Registry) ModulesByNames(names []string) []ServiceModule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]ServiceModule, 0, len(names))
	for _, name := range names {
		if mod := r.modules[name]; mod != nil {
			modules = append(modules, mod)
		}
	}
	return modules
}

// AccountEngines returns registered account engines.
func (r *Registry) AccountEngines() []AccountEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []AccountEngine
	for _, name := range r.orderedModulesLocked() {
		if mod, ok := r.modules[name]; ok {
			if v, ok := mod.(AccountEngine); ok {
				if cap, ok := mod.(AccountCapable); ok && !cap.HasAccount() {
					continue
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// StoreEngines returns registered store engines.
func (r *Registry) StoreEngines() []StoreEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []StoreEngine
	for _, name := range r.orderedModulesLocked() {
		if mod, ok := r.modules[name]; ok {
			if v, ok := mod.(StoreEngine); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

// ComputeEngines returns registered compute engines.
func (r *Registry) ComputeEngines() []ComputeEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.computeEnginesLocked(nil)
}

// ComputeEnginesWithPerms returns compute engines filtered by permissions.
func (r *Registry) ComputeEnginesWithPerms(permsFunc func(string) BusPermissions) []ComputeEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.computeEnginesLocked(permsFunc)
}

func (r *Registry) computeEnginesLocked(permsFunc func(string) BusPermissions) []ComputeEngine {
	names := r.orderedModulesLocked()
	out := make([]ComputeEngine, 0, len(names))
	for _, name := range names {
		if mod, ok := r.modules[name]; ok {
			if v, ok := mod.(ComputeEngine); ok {
				if permsFunc != nil {
					perms := permsFunc(name)
					if !perms.AllowCompute {
						continue
					}
				}
				if cap, ok := mod.(ComputeCapable); ok && !cap.HasCompute() {
					continue
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// DataEngines returns registered data engines.
func (r *Registry) DataEngines() []DataEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dataEnginesLocked(nil)
}

// DataEnginesWithPerms returns data engines filtered by permissions.
func (r *Registry) DataEnginesWithPerms(permsFunc func(string) BusPermissions) []DataEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dataEnginesLocked(permsFunc)
}

func (r *Registry) dataEnginesLocked(permsFunc func(string) BusPermissions) []DataEngine {
	names := r.orderedModulesLocked()
	out := make([]DataEngine, 0, len(names))
	for _, name := range names {
		if mod, ok := r.modules[name]; ok {
			if v, ok := mod.(DataEngine); ok {
				if permsFunc != nil {
					perms := permsFunc(name)
					if !perms.AllowData {
						continue
					}
				}
				if cap, ok := mod.(DataCapable); ok && !cap.HasData() {
					continue
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// EventEngines returns registered event engines.
func (r *Registry) EventEngines() []EventEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.eventEnginesLocked(nil)
}

// EventEnginesWithPerms returns event engines filtered by permissions.
func (r *Registry) EventEnginesWithPerms(permsFunc func(string) BusPermissions) []EventEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.eventEnginesLocked(permsFunc)
}

func (r *Registry) eventEnginesLocked(permsFunc func(string) BusPermissions) []EventEngine {
	names := r.orderedModulesLocked()
	out := make([]EventEngine, 0, len(names))
	for _, name := range names {
		if mod, ok := r.modules[name]; ok {
			if v, ok := mod.(EventEngine); ok {
				if permsFunc != nil {
					perms := permsFunc(name)
					if !perms.AllowEvents {
						continue
					}
				}
				if cap, ok := mod.(EventCapable); ok && !cap.HasEvent() {
					continue
				}
				out = append(out, v)
			}
		}
	}
	return out
}

// LedgerEngines returns registered ledger engines.
func (r *Registry) LedgerEngines() []LedgerEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []LedgerEngine
	for _, mod := range r.modules {
		if lm, ok := mod.(LedgerEngine); ok {
			out = append(out, lm)
		}
	}
	return out
}

// IndexerEngines returns registered indexer engines.
func (r *Registry) IndexerEngines() []IndexerEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []IndexerEngine
	for _, mod := range r.modules {
		if im, ok := mod.(IndexerEngine); ok {
			out = append(out, im)
		}
	}
	return out
}

// RPCEngines returns registered chain RPC hubs.
func (r *Registry) RPCEngines() []RPCEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []RPCEngine
	for _, mod := range r.modules {
		if rm, ok := mod.(RPCEngine); ok {
			out = append(out, rm)
		}
	}
	return out
}

// DataSourceEngines returns registered data source hubs.
func (r *Registry) DataSourceEngines() []DataSourceEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []DataSourceEngine
	for _, mod := range r.modules {
		if dm, ok := mod.(DataSourceEngine); ok {
			out = append(out, dm)
		}
	}
	return out
}

// ContractsEngines returns registered contract managers.
func (r *Registry) ContractsEngines() []ContractsEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []ContractsEngine
	for _, mod := range r.modules {
		if cm, ok := mod.(ContractsEngine); ok {
			out = append(out, cm)
		}
	}
	return out
}

// ServiceBankEngines returns registered service-owned GAS controllers.
func (r *Registry) ServiceBankEngines() []ServiceBankEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []ServiceBankEngine
	for _, mod := range r.modules {
		if sm, ok := mod.(ServiceBankEngine); ok {
			out = append(out, sm)
		}
	}
	return out
}

// CryptoEngines returns registered crypto helpers.
func (r *Registry) CryptoEngines() []CryptoEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []CryptoEngine
	for _, mod := range r.modules {
		if cm, ok := mod.(CryptoEngine); ok {
			out = append(out, cm)
		}
	}
	return out
}

// SecretsEngines returns registered secrets engines.
func (r *Registry) SecretsEngines() []SecretsEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []SecretsEngine
	for _, mod := range r.modules {
		if se, ok := mod.(SecretsEngine); ok {
			out = append(out, se)
		}
	}
	return out
}

// =============================================================================
// Security & Access Control Engine Accessors
// =============================================================================

// SecurityEngines returns registered security engines.
func (r *Registry) SecurityEngines() []SecurityEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []SecurityEngine
	for _, mod := range r.modules {
		if se, ok := mod.(SecurityEngine); ok {
			out = append(out, se)
		}
	}
	return out
}

// PermissionEngines returns registered permission engines.
func (r *Registry) PermissionEngines() []PermissionEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []PermissionEngine
	for _, mod := range r.modules {
		if pe, ok := mod.(PermissionEngine); ok {
			out = append(out, pe)
		}
	}
	return out
}

// AuditEngines returns registered audit engines.
func (r *Registry) AuditEngines() []AuditEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []AuditEngine
	for _, mod := range r.modules {
		if ae, ok := mod.(AuditEngine); ok {
			out = append(out, ae)
		}
	}
	return out
}

// =============================================================================
// Infrastructure Engine Accessors
// =============================================================================

// CacheEngines returns registered cache engines.
func (r *Registry) CacheEngines() []CacheEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []CacheEngine
	for _, mod := range r.modules {
		if ce, ok := mod.(CacheEngine); ok {
			out = append(out, ce)
		}
	}
	return out
}

// QueueEngines returns registered queue engines.
func (r *Registry) QueueEngines() []QueueEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []QueueEngine
	for _, mod := range r.modules {
		if qe, ok := mod.(QueueEngine); ok {
			out = append(out, qe)
		}
	}
	return out
}

// SchedulerEngines returns registered scheduler engines.
func (r *Registry) SchedulerEngines() []SchedulerEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []SchedulerEngine
	for _, mod := range r.modules {
		if se, ok := mod.(SchedulerEngine); ok {
			out = append(out, se)
		}
	}
	return out
}

// NotificationEngines returns registered notification engines.
func (r *Registry) NotificationEngines() []NotificationEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []NotificationEngine
	for _, mod := range r.modules {
		if ne, ok := mod.(NotificationEngine); ok {
			out = append(out, ne)
		}
	}
	return out
}

// =============================================================================
// Observability Engine Accessors
// =============================================================================

// MetricsEngines returns registered metrics engines.
func (r *Registry) MetricsEngines() []MetricsEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []MetricsEngine
	for _, mod := range r.modules {
		if me, ok := mod.(MetricsEngine); ok {
			out = append(out, me)
		}
	}
	return out
}

// TracingEngines returns registered tracing engines.
func (r *Registry) TracingEngines() []TracingEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []TracingEngine
	for _, mod := range r.modules {
		if te, ok := mod.(TracingEngine); ok {
			out = append(out, te)
		}
	}
	return out
}

// orderedModulesLocked returns module names honoring explicit ordering first,
// then remaining registration order. Must be called with lock held.
func (r *Registry) orderedModulesLocked() []string {
	seen := make(map[string]bool, len(r.modules))
	var out []string

	// Honor explicit ordering list
	for _, name := range r.ordering {
		if mod, ok := r.modules[name]; ok && mod != nil {
			out = append(out, name)
			seen[name] = true
		}
	}

	// Then fall back to registration order
	for _, name := range r.order {
		if !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}

	// Deterministic sort of leftovers not in registration/order arrays
	var extras []string
	for name := range r.modules {
		if !seen[name] && !contains(out, name) {
			extras = append(extras, name)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		out = append(out, extras...)
	}

	return out
}

func contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// classify returns a category string for a module based on its implemented interfaces.
func classify(mod ServiceModule) string {
	switch mod.(type) {
	case StoreEngine:
		return "store"
	case AccountEngine:
		if cap, ok := mod.(AccountCapable); ok && !cap.HasAccount() {
			return ""
		}
		return "account"
	case ComputeEngine:
		if cap, ok := mod.(ComputeCapable); ok && !cap.HasCompute() {
			return ""
		}
		return "compute"
	case DataEngine:
		if cap, ok := mod.(DataCapable); ok && !cap.HasData() {
			return ""
		}
		return "data"
	case EventEngine:
		if cap, ok := mod.(EventCapable); ok && !cap.HasEvent() {
			return ""
		}
		return "event"
	case LedgerEngine:
		return "ledger"
	case IndexerEngine:
		return "indexer"
	case RPCEngine:
		return "rpc"
	case DataSourceEngine:
		return "data-source"
	case ContractsEngine:
		return "contracts"
	case ServiceBankEngine:
		return "gasbank"
	case CryptoEngine:
		return "crypto"
	case SecretsEngine:
		return "secrets"
	// Security & Access Control
	case SecurityEngine:
		return "security"
	case PermissionEngine:
		return "permission"
	case AuditEngine:
		return "audit"
	// Infrastructure
	case CacheEngine:
		return "cache"
	case QueueEngine:
		return "queue"
	case SchedulerEngine:
		return "scheduler"
	case NotificationEngine:
		return "notification"
	// Observability
	case MetricsEngine:
		return "metrics"
	case TracingEngine:
		return "tracing"
	default:
		return ""
	}
}

// enumerateInterfaces returns the list of interface names a module implements.
func enumerateInterfaces(mod ServiceModule) []string {
	var ifaces []string

	if _, ok := mod.(StoreEngine); ok {
		ifaces = append(ifaces, "store")
	}
	if _, ok := mod.(AccountEngine); ok {
		if cap, ok := mod.(AccountCapable); ok && !cap.HasAccount() {
			// skip
		} else {
			ifaces = append(ifaces, "account")
		}
	}
	if _, ok := mod.(ComputeEngine); ok {
		if cap, ok := mod.(ComputeCapable); ok && !cap.HasCompute() {
			// skip
		} else {
			ifaces = append(ifaces, "compute")
		}
	}
	if _, ok := mod.(DataEngine); ok {
		if cap, ok := mod.(DataCapable); ok && !cap.HasData() {
			// skip
		} else {
			ifaces = append(ifaces, "data")
		}
	}
	if _, ok := mod.(EventEngine); ok {
		if cap, ok := mod.(EventCapable); ok && !cap.HasEvent() {
			// skip
		} else {
			ifaces = append(ifaces, "event")
		}
	}
	if _, ok := mod.(LedgerEngine); ok {
		ifaces = append(ifaces, "ledger")
	}
	if _, ok := mod.(IndexerEngine); ok {
		ifaces = append(ifaces, "indexer")
	}
	if _, ok := mod.(RPCEngine); ok {
		ifaces = append(ifaces, "rpc")
	}
	if _, ok := mod.(DataSourceEngine); ok {
		ifaces = append(ifaces, "data-source")
	}
	if _, ok := mod.(ContractsEngine); ok {
		ifaces = append(ifaces, "contracts")
	}
	if _, ok := mod.(ServiceBankEngine); ok {
		ifaces = append(ifaces, "gasbank")
	}
	if _, ok := mod.(CryptoEngine); ok {
		ifaces = append(ifaces, "crypto")
	}
	if _, ok := mod.(SecretsEngine); ok {
		ifaces = append(ifaces, "secrets")
	}
	// Security & Access Control
	if _, ok := mod.(SecurityEngine); ok {
		ifaces = append(ifaces, "security")
	}
	if _, ok := mod.(PermissionEngine); ok {
		ifaces = append(ifaces, "permission")
	}
	if _, ok := mod.(AuditEngine); ok {
		ifaces = append(ifaces, "audit")
	}
	// Infrastructure
	if _, ok := mod.(CacheEngine); ok {
		ifaces = append(ifaces, "cache")
	}
	if _, ok := mod.(QueueEngine); ok {
		ifaces = append(ifaces, "queue")
	}
	if _, ok := mod.(SchedulerEngine); ok {
		ifaces = append(ifaces, "scheduler")
	}
	if _, ok := mod.(NotificationEngine); ok {
		ifaces = append(ifaces, "notification")
	}
	// Observability
	if _, ok := mod.(MetricsEngine); ok {
		ifaces = append(ifaces, "metrics")
	}
	if _, ok := mod.(TracingEngine); ok {
		ifaces = append(ifaces, "tracing")
	}

	return ifaces
}

// BusPermissions restrict which bus fan-outs a module participates in.
type BusPermissions struct {
	AllowEvents  bool
	AllowData    bool
	AllowCompute bool
}

// DefaultBusPermissions returns default permissions (all allowed).
func DefaultBusPermissions() BusPermissions {
	return BusPermissions{
		AllowEvents:  true,
		AllowData:    true,
		AllowCompute: true,
	}
}

// trimSpace is a helper to trim strings consistently.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
