// Package recovery provides automatic module recovery for the engine.
// It implements configurable recovery strategies including restart,
// exponential backoff, and circuit breaker patterns.
package recovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/events"
	"github.com/R3E-Network/service_layer/internal/engine/state"
)

// Common errors
var (
	ErrRecoveryInProgress   = errors.New("recovery already in progress")
	ErrRecoveryDisabled     = errors.New("recovery disabled for module")
	ErrMaxRetriesExceeded   = errors.New("max recovery retries exceeded")
	ErrCircuitBreakerOpen   = errors.New("circuit breaker is open")
	ErrRecoveryAborted      = errors.New("recovery aborted")
	ErrModuleNotRecoverable = errors.New("module is not recoverable")
)

// Strategy defines the recovery strategy type.
type Strategy string

const (
	// StrategyRestart simply restarts the module.
	StrategyRestart Strategy = "restart"

	// StrategyBackoff uses exponential backoff between retries.
	StrategyBackoff Strategy = "backoff"

	// StrategyCircuitBreaker stops recovery after repeated failures.
	StrategyCircuitBreaker Strategy = "circuit_breaker"

	// StrategyNone disables recovery for the module.
	StrategyNone Strategy = "none"
)

// Config holds recovery configuration for a module.
type Config struct {
	// Strategy is the recovery strategy to use.
	Strategy Strategy

	// MaxRetries is the maximum number of recovery attempts (0 = unlimited).
	MaxRetries int

	// InitialDelay is the initial delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (for StrategyBackoff).
	Multiplier float64

	// CircuitBreakerThreshold is the number of failures before opening (for StrategyCircuitBreaker).
	CircuitBreakerThreshold int

	// CircuitBreakerResetTime is how long the circuit stays open.
	CircuitBreakerResetTime time.Duration

	// RecoveryTimeout is the maximum time for a recovery attempt.
	RecoveryTimeout time.Duration
}

// DefaultConfig returns the default recovery configuration.
func DefaultConfig() Config {
	return Config{
		Strategy:                StrategyBackoff,
		MaxRetries:              5,
		InitialDelay:            time.Second,
		MaxDelay:                time.Minute,
		Multiplier:              2.0,
		CircuitBreakerThreshold: 3,
		CircuitBreakerResetTime: 5 * time.Minute,
		RecoveryTimeout:         30 * time.Second,
	}
}

// ModuleRecoverer defines the interface for modules that can be recovered.
type ModuleRecoverer interface {
	// Name returns the module name.
	Name() string

	// Domain returns the module domain.
	Domain() string

	// Status returns the current status.
	Status() state.Status

	// Start starts the module.
	Start(ctx context.Context) error

	// Stop stops the module.
	Stop(ctx context.Context) error
}

// RecoveryState tracks the recovery state of a module.
type RecoveryState struct {
	Module        string
	Domain        string
	InProgress    bool
	Attempts      int
	LastAttempt   time.Time
	LastError     error
	NextRetry     time.Time
	CurrentDelay  time.Duration
	CircuitOpen   bool
	CircuitOpened time.Time
}

// Manager manages recovery for all modules.
type Manager struct {
	mu           sync.RWMutex
	configs      map[string]Config
	states       map[string]*RecoveryState
	modules      map[string]ModuleRecoverer
	events       events.EventLogger
	defaultCfg   Config
	shutdownCh   chan struct{}
	wg           sync.WaitGroup

	// Callbacks
	onRecoveryStart func(module string, attempt int)
	onRecoveryEnd   func(module string, attempt int, err error, duration time.Duration)
}

// NewManager creates a new recovery manager.
func NewManager(eventLogger events.EventLogger) *Manager {
	if eventLogger == nil {
		eventLogger = events.NoOpLogger{}
	}
	return &Manager{
		configs:    make(map[string]Config),
		states:     make(map[string]*RecoveryState),
		modules:    make(map[string]ModuleRecoverer),
		events:     eventLogger,
		defaultCfg: DefaultConfig(),
		shutdownCh: make(chan struct{}),
	}
}

// SetDefaultConfig sets the default recovery configuration.
func (m *Manager) SetDefaultConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultCfg = cfg
}

// SetModuleConfig sets recovery configuration for a specific module.
func (m *Manager) SetModuleConfig(module string, cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[module] = cfg
}

// RegisterModule registers a module for recovery management.
func (m *Manager) RegisterModule(module ModuleRecoverer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := module.Name()
	m.modules[name] = module
	if _, ok := m.states[name]; !ok {
		m.states[name] = &RecoveryState{
			Module: name,
			Domain: module.Domain(),
		}
	}
}

// UnregisterModule removes a module from recovery management.
func (m *Manager) UnregisterModule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.modules, name)
	delete(m.states, name)
	delete(m.configs, name)
}

// SetOnRecoveryStart sets the callback for recovery start.
func (m *Manager) SetOnRecoveryStart(fn func(module string, attempt int)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRecoveryStart = fn
}

