package engine

import "log"

// Option configures an Engine.
type Option func(*Engine)

// WithLogger overrides the default logger.
func WithLogger(l *log.Logger) Option {
	return func(e *Engine) {
		if l != nil {
			e.log = l
		}
	}
}

// WithOrder sets an explicit startup order (by module name).
// Unlisted modules start after, in registration order.
func WithOrder(modules ...string) Option {
	return func(e *Engine) {
		e.registry.SetOrdering(modules...)
	}
}

// WithRegistry sets a custom registry.
func WithRegistry(r *Registry) Option {
	return func(e *Engine) {
		if r != nil {
			e.registry = r
		}
	}
}

// WithHealthMonitor sets a custom health monitor.
func WithHealthMonitor(h *HealthMonitor) Option {
	return func(e *Engine) {
		if h != nil {
			e.health = h
		}
	}
}

// WithDependencyManager sets a custom dependency manager.
func WithDependencyManager(d *DependencyManager) Option {
	return func(e *Engine) {
		if d != nil {
			e.deps = d
		}
	}
}

// WithPermissionManager sets a custom permission manager.
func WithPermissionManager(p *PermissionManager) Option {
	return func(e *Engine) {
		if p != nil {
			e.perms = p
		}
	}
}
