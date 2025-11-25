// Package bridge provides integration between the Framework service model
// and the Engine runtime. It translates Framework concepts (Manifest, ServiceBase)
// into Engine concepts (state, events, metrics, recovery).
package bridge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/bus"
	"github.com/R3E-Network/service_layer/internal/engine/events"
	enginemetrics "github.com/R3E-Network/service_layer/internal/engine/metrics"
	"github.com/R3E-Network/service_layer/internal/engine/recovery"
	"github.com/R3E-Network/service_layer/internal/engine/state"
)

// ServiceAdapter adapts a Framework service to the Engine's expectations.
// It tracks state, emits events, collects metrics, and enables recovery.
type ServiceAdapter struct {
	mu sync.RWMutex

	name   string
	domain string

	// State
	status    state.Status
	readiness state.Readiness
	lastError error

	// Timing
	startedAt time.Time
	stoppedAt time.Time

	// Callbacks
	startFn func(context.Context) error
	stopFn  func(context.Context) error
	readyFn func(context.Context) error

	// Integration
	events  events.EventLogger
	metrics enginemetrics.MetricsCollector
}

// AdapterOption configures a ServiceAdapter.
type AdapterOption func(*ServiceAdapter)

// WithStartFunc sets the start callback.
func WithStartFunc(fn func(context.Context) error) AdapterOption {
	return func(a *ServiceAdapter) {
		a.startFn = fn
	}
}

// WithStopFunc sets the stop callback.
func WithStopFunc(fn func(context.Context) error) AdapterOption {
	return func(a *ServiceAdapter) {
		a.stopFn = fn
	}
}

// WithReadyFunc sets the ready check callback.
func WithReadyFunc(fn func(context.Context) error) AdapterOption {
	return func(a *ServiceAdapter) {
		a.readyFn = fn
	}
}

// WithEventLogger sets the event logger.
func WithEventLogger(el events.EventLogger) AdapterOption {
	return func(a *ServiceAdapter) {
		a.events = el
	}
}

// WithMetricsCollector sets the metrics collector.
func WithMetricsCollector(mc enginemetrics.MetricsCollector) AdapterOption {
	return func(a *ServiceAdapter) {
		a.metrics = mc
	}
}

// NewServiceAdapter creates a new service adapter.
func NewServiceAdapter(name, domain string, opts ...AdapterOption) *ServiceAdapter {
	a := &ServiceAdapter{
		name:      name,
		domain:    domain,
		status:    state.StatusUnknown,
		readiness: state.ReadinessUnknown,
		events:    events.NoOpLogger{},
		metrics:   enginemetrics.NewNoOpCollector(),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Name returns the service name.
func (a *ServiceAdapter) Name() string { return a.name }

// Domain returns the service domain.
func (a *ServiceAdapter) Domain() string { return a.domain }

// Status returns the current status.
func (a *ServiceAdapter) Status() state.Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// Readiness returns the current readiness.
func (a *ServiceAdapter) Readiness() state.Readiness {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.readiness
}

// Health returns combined health status.
func (a *ServiceAdapter) Health() state.Health {
	a.mu.RLock()
	defer a.mu.RUnlock()
	h := state.NewHealth(a.status, a.readiness)
	if a.lastError != nil {
		h.Error = a.lastError.Error()
	}
	return h
}

// SetStatus atomically updates the status with validation.
func (a *ServiceAdapter) SetStatus(newStatus state.Status) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.status == newStatus {
		return nil
	}

	if !state.CanTransition(a.status, newStatus) {
		return state.NewTransitionError(a.status, newStatus)
	}

	oldStatus := a.status
	a.status = newStatus
	a.metrics.RecordModuleStatus(a.name, a.domain, int(newStatus))

	a.events.Log(events.Event{
		Type:     statusToEventType(newStatus),
		Module:   a.name,
		Severity: statusToSeverity(newStatus),
		Status:   newStatus,
		Message:  fmt.Sprintf("status changed: %s -> %s", oldStatus, newStatus),
	})

	return nil
}

// SetReadiness updates the readiness state.
func (a *ServiceAdapter) SetReadiness(r state.Readiness) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.readiness == r {
		return
	}

	a.readiness = r
	a.metrics.RecordModuleReadiness(a.name, a.domain, int(r))

	a.events.Log(events.Event{
		Type:      events.EventReadinessChanged,
		Module:    a.name,
		Readiness: r,
		Message:   fmt.Sprintf("readiness: %s", r),
	})
}

