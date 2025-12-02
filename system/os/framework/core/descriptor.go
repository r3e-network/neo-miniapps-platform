package service

import "strings"

// Layer describes the service placement. The platform now treats every
// capability uniformly, so all descriptors advertise the same consolidated
// layer for clarity.
type Layer string

const (
	LayerPlatform Layer = "platform" // legacy alias
	LayerService  Layer = "service"
	LayerRunner   Layer = "runner"
	LayerInfra    Layer = "infra"
)

// Descriptor advertises a service's placement and capabilities. It is optional
// and does not change runtime behavior, but allows orchestration layers and
// documentation to reason about modules consistently.
type Descriptor struct {
	Name         string
	Domain       string
	Layer        Layer
	Capabilities []string
	RequiresAPIs []string
	DependsOn    []string
}

// WithCapabilities returns a copy of the descriptor with additional
// capabilities appended.
func (d Descriptor) WithCapabilities(caps ...string) Descriptor {
	if len(caps) == 0 {
		return d
	}
	combined := make([]string, 0, len(d.Capabilities)+len(caps))
	combined = append(combined, d.Capabilities...)
	combined = append(combined, caps...)
	d.Capabilities = combined
	return d
}

// WithRequires appends required API surfaces.
func (d Descriptor) WithRequires(apis ...string) Descriptor {
	if len(apis) == 0 {
		return d
	}
	combined := make([]string, 0, len(d.RequiresAPIs)+len(apis))
	combined = append(combined, d.RequiresAPIs...)
	for _, api := range apis {
		api = strings.TrimSpace(api)
		if api != "" {
			combined = append(combined, api)
		}
	}
	d.RequiresAPIs = combined
	return d
}

// WithDependsOn appends dependencies.
func (d Descriptor) WithDependsOn(deps ...string) Descriptor {
	if len(deps) == 0 {
		return d
	}
	combined := make([]string, 0, len(d.DependsOn)+len(deps))
	combined = append(combined, d.DependsOn...)
	for _, dep := range deps {
		if dep = strings.TrimSpace(dep); dep != "" {
			combined = append(combined, dep)
		}
	}
	d.DependsOn = combined
	return d
}
