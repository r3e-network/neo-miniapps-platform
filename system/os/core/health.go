package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Module status constants
const (
	StatusRegistered = "registered"
	StatusStarting   = "starting"
	StatusStarted    = "started"
	StatusStopped    = "stopped"
	StatusFailed     = "failed"
	StatusStopError  = "stop-error"
	StatusUnknown    = "unknown"

	ReadyStatusReady    = "ready"
	ReadyStatusNotReady = "not-ready"
	ReadyStatusUnknown  = "unknown"
)

// ModuleHealth captures the latest lifecycle status for a module.
type ModuleHealth struct {
	Name        string     `json:"name"`
	Domain      string     `json:"domain,omitempty"`
	Status      string     `json:"status"` // registered|starting|started|stopped|failed|stop-error|unknown
	Error       string     `json:"error,omitempty"`
	ReadyStatus string     `json:"ready_status,omitempty"` // ready|not-ready|unknown
	ReadyError  string     `json:"ready_error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	StoppedAt   *time.Time `json:"stopped_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartNanos  int64      `json:"start_nanos,omitempty"` // duration nanoseconds between start invocation and completion
	StopNanos   int64      `json:"stop_nanos,omitempty"`  // duration nanoseconds between stop invocation and completion
}

// HealthMonitor tracks health status for all modules.
type HealthMonitor struct {
	mu     sync.RWMutex
	health map[string]ModuleHealth
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		health: make(map[string]ModuleHealth),
	}
}

// SetHealth updates the health status for a module.
func (h *HealthMonitor) SetHealth(name string, health ModuleHealth) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.setHealthLocked(name, health)
}

// Delete removes health data for a module.
// This should be called when a module is unregistered to prevent memory leaks.
func (h *HealthMonitor) Delete(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.health, name)
}

// setHealthLocked updates health without acquiring lock (caller must hold lock).
func (h *HealthMonitor) setHealthLocked(name string, health ModuleHealth) {
	if h.health == nil {
		h.health = make(map[string]ModuleHealth)
	}

	// Merge with existing health to preserve fields not being updated
	if existing, ok := h.health[name]; ok {
		if health.StartedAt == nil {
			health.StartedAt = existing.StartedAt
		}
		if health.StoppedAt == nil {
			health.StoppedAt = existing.StoppedAt
		}
		if health.ReadyStatus == "" {
			health.ReadyStatus = existing.ReadyStatus
			health.ReadyError = existing.ReadyError
		}
		if health.Status == "" {
			health.Status = existing.Status
			health.Error = existing.Error
		}
		if health.StartNanos == 0 {
			health.StartNanos = existing.StartNanos
		}
		if health.StopNanos == 0 {
			health.StopNanos = existing.StopNanos
		}
	}

	if health.UpdatedAt.IsZero() {
		health.UpdatedAt = time.Now().UTC()
	}

	h.health[name] = health
}

// GetHealth returns the health status for a module.
func (h *HealthMonitor) GetHealth(name string) ModuleHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health
	}
	return ModuleHealth{Name: name, Status: StatusUnknown}
}

// GetStatus returns just the status string for a module.
func (h *HealthMonitor) GetStatus(name string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.Status
	}
	return ""
}

// GetError returns the error string for a module.
func (h *HealthMonitor) GetError(name string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.Error
	}
	return ""
}

// GetReadyStatus returns the ready status for a module.
func (h *HealthMonitor) GetReadyStatus(name string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.ReadyStatus
	}
	return ""
}

// GetReadyError returns the ready error for a module.
func (h *HealthMonitor) GetReadyError(name string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.ReadyError
	}
	return ""
}

// GetStartedAt returns when a module started.
func (h *HealthMonitor) GetStartedAt(name string) *time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.StartedAt
	}
	return nil
}

// GetStoppedAt returns when a module stopped.
func (h *HealthMonitor) GetStoppedAt(name string) *time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.StoppedAt
	}
	return nil
}

