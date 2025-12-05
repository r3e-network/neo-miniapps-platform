// Package datastreams provides real-time data streaming service.
package datastreams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "datastreams"
	ServiceName = "DataStreams Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Real-time data streaming service",
		RequiredCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapKeys,
			os.CapSecrets,
			os.CapKeysSign,      // Stream signing
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapCache,         // Stream caching
			os.CapQueue,         // Stream queue
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      256 * 1024 * 1024,
			MaxCPUTime:     60 * time.Second,
			MaxNetworkReqs: 1000,
		},
	}
}

// Service implements the DataStreams service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new DataStreams service.
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
	s.RegisterMetrics("datastreams", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("datastreams_streams_created", "Total number of streams created")
		metrics.RegisterCounter("datastreams_messages_published", "Total number of messages published")
		metrics.RegisterCounter("datastreams_messages_failed", "Total number of failed messages")
		metrics.RegisterGauge("datastreams_active_streams", "Number of active streams")
		metrics.RegisterGauge("datastreams_subscribers", "Number of active subscribers")
		metrics.RegisterHistogram("datastreams_message_latency_seconds", "Message delivery latency", []float64{0.01, 0.05, 0.1, 0.5, 1})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "datastreams/config", &cfg); err != nil {
		s.Logger().Warn("datastreams storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("datastreams config loaded from sealed storage")
	}

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// CreateStream creates a new data stream.
func (s *Service) CreateStream(ctx context.Context, stream *DataStream) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	stream.Status = StreamStatusActive
	stream.SetTimestamps()
	return s.store.CreateStream(ctx, stream)
}

// GetStream gets a stream by ID.
func (s *Service) GetStream(ctx context.Context, id string) (*DataStream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetStream(ctx, id)
}

