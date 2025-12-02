package framework

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestServiceState_String(t *testing.T) {
	tests := []struct {
		state    ServiceState
		expected string
	}{
		{StateUninitialized, "uninitialized"},
		{StateInitializing, "initializing"},
		{StateReady, "ready"},
		{StateNotReady, "not-ready"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
		{ServiceState(99), "unknown"},
	}

	for _, tc := range tests {
		if got := tc.state.String(); got != tc.expected {
			t.Errorf("ServiceState(%d).String() = %q, want %q", tc.state, got, tc.expected)
		}
	}
}

func TestNewServiceBase(t *testing.T) {
	b := NewServiceBase("test-svc", "test-domain")

	if b.Name() != "test-svc" {
		t.Errorf("Name() = %q, want %q", b.Name(), "test-svc")
	}
	if b.Domain() != "test-domain" {
		t.Errorf("Domain() = %q, want %q", b.Domain(), "test-domain")
	}
	if b.State() != StateUninitialized {
		t.Errorf("State() = %v, want %v", b.State(), StateUninitialized)
	}
}

func TestServiceBase_SetNameDomain(t *testing.T) {
	b := &ServiceBase{}

	b.SetName("  my-service  ")
	b.SetDomain("  my-domain  ")

	if b.Name() != "my-service" {
		t.Errorf("Name() = %q, want %q", b.Name(), "my-service")
	}
	if b.Domain() != "my-domain" {
		t.Errorf("Domain() = %q, want %q", b.Domain(), "my-domain")
	}
}

func TestServiceBase_StateTransitions(t *testing.T) {
	b := NewServiceBase("test", "domain")

	// Initial state
	if b.State() != StateUninitialized {
		t.Errorf("initial state = %v, want %v", b.State(), StateUninitialized)
	}

	// Set state
	b.SetState(StateInitializing)
	if b.State() != StateInitializing {
		t.Errorf("state after SetState = %v, want %v", b.State(), StateInitializing)
	}

	// Compare and swap - should succeed
	if !b.CompareAndSwapState(StateInitializing, StateReady) {
		t.Error("CompareAndSwapState should succeed")
	}
	if b.State() != StateReady {
		t.Errorf("state after CAS = %v, want %v", b.State(), StateReady)
	}

	// Compare and swap - should fail (wrong expected)
	if b.CompareAndSwapState(StateInitializing, StateStopped) {
		t.Error("CompareAndSwapState should fail with wrong expected state")
	}
}

func TestServiceBase_SetReady(t *testing.T) {
	b := NewServiceBase("test", "domain")

	// Set ready
	b.SetReady("ready", "")
	if b.State() != StateReady {
		t.Errorf("state = %v, want %v", b.State(), StateReady)
	}

	// Set not ready with error
	b.SetReady("not-ready", "connection lost")
	if b.State() != StateNotReady {
		t.Errorf("state = %v, want %v", b.State(), StateNotReady)
	}
	if b.LastError() == nil || b.LastError().Error() != "connection lost" {
		t.Errorf("LastError() = %v, want 'connection lost'", b.LastError())
	}

	// Case insensitive
	b.SetReady("  READY  ", "")
	if b.State() != StateReady {
		t.Errorf("state = %v, want %v (case insensitive)", b.State(), StateReady)
	}
}

func TestServiceBase_MarkReady(t *testing.T) {
	b := NewServiceBase("test", "domain")

	b.MarkReady(true)
	if !b.IsReady() {
		t.Error("IsReady() should be true after MarkReady(true)")
	}

	b.MarkReady(false)
	if b.IsReady() {
		t.Error("IsReady() should be false after MarkReady(false)")
	}
}

func TestServiceBase_MarkStartedStopped(t *testing.T) {
	b := NewServiceBase("test", "domain")

	if !b.StartedAt().IsZero() {
		t.Error("StartedAt should be zero before MarkStarted")
	}

	b.MarkStarted()
	startedAt := b.StartedAt()
	if startedAt.IsZero() {
		t.Error("StartedAt should be set after MarkStarted")
	}
	if b.State() != StateReady {
		t.Errorf("state = %v, want %v after MarkStarted", b.State(), StateReady)
	}

	time.Sleep(10 * time.Millisecond)
	uptime := b.Uptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("Uptime() = %v, expected >= 10ms", uptime)
	}

	b.MarkStopped()
	if b.StoppedAt().IsZero() {
		t.Error("StoppedAt should be set after MarkStopped")
	}
	if b.State() != StateStopped {
		t.Errorf("state = %v, want %v after MarkStopped", b.State(), StateStopped)
	}
	if !b.IsStopped() {
		t.Error("IsStopped() should be true after MarkStopped")
	}
}