// GetStartNanos returns the start duration in nanoseconds.
func (h *HealthMonitor) GetStartNanos(name string) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.StartNanos
	}
	return 0
}

// GetStopNanos returns the stop duration in nanoseconds.
func (h *HealthMonitor) GetStopNanos(name string) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if health, ok := h.health[name]; ok {
		return health.StopNanos
	}
	return 0
}

// ModulesHealth returns the latest known lifecycle state per module (ordered).
func (h *HealthMonitor) ModulesHealth(orderedNames []string) []ModuleHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]ModuleHealth, 0, len(orderedNames))
	for _, name := range orderedNames {
		if health, ok := h.health[name]; ok {
			out = append(out, health)
		} else {
			out = append(out, ModuleHealth{Name: name, Status: StatusUnknown})
		}
	}
	return out
}

// MarkStarting sets a module's status to starting.
func (h *HealthMonitor) MarkStarting(name, domain string) {
	h.SetHealth(name, ModuleHealth{
		Name:   name,
		Domain: domain,
		Status: StatusStarting,
	})
}

// MarkStarted sets a module's status to started.
func (h *HealthMonitor) MarkStarted(name, domain string, startNanos int64) {
	now := time.Now().UTC()
	h.SetHealth(name, ModuleHealth{
		Name:       name,
		Domain:     domain,
		Status:     StatusStarted,
		StartedAt:  &now,
		StartNanos: startNanos,
	})
}

// MarkFailed sets a module's status to failed.
func (h *HealthMonitor) MarkFailed(name, domain, errMsg string, startNanos int64) {
	h.SetHealth(name, ModuleHealth{
		Name:       name,
		Domain:     domain,
		Status:     StatusFailed,
		Error:      errMsg,
		StartNanos: startNanos,
	})
}

// MarkStopped sets a module's status to stopped.
func (h *HealthMonitor) MarkStopped(name, domain string, stopNanos int64) {
	now := time.Now().UTC()
	h.SetHealth(name, ModuleHealth{
		Name:        name,
		Domain:      domain,
		Status:      StatusStopped,
		ReadyStatus: ReadyStatusNotReady,
		StoppedAt:   &now,
		StopNanos:   stopNanos,
	})
}

// MarkStopError sets a module's status to stop-error.
func (h *HealthMonitor) MarkStopError(name, domain, errMsg string, stopNanos int64) {
	now := time.Now().UTC()
	h.SetHealth(name, ModuleHealth{
		Name:        name,
		Domain:      domain,
		Status:      StatusStopError,
		Error:       errMsg,
		ReadyStatus: ReadyStatusNotReady,
		StoppedAt:   &now,
		StopNanos:   stopNanos,
	})
}

// SetReadyStatus updates only the readiness status for a module.
// This method properly uses setHealthLocked to merge with existing health data,
// avoiding race conditions where fields could be incorrectly reset.
func (h *HealthMonitor) SetReadyStatus(name, domain, readyStatus, readyErr string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Use setHealthLocked to properly merge with existing health data.
	// Only set the fields we want to update; setHealthLocked preserves others.
	h.setHealthLocked(name, ModuleHealth{
		Name:        name,
		Domain:      domain,
		ReadyStatus: readyStatus,
		ReadyError:  readyErr,
		UpdatedAt:   time.Now().UTC(),
	})
}

