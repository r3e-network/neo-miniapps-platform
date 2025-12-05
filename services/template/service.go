// Package template provides a template for creating new services.
// Copy this package and modify it to create your own service.
package template

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "template"
	ServiceName = "Template Service"
	Version     = "1.0.0"
)

// =============================================================================
// Service Manifest
// =============================================================================

// Manifest returns the service manifest declaring required capabilities.
// This is like Android's AndroidManifest.xml - it declares what the service needs.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Template service demonstrating ServiceOS capabilities",

		// Required capabilities - service won't start without these
		RequiredCapabilities: []os.Capability{
			os.CapStorage,  // Persistent storage
			os.CapConfig,   // Configuration management
			os.CapMetrics,  // Metrics collection
		},

		// Optional capabilities - service can work without these
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,     // Secret storage
			os.CapNetwork,     // Network access
			os.CapCache,       // Caching
			os.CapScheduler,   // Task scheduling
			os.CapDatabase,    // Database access
			os.CapNeo,         // Neo N3 blockchain
		},

		// Resource limits
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      128 * 1024 * 1024, // 128MB
			MaxCPUTime:     30 * time.Second,
			MaxNetworkReqs: 100,
			MaxSecrets:     50,
		},
	}
}

// =============================================================================
// Service Implementation
// =============================================================================

// Service implements the template service.
type Service struct {
	*base.BaseService

	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new template service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)

	enclave, err := NewEnclave(serviceOS)
	if err != nil {
		return nil, fmt.Errorf("create enclave: %w", err)
	}

	store := NewStore(serviceOS)

	return &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}, nil
}

// Start starts the service.
func (s *Service) Start(ctx context.Context) error {
	s.Logger().Info("starting template service")

	// Start base service
	if err := s.BaseService.Start(ctx); err != nil {
		return fmt.Errorf("start base service: %w", err)
	}

	// Initialize enclave
	if err := s.enclave.Initialize(ctx); err != nil {
		s.SetState(base.StateFailed)
		return fmt.Errorf("initialize enclave: %w", err)
	}

	// Initialize store
	if err := s.store.Initialize(ctx); err != nil {
		s.SetState(base.StateFailed)
		return fmt.Errorf("initialize store: %w", err)
	}

	// Register metrics
	s.registerMetrics()

	// Start background tasks if scheduler is available
	if s.OS().HasCapability(os.CapScheduler) {
		s.startBackgroundTasks(ctx)
	}

	s.Logger().Info("template service started")
	return nil
}

// Stop stops the service.
func (s *Service) Stop(ctx context.Context) error {
	s.Logger().Info("stopping template service")

	if err := s.enclave.Shutdown(ctx); err != nil {
		s.Logger().Error("shutdown enclave failed", "error", err)
	}

	if err := s.store.Close(ctx); err != nil {
		s.Logger().Error("close store failed", "error", err)
	}

	if err := s.BaseService.Stop(ctx); err != nil {
		return fmt.Errorf("stop base service: %w", err)
	}

	s.Logger().Info("template service stopped")
	return nil
}

// Health checks service health.
func (s *Service) Health(ctx context.Context) error {
	if err := s.BaseService.Health(ctx); err != nil {
		return err
	}
	if err := s.enclave.Health(ctx); err != nil {
		return fmt.Errorf("enclave unhealthy: %w", err)
	}
	if err := s.store.Health(ctx); err != nil {
		return fmt.Errorf("store unhealthy: %w", err)
	}
	return nil
}

// =============================================================================
// Business Logic Examples
// =============================================================================

// CreateItem demonstrates creating an item with storage.
func (s *Service) CreateItem(ctx context.Context, item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	// Validate
	if item.Name == "" {
		return fmt.Errorf("item name is required")
	}

	// Set timestamps
	item.SetTimestamps()

	// Store in database if available
	if s.OS().HasCapability(os.CapDatabase) {
		if err := s.store.CreateItem(ctx, item); err != nil {
			return fmt.Errorf("store item: %w", err)
		}
	}

	// Record metric
	s.OS().Metrics().Counter("template_items_created", 1)

	return nil
}

// GetItem retrieves an item by ID.
func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try cache first
	if s.OS().HasCapability(os.CapCache) {
		cacheKey := "item:" + id
		data, err := s.OS().Cache().Get(ctx, cacheKey)
		if err == nil && data != nil {
			// Cache hit - deserialize and return
			s.OS().Metrics().Counter("template_cache_hits", 1)
			// TODO: deserialize data to Item
		}
	}

	// Cache miss - get from store
	item, err := s.store.GetItem(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update cache
	if s.OS().HasCapability(os.CapCache) {
		cacheKey := "item:" + id
		s.OS().Cache().Set(ctx, cacheKey, item, 5*time.Minute)
	}

	return item, nil
}

// ProcessSecurely demonstrates using secrets and enclave.
func (s *Service) ProcessSecurely(ctx context.Context, data []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Process in enclave with secret access
	return s.enclave.ProcessWithSecret(ctx, data, "api_key")
}

// FetchExternalData demonstrates network access with secrets.
func (s *Service) FetchExternalData(ctx context.Context, url string) ([]byte, error) {
	if !s.OS().HasCapability(os.CapNetwork) {
		return nil, fmt.Errorf("network capability not available")
	}

	// Use secret for authentication
	if s.OS().HasCapability(os.CapSecrets) {
		resp, err := s.OS().Network().FetchWithSecret(ctx, os.HTTPRequest{
			Method: "GET",
			URL:    url,
		}, "api_token", os.AuthBearer)
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	}

	// Fallback to unauthenticated request
	resp, err := s.OS().Network().Fetch(ctx, os.HTTPRequest{
		Method: "GET",
		URL:    url,
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// =============================================================================
// Helper Methods
// =============================================================================

func (s *Service) registerMetrics() {
	metrics := s.OS().Metrics()
	metrics.RegisterCounter("template_items_created", "Number of items created")
	metrics.RegisterCounter("template_cache_hits", "Number of cache hits")
	metrics.RegisterGauge("template_active_items", "Number of active items")
	metrics.RegisterHistogram("template_processing_duration", "Processing duration", []float64{0.1, 0.5, 1, 5, 10})
}

func (s *Service) startBackgroundTasks(ctx context.Context) {
	scheduler := s.OS().Scheduler()

	// Schedule periodic cleanup
	scheduler.ScheduleInterval(ctx, 1*time.Hour, &os.ScheduledTask{
		Name:       "cleanup",
		Handler:    "cleanup",
		MaxRetries: 3,
		Timeout:    5 * time.Minute,
	})

	// Schedule daily report
	scheduler.ScheduleCron(ctx, "0 0 * * *", &os.ScheduledTask{
		Name:       "daily_report",
		Handler:    "daily_report",
		MaxRetries: 3,
		Timeout:    10 * time.Minute,
	})
}
