package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DependencyManager manages module dependencies and startup ordering.
type DependencyManager struct {
	mu   sync.RWMutex
	deps map[string][]string // module name -> dependencies
}

// NewDependencyManager creates a new dependency manager.
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		deps: make(map[string][]string),
	}
}

// SetDeps records dependencies for a module.
func (d *DependencyManager) SetDeps(name string, deps ...string) {
	if d == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	filtered := make([]string, 0, len(deps))
	for _, dep := range deps {
		if dep = strings.TrimSpace(dep); dep != "" {
			filtered = append(filtered, dep)
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.deps[name] = filtered
}

// GetDeps returns the dependencies for a module.
func (d *DependencyManager) GetDeps(name string) []string {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]string{}, d.deps[name]...)
}

// Verify ensures all declared dependencies are registered.
func (d *DependencyManager) Verify(registeredModules []string) error {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	registered := make(map[string]bool, len(registeredModules))
	for _, name := range registeredModules {
		registered[name] = true
	}

	for mod, deps := range d.deps {
		for _, dep := range deps {
			if dep == "" {
				continue
			}
			if !registered[dep] {
				return fmt.Errorf("module %q missing dependency %q", mod, dep)
			}
		}
	}

	return nil
}

// ResolveOrder returns a startup ordering that satisfies declared dependencies
// while preserving the provided ordering as much as possible.
// Errors indicate cycles or unresolved deps.
func (d *DependencyManager) ResolveOrder(names []string) ([]string, error) {
	if d == nil || len(names) == 0 {
		return names, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}

	resolved := make([]string, 0, len(names))
	done := make(map[string]bool, len(names))

	for len(resolved) < len(names) {
		progressed := false

		for _, name := range names {
			if done[name] {
				continue
			}

			deps := d.deps[name]
			waiting := false

			for _, dep := range deps {
				if dep = strings.TrimSpace(dep); dep == "" {
					continue
				}
				if !set[dep] {
					// Missing deps are caught by Verify before this
					continue
				}
				if !done[dep] {
					waiting = true
					break
				}
			}

			if waiting {
				continue
			}

			resolved = append(resolved, name)
			done[name] = true
			progressed = true
		}

		if !progressed {
			var unresolved []string
			for _, name := range names {
				if !done[name] {
					unresolved = append(unresolved, name)
				}
			}
			sort.Strings(unresolved)
			return nil, fmt.Errorf("dependency cycle or unresolved dependencies for: %v", unresolved)
		}
	}

	return resolved, nil
}

// DepsReady checks if all declared deps for a module are currently ready.
func (d *DependencyManager) DepsReady(name string, health *HealthMonitor) bool {
	ok, _ := d.DepsReadyWithReasons(name, health)
	return ok
}

// DepsReadyWithReasons returns readiness along with human-readable reasons for missing deps.
func (d *DependencyManager) DepsReadyWithReasons(name string, health *HealthMonitor) (bool, []string) {
	if d == nil {
		return true, nil
	}

	d.mu.RLock()
	deps := d.deps[name]
	d.mu.RUnlock()

	if len(deps) == 0 {
		return true, nil
	}

	return DepsReadyWithReasons(health, deps)
}

// AllDeps returns all dependencies map (for debugging/introspection).
func (d *DependencyManager) AllDeps() map[string][]string {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string][]string, len(d.deps))
	for k, v := range d.deps {
		result[k] = append([]string{}, v...)
	}
	return result
}

// HasDependents returns true if any module depends on the given module.
func (d *DependencyManager) HasDependents(name string) bool {
	if d == nil {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, deps := range d.deps {
		for _, dep := range deps {
			if dep == name {
				return true
			}
		}
	}
	return false
}

// Dependents returns all modules that depend on the given module.
func (d *DependencyManager) Dependents(name string) []string {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var dependents []string
	for mod, deps := range d.deps {
		for _, dep := range deps {
			if dep == name {
				dependents = append(dependents, mod)
				break
			}
		}
	}

	sort.Strings(dependents)
	return dependents
}

// Clear removes all dependency records.
func (d *DependencyManager) Clear() {
	if d == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.deps = make(map[string][]string)
}

// RemoveDeps removes dependency records for a module.
func (d *DependencyManager) RemoveDeps(name string) {
	if d == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.deps, name)
}
