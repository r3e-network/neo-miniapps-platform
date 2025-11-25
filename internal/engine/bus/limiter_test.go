package bus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(DefaultLimiterConfig())
	if l == nil {
		t.Fatal("NewLimiter returned nil")
	}
	if l.Active() != 0 {
		t.Errorf("Active() = %d, want 0", l.Active())
	}
}

func TestLimiter_Acquire_Release(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 5}
	l := NewLimiter(cfg)

	ctx := context.Background()

	// Acquire
	err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if l.Active() != 1 {
		t.Errorf("Active() = %d, want 1", l.Active())
	}

	// Release
	l.Release()
	if l.Active() != 0 {
		t.Errorf("Active() = %d, want 0", l.Active())
	}
}

func TestLimiter_MaxConcurrent(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 3}
	l := NewLimiter(cfg)

	ctx := context.Background()

	// Acquire all permits
	for i := 0; i < 3; i++ {
		if err := l.Acquire(ctx); err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	if l.Active() != 3 {
		t.Errorf("Active() = %d, want 3", l.Active())
	}

	// Try to acquire one more (should block/timeout)
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := l.Acquire(ctx2)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Acquire should timeout, got: %v", err)
	}

	// Release one
	l.Release()

	// Now should be able to acquire
	err = l.Acquire(context.Background())
	if err != nil {
		t.Errorf("Acquire after release failed: %v", err)
	}

	l.Close()
}

func TestLimiter_TryAcquire(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 2}
	l := NewLimiter(cfg)

	// First two should succeed
	if !l.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}
	if !l.TryAcquire() {
		t.Error("second TryAcquire should succeed")
	}

	// Third should fail
	if l.TryAcquire() {
		t.Error("third TryAcquire should fail")
	}

	l.Release()

	// Now should succeed
	if !l.TryAcquire() {
		t.Error("TryAcquire after release should succeed")
	}
}

func TestLimiter_QueueSize(t *testing.T) {
	cfg := LimiterConfig{
		MaxConcurrent: 1,
		QueueSize:     2,
	}
	l := NewLimiter(cfg)

	// Acquire the only permit
	ctx := context.Background()
	l.Acquire(ctx)

	// Start two goroutines waiting
	var wg sync.WaitGroup
	startedWaiting := make(chan struct{}, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startedWaiting <- struct{}{}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			l.Acquire(ctx)
		}()
	}

	// Wait for both to start waiting
	<-startedWaiting
	<-startedWaiting
	time.Sleep(10 * time.Millisecond)

	// Third should be rejected immediately due to queue limit
	err := l.Acquire(context.Background())
	if !errors.Is(err, ErrLimitExceeded) {
		t.Errorf("expected ErrLimitExceeded, got: %v", err)
	}

	// Release to let waiters proceed
	l.Release()
	wg.Wait()
}

func TestLimiter_AcquireTimeout(t *testing.T) {
	cfg := LimiterConfig{
		MaxConcurrent:  1,
		AcquireTimeout: 50 * time.Millisecond,
	}
	l := NewLimiter(cfg)

	// Acquire the only permit
	l.Acquire(context.Background())

	// Second acquire should timeout
	err := l.Acquire(context.Background())
	if !errors.Is(err, ErrAcquireTimeout) {
		t.Errorf("expected ErrAcquireTimeout, got: %v", err)
	}

	stats := l.Stats()
	if stats.TotalTimeouts != 1 {
		t.Errorf("TotalTimeouts = %d, want 1", stats.TotalTimeouts)
	}
}

func TestLimiter_Unlimited(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 0} // Unlimited
	l := NewLimiter(cfg)

	ctx := context.Background()

	// Should be able to acquire many
	for i := 0; i < 100; i++ {
		if err := l.Acquire(ctx); err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	if l.Active() != 100 {
		t.Errorf("Active() = %d, want 100", l.Active())
	}

	if l.Available() != -1 {
		t.Errorf("Available() = %d, want -1 (unlimited)", l.Available())
	}
}

func TestLimiter_Close(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 5}
	l := NewLimiter(cfg)

	l.Acquire(context.Background())
	l.Close()

	// Should fail after close
	err := l.Acquire(context.Background())
	if !errors.Is(err, ErrLimiterClosed) {
		t.Errorf("expected ErrLimiterClosed, got: %v", err)
	}

	if !l.TryAcquire() == false {
		// TryAcquire returns false when closed
	}
}

func TestLimiter_Stats(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 5}
	l := NewLimiter(cfg)

	l.Acquire(context.Background())
	l.TryAcquire()
	l.Release()

	stats := l.Stats()
	if stats.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want 5", stats.MaxConcurrent)
	}
	if stats.Active != 1 {
		t.Errorf("Active = %d, want 1", stats.Active)
	}
	if stats.TotalAcquired != 2 {
		t.Errorf("TotalAcquired = %d, want 2", stats.TotalAcquired)
	}
	if stats.TotalReleased != 1 {
		t.Errorf("TotalReleased = %d, want 1", stats.TotalReleased)
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	cfg := LimiterConfig{MaxConcurrent: 10}
	l := NewLimiter(cfg)

	var wg sync.WaitGroup
	var completed int64

	// 100 goroutines competing for 10 permits
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if err := l.Acquire(ctx); err != nil {
				return
			}
			defer l.Release()

			// Simulate work
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&completed, 1)
		}()
	}

	wg.Wait()

	if completed != 100 {
		t.Errorf("completed = %d, want 100", completed)
	}

	stats := l.Stats()
	if stats.Active != 0 {
		t.Errorf("Active = %d, want 0", stats.Active)
	}
	if stats.TotalAcquired != 100 {
		t.Errorf("TotalAcquired = %d, want 100", stats.TotalAcquired)
	}
}

