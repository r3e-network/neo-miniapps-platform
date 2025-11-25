package bridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/events"
	"github.com/R3E-Network/service_layer/internal/engine/recovery"
	"github.com/R3E-Network/service_layer/internal/engine/state"
)

func TestNewServiceAdapter(t *testing.T) {
	a := NewServiceAdapter("test-svc", "test-domain")
	if a == nil {
		t.Fatal("NewServiceAdapter returned nil")
	}
	if a.Name() != "test-svc" {
		t.Errorf("Name() = %q, want 'test-svc'", a.Name())
	}
	if a.Domain() != "test-domain" {
		t.Errorf("Domain() = %q, want 'test-domain'", a.Domain())
	}
	if a.Status() != state.StatusUnknown {
		t.Errorf("Status() = %v, want StatusUnknown", a.Status())
	}
}

func TestServiceAdapter_StartStop(t *testing.T) {
	started := false
	stopped := false

	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error {
			started = true
			return nil
		}),
		WithStopFunc(func(ctx context.Context) error {
			stopped = true
			return nil
		}),
	)

	// Register first
	_ = a.SetStatus(state.StatusRegistered)

	// Start
	ctx := context.Background()
	err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !started {
		t.Error("startFn was not called")
	}
	if a.Status() != state.StatusRunning {
		t.Errorf("Status() = %v, want StatusRunning", a.Status())
	}
	if a.Readiness() != state.ReadinessReady {
		t.Errorf("Readiness() = %v, want ReadinessReady", a.Readiness())
	}

	// Stop
	err = a.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if !stopped {
		t.Error("stopFn was not called")
	}
	if a.Status() != state.StatusStopped {
		t.Errorf("Status() = %v, want StatusStopped", a.Status())
	}
	if a.Readiness() != state.ReadinessNotReady {
		t.Errorf("Readiness() = %v, want ReadinessNotReady", a.Readiness())
	}
}

func TestServiceAdapter_StartFailure(t *testing.T) {
	startErr := errors.New("start failed")

	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error {
			return startErr
		}),
	)

	_ = a.SetStatus(state.StatusRegistered)

	err := a.Start(context.Background())
	if err != startErr {
		t.Errorf("Start() err = %v, want %v", err, startErr)
	}
	if a.Status() != state.StatusFailed {
		t.Errorf("Status() = %v, want StatusFailed", a.Status())
	}
	if a.LastError() != startErr {
		t.Errorf("LastError() = %v, want %v", a.LastError(), startErr)
	}
}

func TestServiceAdapter_StopFailure(t *testing.T) {
	stopErr := errors.New("stop failed")

	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
		WithStopFunc(func(ctx context.Context) error { return stopErr }),
	)

	_ = a.SetStatus(state.StatusRegistered)
	_ = a.Start(context.Background())

	err := a.Stop(context.Background())
	if err != stopErr {
		t.Errorf("Stop() err = %v, want %v", err, stopErr)
	}
	if a.Status() != state.StatusStopFailed {
		t.Errorf("Status() = %v, want StatusStopFailed", a.Status())
	}
}

func TestServiceAdapter_InvalidTransition(t *testing.T) {
	a := NewServiceAdapter("test", "domain")

	// Can't start from unknown (must register first)
	err := a.Start(context.Background())
	if err == nil {
		t.Error("Start from unknown should fail")
	}

	// Register
	_ = a.SetStatus(state.StatusRegistered)

	// Can't stop from registered
	err = a.Stop(context.Background())
	if err == nil {
		t.Error("Stop from registered should fail")
	}
}

func TestServiceAdapter_SetStatus(t *testing.T) {
	eventLog := events.NewRingBuffer(100)
	a := NewServiceAdapter("test", "domain",
		WithEventLogger(eventLog),
	)

	// Valid transition
	err := a.SetStatus(state.StatusRegistered)
	if err != nil {
		t.Errorf("SetStatus(Registered) failed: %v", err)
	}

	// Invalid transition
	err = a.SetStatus(state.StatusRunning) // Can't skip Starting
	if err == nil {
		t.Error("Invalid transition should fail")
	}
	var te state.TransitionError
	if !errors.As(err, &te) {
		t.Error("Error should be TransitionError")
	}

	// Check events were logged
	recent := eventLog.Recent(10)
	if len(recent) != 1 {
		t.Errorf("Expected 1 event, got %d", len(recent))
	}
}

func TestServiceAdapter_Ready(t *testing.T) {
	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
		WithReadyFunc(func(ctx context.Context) error { return nil }),
	)

	_ = a.SetStatus(state.StatusRegistered)

	// Not running
	err := a.Ready(context.Background())
	if err == nil {
		t.Error("Ready should fail when not running")
	}

	// Start
	_ = a.Start(context.Background())

	// Now should work
	err = a.Ready(context.Background())
	if err != nil {
		t.Errorf("Ready failed: %v", err)
	}
	if a.Readiness() != state.ReadinessReady {
		t.Errorf("Readiness() = %v, want ReadinessReady", a.Readiness())
	}
}