// Start starts the service with proper state tracking.
func (a *ServiceAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if !a.status.CanStart() {
		a.mu.Unlock()
		return fmt.Errorf("cannot start from status %s", a.status)
	}
	a.mu.Unlock()

	if err := a.SetStatus(state.StatusStarting); err != nil {
		return err
	}

	start := time.Now()

	var err error
	if a.startFn != nil {
		err = a.startFn(ctx)
	}

	duration := time.Since(start)
	a.metrics.RecordModuleStart(a.name, a.domain, duration, err)

	if err != nil {
		a.mu.Lock()
		a.lastError = err
		a.mu.Unlock()
		_ = a.SetStatus(state.StatusFailed)
		return err
	}

	a.mu.Lock()
	a.startedAt = time.Now()
	a.lastError = nil
	a.mu.Unlock()

	if err := a.SetStatus(state.StatusRunning); err != nil {
		return err
	}

	// Set readiness if not already set
	a.mu.RLock()
	isUnknown := a.readiness == state.ReadinessUnknown
	a.mu.RUnlock()

	if isUnknown {
		a.SetReadiness(state.ReadinessReady)
	}

	return nil
}

// Stop stops the service with proper state tracking.
func (a *ServiceAdapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	if !a.status.CanStop() {
		a.mu.Unlock()
		return fmt.Errorf("cannot stop from status %s", a.status)
	}
	a.mu.Unlock()

	if err := a.SetStatus(state.StatusStopping); err != nil {
		return err
	}

	// Set not ready immediately
	a.SetReadiness(state.ReadinessNotReady)

	start := time.Now()

	var err error
	if a.stopFn != nil {
		err = a.stopFn(ctx)
	}

	duration := time.Since(start)
	a.metrics.RecordModuleStop(a.name, a.domain, duration, err)

	a.mu.Lock()
	a.stoppedAt = time.Now()
	a.mu.Unlock()

	if err != nil {
		a.mu.Lock()
		a.lastError = err
		a.mu.Unlock()
		_ = a.SetStatus(state.StatusStopFailed)
		return err
	}

	return a.SetStatus(state.StatusStopped)
}

// Ready checks if the service is ready.
func (a *ServiceAdapter) Ready(ctx context.Context) error {
	a.mu.RLock()
	status := a.status
	a.mu.RUnlock()

	if status != state.StatusRunning {
		return fmt.Errorf("service not running: %s", status)
	}

	if a.readyFn != nil {
		err := a.readyFn(ctx)
		if err != nil {
			a.SetReadiness(state.ReadinessNotReady)
			return err
		}
	}

	a.SetReadiness(state.ReadinessReady)
	return nil
}

// Uptime returns how long the service has been running.
func (a *ServiceAdapter) Uptime() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.startedAt.IsZero() {
		return 0
	}
	if !a.stoppedAt.IsZero() && a.stoppedAt.After(a.startedAt) {
		return a.stoppedAt.Sub(a.startedAt)
	}
	return time.Since(a.startedAt)
}

// LastError returns the last error.
func (a *ServiceAdapter) LastError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastError
}

func statusToEventType(s state.Status) events.EventType {
	switch s {
	case state.StatusRegistered:
		return events.EventModuleRegistered
	case state.StatusStarting:
		return events.EventModuleStarting
	case state.StatusRunning:
		return events.EventModuleStarted
	case state.StatusStopping:
		return events.EventModuleStopping
	case state.StatusStopped:
		return events.EventModuleStopped
	case state.StatusFailed:
		return events.EventModuleStartFailed
	case state.StatusStopFailed:
		return events.EventModuleStopFailed
	default:
		return events.EventModuleRegistered
	}
}

