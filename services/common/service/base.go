package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/logging"
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
	Logger  *logging.Logger
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

	logger *logging.Logger
}

// NewBase constructs a BaseService from shared config.
func NewBase(cfg *BaseConfig) *BaseService {
	cfgValue := BaseConfig{}
	if cfg != nil {
		cfgValue = *cfg
	}

	requiredSecrets := mergeUniqueStrings(cfgValue.RequiredSecrets)
	if cfgValue.DB != nil {
		requiredSecrets = mergeUniqueStrings(requiredSecrets, defaultDBSecrets...)
	}

	logger := cfgValue.Logger
	if logger == nil {
		serviceName := cfgValue.ID
		if serviceName == "" {
			serviceName = "service"
		}
		logger = logging.NewFromEnv(serviceName)
	}

	return &BaseService{
		Service: marble.NewService(marble.ServiceConfig{
			ID:      cfgValue.ID,
			Name:    cfgValue.Name,
			Version: cfgValue.Version,
			Marble:  cfgValue.Marble,
			DB:      cfgValue.DB,
		}),
		stopCh:          make(chan struct{}),
		requiredSecrets: requiredSecrets,
		dbHealthy:       cfgValue.DB == nil,
		secretsLoaded:   len(requiredSecrets) == 0,
		logger:          logger,
	}
}

// Logger returns the service's structured logger.
func (b *BaseService) Logger() *logging.Logger {
	if b == nil {
		return logging.NewFromEnv("service")
	}
	if b.logger != nil {
		return b.logger
	}
	serviceName := b.ID()
	if serviceName == "" {
		serviceName = "service"
	}
	b.logger = logging.NewFromEnv(serviceName)
	return b.logger
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

type tickerWorkerConfig struct {
	name           string
	runImmediately bool
}

// TickerWorkerOption configures AddTickerWorker behavior.
type TickerWorkerOption func(*tickerWorkerConfig)

// WithTickerWorkerName sets a friendly name used in error logs.
func WithTickerWorkerName(name string) TickerWorkerOption {
	return func(cfg *tickerWorkerConfig) {
		cfg.name = name
	}
}

// WithTickerWorkerImmediate causes the worker to run once immediately on start
// (before waiting for the first ticker interval).
func WithTickerWorkerImmediate() TickerWorkerOption {
	return func(cfg *tickerWorkerConfig) {
		cfg.runImmediately = true
	}
}

// AddTickerWorker registers a periodic background worker.
// This is a convenience method that wraps the common ticker loop pattern.
// The worker function is called at the specified interval until Stop() is called.
func (b *BaseService) AddTickerWorker(interval time.Duration, fn func(context.Context) error, opts ...TickerWorkerOption) *BaseService {
	cfg := tickerWorkerConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}

	worker := func(ctx context.Context) {
		logWorkerError := func(err error) {
			if err == nil {
				return
			}
			entry := b.Logger().WithContext(ctx).WithError(err)
			if cfg.name != "" {
				entry = entry.WithField("worker", cfg.name)
			}
			entry.Warn("worker error")
		}

		if cfg.runImmediately {
			select {
			case <-ctx.Done():
				return
			case <-b.stopCh:
				return
			default:
			}

			if err := fn(ctx); err != nil {
				logWorkerError(err)
			}
		}

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
					logWorkerError(err)
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
		secretsLoaded = true
		for _, name := range b.requiredSecrets {
			if name == "" {
				continue
			}

			if m := b.Marble(); m != nil {
				if secret, ok := m.Secret(name); ok && len(secret) > 0 {
					continue
				}
			}

			if envValue := os.Getenv(name); envValue != "" {
				continue
			}

			secretsLoaded = false
			break
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