func TestServiceAdapter_ReadyFailure(t *testing.T) {
	readyErr := errors.New("not ready")

	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
		WithReadyFunc(func(ctx context.Context) error { return readyErr }),
	)

	_ = a.SetStatus(state.StatusRegistered)
	_ = a.Start(context.Background())

	err := a.Ready(context.Background())
	if err != readyErr {
		t.Errorf("Ready() err = %v, want %v", err, readyErr)
	}
	if a.Readiness() != state.ReadinessNotReady {
		t.Errorf("Readiness() = %v, want ReadinessNotReady", a.Readiness())
	}
}

func TestServiceAdapter_Health(t *testing.T) {
	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
	)

	_ = a.SetStatus(state.StatusRegistered)
	_ = a.Start(context.Background())

	h := a.Health()
	if h.Status != state.StatusRunning {
		t.Errorf("Health.Status = %v, want StatusRunning", h.Status)
	}
	if h.Readiness != state.ReadinessReady {
		t.Errorf("Health.Readiness = %v, want ReadinessReady", h.Readiness)
	}
	if !h.IsHealthy() {
		t.Error("Health.IsHealthy() = false, want true")
	}
}

func TestServiceAdapter_Uptime(t *testing.T) {
	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
	)

	if a.Uptime() != 0 {
		t.Errorf("Uptime before start = %v, want 0", a.Uptime())
	}

	_ = a.SetStatus(state.StatusRegistered)
	_ = a.Start(context.Background())

	time.Sleep(10 * time.Millisecond)
	uptime := a.Uptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("Uptime = %v, want >= 10ms", uptime)
	}
}

func TestNewRuntime(t *testing.T) {
	rt := NewRuntime(1000, "test")
	if rt == nil {
		t.Fatal("NewRuntime returned nil")
	}
	if rt.Events == nil {
		t.Error("Events is nil")
	}
	if rt.Metrics == nil {
		t.Error("Metrics is nil")
	}
	if rt.Recovery == nil {
		t.Error("Recovery is nil")
	}
	if rt.BusLimiter == nil {
		t.Error("BusLimiter is nil")
	}
}

func TestNewNoOpRuntime(t *testing.T) {
	rt := NewNoOpRuntime()
	if rt == nil {
		t.Fatal("NewNoOpRuntime returned nil")
	}
}

func TestRuntime_Configure(t *testing.T) {
	rt := NewRuntime(100, "test")

	cfg := RuntimeConfig{
		EventBusConcurrency:   10,
		DataBusConcurrency:    20,
		ComputeBusConcurrency: 5,
		BusAcquireTimeout:     5 * time.Second,
	}
	rt.Configure(cfg)

	stats := rt.BusLimiter.Stats()
	if len(stats) != 3 {
		t.Errorf("len(stats) = %d, want 3", len(stats))
	}
}

func TestRuntime_Shutdown(t *testing.T) {
	rt := NewRuntime(100, "test")
	rt.Configure(DefaultRuntimeConfig())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := rt.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestAdaptService(t *testing.T) {
	rt := NewRuntime(100, "test")

	a := AdaptService(
		"test-svc", "domain",
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
		nil,
		rt,
	)

	if a.Name() != "test-svc" {
		t.Errorf("Name() = %q, want 'test-svc'", a.Name())
	}
}

func TestRegisterForRecovery(t *testing.T) {
	rt := NewRuntime(100, "test")

	a := NewServiceAdapter("test", "domain",
		WithStartFunc(func(ctx context.Context) error { return nil }),
		WithStopFunc(func(ctx context.Context) error { return nil }),
	)

	RegisterForRecovery(a, rt.Recovery, recovery.DefaultConfig())

	// Should be registered
	_, ok := rt.Recovery.GetState("test")
	if !ok {
		t.Error("adapter should be registered")
	}
}

func TestDefaultRuntimeConfig(t *testing.T) {
	cfg := DefaultRuntimeConfig()
	if cfg.EventBusConcurrency != 100 {
		t.Errorf("EventBusConcurrency = %d, want 100", cfg.EventBusConcurrency)
	}
	if cfg.DataBusConcurrency != 100 {
		t.Errorf("DataBusConcurrency = %d, want 100", cfg.DataBusConcurrency)
	}
	if cfg.ComputeBusConcurrency != 50 {
		t.Errorf("ComputeBusConcurrency = %d, want 50", cfg.ComputeBusConcurrency)
	}
}

func TestStatusToEventType(t *testing.T) {
	tests := []struct {
		status   state.Status
		expected events.EventType
	}{
		{state.StatusRegistered, events.EventModuleRegistered},
		{state.StatusStarting, events.EventModuleStarting},
		{state.StatusRunning, events.EventModuleStarted},
		{state.StatusStopping, events.EventModuleStopping},
		{state.StatusStopped, events.EventModuleStopped},
		{state.StatusFailed, events.EventModuleStartFailed},
		{state.StatusStopFailed, events.EventModuleStopFailed},
	}

	for _, tc := range tests {
		got := statusToEventType(tc.status)
		if got != tc.expected {
			t.Errorf("statusToEventType(%v) = %v, want %v", tc.status, got, tc.expected)
		}
	}
}
