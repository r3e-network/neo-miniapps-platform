package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

// BaseConfig contains shared configuration for all marbles.
type BaseConfig struct {
	ID      string
	Name    string
	Version string
	Marble  *marble.Marble
	DB      database.RepositoryInterface
}

// BaseService wraps marble.Service with hydrate/worker wiring and stop handling.
// It provides a consistent foundation for all marble services with:
// - Safe stop channel management (sync.Once prevents double-close panic)
// - Optional hydration hook for loading state on startup
// - Background worker management
// - Statistics provider for /info endpoint
type BaseService struct {
	*marble.Service

	// Lifecycle management
	stopCh   chan struct{}
	stopOnce sync.Once

	// Extensibility hooks
	hydrate func(context.Context) error
	statsFn func() map[string]any

	// Worker management
	workers []func(context.Context)
}

// NewBase constructs a BaseService from shared config.
func NewBase(cfg BaseConfig) *BaseService {
	return &BaseService{
		Service: marble.NewService(marble.ServiceConfig{
			ID:      cfg.ID,
			Name:    cfg.Name,
			Version: cfg.Version,
			Marble:  cfg.Marble,
			DB:      cfg.DB,
		}),
		stopCh: make(chan struct{}),
	}
}

// WithHydrate sets an optional hydrate hook executed during Start.
// The hydrate function is called after the base service starts but before
// background workers are launched. Use this for loading persistent state.
func (b *BaseService) WithHydrate(fn func(context.Context) error) *BaseService {
	b.hydrate = fn
	return b
}

// WithStats sets a statistics provider function for the /info endpoint.
// The function will be called on each /info request to get current statistics.
func (b *BaseService) WithStats(fn func() map[string]any) *BaseService {
	b.statsFn = fn
	return b
}

// AddWorker registers a background worker started after hydrate completes.
// Workers receive the context and should respect context cancellation.
// Workers should also monitor StopChan() for service shutdown signals.
func (b *BaseService) AddWorker(fn func(context.Context)) *BaseService {
	b.workers = append(b.workers, fn)
	return b
}

// AddTickerWorker registers a periodic background worker.
// This is a convenience method that wraps the common ticker loop pattern.
// The worker function is called at the specified interval until Stop() is called.
func (b *BaseService) AddTickerWorker(interval time.Duration, fn func(context.Context) error) *BaseService {
	worker := func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopCh:
				return
			case <-ticker.C:
				if err := fn(ctx); err != nil {
					// Log error but continue - worker should handle its own errors
					fmt.Printf("[%s] worker error: %v\n", b.Name(), err)
				}
			}
		}
	}
	b.workers = append(b.workers, worker)
	return b
}

// StopChan exposes the stop channel for worker goroutines.
func (b *BaseService) StopChan() <-chan struct{} {
	return b.stopCh
}

// Start starts the underlying marble.Service, runs hydrate once, then spins workers.
func (b *BaseService) Start(ctx context.Context) error {
	if err := b.Service.Start(ctx); err != nil {
		return err
	}

	if b.hydrate != nil {
		if err := b.hydrate(ctx); err != nil {
			return fmt.Errorf("hydrate: %w", err)
		}
	}

	for _, w := range b.workers {
		worker := w
		go worker(ctx)
	}
	return nil
}

// Stop signals workers and stops the underlying marble.Service.
// This method is idempotent - calling it multiple times is safe due to sync.Once.
func (b *BaseService) Stop() error {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
	return b.Service.Stop()
}

// WorkerCount returns the number of registered workers.
func (b *BaseService) WorkerCount() int {
	return len(b.workers)
}

// =============================================================================
// Interface Compliance
// =============================================================================

// Ensure BaseService implements MarbleService interface.
var _ MarbleService = (*BaseService)(nil)
