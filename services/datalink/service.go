// Package datalink provides data linking service.
package datalink

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "datalink"
	ServiceName = "DataLink Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Data linking and synchronization service",
		RequiredCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapScheduler,     // Scheduled sync
			os.CapCache,         // Data caching
			os.CapQueue,         // Sync queue
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      128 * 1024 * 1024,
			MaxCPUTime:     60 * time.Second,
			MaxNetworkReqs: 300,
		},
	}
}

// Service implements the DataLink service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new DataLink service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)

	enclave := NewEnclave(serviceOS)
	store := NewStore()

	svc := &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}

	// Register components with BaseService for automatic lifecycle management
	baseService.SetEnclave(enclave)
	baseService.SetStore(store)

	// Set lifecycle hooks for service-specific initialization
	baseService.SetHooks(base.LifecycleHooks{
		OnAfterStart: svc.onAfterStart,
	})

	return svc, nil
}

// onAfterStart is called after enclave and store are initialized.
// This handles service-specific initialization like metrics, config hydration, etc.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Register metrics using BaseService helper
	s.RegisterMetrics("datalink", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("datalink_syncs_total", "Total number of sync operations")
		metrics.RegisterCounter("datalink_syncs_failed", "Total number of failed syncs")
		metrics.RegisterCounter("datalink_links_created", "Total number of links created")
		metrics.RegisterGauge("datalink_active_links", "Number of active data links")
		metrics.RegisterHistogram("datalink_sync_duration_seconds", "Sync operation duration", []float64{0.1, 0.5, 1, 5, 10, 30})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "datalink/config", &cfg); err != nil {
		s.Logger().Warn("datalink storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("datalink config loaded from sealed storage")
	}

	// Start scheduled sync if scheduler available
	if s.OS().HasCapability(os.CapScheduler) {
		s.startScheduledSync(ctx)
	}

	return nil
}

// startScheduledSync starts scheduled data synchronization.
func (s *Service) startScheduledSync(ctx context.Context) {
	scheduler := s.OS().Scheduler()

	// Schedule periodic sync check (every 5 minutes)
	_, err := scheduler.ScheduleInterval(ctx, 5*time.Minute, &os.ScheduledTask{
		Name:       "sync_links",
		Handler:    "sync_links",
		MaxRetries: 3,
		Timeout:    2 * time.Minute,
	})
	if err != nil {
		s.Logger().Warn("failed to schedule link sync", "error", err)
	}

	s.Logger().Info("scheduled link sync started")
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// CreateLink creates a new data link.
func (s *Service) CreateLink(ctx context.Context, link *DataLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	link.Status = LinkStatusActive
	link.SetTimestamps()
	return s.store.CreateLink(ctx, link)
}

// SyncLink syncs a data link.
func (s *Service) SyncLink(ctx context.Context, linkID string) (*SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	link, err := s.store.GetLink(ctx, linkID)
	if err != nil {
		return nil, err
	}
	return s.enclave.SyncData(ctx, link)
}