func TestBusLimiter_Configure(t *testing.T) {
	bl := NewBusLimiter()

	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 10})
	bl.Configure(KindData, LimiterConfig{MaxConcurrent: 20})
	bl.Configure(KindCompute, LimiterConfig{MaxConcurrent: 5})

	stats := bl.Stats()
	if len(stats) != 3 {
		t.Errorf("len(stats) = %d, want 3", len(stats))
	}

	eventStats, ok := bl.StatsFor(KindEvent)
	if !ok {
		t.Error("event stats not found")
	}
	if eventStats.MaxConcurrent != 10 {
		t.Errorf("event MaxConcurrent = %d, want 10", eventStats.MaxConcurrent)
	}
}

func TestBusLimiter_AcquireRelease(t *testing.T) {
	bl := NewBusLimiter()
	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 2})

	ctx := context.Background()

	// First two should succeed
	if err := bl.Acquire(ctx, KindEvent); err != nil {
		t.Errorf("first acquire failed: %v", err)
	}
	if err := bl.Acquire(ctx, KindEvent); err != nil {
		t.Errorf("second acquire failed: %v", err)
	}

	// Third should timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := bl.Acquire(ctx2, KindEvent)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected timeout, got: %v", err)
	}

	// Release and try again
	bl.Release(KindEvent)
	if err := bl.Acquire(ctx, KindEvent); err != nil {
		t.Errorf("acquire after release failed: %v", err)
	}
}

func TestBusLimiter_UnconfiguredKind(t *testing.T) {
	bl := NewBusLimiter()

	// Should succeed for unconfigured kind (no limit)
	if err := bl.Acquire(context.Background(), KindEvent); err != nil {
		t.Errorf("acquire for unconfigured kind failed: %v", err)
	}

	if !bl.TryAcquire(KindData) {
		t.Error("TryAcquire for unconfigured kind should succeed")
	}

	// Release should not panic for unconfigured kind
	bl.Release(KindCompute)
}

func TestBusLimiter_Close(t *testing.T) {
	bl := NewBusLimiter()
	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 5})
	bl.Configure(KindData, LimiterConfig{MaxConcurrent: 5})

	bl.Close()

	stats := bl.Stats()
	if len(stats) != 0 {
		t.Errorf("len(stats) after close = %d, want 0", len(stats))
	}
}

func TestGuard(t *testing.T) {
	bl := NewBusLimiter()
	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 2})

	ctx := context.Background()

	// Create guard
	g, err := NewGuard(ctx, bl, KindEvent)
	if err != nil {
		t.Fatalf("NewGuard failed: %v", err)
	}

	stats, _ := bl.StatsFor(KindEvent)
	if stats.Active != 1 {
		t.Errorf("Active = %d, want 1", stats.Active)
	}

	// Release
	g.Release()

	stats, _ = bl.StatsFor(KindEvent)
	if stats.Active != 0 {
		t.Errorf("Active after release = %d, want 0", stats.Active)
	}

	// Multiple releases should be safe
	g.Release()
	g.Release()
}

func TestTryGuard(t *testing.T) {
	bl := NewBusLimiter()
	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 1})

	g1 := TryGuard(bl, KindEvent)
	if g1 == nil {
		t.Error("first TryGuard should succeed")
	}

	g2 := TryGuard(bl, KindEvent)
	if g2 != nil {
		t.Error("second TryGuard should fail")
	}

	g1.Release()

	g3 := TryGuard(bl, KindEvent)
	if g3 == nil {
		t.Error("TryGuard after release should succeed")
	}
	g3.Release()
}

func TestGuard_NilSafe(t *testing.T) {
	var g *Guard
	// Should not panic
	g.Release()
}

func BenchmarkLimiter_AcquireRelease(b *testing.B) {
	l := NewLimiter(LimiterConfig{MaxConcurrent: 100})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Acquire(ctx)
		l.Release()
	}
}

func BenchmarkLimiter_TryAcquire(b *testing.B) {
	l := NewLimiter(LimiterConfig{MaxConcurrent: 100})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if l.TryAcquire() {
			l.Release()
		}
	}
}

func BenchmarkLimiter_Unlimited(b *testing.B) {
	l := NewLimiter(LimiterConfig{MaxConcurrent: 0})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Acquire(ctx)
		l.Release()
	}
}

func BenchmarkBusLimiter_Acquire(b *testing.B) {
	bl := NewBusLimiter()
	bl.Configure(KindEvent, LimiterConfig{MaxConcurrent: 100})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bl.Acquire(ctx, KindEvent)
		bl.Release(KindEvent)
	}
}
