package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Default timeout for bus operations per engine.
const (
	DefaultBusTimeout = 5 * time.Second
)

// BusConfig holds configuration for the bus.
type BusConfig struct {
	// Timeout is the per-engine timeout for bus operations.
	// If zero, DefaultBusTimeout is used.
	Timeout time.Duration

	// MaxConcurrency limits the number of concurrent engine invocations.
	// If zero, no limit is applied (all engines invoked in parallel).
	MaxConcurrency int
}

// Bus handles event publishing, data pushing, and compute invocation.
// It provides timeout control and concurrent execution for fan-out operations.
type Bus struct {
	mu       sync.RWMutex
	subs     map[string][]EventHandler
	registry *Registry
	perms    *BusPermissionManager
	config   BusConfig
}

// NewBus creates a new bus instance with default configuration.
func NewBus(registry *Registry, perms *BusPermissionManager) *Bus {
	return &Bus{
		subs:     make(map[string][]EventHandler),
		registry: registry,
		perms:    perms,
		config: BusConfig{
			Timeout: DefaultBusTimeout,
		},
	}
}

// NewBusWithConfig creates a new bus instance with custom configuration.
func NewBusWithConfig(registry *Registry, perms *BusPermissionManager, config BusConfig) *Bus {
	if config.Timeout == 0 {
		config.Timeout = DefaultBusTimeout
	}
	return &Bus{
		subs:     make(map[string][]EventHandler),
		registry: registry,
		perms:    perms,
		config:   config,
	}
}

// SetTimeout updates the per-engine timeout for bus operations.
func (b *Bus) SetTimeout(timeout time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if timeout > 0 {
		b.config.Timeout = timeout
	}
}

// GetTimeout returns the current per-engine timeout.
func (b *Bus) GetTimeout() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config.Timeout
}

// SubscribeEvent registers a handler for an event across all event engines and the in-process bus.
// It returns a joined error if any underlying subscriber rejects the registration.
func (b *Bus) SubscribeEvent(ctx context.Context, event string, handler EventHandler) error {
	if event == "" {
		return fmt.Errorf("event required")
	}
	if handler == nil {
		return fmt.Errorf("event handler is nil")
	}

	b.mu.Lock()
	b.subs[event] = append(b.subs[event], handler)
	b.mu.Unlock()

	// Get event engines with permission filtering
	var engines []EventEngine
	if b.registry != nil {
		if b.perms != nil {
			engines = b.registry.EventEnginesWithPerms(b.perms.GetPermissions)
		} else {
			engines = b.registry.EventEngines()
		}
	}

	var errs []error
	for _, eng := range engines {
		if err := eng.Subscribe(ctx, event, handler); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", eng.Name(), err))
		}
	}

	return errors.Join(errs...)
}

