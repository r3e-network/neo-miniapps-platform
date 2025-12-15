// Package marble provides background worker utilities for MarbleRun services.
package marble

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Worker represents a background worker with lifecycle management.
//
// Deprecated: prefer `services/common/service.BaseService` worker management
// (`AddWorker`/`AddTickerWorker`) for all marble services.
type Worker struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context) error
	stopCh   chan struct{}
	doneCh   chan struct{}
	running  bool
	mu       sync.Mutex
}

// WorkerConfig holds worker configuration.
//
// Deprecated: prefer `services/common/service.BaseService` worker management.
type WorkerConfig struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

// NewWorker creates a new background worker.
//
// Deprecated: prefer `services/common/service.BaseService.AddTickerWorker`.
func NewWorker(cfg WorkerConfig) *Worker {
	return &Worker{
		name:     cfg.Name,
		interval: cfg.Interval,
		fn:       cfg.Fn,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start starts the worker.
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("worker %s already running", w.name)
	}
	w.running = true
	w.mu.Unlock()

	go w.run(ctx)
	return nil
}

// Stop stops the worker and waits for it to finish.
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	close(w.stopCh)
	<-w.doneCh
}

// IsRunning returns true if the worker is running.
func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Worker) run(ctx context.Context) {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		close(w.doneCh)
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.fn(ctx); err != nil {
				// Log error but continue
				fmt.Printf("[%s] worker error: %v\n", w.name, err)
			}
		}
	}
}

// WorkerGroup manages multiple workers.
//
// Deprecated: prefer `services/common/service.BaseService` worker management.
type WorkerGroup struct {
	workers []*Worker
	mu      sync.Mutex
}

// NewWorkerGroup creates a new worker group.
//
// Deprecated: prefer `services/common/service.BaseService` worker management.
func NewWorkerGroup() *WorkerGroup {
	return &WorkerGroup{
		workers: make([]*Worker, 0),
	}
}

// Add adds a worker to the group.
//
// Deprecated: prefer `services/common/service.BaseService.AddWorker`/`AddTickerWorker`.
func (g *WorkerGroup) Add(w *Worker) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.workers = append(g.workers, w)
}

// AddFunc adds a worker function to the group.
//
// Deprecated: prefer `services/common/service.BaseService.AddTickerWorker`.
func (g *WorkerGroup) AddFunc(name string, interval time.Duration, fn func(ctx context.Context) error) *Worker {
	w := NewWorker(WorkerConfig{
		Name:     name,
		Interval: interval,
		Fn:       fn,
	})
	g.Add(w)
	return w
}

// Start starts all workers in the group.
//
// Deprecated: prefer `services/common/service.BaseService.Start`.
func (g *WorkerGroup) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, w := range g.workers {
		if err := w.Start(ctx); err != nil {
			// Stop already started workers
			for _, started := range g.workers {
				if started.IsRunning() {
					started.Stop()
				}
			}
			return fmt.Errorf("start worker %s: %w", w.name, err)
		}
	}
	return nil
}

// Stop stops all workers in the group.
//
// Deprecated: prefer `services/common/service.BaseService.Stop`.
func (g *WorkerGroup) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()

	var wg sync.WaitGroup
	for _, w := range g.workers {
		wg.Add(1)
		go func(worker *Worker) {
			defer wg.Done()
			worker.Stop()
		}(w)
	}
	wg.Wait()
}

// TickerLoop runs a function on a ticker until context is canceled or stop channel is closed.
// This is a simpler alternative to Worker for inline use.
//
// Deprecated: prefer `services/common/service.BaseService.AddTickerWorker`.
func TickerLoop(ctx context.Context, stopCh <-chan struct{}, interval time.Duration, fn func(ctx context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// ChannelLoop processes items from a channel until context is canceled or stop channel is closed.
//
// Deprecated: prefer `services/common/service.BaseService.AddWorker`.
func ChannelLoop[T any](ctx context.Context, stopCh <-chan struct{}, ch <-chan T, fn func(ctx context.Context, item T)) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case item, ok := <-ch:
			if !ok {
				return
			}
			fn(ctx, item)
		}
	}
}

// RetryWithBackoff retries a function with exponential backoff.
//
// Deprecated: prefer service-specific retry mechanisms, or wrap retries in a
// `services/common/service.BaseService` worker.
func RetryWithBackoff(ctx context.Context, maxRetries int, initialDelay time.Duration, fn func() error) error {
	delay := initialDelay
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
