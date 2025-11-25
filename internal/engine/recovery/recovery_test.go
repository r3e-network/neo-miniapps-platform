package recovery

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/events"
	"github.com/R3E-Network/service_layer/internal/engine/state"
)

// mockModule implements ModuleRecoverer for testing.
type mockModule struct {
	name       string
	domain     string
	status     state.Status
	startErr   error
	stopErr    error
	startCount int32
	stopCount  int32
	startDelay time.Duration
	mu         sync.Mutex
}

func newMockModule(name, domain string) *mockModule {
	return &mockModule{
		name:   name,
		domain: domain,
		status: state.StatusFailed,
	}
}

func (m *mockModule) Name() string   { return m.name }
func (m *mockModule) Domain() string { return m.domain }

func (m *mockModule) Status() state.Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *mockModule) SetStatus(s state.Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = s
}

func (m *mockModule) Start(ctx context.Context) error {
	atomic.AddInt32(&m.startCount, 1)
	if m.startDelay > 0 {
		select {
		case <-time.After(m.startDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		m.status = state.StatusFailed
		return m.startErr
	}
	m.status = state.StatusRunning
	return nil
}

func (m *mockModule) Stop(ctx context.Context) error {
	atomic.AddInt32(&m.stopCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		m.status = state.StatusStopFailed
		return m.stopErr
	}
	m.status = state.StatusStopped
	return nil
}

func (m *mockModule) StartCount() int { return int(atomic.LoadInt32(&m.startCount)) }
func (m *mockModule) StopCount() int  { return int(atomic.LoadInt32(&m.stopCount)) }

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_RegisterModule(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")

	m.RegisterModule(mod)

	st, ok := m.GetState("test")
	if !ok {
		t.Error("module should be registered")
	}
	if st.Module != "test" {
		t.Errorf("Module = %q, want 'test'", st.Module)
	}
}

func TestManager_UnregisterModule(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")

	m.RegisterModule(mod)
	m.UnregisterModule("test")

	_, ok := m.GetState("test")
	if ok {
		t.Error("module should be unregistered")
	}
}

func TestManager_TriggerRecovery_Success(t *testing.T) {
	eventLog := events.NewRingBuffer(100)
	m := NewManager(eventLog)

	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.InitialDelay = 0
	cfg.RecoveryTimeout = time.Second
	m.SetModuleConfig("test", cfg)

	var ended int32
	m.SetOnRecoveryEnd(func(module string, attempt int, err error, d time.Duration) {
		atomic.AddInt32(&ended, 1)
	})

	err := m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Errorf("TriggerRecovery failed: %v", err)
	}

	// Wait for recovery to complete
	time.Sleep(100 * time.Millisecond)

	if mod.StartCount() != 1 {
		t.Errorf("StartCount = %d, want 1", mod.StartCount())
	}
	if atomic.LoadInt32(&ended) != 1 {
		t.Errorf("ended callback count = %d, want 1", atomic.LoadInt32(&ended))
	}
	if mod.Status() != state.StatusRunning {
		t.Errorf("Status = %v, want Running", mod.Status())
	}
}

func TestManager_TriggerRecovery_Failure(t *testing.T) {
	m := NewManager(nil)

	mod := newMockModule("test", "domain")
	mod.startErr = errors.New("start failed")
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.InitialDelay = 0
	cfg.RecoveryTimeout = time.Second
	m.SetModuleConfig("test", cfg)

	err := m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Errorf("TriggerRecovery failed: %v", err)
	}

	// Wait for recovery to complete
	time.Sleep(100 * time.Millisecond)

	st, _ := m.GetState("test")
	if st.LastError == nil {
		t.Error("LastError should be set")
	}
	if st.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", st.Attempts)
	}
}

func TestManager_TriggerRecovery_ModuleNotFound(t *testing.T) {
	m := NewManager(nil)

	err := m.TriggerRecovery(context.Background(), "nonexistent")
	if !errors.Is(err, ErrModuleNotRecoverable) {
		t.Errorf("err = %v, want ErrModuleNotRecoverable", err)
	}
}

func TestManager_TriggerRecovery_Disabled(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.Strategy = StrategyNone
	m.SetModuleConfig("test", cfg)

	err := m.TriggerRecovery(context.Background(), "test")
	if !errors.Is(err, ErrRecoveryDisabled) {
		t.Errorf("err = %v, want ErrRecoveryDisabled", err)
	}
}

func TestManager_TriggerRecovery_InProgress(t *testing.T) {
	m := NewManager(nil)

	mod := newMockModule("test", "domain")
	mod.startDelay = time.Second // Slow start
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.InitialDelay = 0
	m.SetModuleConfig("test", cfg)

	// Trigger first recovery
	err := m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Errorf("First TriggerRecovery failed: %v", err)
	}

	// Try to trigger again while in progress
	err = m.TriggerRecovery(context.Background(), "test")
	if !errors.Is(err, ErrRecoveryInProgress) {
		t.Errorf("err = %v, want ErrRecoveryInProgress", err)
	}
}

func TestManager_MaxRetries(t *testing.T) {
	m := NewManager(nil)

	mod := newMockModule("test", "domain")
	mod.startErr = errors.New("always fails")
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.Strategy = StrategyRestart
	cfg.MaxRetries = 2
	cfg.InitialDelay = 0
	cfg.RecoveryTimeout = 100 * time.Millisecond
	m.SetModuleConfig("test", cfg)

	// First attempt
	_ = m.TriggerRecovery(context.Background(), "test")
	time.Sleep(150 * time.Millisecond)

	// Second attempt
	_ = m.TriggerRecovery(context.Background(), "test")
	time.Sleep(150 * time.Millisecond)

	// Third attempt should fail with max retries exceeded
	err := m.TriggerRecovery(context.Background(), "test")
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("err = %v, want ErrMaxRetriesExceeded", err)
	}
}

