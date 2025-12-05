// Package confidential provides confidential computing service.
package confidential

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "confidential"
	ServiceName = "Confidential Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Confidential computing service with TEE protection",
		RequiredCapabilities: []os.Capability{
			os.CapCompute,
			os.CapSecrets,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapAttestation,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapKeys,          // Key management
			os.CapKeysSign,      // Signing
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  256 * 1024 * 1024,
			MaxCPUTime: 120 * time.Second,
		},
	}
}

// Service implements the Confidential service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new Confidential service.
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

// Store exposes the Confidential store.
func (s *Service) Store() *Store {
	return s.store
}

// onAfterStart is called after enclave and store are initialized.
// This handles service-specific initialization like metrics, config hydration, etc.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Register metrics using BaseService helper
	s.RegisterMetrics("confidential", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("confidential_compute_total", "Total number of confidential computations")
		metrics.RegisterCounter("confidential_compute_errors", "Total number of computation errors")
		metrics.RegisterGauge("confidential_active_requests", "Number of active compute requests")
		metrics.RegisterHistogram("confidential_compute_duration_seconds", "Computation duration", []float64{0.1, 0.5, 1, 5, 10, 30, 60})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg Config
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "confidential/config", &cfg); err != nil {
		s.Logger().Warn("confidential storage hydrate failed", "err", err)
	} else if loaded {
		// Apply allowed hosts to manifest-level allowlist if provided
		if len(cfg.AllowedHosts) > 0 {
			s.Logger().Info("applying confidential allowed hosts from sealed config", "hosts", cfg.AllowedHosts)
			// best-effort: not mutating manifest here, but enclave can enforce/forward to SecureNetwork if needed
		}
	}

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// Execute executes confidential computation.
func (s *Service) Execute(ctx context.Context, req *ComputeRequest) (*ComputeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	return s.enclave.Execute(ctx, req)
}
