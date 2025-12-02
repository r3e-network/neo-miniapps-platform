package lifecycle

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGracefulShutdown_Basic(t *testing.T) {
	gs := NewGracefulShutdown()

	// Initial state
	if gs.InFlight() != 0 {
		t.Errorf("initial InFlight() = %d, want 0", gs.InFlight())
	}
	if gs.IsShuttingDown() {
		t.Error("IsShuttingDown() should be false initially")
	}

	// Add operations
	if !gs.Add() {
		t.Error("Add() should return true")
	}
	if gs.InFlight() != 1 {
		t.Errorf("InFlight() = %d, want 1", gs.InFlight())
	}

	// Done
	gs.Done()
	if gs.InFlight() != 0 {
		t.Errorf("InFlight() after Done = %d, want 0", gs.InFlight())
	}
}

func TestGracefulShutdown_Shutdown(t *testing.T) {
	gs := NewGracefulShutdown()

	// Add before shutdown
	if !gs.Add() {
		t.Error("Add() should succeed before shutdown")
	}
	gs.Done()

	// Initiate shutdown
	gs.Shutdown()

	if !gs.IsShuttingDown() {
		t.Error("IsShuttingDown() should be true after Shutdown()")
	}

	// Add after shutdown should fail
	if gs.Add() {
		t.Error("Add() should fail after shutdown")
	}

	// Multiple shutdowns should be safe
	gs.Shutdown()
	gs.Shutdown()
}

func TestGracefulShutdown_ShutdownCh(t *testing.T) {
	gs := NewGracefulShutdown()

	ch := gs.ShutdownCh()

	// Should not be closed yet
	select {
	case <-ch:
		t.Error("ShutdownCh should not be closed before Shutdown()")
	default:
		// OK
	}

	gs.Shutdown()

	// Should be closed now
	select {
	case <-ch:
		// OK
	default:
		t.Error("ShutdownCh should be closed after Shutdown()")
	}
}

func TestGracefulShutdown_Wait(t *testing.T) {
	gs := NewGracefulShutdown()

	// Wait with no in-flight should return immediately
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := gs.Wait(ctx); err != nil {
		t.Errorf("Wait() with no in-flight = %v, want nil", err)
	}

	// Add in-flight and wait
	gs.Add()

	go func() {
		time.Sleep(50 * time.Millisecond)
		gs.Done()
	}()

	start := time.Now()
	if err := gs.Wait(ctx); err != nil {
		t.Errorf("Wait() = %v, want nil", err)
	}
	elapsed := time.Since(start)

	if elapsed < 40*time.Millisecond {
		t.Errorf("Wait() returned too fast: %v", elapsed)
	}
}

func TestGracefulShutdown_WaitTimeout(t *testing.T) {
	gs := NewGracefulShutdown()
	gs.Add() // Never done

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := gs.Wait(ctx)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("Wait() = %v, want DeadlineExceeded", err)
	}

	if elapsed < 40*time.Millisecond {
		t.Errorf("Wait() returned too fast: %v", elapsed)
	}
}

func TestGracefulShutdown_WaitWithTimeout(t *testing.T) {
	gs := NewGracefulShutdown()

	// Fast case - no in-flight
	if err := gs.WaitWithTimeout(100 * time.Millisecond); err != nil {
		t.Errorf("WaitWithTimeout() = %v, want nil", err)
	}

	// Timeout case
	gs.Add()
	err := gs.WaitWithTimeout(50 * time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitWithTimeout() = %v, want DeadlineExceeded", err)
	}
}

func TestGracefulShutdown_ShutdownAndWait(t *testing.T) {
	gs := NewGracefulShutdown()

	// Add operation that completes quickly
	gs.Add()
	go func() {
		time.Sleep(20 * time.Millisecond)
		gs.Done()
	}()

	err := gs.ShutdownAndWait(100 * time.Millisecond)
	if err != nil {
		t.Errorf("ShutdownAndWait() = %v, want nil", err)
	}

	if !gs.IsShuttingDown() {
		t.Error("should be shutting down after ShutdownAndWait")
	}
}

