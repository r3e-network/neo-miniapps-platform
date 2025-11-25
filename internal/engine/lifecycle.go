package engine

import (
	"context"
	"fmt"
	"log"
	"time"
)

// LifecycleManager handles module startup and shutdown.
type LifecycleManager struct {
	registry *Registry
	deps     *DependencyManager
	health   *HealthMonitor
	log      *log.Logger
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager(registry *Registry, deps *DependencyManager, health *HealthMonitor, logger *log.Logger) *LifecycleManager {
	if logger == nil {
		logger = log.Default()
	}
	return &LifecycleManager{
		registry: registry,
		deps:     deps,
		health:   health,
		log:      logger,
	}
}

// Start walks registered modules in dependency order.
func (lm *LifecycleManager) Start(ctx context.Context) error {
	names := lm.registry.Modules()

	// Verify dependencies
	if err := lm.deps.Verify(names); err != nil {
		return err
	}

	// Resolve dependency order
	reordered, err := lm.deps.ResolveOrder(names)
	if err != nil {
		return err
	}

	modules := lm.registry.ModulesByNames(reordered)

	started := make([]ServiceModule, 0, len(modules))
	for _, mod := range modules {
		if ctx.Err() != nil {
			lm.stopReverse(ctx, started)
			return ctx.Err()
		}

		name := mod.Name()
		domain := mod.Domain()

		lm.health.MarkStarting(name, domain)

		startNow := time.Now()
		if err := mod.Start(ctx); err != nil {
			lm.health.MarkFailed(name, domain, err.Error(), time.Since(startNow).Nanoseconds())
			lm.stopReverse(ctx, started)
			return fmt.Errorf("start %s: %w", name, err)
		}

		started = append(started, mod)
		lm.health.MarkStarted(name, domain, time.Since(startNow).Nanoseconds())
	}

	return nil
}

// Stop walks registered modules in reverse registration order.
func (lm *LifecycleManager) Stop(ctx context.Context) error {
	names := lm.registry.Modules()
	modules := lm.registry.ModulesByNames(names)

	// Stop in reverse order
	for i := len(modules) - 1; i >= 0; i-- {
		mod := modules[i]
		name := mod.Name()
		domain := mod.Domain()

		stopNow := time.Now()
		if err := mod.Stop(ctx); err != nil {
			// Log and continue shutdown to avoid leaking other resources
			lm.log.Printf("stop %s: %v", name, err)
			lm.health.MarkStopError(name, domain, err.Error(), time.Since(stopNow).Nanoseconds())
		} else {
			lm.health.MarkStopped(name, domain, time.Since(stopNow).Nanoseconds())

			if setter, ok := mod.(ReadySetter); ok {
				setter.SetReady(ReadyStatusNotReady, "")
			}
		}
	}

	return nil
}

// stopReverse stops modules in reverse order (for rollback on startup failure).
func (lm *LifecycleManager) stopReverse(ctx context.Context, mods []ServiceModule) {
	for i := len(mods) - 1; i >= 0; i-- {
		mod := mods[i]
		name := mod.Name()
		domain := mod.Domain()

		status := StatusStopped
		errStr := ""

		if err := mod.Stop(ctx); err != nil {
			status = StatusStopError
			errStr = err.Error()
			lm.log.Printf("stop %s: %v", name, err)
		}

		now := time.Now().UTC()
		lm.health.SetHealth(name, ModuleHealth{
			Name:        name,
			Domain:      domain,
			Status:      status,
			Error:       errStr,
			ReadyStatus: ReadyStatusNotReady,
			StoppedAt:   &now,
		})

		if setter, ok := mod.(ReadySetter); ok {
			setter.SetReady(ReadyStatusNotReady, errStr)
		}
	}
}

// MarkStarted records a started status for the given module names.
// When names is empty, all registered modules are marked as started.
func (lm *LifecycleManager) MarkStarted(names ...string) {
	if len(names) == 0 {
		names = lm.registry.Modules()
	}

	var mods []ServiceModule
	for _, name := range names {
		if name == "" {
			continue
		}
		if mod := lm.registry.Lookup(name); mod != nil {
			mods = append(mods, mod)
		}
	}

	lm.health.MarkModulesStarted(mods)
}

// MarkStopped records a stopped status for the given module names.
// When names is empty, all registered modules are marked as stopped.
func (lm *LifecycleManager) MarkStopped(names ...string) {
	if len(names) == 0 {
		names = lm.registry.Modules()
	}

	var mods []ServiceModule
	for _, name := range names {
		if name == "" {
			continue
		}
		if mod := lm.registry.Lookup(name); mod != nil {
			mods = append(mods, mod)
		}
	}

	lm.health.MarkModulesStopped(mods)
}

// MarkReady updates readiness for the provided modules (or all modules when names are empty).
// Status defaults to "ready" when blank.
func (lm *LifecycleManager) MarkReady(status, errMsg string, names ...string) {
	if status == "" {
		status = ReadyStatusReady
	}

	if len(names) == 0 {
		names = lm.registry.Modules()
	}

	var mods []ServiceModule
	for _, name := range names {
		if name == "" {
			continue
		}
		if mod := lm.registry.Lookup(name); mod != nil {
			mods = append(mods, mod)
		}
	}

	lm.health.MarkReady(status, errMsg, mods)
}

// ProbeReadiness runs lightweight readiness checks for modules that implement ReadyChecker.
func (lm *LifecycleManager) ProbeReadiness(ctx context.Context) {
	names := lm.registry.Modules()
	modules := lm.registry.ModulesByNames(names)

	depsReadyFunc := func(name string) (bool, []string) {
		return lm.deps.DepsReadyWithReasons(name, lm.health)
	}

	// Log dependency waiting only when state changes
	for _, mod := range modules {
		prevReady := lm.health.GetReadyStatus(mod.Name())
		prevReadyErr := lm.health.GetReadyError(mod.Name())

		ok, reasons := depsReadyFunc(mod.Name())
		if !ok {
			newErr := "waiting for dependencies: " + joinStrings(reasons, "; ")
			if prevReady != ReadyStatusNotReady || prevReadyErr != newErr {
				lm.log.Printf("module %s waiting for dependencies: %s", mod.Name(), joinStrings(reasons, "; "))
			}
		}
	}

	lm.health.ProbeReadiness(ctx, modules, depsReadyFunc)
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