// PublishEvent fan-outs an event to all registered EventEngines plus local subscribers.
// Each engine invocation has a timeout to prevent slow modules from blocking the entire bus.
// Engine invocations run concurrently for improved performance.
func (b *Bus) PublishEvent(ctx context.Context, event string, payload any) error {
	// Get event engines with permission filtering
	var engines []EventEngine
	if b.registry != nil {
		if b.perms != nil {
			engines = b.registry.EventEnginesWithPerms(b.perms.GetPermissions)
		} else {
			engines = b.registry.EventEngines()
		}
	}

	b.mu.RLock()
	localSubs := append([]EventHandler{}, b.subs[event]...)
	timeout := b.config.Timeout
	b.mu.RUnlock()

	// Collect errors from concurrent operations
	errChan := make(chan error, len(engines)+len(localSubs))
	var wg sync.WaitGroup

	// Publish to all event engines concurrently with timeout
	for _, eng := range engines {
		wg.Add(1)
		go func(e EventEngine) {
			defer wg.Done()

			// Create timeout context for this engine
			engCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			if err := e.Publish(engCtx, event, payload); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errChan <- fmt.Errorf("%s: timeout after %v", e.Name(), timeout)
				} else {
					errChan <- fmt.Errorf("%s: %w", e.Name(), err)
				}
			}
		}(eng)
	}

	// Notify local subscribers concurrently with timeout
	for i, handler := range localSubs {
		if handler == nil {
			continue
		}
		wg.Add(1)
		go func(h EventHandler, idx int) {
			defer wg.Done()

			subCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			if err := h(subCtx, payload); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errChan <- fmt.Errorf("subscriber[%d]: timeout after %v", idx, timeout)
				} else {
					errChan <- fmt.Errorf("subscriber[%d]: %w", idx, err)
				}
			}
		}(handler, i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect all errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// PushData dispatches a payload to every registered DataEngine.
// Each engine invocation has a timeout and runs concurrently.
func (b *Bus) PushData(ctx context.Context, topic string, payload any) error {
	// Get data engines with permission filtering
	var engines []DataEngine
	if b.registry != nil {
		if b.perms != nil {
			engines = b.registry.DataEnginesWithPerms(b.perms.GetPermissions)
		} else {
			engines = b.registry.DataEngines()
		}
	}

	if len(engines) == 0 {
		return nil
	}

	b.mu.RLock()
	timeout := b.config.Timeout
	b.mu.RUnlock()

	errChan := make(chan error, len(engines))
	var wg sync.WaitGroup

	for _, eng := range engines {
		wg.Add(1)
		go func(e DataEngine) {
			defer wg.Done()

			engCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			if err := e.Push(engCtx, topic, payload); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					errChan <- fmt.Errorf("%s: timeout after %v", e.Name(), timeout)
				} else {
					errChan <- fmt.Errorf("%s: %w", e.Name(), err)
				}
			}
		}(eng)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// InvokeComputeAll invokes every registered ComputeEngine with the provided payload.
// It returns per-module results and a joined error if any invocation fails.
// Each engine invocation has a timeout and runs concurrently.
func (b *Bus) InvokeComputeAll(ctx context.Context, payload any) ([]InvokeResult, error) {
	// Get compute engines with permission filtering
	var engines []ComputeEngine
	if b.registry != nil {
		if b.perms != nil {
			engines = b.registry.ComputeEnginesWithPerms(b.perms.GetPermissions)
		} else {
			engines = b.registry.ComputeEngines()
		}
	}

	if len(engines) == 0 {
		return nil, nil
	}

	b.mu.RLock()
	timeout := b.config.Timeout
	b.mu.RUnlock()

	type result struct {
		index  int
		result InvokeResult
	}

	resultChan := make(chan result, len(engines))
	var wg sync.WaitGroup

	for i, eng := range engines {
		wg.Add(1)
		go func(idx int, e ComputeEngine) {
			defer wg.Done()

			engCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			res, err := e.Invoke(engCtx, payload)
			if err != nil && errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("timeout after %v", timeout)
			}

			resultChan <- result{
				index: idx,
				result: InvokeResult{
					Module: e.Name(),
					Result: res,
					Err:    err,
				},
			}
		}(i, eng)
	}

	wg.Wait()
	close(resultChan)

	// Collect results in original order
	results := make([]InvokeResult, len(engines))
	var errs []error

	for r := range resultChan {
		results[r.index] = r.result
		if r.result.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.result.Module, r.result.Err))
		}
	}

	return results, errors.Join(errs...)
}

// LocalSubscribers returns the number of local subscribers for an event.
func (b *Bus) LocalSubscribers(event string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs[event])
}

// LocalEvents returns all events with local subscribers.
// Note: This only returns locally subscribed events, not events from EventEngines.
func (b *Bus) LocalEvents() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	events := make([]string, 0, len(b.subs))
	for event := range b.subs {
		events = append(events, event)
	}
	return events
}

// AllEvents is an alias for LocalEvents for backward compatibility.
// Deprecated: Use LocalEvents() for clarity.
func (b *Bus) AllEvents() []string {
	return b.LocalEvents()
}

// ClearSubscribers removes all local subscribers.
func (b *Bus) ClearSubscribers() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = make(map[string][]EventHandler)
}

// BusPermissionManager manages bus permissions for modules.
type BusPermissionManager struct {
	mu    sync.RWMutex
	perms map[string]BusPermissions
}

// NewBusPermissionManager creates a new bus permission manager.
func NewBusPermissionManager() *BusPermissionManager {
	return &BusPermissionManager{
		perms: make(map[string]BusPermissions),
	}
}

// SetPermissions sets the bus permissions for a module.
func (p *BusPermissionManager) SetPermissions(name string, perms BusPermissions) {
	if p == nil {
		return
	}
	name = trimSpace(name)
	if name == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.perms[name] = perms
}

// GetPermissions returns the bus permissions for a module.
// Returns default permissions (all allowed) if not explicitly set.
func (p *BusPermissionManager) GetPermissions(name string) BusPermissions {
	if p == nil {
		return DefaultBusPermissions()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if perms, ok := p.perms[name]; ok {
		return perms
	}
	return DefaultBusPermissions()
}

// HasPermission checks if a module has specific bus permission.
func (p *BusPermissionManager) HasPermission(name string, permType string) bool {
	perms := p.GetPermissions(name)

	switch permType {
	case "events":
		return perms.AllowEvents
	case "data":
		return perms.AllowData
	case "compute":
		return perms.AllowCompute
	default:
		return false
	}
}

// AllPermissions returns all permissions map.
func (p *BusPermissionManager) AllPermissions() map[string]BusPermissions {
	if p == nil {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]BusPermissions, len(p.perms))
	for k, v := range p.perms {
		result[k] = v
	}
	return result
}

// RemovePermissions removes permissions for a module.
func (p *BusPermissionManager) RemovePermissions(name string) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.perms, name)
}

// Clear removes all permissions.
func (p *BusPermissionManager) Clear() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.perms = make(map[string]BusPermissions)
}