func TestServiceBase_MarkFailed(t *testing.T) {
	b := NewServiceBase("test", "domain")

	err := errors.New("fatal error")
	b.MarkFailed(err)

	if b.State() != StateFailed {
		t.Errorf("state = %v, want %v", b.State(), StateFailed)
	}
	if b.LastError() != err {
		t.Errorf("LastError() = %v, want %v", b.LastError(), err)
	}
	if !b.IsStopped() {
		t.Error("IsStopped() should be true for failed state")
	}
}

func TestServiceBase_Ready(t *testing.T) {
	ctx := context.Background()

	t.Run("ready state returns nil", func(t *testing.T) {
		b := NewServiceBase("test", "domain")
		b.MarkReady(true)

		if err := b.Ready(ctx); err != nil {
			t.Errorf("Ready() = %v, want nil", err)
		}
	})

	t.Run("not ready with last error", func(t *testing.T) {
		b := NewServiceBase("test", "domain")
		b.MarkFailed(errors.New("db connection failed"))

		err := b.Ready(ctx)
		if err == nil {
			t.Fatal("Ready() should return error")
		}
		if !errors.Is(err, b.LastError()) {
			t.Errorf("error should wrap last error")
		}
	})

	t.Run("not ready without name shows state", func(t *testing.T) {
		b := &ServiceBase{}
		b.SetState(StateNotReady)

		err := b.Ready(ctx)
		if err == nil {
			t.Fatal("Ready() should return error")
		}
		if err.Error() != "service not-ready" {
			t.Errorf("error = %q, want 'service not-ready'", err.Error())
		}
	})

	t.Run("not ready with name shows name and state", func(t *testing.T) {
		b := NewServiceBase("my-svc", "domain")
		b.SetState(StateNotReady)

		err := b.Ready(ctx)
		if err == nil {
			t.Fatal("Ready() should return error")
		}
		if err.Error() != "my-svc: not-ready" {
			t.Errorf("error = %q, want 'my-svc: not-ready'", err.Error())
		}
	})
}

func TestServiceBase_Metadata(t *testing.T) {
	b := NewServiceBase("test", "domain")

	// Set and get
	b.SetMetadata("key1", "value1")
	b.SetMetadata("key2", "value2")

	v, ok := b.GetMetadata("key1")
	if !ok || v != "value1" {
		t.Errorf("GetMetadata(key1) = %q, %v; want 'value1', true", v, ok)
	}

	// Not found
	_, ok = b.GetMetadata("nonexistent")
	if ok {
		t.Error("GetMetadata should return false for nonexistent key")
	}

	// All metadata returns copy
	all := b.AllMetadata()
	if len(all) != 2 {
		t.Errorf("AllMetadata() len = %d, want 2", len(all))
	}

	// Modify returned map shouldn't affect original
	all["key3"] = "value3"
	_, ok = b.GetMetadata("key3")
	if ok {
		t.Error("AllMetadata should return a copy")
	}
}

func TestServiceBase_ConcurrentAccess(t *testing.T) {
	b := NewServiceBase("concurrent", "domain")

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent state transitions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.SetState(StateReady)
			b.SetState(StateNotReady)
			_ = b.State()
			_ = b.IsReady()
		}()
	}

	// Concurrent name/domain access
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b.SetName("name-" + string(rune('a'+idx)))
			_ = b.Name()
			_ = b.Domain()
		}(i)
	}

	// Concurrent metadata access
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key-" + string(rune('a'+idx))
			b.SetMetadata(key, "value")
			_, _ = b.GetMetadata(key)
			_ = b.AllMetadata()
		}(i)
	}

	// Concurrent Ready checks
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Ready(ctx)
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestServiceBase_Uptime(t *testing.T) {
	b := NewServiceBase("test", "domain")

	// Not started yet
	if b.Uptime() != 0 {
		t.Errorf("Uptime() before start = %v, want 0", b.Uptime())
	}

	// Started but not stopped
	b.MarkStarted()
	time.Sleep(20 * time.Millisecond)
	uptime1 := b.Uptime()
	if uptime1 < 20*time.Millisecond {
		t.Errorf("Uptime() while running = %v, expected >= 20ms", uptime1)
	}

	// After stop, uptime should be fixed
	b.MarkStopped()
	uptime2 := b.Uptime()
	time.Sleep(10 * time.Millisecond)
	uptime3 := b.Uptime()

	if uptime2 != uptime3 {
		t.Errorf("Uptime() after stop should be fixed: %v vs %v", uptime2, uptime3)
	}
}