// SetOnRecoveryEnd sets the callback for recovery end.
func (m *Manager) SetOnRecoveryEnd(fn func(module string, attempt int, err error, duration time.Duration)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRecoveryEnd = fn
}

// TriggerRecovery initiates recovery for a failed module.
func (m *Manager) TriggerRecovery(ctx context.Context, moduleName string) error {
	m.mu.Lock()
	module, ok := m.modules[moduleName]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("module %s not registered: %w", moduleName, ErrModuleNotRecoverable)
	}

	cfg := m.getConfig(moduleName)
	if cfg.Strategy == StrategyNone {
		m.mu.Unlock()
		return ErrRecoveryDisabled
	}

	st := m.getOrCreateState(moduleName, module.Domain())
	if st.InProgress {
		m.mu.Unlock()
		return ErrRecoveryInProgress
	}

	// Check circuit breaker
	if cfg.Strategy == StrategyCircuitBreaker && st.CircuitOpen {
		if time.Since(st.CircuitOpened) < cfg.CircuitBreakerResetTime {
			m.mu.Unlock()
			return ErrCircuitBreakerOpen
		}
		// Reset circuit breaker (half-open state)
		st.CircuitOpen = false
		st.Attempts = 0
	}

	// Check max retries
	if cfg.MaxRetries > 0 && st.Attempts >= cfg.MaxRetries {
		m.mu.Unlock()
		return ErrMaxRetriesExceeded
	}

	st.InProgress = true
	st.Attempts++
	attempt := st.Attempts
	onStart := m.onRecoveryStart
	onEnd := m.onRecoveryEnd
	m.mu.Unlock()

	// Log recovery start event
	m.events.Log(events.Event{
		Type:     events.EventRecoveryStarted,
		Module:   moduleName,
		Severity: events.SeverityWarning,
		Message:  fmt.Sprintf("recovery attempt %d started", attempt),
		Metadata: map[string]string{
			"strategy": string(cfg.Strategy),
			"attempt":  fmt.Sprintf("%d", attempt),
		},
	})

	if onStart != nil {
		onStart(moduleName, attempt)
	}

	// Start recovery in background
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.executeRecovery(ctx, module, cfg, st, attempt, onEnd)
	}()

	return nil
}

func (m *Manager) executeRecovery(
	ctx context.Context,
	module ModuleRecoverer,
	cfg Config,
	st *RecoveryState,
	attempt int,
	onEnd func(string, int, error, time.Duration),
) {
	moduleName := module.Name()
	start := time.Now()

	// Calculate delay based on strategy
	delay := m.calculateDelay(cfg, attempt)

	// Wait for delay (unless first attempt)
	if attempt > 1 && delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			m.completeRecovery(st, moduleName, attempt, ErrRecoveryAborted, time.Since(start), onEnd)
			return
		case <-m.shutdownCh:
			m.completeRecovery(st, moduleName, attempt, ErrRecoveryAborted, time.Since(start), onEnd)
			return
		}
	}

	// Create recovery context with timeout
	recoveryCtx := ctx
	if cfg.RecoveryTimeout > 0 {
		var cancel context.CancelFunc
		recoveryCtx, cancel = context.WithTimeout(ctx, cfg.RecoveryTimeout)
		defer cancel()
	}

	// Attempt to stop the module first (if it's not already stopped)
	status := module.Status()
	if status != state.StatusStopped && status != state.StatusFailed {
		_ = module.Stop(recoveryCtx) // Ignore stop errors during recovery
	}

	// Attempt to start the module
	err := module.Start(recoveryCtx)
	duration := time.Since(start)

	m.completeRecovery(st, moduleName, attempt, err, duration, onEnd)
}

func (m *Manager) completeRecovery(
	st *RecoveryState,
	moduleName string,
	attempt int,
	err error,
	duration time.Duration,
	onEnd func(string, int, error, time.Duration),
) {
	m.mu.Lock()
	st.InProgress = false
	st.LastAttempt = time.Now()
	st.LastError = err

	cfg := m.getConfig(moduleName)

	if err != nil {
		// Handle failure
		st.CurrentDelay = m.calculateDelay(cfg, attempt+1)
		st.NextRetry = time.Now().Add(st.CurrentDelay)

		// Check if we should open circuit breaker
		if cfg.Strategy == StrategyCircuitBreaker &&
			st.Attempts >= cfg.CircuitBreakerThreshold {
			st.CircuitOpen = true
			st.CircuitOpened = time.Now()
		}

		m.mu.Unlock()

		m.events.Log(events.Event{
			Type:     events.EventRecoveryFailed,
			Module:   moduleName,
			Severity: events.SeverityError,
			Message:  fmt.Sprintf("recovery attempt %d failed", attempt),
			Error:    err.Error(),
			Duration: duration,
			Metadata: map[string]string{
				"attempt":    fmt.Sprintf("%d", attempt),
				"next_retry": st.NextRetry.Format(time.RFC3339),
			},
		})
	} else {
		// Handle success - reset state
		st.Attempts = 0
		st.CurrentDelay = 0
		st.CircuitOpen = false

		m.mu.Unlock()

		m.events.Log(events.Event{
			Type:     events.EventRecoverySucceeded,
			Module:   moduleName,
			Severity: events.SeverityInfo,
			Message:  fmt.Sprintf("recovery attempt %d succeeded", attempt),
			Duration: duration,
		})
	}

	if onEnd != nil {
		onEnd(moduleName, attempt, err, duration)
	}
}