func TestManager_CircuitBreaker(t *testing.T) {
	m := NewManager(nil)

	mod := newMockModule("test", "domain")
	mod.startErr = errors.New("always fails")
	m.RegisterModule(mod)

	cfg := DefaultConfig()
	cfg.Strategy = StrategyCircuitBreaker
	cfg.CircuitBreakerThreshold = 2
	cfg.CircuitBreakerResetTime = 200 * time.Millisecond
	cfg.MaxRetries = 0 // Unlimited
	cfg.InitialDelay = 0
	cfg.RecoveryTimeout = 50 * time.Millisecond
	m.SetModuleConfig("test", cfg)

	// Helper to wait for recovery completion
	waitForCompletion := func() {
		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			st, _ := m.GetState("test")
			if !st.InProgress {
				return
			}
		}
	}

	// First attempt
	err := m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Fatalf("first attempt failed: %v", err)
	}
	waitForCompletion()

	// Second attempt (should trip circuit breaker after completion)
	err = m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Fatalf("second attempt failed: %v", err)
	}
	waitForCompletion()

	// Third attempt should fail with circuit breaker open
	err = m.TriggerRecovery(context.Background(), "test")
	if !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Errorf("err = %v, want ErrCircuitBreakerOpen", err)
	}

	if !m.IsCircuitOpen("test") {
		t.Error("circuit should be open")
	}

	// Wait for circuit breaker to reset
	time.Sleep(250 * time.Millisecond)

	if m.IsCircuitOpen("test") {
		t.Error("circuit should be closed after reset time")
	}
}

func TestManager_ExponentialBackoff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Strategy = StrategyBackoff
	cfg.InitialDelay = 100 * time.Millisecond
	cfg.MaxDelay = time.Second
	cfg.Multiplier = 2.0

	m := NewManager(nil)

	// Test delay calculation
	d1 := m.calculateDelay(cfg, 1)
	if d1 != 100*time.Millisecond {
		t.Errorf("delay for attempt 1 = %v, want 100ms", d1)
	}

	d2 := m.calculateDelay(cfg, 2)
	if d2 != 200*time.Millisecond {
		t.Errorf("delay for attempt 2 = %v, want 200ms", d2)
	}

	d3 := m.calculateDelay(cfg, 3)
	if d3 != 400*time.Millisecond {
		t.Errorf("delay for attempt 3 = %v, want 400ms", d3)
	}

	// Should cap at max delay
	d10 := m.calculateDelay(cfg, 10)
	if d10 != time.Second {
		t.Errorf("delay for attempt 10 = %v, want 1s (max)", d10)
	}
}

func TestManager_ResetState(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	// Manually set some state
	m.mu.Lock()
	st := m.states["test"]
	st.Attempts = 5
	st.LastError = errors.New("test error")
	st.CircuitOpen = true
	m.mu.Unlock()

	m.ResetState("test")

	st2, _ := m.GetState("test")
	if st2.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0", st2.Attempts)
	}
	if st2.LastError != nil {
		t.Error("LastError should be nil")
	}
	if st2.CircuitOpen {
		t.Error("CircuitOpen should be false")
	}
}

func TestManager_AbortRecovery(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	// Manually mark as in progress
	m.mu.Lock()
	m.states["test"].InProgress = true
	m.mu.Unlock()

	aborted := m.AbortRecovery("test")
	if !aborted {
		t.Error("should have aborted recovery")
	}

	st, _ := m.GetState("test")
	if st.InProgress {
		t.Error("InProgress should be false after abort")
	}
}

func TestManager_DisableEnableRecovery(t *testing.T) {
	m := NewManager(nil)
	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	m.DisableRecovery("test")

	err := m.TriggerRecovery(context.Background(), "test")
	if !errors.Is(err, ErrRecoveryDisabled) {
		t.Errorf("err = %v, want ErrRecoveryDisabled", err)
	}

	m.EnableRecovery("test", StrategyBackoff)

	err = m.TriggerRecovery(context.Background(), "test")
	if err != nil {
		t.Errorf("recovery should be enabled: %v", err)
	}
}

func TestManager_GetAllStates(t *testing.T) {
	m := NewManager(nil)
	m.RegisterModule(newMockModule("mod1", "domain1"))
	m.RegisterModule(newMockModule("mod2", "domain2"))

	states := m.GetAllStates()
	if len(states) != 2 {
		t.Errorf("len(states) = %d, want 2", len(states))
	}
}

func TestManager_GetRecoveryInfo(t *testing.T) {
	m := NewManager(nil)
	m.RegisterModule(newMockModule("mod1", "domain1"))
	m.RegisterModule(newMockModule("mod2", "domain2"))

	info := m.GetRecoveryInfo()
	if len(info) != 2 {
		t.Errorf("len(info) = %d, want 2", len(info))
	}
}

func TestManager_Shutdown(t *testing.T) {
	m := NewManager(nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := m.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestManager_SetDefaultConfig(t *testing.T) {
	m := NewManager(nil)

	cfg := DefaultConfig()
	cfg.MaxRetries = 10
	m.SetDefaultConfig(cfg)

	mod := newMockModule("test", "domain")
	m.RegisterModule(mod)

	info := m.GetRecoveryInfo()
	if len(info) != 1 {
		t.Fatal("expected 1 recovery info")
	}
	if info[0].MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10", info[0].MaxRetries)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Strategy != StrategyBackoff {
		t.Errorf("Strategy = %v, want StrategyBackoff", cfg.Strategy)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", cfg.Multiplier)
	}
}
