// Package lifecycle provides lifecycle management utilities for services.
package lifecycle

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// GracefulShutdown coordinates graceful shutdown of a service.
// It tracks in-flight operations and provides a mechanism to wait for them to complete.
type GracefulShutdown struct {
	mu         sync.Mutex
	inFlight   int64
	shutdownCh chan struct{}
	closed     int32
}

// NewGracefulShutdown creates a new GracefulShutdown instance.
func NewGracefulShutdown() *GracefulShutdown {
	return &GracefulShutdown{
		shutdownCh: make(chan struct{}),
	}
}

// Add increments the in-flight counter.
// Returns false if shutdown has already been initiated.
func (g *GracefulShutdown) Add() bool {
	if atomic.LoadInt32(&g.closed) != 0 {
		return false
	}
	atomic.AddInt64(&g.inFlight, 1)
	return true
}

// Done decrements the in-flight counter.
func (g *GracefulShutdown) Done() {
	atomic.AddInt64(&g.inFlight, -1)
}

// InFlight returns the current number of in-flight operations.
func (g *GracefulShutdown) InFlight() int64 {
	return atomic.LoadInt64(&g.inFlight)
}

// IsShuttingDown returns true if shutdown has been initiated.
func (g *GracefulShutdown) IsShuttingDown() bool {
	return atomic.LoadInt32(&g.closed) != 0
}

// Shutdown initiates graceful shutdown.
// It closes the shutdown channel to signal waiting goroutines.
func (g *GracefulShutdown) Shutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if atomic.CompareAndSwapInt32(&g.closed, 0, 1) {
		close(g.shutdownCh)
	}
}

// ShutdownCh returns a channel that is closed when shutdown is initiated.
// Use this in select statements to detect shutdown.
func (g *GracefulShutdown) ShutdownCh() <-chan struct{} {
	return g.shutdownCh
}

// Wait waits for all in-flight operations to complete or until the context is canceled.
// Returns nil if all operations completed, or the context error if canceled.
func (g *GracefulShutdown) Wait(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if atomic.LoadInt64(&g.inFlight) <= 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check again
		}
	}
}

// WaitWithTimeout waits for all in-flight operations with a timeout.
func (g *GracefulShutdown) WaitWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return g.Wait(ctx)
}

// ShutdownAndWait initiates shutdown and waits for completion with a timeout.
func (g *GracefulShutdown) ShutdownAndWait(timeout time.Duration) error {
	g.Shutdown()
	return g.WaitWithTimeout(timeout)
}

// Reset resets the shutdown state. Use with caution - typically only for testing.
func (g *GracefulShutdown) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	atomic.StoreInt32(&g.closed, 0)
	atomic.StoreInt64(&g.inFlight, 0)
	g.shutdownCh = make(chan struct{})
}

// OperationGuard provides RAII-style operation tracking.
// Create with NewOperationGuard and defer Close().
type OperationGuard struct {
	gs      *GracefulShutdown
	added   bool
	closeFn func()
}

// NewOperationGuard creates a new operation guard.
// Returns nil if shutdown has already been initiated.
// Usage:
//
//	guard := lifecycle.NewOperationGuard(gs)
//	if guard == nil {
//	    return ErrShuttingDown
//	}
//	defer guard.Close()
func NewOperationGuard(gs *GracefulShutdown) *OperationGuard {
	if gs == nil {
		return &OperationGuard{added: false}
	}

	if !gs.Add() {
		return nil
	}

	return &OperationGuard{
		gs:    gs,
		added: true,
	}
}

// Close releases the operation guard.
func (o *OperationGuard) Close() {
	if o != nil && o.added && o.gs != nil {
		o.gs.Done()
		o.added = false
	}
	if o != nil && o.closeFn != nil {
		o.closeFn()
	}
}

// OnClose registers a function to call when the guard is closed.
func (o *OperationGuard) OnClose(fn func()) {
	if o != nil {
		o.closeFn = fn
	}
}
