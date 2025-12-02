// Package lifecycle provides lifecycle management utilities for services.
package lifecycle

import (
	"context"
	"fmt"
	"sync"
)

// HookFunc is a function that runs during a lifecycle phase.
// It receives a context and returns an error if the hook fails.
type HookFunc func(ctx context.Context) error

// NamedHook is a hook with an optional name for debugging and error reporting.
type NamedHook struct {
	Name string
	Fn   HookFunc
}

// Hooks manages lifecycle hooks for a service.
// It provides pre/post hooks for start and stop phases.
type Hooks struct {
	mu sync.RWMutex

	preStart  []NamedHook
	postStart []NamedHook
	preStop   []NamedHook
	postStop  []NamedHook
}

// NewHooks creates a new Hooks instance.
func NewHooks() *Hooks {
	return &Hooks{
		preStart:  make([]NamedHook, 0),
		postStart: make([]NamedHook, 0),
		preStop:   make([]NamedHook, 0),
		postStop:  make([]NamedHook, 0),
	}
}

// OnPreStart adds a hook to run before the service starts.
func (h *Hooks) OnPreStart(fn HookFunc) {
	h.OnPreStartNamed("", fn)
}

// OnPreStartNamed adds a named hook to run before the service starts.
func (h *Hooks) OnPreStartNamed(name string, fn HookFunc) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preStart = append(h.preStart, NamedHook{Name: name, Fn: fn})
}

// OnPostStart adds a hook to run after the service starts successfully.
func (h *Hooks) OnPostStart(fn HookFunc) {
	h.OnPostStartNamed("", fn)
}

// OnPostStartNamed adds a named hook to run after the service starts successfully.
func (h *Hooks) OnPostStartNamed(name string, fn HookFunc) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.postStart = append(h.postStart, NamedHook{Name: name, Fn: fn})
}

// OnPreStop adds a hook to run before the service stops.
func (h *Hooks) OnPreStop(fn HookFunc) {
	h.OnPreStopNamed("", fn)
}

// OnPreStopNamed adds a named hook to run before the service stops.
func (h *Hooks) OnPreStopNamed(name string, fn HookFunc) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preStop = append(h.preStop, NamedHook{Name: name, Fn: fn})
}

// OnPostStop adds a hook to run after the service stops.
func (h *Hooks) OnPostStop(fn HookFunc) {
	h.OnPostStopNamed("", fn)
}

// OnPostStopNamed adds a named hook to run after the service stops.
func (h *Hooks) OnPostStopNamed(name string, fn HookFunc) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.postStop = append(h.postStop, NamedHook{Name: name, Fn: fn})
}

// RunPreStart runs all pre-start hooks in order.
// Returns on first error.
func (h *Hooks) RunPreStart(ctx context.Context) error {
	return h.runHooks(ctx, "PreStart", h.preStart)
}

// RunPostStart runs all post-start hooks in order.
// Returns on first error.
func (h *Hooks) RunPostStart(ctx context.Context) error {
	return h.runHooks(ctx, "PostStart", h.postStart)
}

// RunPreStop runs all pre-stop hooks in order.
// Returns on first error.
func (h *Hooks) RunPreStop(ctx context.Context) error {
	return h.runHooks(ctx, "PreStop", h.preStop)
}

// RunPostStop runs all post-stop hooks in REVERSE order.
// This ensures cleanup happens in the opposite order of setup.
// Returns on first error.
func (h *Hooks) RunPostStop(ctx context.Context) error {
	h.mu.RLock()
	hooks := make([]NamedHook, len(h.postStop))
	copy(hooks, h.postStop)
	h.mu.RUnlock()

	// Reverse the slice for LIFO cleanup
	for i, j := 0, len(hooks)-1; i < j; i, j = i+1, j-1 {
		hooks[i], hooks[j] = hooks[j], hooks[i]
	}

	return h.runHooksSlice(ctx, "PostStop", hooks)
}

// runHooks runs a slice of hooks in order.
func (h *Hooks) runHooks(ctx context.Context, phase string, hooks []NamedHook) error {
	h.mu.RLock()
	hooksCopy := make([]NamedHook, len(hooks))
	copy(hooksCopy, hooks)
	h.mu.RUnlock()

	return h.runHooksSlice(ctx, phase, hooksCopy)
}

// runHooksSlice runs hooks without copying (caller must have already copied).
func (h *Hooks) runHooksSlice(ctx context.Context, phase string, hooks []NamedHook) error {
	for i, hook := range hooks {
		if hook.Fn == nil {
			continue
		}

		if err := hook.Fn(ctx); err != nil {
			if hook.Name != "" {
				return fmt.Errorf("%s hook %q (#%d) failed: %w", phase, hook.Name, i, err)
			}
			return fmt.Errorf("%s hook #%d failed: %w", phase, i, err)
		}
	}
	return nil
}

// Clear removes all hooks.
func (h *Hooks) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.preStart = make([]NamedHook, 0)
	h.postStart = make([]NamedHook, 0)
	h.preStop = make([]NamedHook, 0)
	h.postStop = make([]NamedHook, 0)
}

// HookCounts returns the number of hooks registered for each phase.
type HookCounts struct {
	PreStart  int
	PostStart int
	PreStop   int
	PostStop  int
}

// Counts returns the number of hooks registered for each phase.
func (h *Hooks) Counts() HookCounts {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return HookCounts{
		PreStart:  len(h.preStart),
		PostStart: len(h.postStart),
		PreStop:   len(h.preStop),
		PostStop:  len(h.postStop),
	}
}