// ProbeReadiness runs lightweight readiness checks for modules that implement ReadyChecker.
func (h *HealthMonitor) ProbeReadiness(ctx context.Context, modules []ServiceModule, depsReadyFunc func(string) (bool, []string)) {
	for _, mod := range modules {
		rc, ok := mod.(ReadyChecker)
		if !ok {
			continue
		}

		err := rc.Ready(ctx)
		readyStatus := ReadyStatusReady
		readyErr := ""

		if err != nil {
			readyStatus = ReadyStatusNotReady
			readyErr = err.Error()
		}

		// Check dependencies if function provided
		if depsReadyFunc != nil {
			if ok, reasons := depsReadyFunc(mod.Name()); !ok {
				readyStatus = ReadyStatusNotReady
				if readyErr == "" && len(reasons) > 0 {
					readyErr = "waiting for dependencies: " + strings.Join(reasons, "; ")
				} else if len(reasons) > 0 {
					readyErr = readyErr + " (deps: " + strings.Join(reasons, "; ") + ")"
				}
			}
		}

		h.SetReadyStatus(mod.Name(), mod.Domain(), readyStatus, readyErr)

		// Also notify the module if it implements ReadySetter
		if setter, ok := mod.(ReadySetter); ok {
			setter.SetReady(readyStatus, readyErr)
		}
	}
}

// MarkReady updates readiness for modules.
func (h *HealthMonitor) MarkReady(status, errMsg string, modules []ServiceModule) {
	if status == "" {
		status = ReadyStatusReady
	}

	for _, mod := range modules {
		h.SetReadyStatus(mod.Name(), mod.Domain(), status, errMsg)

		if setter, ok := mod.(ReadySetter); ok {
			setter.SetReady(status, errMsg)
		}
	}
}

// MarkModulesStarted records a started status for modules.
func (h *HealthMonitor) MarkModulesStarted(modules []ServiceModule) {
	for _, mod := range modules {
		startedAt := h.GetStartedAt(mod.Name())
		if startedAt == nil {
			now := time.Now().UTC()
			startedAt = &now
		}
		h.SetHealth(mod.Name(), ModuleHealth{
			Name:      mod.Name(),
			Domain:    mod.Domain(),
			Status:    StatusStarted,
			StartedAt: startedAt,
		})
	}
}

// MarkModulesStopped records a stopped status for modules.
func (h *HealthMonitor) MarkModulesStopped(modules []ServiceModule) {
	for _, mod := range modules {
		stoppedAt := h.GetStoppedAt(mod.Name())
		if stoppedAt == nil {
			now := time.Now().UTC()
			stoppedAt = &now
		}
		h.SetHealth(mod.Name(), ModuleHealth{
			Name:        mod.Name(),
			Domain:      mod.Domain(),
			Status:      StatusStopped,
			ReadyStatus: ReadyStatusNotReady,
			ReadyError:  "",
			StoppedAt:   stoppedAt,
			StartedAt:   h.GetStartedAt(mod.Name()),
			StartNanos:  h.GetStartNanos(mod.Name()),
			StopNanos:   h.GetStopNanos(mod.Name()),
		})

		if setter, ok := mod.(ReadySetter); ok {
			setter.SetReady(ReadyStatusNotReady, "")
		}
	}
}

// DepsReadyWithReasons checks if all dependencies for a module are ready.
func DepsReadyWithReasons(health *HealthMonitor, deps []string) (bool, []string) {
	if health == nil || len(deps) == 0 {
		return true, nil
	}

	var reasons []string
	for _, dep := range deps {
		h := health.GetHealth(dep)

		status := strings.ToLower(strings.TrimSpace(h.Status))
		if status == "" || status == StatusUnknown {
			reasons = append(reasons, fmt.Sprintf("%s: not started", dep))
			continue
		}
		if status != StatusStarted {
			reasons = append(reasons, fmt.Sprintf("%s: status=%s", dep, status))
			continue
		}

		ready := strings.ToLower(strings.TrimSpace(h.ReadyStatus))
		if ready != "" && ready != ReadyStatusReady {
			reasons = append(reasons, fmt.Sprintf("%s: ready=%s", dep, ready))
			continue
		}

		if status == StatusFailed || status == StatusStopError {
			reasons = append(reasons, fmt.Sprintf("%s: status=%s", dep, status))
		}
	}

	return len(reasons) == 0, reasons
}

// firstTime returns the provided time or current time if nil.
func firstTime(t *time.Time) *time.Time {
	if t != nil {
		return t
	}
	now := time.Now().UTC()
	return &now
}
