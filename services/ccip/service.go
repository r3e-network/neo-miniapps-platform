// Package ccip provides Cross-Chain Interoperability Protocol service.
package ccip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "ccip"
	ServiceName = "CCIP Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Cross-Chain Interoperability Protocol service",
		RequiredCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapKeys,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapKeysSign,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapChain,         // Multi-chain operations
			os.CapQueue,         // Message queue
			os.CapCache,         // Message caching
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      128 * 1024 * 1024,
			MaxCPUTime:     60 * time.Second,
			MaxNetworkReqs: 200,
		},
	}
}

// Service implements the CCIP service.
type Service struct {
	*base.BaseService
	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new CCIP service.
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
	s.RegisterMetrics("ccip", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("ccip_messages_sent", "Total number of cross-chain messages sent")
		metrics.RegisterCounter("ccip_messages_received", "Total number of cross-chain messages received")
		metrics.RegisterCounter("ccip_messages_failed", "Total number of failed messages")
		metrics.RegisterGauge("ccip_pending_messages", "Number of pending messages")
		metrics.RegisterHistogram("ccip_message_latency_seconds", "Message delivery latency", []float64{1, 5, 10, 30, 60, 300})
	})

	// Hydrate configs from sealed storage (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "ccip/config", &cfg); err != nil {
		s.Logger().Warn("ccip storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("ccip config loaded from sealed storage")
	}

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// SendMessage sends a cross-chain message.
func (s *Service) SendMessage(ctx context.Context, msg *CrossChainMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}
	msg.Status = MessageStatusPending
	msg.SetTimestamps()
	return s.store.CreateMessage(ctx, msg)
}

// GetMessage gets a message by ID.
func (s *Service) GetMessage(ctx context.Context, id string) (*CrossChainMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetMessage(ctx, id)
}