func (m *Manager) calculateDelay(cfg Config, attempt int) time.Duration {
	if attempt <= 1 {
		return cfg.InitialDelay
	}

	switch cfg.Strategy {
	case StrategyBackoff:
		// Exponential backoff
		delay := float64(cfg.InitialDelay)
		for i := 1; i < attempt; i++ {
			delay *= cfg.Multiplier
			if delay > float64(cfg.MaxDelay) {
				delay = float64(cfg.MaxDelay)
				break
			}
		}
		return time.Duration(delay)

	case StrategyRestart, StrategyCircuitBreaker:
		return cfg.InitialDelay

	default:
		return cfg.InitialDelay
	}
}

func (m *Manager) getConfig(module string) Config {
	if cfg, ok := m.configs[module]; ok {
		return cfg
	}
	return m.defaultCfg
}

func (m *Manager) getOrCreateState(module, domain string) *RecoveryState {
	if st, ok := m.states[module]; ok {
		return st
	}
	st := &RecoveryState{
		Module: module,
		Domain: domain,
	}
	m.states[module] = st
	return st
}

// GetState returns the recovery state for a module.
func (m *Manager) GetState(module string) (RecoveryState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if st, ok := m.states[module]; ok {
		return *st, true
	}
	return RecoveryState{}, false
}

// GetAllStates returns recovery states for all registered modules.
func (m *Manager) GetAllStates() []RecoveryState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]RecoveryState, 0, len(m.states))
	for _, st := range m.states {
		result = append(result, *st)
	}
	return result
}

// ResetState resets the recovery state for a module.
func (m *Manager) ResetState(module string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.states[module]; ok {
		st.Attempts = 0
		st.LastError = nil
		st.CurrentDelay = 0
		st.CircuitOpen = false
		st.InProgress = false
	}
}

// IsCircuitOpen returns whether the circuit breaker is open for a module.
func (m *Manager) IsCircuitOpen(module string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if st, ok := m.states[module]; ok {
		if !st.CircuitOpen {
			return false
		}
		cfg := m.getConfig(module)
		// Check if circuit should be reset (half-open)
		return time.Since(st.CircuitOpened) < cfg.CircuitBreakerResetTime
	}
	return false
}

// Shutdown gracefully shuts down the recovery manager.
func (m *Manager) Shutdown(ctx context.Context) error {
	close(m.shutdownCh)

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AbortRecovery aborts an ongoing recovery for a module.
func (m *Manager) AbortRecovery(module string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.states[module]; ok && st.InProgress {
		// Mark as aborted - the recovery goroutine will check this
		st.InProgress = false

		m.events.Log(events.Event{
			Type:     events.EventRecoveryAborted,
			Module:   module,
			Severity: events.SeverityWarning,
			Message:  "recovery aborted by request",
		})
		return true
	}
	return false
}

// DisableRecovery disables recovery for a module.
func (m *Manager) DisableRecovery(module string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.getConfig(module)
	cfg.Strategy = StrategyNone
	m.configs[module] = cfg
}

// EnableRecovery enables recovery for a module with the specified strategy.
func (m *Manager) EnableRecovery(module string, strategy Strategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.getConfig(module)
	cfg.Strategy = strategy
	m.configs[module] = cfg
}

// RecoveryInfo provides a summary of recovery information for status endpoints.
type RecoveryInfo struct {
	Module        string        `json:"module"`
	Domain        string        `json:"domain"`
	Strategy      Strategy      `json:"strategy"`
	InProgress    bool          `json:"in_progress"`
	Attempts      int           `json:"attempts"`
	MaxRetries    int           `json:"max_retries"`
	LastError     string        `json:"last_error,omitempty"`
	NextRetry     *time.Time    `json:"next_retry,omitempty"`
	CircuitOpen   bool          `json:"circuit_open"`
	CurrentDelay  time.Duration `json:"current_delay_ns,omitempty"`
}

// GetRecoveryInfo returns recovery info for all modules.
func (m *Manager) GetRecoveryInfo() []RecoveryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RecoveryInfo, 0, len(m.states))
	for name, st := range m.states {
		cfg := m.getConfig(name)
		info := RecoveryInfo{
			Module:       st.Module,
			Domain:       st.Domain,
			Strategy:     cfg.Strategy,
			InProgress:   st.InProgress,
			Attempts:     st.Attempts,
			MaxRetries:   cfg.MaxRetries,
			CircuitOpen:  st.CircuitOpen,
			CurrentDelay: st.CurrentDelay,
		}
		if st.LastError != nil {
			info.LastError = st.LastError.Error()
		}
		if !st.NextRetry.IsZero() && st.Attempts > 0 {
			info.NextRetry = &st.NextRetry
		}
		result = append(result, info)
	}
	return result
}