func TestGracefulShutdown_Reset(t *testing.T) {
	gs := NewGracefulShutdown()

	gs.Add()
	gs.Shutdown()

	// State before reset
	if !gs.IsShuttingDown() {
		t.Error("should be shutting down before reset")
	}

	gs.Reset()

	// State after reset
	if gs.IsShuttingDown() {
		t.Error("should not be shutting down after reset")
	}
	if gs.InFlight() != 0 {
		t.Errorf("InFlight() after reset = %d, want 0", gs.InFlight())
	}

	// Should be able to add again
	if !gs.Add() {
		t.Error("Add() should succeed after reset")
	}
}

func TestGracefulShutdown_Concurrent(t *testing.T) {
	gs := NewGracefulShutdown()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent adds and dones
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if gs.Add() {
				time.Sleep(time.Duration(i%10) * time.Millisecond)
				gs.Done()
			}
		}()
	}

	// Concurrent checks
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = gs.InFlight()
			_ = gs.IsShuttingDown()
		}()
	}

	// Shutdown in the middle
	time.Sleep(5 * time.Millisecond)
	gs.Shutdown()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestOperationGuard_Basic(t *testing.T) {
	gs := NewGracefulShutdown()

	// Create guard
	guard := NewOperationGuard(gs)
	if guard == nil {
		t.Fatal("NewOperationGuard returned nil")
	}

	if gs.InFlight() != 1 {
		t.Errorf("InFlight() = %d, want 1", gs.InFlight())
	}

	// Close guard
	guard.Close()
	if gs.InFlight() != 0 {
		t.Errorf("InFlight() after Close = %d, want 0", gs.InFlight())
	}

	// Double close should be safe
	guard.Close()
}

func TestOperationGuard_NilGracefulShutdown(t *testing.T) {
	guard := NewOperationGuard(nil)
	if guard == nil {
		t.Fatal("NewOperationGuard(nil) should not return nil")
	}

	// Close should be safe
	guard.Close()
}

func TestOperationGuard_AfterShutdown(t *testing.T) {
	gs := NewGracefulShutdown()
	gs.Shutdown()

	guard := NewOperationGuard(gs)
	if guard != nil {
		t.Error("NewOperationGuard should return nil after shutdown")
	}
}

func TestOperationGuard_OnClose(t *testing.T) {
	gs := NewGracefulShutdown()
	guard := NewOperationGuard(gs)

	called := false
	guard.OnClose(func() {
		called = true
	})

	guard.Close()

	if !called {
		t.Error("OnClose callback was not called")
	}
}

func TestOperationGuard_DeferPattern(t *testing.T) {
	gs := NewGracefulShutdown()

	// Simulate typical usage
	func() {
		guard := NewOperationGuard(gs)
		if guard == nil {
			t.Fatal("guard should not be nil")
		}
		defer guard.Close()

		// Do work
		time.Sleep(10 * time.Millisecond)

		if gs.InFlight() != 1 {
			t.Errorf("InFlight() during work = %d, want 1", gs.InFlight())
		}
	}()

	if gs.InFlight() != 0 {
		t.Errorf("InFlight() after defer = %d, want 0", gs.InFlight())
	}
}

func TestOperationGuard_PanicRecovery(t *testing.T) {
	gs := NewGracefulShutdown()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}

		// After panic recovery, in-flight should still be decremented by defer
		if gs.InFlight() != 0 {
			t.Errorf("InFlight() after panic = %d, want 0", gs.InFlight())
		}
	}()

	func() {
		guard := NewOperationGuard(gs)
		defer guard.Close()

		panic("test panic")
	}()
}

func TestGracefulShutdown_SelectPattern(t *testing.T) {
	gs := NewGracefulShutdown()

	done := make(chan bool, 1)
	ready := make(chan struct{})

	go func() {
		close(ready)
		select {
		case <-gs.ShutdownCh():
			done <- true
		case <-time.After(500 * time.Millisecond):
			done <- false
		}
	}()

	<-ready
	time.Sleep(10 * time.Millisecond)
	gs.Shutdown()

	result := <-done
	if !result {
		t.Error("goroutine should have received shutdown signal")
	}
}
