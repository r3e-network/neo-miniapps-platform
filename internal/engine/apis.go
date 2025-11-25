package engine

import "strings"

// APISurface names a system-level surface that modules can expose. This keeps
// the engine focused on a few fundamental channels (lifecycle, readiness,
// account/store/compute/data/event buses) so services look like applications
// running on top of a lightweight OS.
type APISurface string

const (
	APISurfaceLifecycle  APISurface = "lifecycle"
	APISurfaceReadiness  APISurface = "readiness"
	APISurfaceAccount    APISurface = "account"
	APISurfaceStore      APISurface = "store"
	APISurfaceCompute    APISurface = "compute"
	APISurfaceData       APISurface = "data"
	APISurfaceEvent      APISurface = "event"
	APISurfaceRPC        APISurface = "rpc"
	APISurfaceIndexer    APISurface = "indexer"
	APISurfaceLedger     APISurface = "ledger"
	APISurfaceDataSource APISurface = "data-source"
	APISurfaceContracts  APISurface = "contracts"
	APISurfaceGasBank    APISurface = "gasbank"
	APISurfaceCrypto     APISurface = "crypto"
)

// APIDescriptor describes a standard API surface a module participates in.
// Modules can extend these by implementing APIDescriber.
type APIDescriptor struct {
	Name      string     `json:"name"`
	Surface   APISurface `json:"surface,omitempty"`
	Summary   string     `json:"summary,omitempty"`
	Stability string     `json:"stability,omitempty"` // alpha|beta|stable
}

// APIDescriber allows modules to advertise custom API surfaces in addition to
// the standard ones inferred from engine interfaces.
type APIDescriber interface {
	APIs() []APIDescriptor
}

// standardAPIs infers the built-in API surfaces exposed by a module based on
// its implemented engine interfaces and bus permissions.
func standardAPIs(mod ServiceModule, perms BusPermissions) []APIDescriptor {
	if mod == nil {
		return nil
	}
	var apis []APIDescriptor
	apis = append(apis, APIDescriptor{
		Name:      "lifecycle",
		Surface:   APISurfaceLifecycle,
		Summary:   "Start/stop managed by the service engine",
		Stability: "stable",
	})

	if _, ok := mod.(ReadyChecker); ok {
		apis = append(apis, APIDescriptor{
			Name:      "readiness",
			Surface:   APISurfaceReadiness,
			Summary:   "Readiness probe for module health",
			Stability: "stable",
		})
	}
	if _, ok := mod.(StoreEngine); ok {
		apis = append(apis, APIDescriptor{
			Name:      "store",
			Surface:   APISurfaceStore,
			Summary:   "Persistent store connectivity (Ping)",
			Stability: "stable",
		})
	}
	if _, ok := mod.(AccountEngine); ok {
		if cap, ok := mod.(AccountCapable); ok && !cap.HasAccount() {
			// skip
		} else {
			apis = append(apis, APIDescriptor{
				Name:      "accounts",
				Surface:   APISurfaceAccount,
				Summary:   "Account registry (create/list) via engine contract",
				Stability: "stable",
			})
		}
	}
	if _, ok := mod.(ComputeEngine); ok {
		if cap, ok := mod.(ComputeCapable); ok && !cap.HasCompute() {
			// skip
		} else if perms.AllowCompute {
			apis = append(apis, APIDescriptor{
				Name:      "compute",
				Surface:   APISurfaceCompute,
				Summary:   "Function/job execution through the engine bus",
				Stability: "stable",
			})
		}
	}
	if _, ok := mod.(DataEngine); ok {
		if cap, ok := mod.(DataCapable); ok && !cap.HasData() {
			// skip
		} else if perms.AllowData {
			apis = append(apis, APIDescriptor{
				Name:      "data-bus",
				Surface:   APISurfaceData,
				Summary:   "Data push fan-out via /system/data and SDKs",
				Stability: "stable",
			})
		}
	}
	if _, ok := mod.(EventEngine); ok {
		if cap, ok := mod.(EventCapable); ok && !cap.HasEvent() {
			// skip
		} else if perms.AllowEvents {
			apis = append(apis, APIDescriptor{
				Name:      "event-bus",
				Surface:   APISurfaceEvent,
				Summary:   "Publish/subscribe over the engine event bus",
				Stability: "stable",
			})
		}
	}
	return apis
}

// mergeAPIs combines standard and custom API descriptors, deduping by name and
// surface while preserving the provided order.
func mergeAPIs(base, extra []APIDescriptor) []APIDescriptor {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make([]APIDescriptor, 0, len(base)+len(extra))
	seen := make(map[string]bool)
	appendUnique := func(api APIDescriptor) {
		api.Name = strings.TrimSpace(api.Name)
		if api.Name == "" {
			return
		}
		key := strings.ToLower(api.Name) + "|" + strings.ToLower(string(api.Surface))
		if seen[key] {
			return
		}
		if api.Stability == "" {
			api.Stability = "stable"
		}
		out = append(out, api)
		seen[key] = true
	}
	for _, api := range base {
		appendUnique(api)
	}
	for _, api := range extra {
		appendUnique(api)
	}
	return out
}