func statusToSeverity(s state.Status) events.Severity {
	switch s {
	case state.StatusFailed, state.StatusStopFailed:
		return events.SeverityError
	case state.StatusStarting, state.StatusStopping:
		return events.SeverityInfo
	default:
		return events.SeverityInfo
	}
}

// Runtime bundles the engine components for service integration.
type Runtime struct {
	Events     events.EventLogger
	Metrics    enginemetrics.MetricsCollector
	Recovery   *recovery.Manager
	BusLimiter *bus.BusLimiter
}

// NewRuntime creates a new engine runtime with all components.
func NewRuntime(eventBufferSize int, metricsNamespace string) *Runtime {
	el := events.NewRingBuffer(eventBufferSize)

	return &Runtime{
		Events:     el,
		Metrics:    enginemetrics.NewCollector(metricsNamespace),
		Recovery:   recovery.NewManager(el),
		BusLimiter: bus.NewBusLimiter(),
	}
}

// NewNoOpRuntime creates a runtime with no-op implementations.
func NewNoOpRuntime() *Runtime {
	return &Runtime{
		Events:     events.NoOpLogger{},
		Metrics:    enginemetrics.NewNoOpCollector(),
		Recovery:   recovery.NewManager(nil),
		BusLimiter: bus.NewBusLimiter(),
	}
}

// Configure applies bus limiter configuration.
func (r *Runtime) Configure(cfg RuntimeConfig) {
	if cfg.EventBusConcurrency > 0 {
		r.BusLimiter.Configure(bus.KindEvent, bus.LimiterConfig{
			MaxConcurrent:  cfg.EventBusConcurrency,
			AcquireTimeout: cfg.BusAcquireTimeout,
		})
	}
	if cfg.DataBusConcurrency > 0 {
		r.BusLimiter.Configure(bus.KindData, bus.LimiterConfig{
			MaxConcurrent:  cfg.DataBusConcurrency,
			AcquireTimeout: cfg.BusAcquireTimeout,
		})
	}
	if cfg.ComputeBusConcurrency > 0 {
		r.BusLimiter.Configure(bus.KindCompute, bus.LimiterConfig{
			MaxConcurrent:  cfg.ComputeBusConcurrency,
			AcquireTimeout: cfg.BusAcquireTimeout,
		})
	}
}

// Shutdown gracefully shuts down all components.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.BusLimiter.Close()
	return r.Recovery.Shutdown(ctx)
}

// RuntimeConfig holds configuration for the engine runtime.
type RuntimeConfig struct {
	EventBusConcurrency   int
	DataBusConcurrency    int
	ComputeBusConcurrency int
	BusAcquireTimeout     time.Duration
}

// DefaultRuntimeConfig returns sensible defaults.
func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		EventBusConcurrency:   100,
		DataBusConcurrency:    100,
		ComputeBusConcurrency: 50,
		BusAcquireTimeout:     30 * time.Second,
	}
}

// AdaptService creates a ServiceAdapter from callbacks.
func AdaptService(
	name, domain string,
	startFn, stopFn, readyFn func(context.Context) error,
	runtime *Runtime,
) *ServiceAdapter {
	opts := []AdapterOption{
		WithStartFunc(startFn),
		WithStopFunc(stopFn),
		WithReadyFunc(readyFn),
	}
	if runtime != nil {
		opts = append(opts,
			WithEventLogger(runtime.Events),
			WithMetricsCollector(runtime.Metrics),
		)
	}
	return NewServiceAdapter(name, domain, opts...)
}

// RegisterForRecovery registers a service adapter with the recovery manager.
func RegisterForRecovery(adapter *ServiceAdapter, rm *recovery.Manager, cfg recovery.Config) {
	if rm == nil || adapter == nil {
		return
	}
	rm.RegisterModule(adapter)
	rm.SetModuleConfig(adapter.Name(), cfg)
}
