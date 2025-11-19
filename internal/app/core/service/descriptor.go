package service

// Layer describes the service placement. The platform now treats every
// capability uniformly, so all descriptors advertise the same consolidated
// layer for clarity.
type Layer string

const (
	LayerPlatform Layer = "platform"
)

// Descriptor advertises a service's placement and capabilities. It is optional
// and does not change runtime behavior, but allows orchestration layers and
// documentation to reason about modules consistently.
type Descriptor struct {
	Name         string
	Domain       string
	Layer        Layer
	Capabilities []string
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
