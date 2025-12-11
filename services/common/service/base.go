package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

const healthCheckTimeout = 5 * time.Second

var defaultDBSecrets = []string{"SUPABASE_URL", "SUPABASE_SERVICE_KEY"}

// BaseConfig contains shared configuration for all marbles.
type BaseConfig struct {
	ID      string
	Name    string
	Version string
	Marble  *marble.Marble
	DB      database.RepositoryInterface
	// RequiredSecrets defines secrets that must be present for the service to be healthy.
	RequiredSecrets []string
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

	// Health tracking
	requiredSecrets []string
	healthMu        sync.RWMutex
	dbHealthy       bool
	secretsLoaded   bool
	lastHealthCheck time.Time
	startTime       time.Time
}

// NewBase constructs a BaseService from shared config.
func NewBase(cfg BaseConfig) *BaseService {
	requiredSecrets := mergeUniqueStrings(cfg.RequiredSecrets)
	if cfg.DB != nil {
		requiredSecrets = mergeUniqueStrings(requiredSecrets, defaultDBSecrets...)
	}
	return &BaseService{
		Service: marble.NewService(marble.ServiceConfig{
			ID:      cfg.ID,
			Name:    cfg.Name,
			Version: cfg.Version,
			Marble:  cfg.Marble,
			DB:      cfg.DB,
		}),
		stopCh:          make(chan struct{}),
		requiredSecrets: requiredSecrets,
		dbHealthy:       cfg.DB == nil,
		secretsLoaded:   len(requiredSecrets) == 0,
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

	b.healthMu.Lock()
	if b.startTime.IsZero() {
		b.startTime = time.Now()
	}
	b.healthMu.Unlock()

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

// CheckHealth refreshes the cached health state by probing critical dependencies.
func (b *BaseService) CheckHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	dbHealthy := true
	if repo := b.DB(); repo != nil {
		if err := repo.HealthCheck(ctx); err != nil {
			dbHealthy = false
		}
	}

	secretsLoaded := true
	if len(b.requiredSecrets) > 0 {
		secretsLoaded = false
		if m := b.Marble(); m != nil {
			secretsLoaded = true
			for _, name := range b.requiredSecrets {
				if len(name) == 0 {
					continue
				}
				secret, ok := m.Secret(name)
				if !ok || len(secret) == 0 {
					secretsLoaded = false
					break
				}
			}
		}
	}

	b.healthMu.Lock()
	b.dbHealthy = dbHealthy
	b.secretsLoaded = secretsLoaded || len(b.requiredSecrets) == 0
	b.lastHealthCheck = time.Now()
	b.healthMu.Unlock()
}

// HealthStatus returns the aggregated health status string.
func (b *BaseService) HealthStatus() string {
	b.CheckHealth()
	b.healthMu.RLock()
	defer b.healthMu.RUnlock()
	return b.healthStatusLocked()
}

// HealthDetails returns a map describing the most recent health state.
func (b *BaseService) HealthDetails() map[string]any {
	b.healthMu.RLock()
	defer b.healthMu.RUnlock()

	details := map[string]any{
		"db_connected":   b.dbHealthy,
		"secrets_loaded": len(b.requiredSecrets) == 0 || b.secretsLoaded,
		"enclave_mode":   b.Marble() != nil && b.Marble().IsEnclave(),
	}

	if !b.lastHealthCheck.IsZero() {
		details["last_check"] = b.lastHealthCheck.Format(time.RFC3339)
	} else {
		details["last_check"] = ""
	}

	uptime := time.Duration(0)
	if !b.startTime.IsZero() {
		uptime = time.Since(b.startTime)
	}
	details["uptime"] = uptime.String()

	return details
}

func (b *BaseService) healthStatusLocked() string {
	if b.DB() != nil && !b.dbHealthy {
		return "unhealthy"
	}
	if len(b.requiredSecrets) > 0 && !b.secretsLoaded {
		return "degraded"
	}
	return "healthy"
}

func mergeUniqueStrings(values []string, extras ...string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values)+len(extras))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	for _, v := range extras {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// =============================================================================
// Interface Compliance
// =============================================================================

// Ensure BaseService implements MarbleService interface.
var _ MarbleService = (*BaseService)(nil)
var _ HealthChecker = (*BaseService)(nil)
