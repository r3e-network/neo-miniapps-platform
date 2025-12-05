// Package dta provides Data Trust Authority service.
package dta

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "dta"
	ServiceName = "DTA Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Data Trust Authority service for data certification",
		RequiredCapabilities: []os.Capability{
			os.CapKeys,
			os.CapKeysSign,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapAttestation,
			os.CapSecrets,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapCache,         // Certificate caching
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  64 * 1024 * 1024,
			MaxCPUTime: 30 * time.Second,
		},
	}
}

// Service implements the DTA service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new DTA service.
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
	s.RegisterMetrics("dta", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("dta_certificates_issued", "Total number of certificates issued")
		metrics.RegisterCounter("dta_certificates_verified", "Total number of certificates verified")
		metrics.RegisterCounter("dta_verification_failed", "Total number of failed verifications")
		metrics.RegisterGauge("dta_active_certificates", "Number of active certificates")
		metrics.RegisterHistogram("dta_signing_duration_seconds", "Certificate signing duration", []float64{0.01, 0.05, 0.1, 0.5, 1})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "dta/config", &cfg); err != nil {
		s.Logger().Warn("dta storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("dta config loaded from sealed storage")
	}

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// IssueCertificate issues a new data certificate.
func (s *Service) IssueCertificate(ctx context.Context, dataHash []byte, metadata map[string]string) (*Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}
	return s.enclave.IssueCertificate(ctx, dataHash, metadata)
}

// VerifyCertificate verifies a certificate.
func (s *Service) VerifyCertificate(ctx context.Context, cert *Certificate) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.State() != base.StateRunning {
		return false, fmt.Errorf("service not running")
	}
	return s.enclave.VerifyCertificate(ctx, cert)
}

// GetCertificate gets a certificate by ID.
func (s *Service) GetCertificate(ctx context.Context, id string) (*Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetCertificate(ctx, id)
}

