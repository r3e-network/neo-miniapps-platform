package system

import (
	"context"

	core "github.com/R3E-Network/service_layer/internal/services/core"
)

// Service represents a lifecycle-managed component. All application modules
// must implement this interface so the system manager can start and stop them
// deterministically.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// LifecycleService is the common contract for engine-managed services that expose readiness.
// All domain services and runners should implement this so they can be wired into the engine
// and surfaced consistently via /system/status, CLI, and dashboard.
type LifecycleService interface {
	Service
	Ready(ctx context.Context) error
}

// DescriptorProvider optionally advertises service metadata (layer, capabilities).
type DescriptorProvider interface {
	Descriptor() core.Descriptor
}
