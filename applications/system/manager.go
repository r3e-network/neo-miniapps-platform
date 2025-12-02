package system

import (
	"context"
	"fmt"
	"sync"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Manager owns the lifecycle of registered services. It guarantees
// deterministic start/stop ordering and guards against duplicate invocations.
type Manager struct {
	mu        sync.Mutex
	services  []Service
	started   bool
	startOnce sync.Once
	stopOnce  sync.Once
	descr     []DescriptorProvider
}

// NewManager creates an empty lifecycle manager.
func NewManager() *Manager {
	return &Manager{services: make([]Service, 0)}
}

// Register appends the provided service to the lifecycle queue. Registration
// must occur before Start. Trying to register after Start returns an error.
func (m *Manager) Register(svc Service) error {
	if svc == nil {
		return fmt.Errorf("cannot register a nil service")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("service %q registered after manager start", svc.Name())
	}

	m.services = append(m.services, svc)
	if d, ok := svc.(DescriptorProvider); ok {
		m.descr = append(m.descr, d)
	}
	return nil
}

// Start executes Start on all registered services in order. If any service
// returns an error, the manager stops already-started services in reverse order
// before returning the error to the caller.
func (m *Manager) Start(ctx context.Context) error {
	var startErr error
	m.startOnce.Do(func() {
		m.mu.Lock()
		m.started = true
		services := append([]Service(nil), m.services...)
		m.mu.Unlock()

		for idx, svc := range services {
			if err := svc.Start(ctx); err != nil {
				startErr = fmt.Errorf("start %s: %w", svc.Name(), err)
				for i := idx - 1; i >= 0; i-- {
					_ = services[i].Stop(ctx)
				}
				break
			}
		}
	})
	return startErr
}

// Stop invokes Stop on all registered services in reverse order. It is
// idempotent and returns the first error encountered.
func (m *Manager) Stop(ctx context.Context) error {
	var stopErr error
	m.stopOnce.Do(func() {
		m.mu.Lock()
		services := append([]Service(nil), m.services...)
		m.mu.Unlock()

		for i := len(services) - 1; i >= 0; i-- {
			if err := services[i].Stop(ctx); err != nil && stopErr == nil {
				stopErr = fmt.Errorf("stop %s: %w", services[i].Name(), err)
			}
		}
	})
	return stopErr
}

// DescriptorProviders returns a snapshot of registered descriptor providers.
func (m *Manager) DescriptorProviders() []DescriptorProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DescriptorProvider, len(m.descr))
	copy(out, m.descr)
	return out
}

// Descriptors returns collected descriptors sorted for presentation.
func (m *Manager) Descriptors() []core.Descriptor {
	return CollectDescriptors(m.DescriptorProviders())
}
