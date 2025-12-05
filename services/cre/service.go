// Package cre provides Chainlink Runtime Environment service.
package cre

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "cre"
	ServiceName = "CRE Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Chainlink Runtime Environment service",
		RequiredCapabilities: []os.Capability{
			os.CapCompute,
			os.CapNetwork,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapKeys,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapScheduler,     // Workflow scheduling
			os.CapQueue,         // Task queue
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  256 * 1024 * 1024,
			MaxCPUTime: 120 * time.Second,
		},
	}
}

// Service implements the CRE service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new CRE service.
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
	s.RegisterMetrics("cre", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("cre_workflows_executed", "Total number of workflows executed")
		metrics.RegisterCounter("cre_workflows_failed", "Total number of failed workflows")
		metrics.RegisterGauge("cre_active_workflows", "Number of active workflows")
		metrics.RegisterHistogram("cre_workflow_duration_seconds", "Workflow execution duration", []float64{0.5, 1, 5, 10, 30, 60, 120})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "cre/config", &cfg); err != nil {
		s.Logger().Warn("cre storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("cre config loaded from sealed storage")
	}

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// ExecuteWorkflow executes a CRE workflow.
func (s *Service) ExecuteWorkflow(ctx context.Context, workflow *Workflow) (*WorkflowResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	return s.enclave.ExecuteWorkflow(ctx, workflow)
}

